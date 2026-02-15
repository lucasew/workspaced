package github

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/driver"
	"workspaced/pkg/driver/fetchurl"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/provider"
)

func init() {
	tool.RegisterProvider(&Provider{})
}

type Provider struct{}

func (p *Provider) ID() string   { return "github" }
func (p *Provider) Name() string { return "GitHub Releases" }

func (p *Provider) ParsePackage(spec string) (provider.PackageConfig, error) {
	parts := strings.Split(spec, "/")
	if len(parts) != 2 {
		return provider.PackageConfig{}, fmt.Errorf("invalid GitHub spec: %s (expected owner/repo)", spec)
	}

	return provider.PackageConfig{
		Provider: "github",
		Spec:     spec,
		Repo:     spec,
	}, nil
}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (p *Provider) ListVersions(ctx context.Context, pkg provider.PackageConfig) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", pkg.Repo)
	slog.Debug("fetching versions", "url", url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Use httpclient driver (handles Termux DNS/certs)
	httpClient, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get http client: %w", err)
	}

	resp, err := httpClient.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api error: %s", resp.Status)
	}

	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	var versions []string
	for _, r := range releases {
		versions = append(versions, r.TagName)
	}
	slog.Debug("found versions", "count", len(versions))
	return versions, nil
}

func (p *Provider) GetArtifacts(ctx context.Context, pkg provider.PackageConfig, version string) ([]provider.Artifact, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", pkg.Repo, version)
	if version == "latest" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", pkg.Repo)
	}
	slog.Debug("fetching release info", "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Use httpclient driver (handles Termux DNS/certs)
	httpClient, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get http client: %w", err)
	}

	resp, err := httpClient.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api error: %s", resp.Status)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var artifacts []provider.Artifact
	for _, a := range r.Assets {
		osName, arch, ok := parseAssetName(a.Name)
		if !ok {
			continue
		}

		// Extract hash from digest (format: "sha256:HASH")
		hash := ""
		if a.Digest != "" {
			parts := strings.SplitN(a.Digest, ":", 2)
			if len(parts) == 2 {
				// Store as "algo:hash" format for fetchurl compatibility
				hash = a.Digest
				slog.Debug("found checksum", "asset", a.Name, "algo", parts[0], "hash", parts[1][:16]+"...")
			}
		}

		artifacts = append(artifacts, provider.Artifact{
			OS:   osName,
			Arch: arch,
			URL:  a.BrowserDownloadURL,
			Hash: hash,
		})
	}
	slog.Debug("found assets", "total_assets", len(r.Assets), "matched_artifacts", len(artifacts))

	return artifacts, nil
}

func (p *Provider) Install(ctx context.Context, artifact provider.Artifact, destPath string) error {
	tmpDir := destPath + ".tmp"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	filename := filepath.Base(artifact.URL)
	downloadPath := filepath.Join(tmpDir, filename)
	outFile, err := os.Create(downloadPath)
	if err != nil {
		return err
	}

	// Try to use fetchurl if hash is present
	downloaded := false
	if artifact.Hash != "" {
		if fetcher, err := driver.Get[fetchurl.Driver](ctx); err == nil {
			slog.Debug("downloading with fetchurl", "url", artifact.URL, "hash", artifact.Hash)
			algo, hash := "", ""
			if parts := strings.SplitN(artifact.Hash, ":", 2); len(parts) == 2 {
				algo, hash = parts[0], parts[1]
			} else {
				algo = "sha256"
				hash = artifact.Hash
			}
			opts := fetchurl.FetchOptions{
				URLs: []string{artifact.URL},
				Algo: algo,
				Hash: hash,
				Out:  outFile,
			}
			if err := fetcher.Fetch(ctx, opts); err == nil {
				downloaded = true
			} else {
				slog.Debug("fetchurl failed, falling back", "error", err)
			}
		}
	}

	if !downloaded {
		slog.Debug("downloading directly", "url", artifact.URL)
		// Fallback to direct download
		// Reset file position
		outFile.Seek(0, 0)
		outFile.Truncate(0)

		req, err := http.NewRequestWithContext(ctx, "GET", artifact.URL, nil)
		if err != nil {
			outFile.Close()
			return err
		}
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		// Use httpclient driver (handles Termux DNS/certs)
		httpClient, err := driver.Get[httpclient.Driver](ctx)
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to get http client: %w", err)
		}

		resp, err := httpClient.Client().Do(req)
		if err != nil {
			outFile.Close()
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			outFile.Close()
			return fmt.Errorf("download failed: %s", resp.Status)
		}

		if _, err := io.Copy(outFile, resp.Body); err != nil {
			outFile.Close()
			return err
		}
	}

	outFile.Close()

	// Extract
	slog.Debug("extracting", "file", downloadPath, "dest", destPath)
	if err := extract(downloadPath, destPath); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Strip single top-level directory if present
	if err := stripTopLevelDir(destPath); err != nil {
		slog.Warn("failed to strip top-level directory", "error", err)
	}

	return nil
}

