package apps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"workspaced/pkg/driver"
	fetchurldriver "workspaced/pkg/driver/fetchurl"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/tool/provider"
	"workspaced/pkg/tool/provider/registry"
)

func init() {
	registry.RegisterRegistryTool("nodejs", WrapNewTool(newNodejs, ""))
}

type nodejsTool struct{}

func newNodejs(_ string) (provider.Tool, error) {
	return &nodejsTool{}, nil
}

func (t *nodejsTool) ListVersions(ctx context.Context) ([]string, error) {
	return t.listVersions(ctx)
}

func (t *nodejsTool) Install(ctx context.Context, version string, destDir string) error {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		vers, err := t.listVersions(ctx)
		if err != nil {
			return err
		}
		if len(vers) == 0 {
			return fmt.Errorf("no node versions found")
		}
		v = vers[0]
	}
	arts, err := t.ListArtifacts(ctx, v)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return fmt.Errorf("no artifact for current platform")
	}
	return t.InstallArtifact(ctx, arts[0], destDir)
}

func (t *nodejsTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.Provider = "registry"
	if strings.TrimSpace(entry.CurrentValue) == "" {
		entry.CurrentValue = entry.Version
	}
	// No standard renovate datasource for direct nodejs.org; shasums give us
	// verification at install time via fetchurl.
}

func (t *nodejsTool) ListArtifacts(ctx context.Context, version string) ([]provider.Artifact, error) {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		vers, err := t.listVersions(ctx)
		if err != nil {
			return nil, err
		}
		if len(vers) == 0 {
			return nil, fmt.Errorf("no node versions")
		}
		v = vers[0]
	}

	osPart, archPart, ext := t.nodePlatformAndExt()
	filename := fmt.Sprintf("node-%s-%s-%s%s", v, osPart, archPart, ext)
	url := fmt.Sprintf("https://nodejs.org/dist/%s/%s", v, filename)

	// Fetch SHASUMS256.txt so we can attach hash and use fetchurl backend.
	sums, _ := t.fetchShasums(ctx, v)
	hash := ""
	if h, ok := sums[filename]; ok && h != "" {
		hash = "sha256:" + h
	}

	return []provider.Artifact{{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		URL:  url,
		Hash: hash,
	}}, nil
}

func (t *nodejsTool) InstallArtifact(ctx context.Context, artifact provider.Artifact, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	tmpDir := destDir + ".tmp"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archiveName := filepath.Base(artifact.URL)
	archivePath := filepath.Join(tmpDir, archiveName)

	// Set up file + progress bar (wrapped Out) so fetchurl path gets progress updates
	// (matching how github provider always wraps progress around fetchurl/direct downloads).
	// Size unknown upfront -> spinner; direct fallback will use ContentLength for better bar.
	outFile, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	progress := newDownloadProgressBar(archiveName, 0)
	outWriter := io.Writer(outFile)
	if progress != nil {
		outWriter = io.MultiWriter(outFile, progress)
	}

	usedFetchurl := false
	if artifact.Hash != "" {
		if fetcher, err := driver.Get[fetchurldriver.Driver](ctx); err == nil {
			algo, h := "sha256", artifact.Hash
			if strings.Contains(artifact.Hash, ":") {
				parts := strings.SplitN(artifact.Hash, ":", 2)
				algo, h = parts[0], parts[1]
			}
			opts := fetchurldriver.FetchOptions{
				URLs: []string{artifact.URL},
				Algo: algo,
				Hash: h,
				Out:  outWriter,
			}
			if ferr := fetcher.Fetch(ctx, opts); ferr == nil {
				usedFetchurl = true
				if progress != nil {
					_ = progress.Finish()
				}
			}
		}
	}

	outFile.Close()

	if !usedFetchurl {
		// remove any partial from failed fetchurl attempt
		_ = os.Remove(archivePath)
		// Direct with progress bar (same style as github tools and our grok-build).
		if err := t.downloadRaw(ctx, artifact.URL, archivePath); err != nil {
			return err
		}
	}

	// Extract to a subdir then strip the top-level "node-vX.Y.Z-xxx/" directory.
	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}

	if err := t.extractArchive(archivePath, extractDir); err != nil {
		return fmt.Errorf("extract %s: %w", archiveName, err)
	}

	_ = stripTopLevel(extractDir)

	// Move resulting tree into destDir
	ents, _ := os.ReadDir(extractDir)
	for _, e := range ents {
		src := filepath.Join(extractDir, e.Name())
		dst := filepath.Join(destDir, e.Name())
		_ = os.Rename(src, dst)
	}

	return nil
}