func parseAssetName(name string) (osName, arch string, ok bool) {
	name = strings.ToLower(name)

	// OS Detection
	if strings.Contains(name, "linux") {
		osName = "linux"
	} else if strings.Contains(name, "darwin") || strings.Contains(name, "macos") || strings.Contains(name, "apple") {
		osName = "darwin"
	} else if strings.Contains(name, "windows") {
		osName = "windows"
	} else {
		return "", "", false
	}

	// Arch Detection
	if strings.Contains(name, "amd64") || strings.Contains(name, "x86_64") || strings.Contains(name, "x64") {
		arch = "amd64"
	} else if strings.Contains(name, "arm64") || strings.Contains(name, "aarch64") {
		arch = "arm64"
	} else if strings.Contains(name, "386") || strings.Contains(name, "x86") {
		arch = "386"
	} else {
		return "", "", false
	}

	return osName, arch, true
}

func extract(src, dest string) error {
	if strings.HasSuffix(src, ".zip") {
		return unzip(src, dest)
	} else if strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		return untargz(src, dest)
	}

	// Assume binary
	binName := filepath.Base(src)
	slog.Debug("assuming binary file", "name", binName)

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	outPath := filepath.Join(dest, binName)
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return os.Chmod(outPath, 0755)
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}

		// Restore permissions (especially execute bit)
		if f.Mode()&0111 != 0 {
			os.Chmod(fpath, f.Mode())
		}
	}
	return nil
}

// stripTopLevelDir checks if destPath contains only one directory,
// and if so, moves its contents up one level.
// This handles archives like "tool-v1.0.0/bin/tool" -> "bin/tool"
func stripTopLevelDir(destPath string) error {
	entries, err := os.ReadDir(destPath)
	if err != nil {
		return err
	}

	// Only strip if there's exactly one entry and it's a directory
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}

	singleDir := filepath.Join(destPath, entries[0].Name())
	slog.Debug("stripping top-level directory", "dir", entries[0].Name())

	// Move contents to temp location
	tempDir := destPath + ".strip-tmp"
	if err := os.Rename(singleDir, tempDir); err != nil {
		return err
	}

	// Move contents of temp dir to destPath
	tempEntries, err := os.ReadDir(tempDir)
	if err != nil {
		os.Rename(tempDir, singleDir) // Try to restore
		return err
	}

	for _, entry := range tempEntries {
		oldPath := filepath.Join(tempDir, entry.Name())
		newPath := filepath.Join(destPath, entry.Name())
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	}

	// Remove now-empty temp dir
	return os.Remove(tempDir)
}

func untargz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", target)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()

			// Explicitly restore permissions (including execute bit)
			if header.Mode&0111 != 0 {
				os.Chmod(target, os.FileMode(header.Mode))
			}
		}
	}
	return nil
}