func (t *nodejsTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	if err := t.Install(ctx, version, destDir); err != nil {
		return "", err
	}

	candidates := []string{
		filepath.Join(destDir, "bin", cmdName),
		filepath.Join(destDir, "bin", cmdName+".exe"),
		filepath.Join(destDir, "bin", cmdName+".cmd"),
		filepath.Join(destDir, cmdName),
		filepath.Join(destDir, cmdName+".exe"),
		filepath.Join(destDir, cmdName+".cmd"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	// fallback
	return filepath.Join(destDir, "bin", "node"), nil
}

// --- helpers ---

func (t *nodejsTool) listVersions(ctx context.Context) ([]string, error) {
	u := "https://nodejs.org/dist/index.json"
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := hc.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index.json: %s", resp.Status)
	}
	var infos []struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return nil, err
	}
	out := make([]string, len(infos))
	for i, v := range infos {
		out[i] = v.Version
	}
	return out, nil
}

func (t *nodejsTool) fetchShasums(ctx context.Context, ver string) (map[string]string, error) {
	u := fmt.Sprintf("https://nodejs.org/dist/%s/SHASUMS256.txt", ver)
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := hc.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SHASUMS: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		fs := strings.Fields(line)
		if len(fs) >= 2 {
			m[fs[1]] = fs[0]
		}
	}
	return m, nil
}

func (t *nodejsTool) nodePlatformAndExt() (osPart, archPart, ext string) {
	osPart = runtime.GOOS
	archPart = runtime.GOARCH
	ext = ".tar.gz"

	switch osPart {
	case "darwin":
		osPart = "darwin"
	case "linux":
		osPart = "linux"
	case "windows":
		osPart = "win"
		ext = ".zip"
	}

	switch archPart {
	case "amd64":
		archPart = "x64"
	case "arm64":
		archPart = "arm64"
	case "386":
		archPart = "x86"
	}

	return osPart, archPart, ext
}

func (t *nodejsTool) downloadRaw(ctx context.Context, url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	outFile, err := os.Create(tmp)
	if err != nil {
		return err
	}

	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		outFile.Close()
		_ = os.Remove(tmp)
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := hc.Client().Do(req)
	if err != nil {
		outFile.Close()
		_ = os.Remove(tmp)
		return err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		outFile.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	size := resp.ContentLength
	progress := newDownloadProgressBar(filepath.Base(url), size)
	outWriter := io.Writer(outFile)
	if progress != nil {
		outWriter = io.MultiWriter(outFile, progress)
	}

	if _, err := io.Copy(outWriter, resp.Body); err != nil {
		outFile.Close()
		_ = os.Remove(tmp)
		return err
	}
	if progress != nil {
		_ = progress.Finish()
	}
	outFile.Close()

	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (t *nodejsTool) extractArchive(archive, dest string) error {
	if strings.HasSuffix(archive, ".zip") {
		return unzip(archive, dest)
	}
	return untargz(archive, dest)
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
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
			_ = os.Chmod(target, os.FileMode(hdr.Mode))
		}
	}
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), 0755)
		out, err := os.Create(fpath)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, _ = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if f.Mode().Perm()&0111 != 0 {
			_ = os.Chmod(fpath, 0755)
		}
	}
	return nil
}

func stripTopLevel(dir string) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if e.IsDir() {
			inner := filepath.Join(dir, e.Name())
			children, _ := os.ReadDir(inner)
			for _, ch := range children {
				src := filepath.Join(inner, ch.Name())
				dst := filepath.Join(dir, ch.Name())
				_ = os.Rename(src, dst)
			}
			_ = os.RemoveAll(inner)
			return nil
		}
	}
	return nil
}
