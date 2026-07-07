package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"workspaced/pkg/driver"
	httpclientdriver "workspaced/pkg/driver/httpclient"
	"workspaced/internal/githubutil"
	"workspaced/pkg/logging"
)

func downloadAndExtractTarball(ctx context.Context, source Source, destDir string, expectedHash string) (sourceMeta, error) {
	url, err := source.ResolvePinnedTarballURL(ctx)
	if err != nil {
		return sourceMeta{}, err
	}
	hash, err := fetchAndExtractTarballURL(ctx, url, destDir, expectedHash)
	if err != nil {
		return sourceMeta{}, err
	}
	return sourceMeta{
		URL:  url,
		Hash: hash,
	}, nil
}

// fetchAndExtractTarballURL downloads a GitHub tarball via httpclient with auth.
// We cannot use the fetchurl driver here: private repos need Authorization on the
// request, and fetchurl has no ConfigureRequest hook. Hash is verified locally
// when expectedHash is set.
func fetchAndExtractTarballURL(ctx context.Context, url string, destDir string, expectedHash string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "workspaced (+https://github.com/lucasew/.dotfiles)")
	githubutil.ApplyAuth(ctx, req)

	httpDriver, err := driver.Get[httpclientdriver.Driver](ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get http client driver: %w", err)
	}
	resp, err := httpDriver.Client().Do(req)
	if err != nil {
		return "", err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		hint := ""
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
			if githubutil.Token(ctx) == "" {
				hint = " (private repos require GITHUB_TOKEN or 'gh auth login')"
			}
		}
		return "", fmt.Errorf("unexpected status: %s%s", resp.Status, hint)
	}

	h := sha256.New()
	if err := extractTarGz(ctx, io.TeeReader(resp.Body, h), destDir); err != nil {
		return "", err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if expectedHash != "" && got != expectedHash {
		return "", fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, got)
	}
	return got, nil
}

func extractTarGz(ctx context.Context, r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer logging.Close(ctx, gzr)

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		target, ok := mapTarEntryTarget(hdr.Name, destDir)
		if !ok {
			continue
		}
		if err := extractTarEntry(ctx, tr, hdr, target); err != nil {
			return err
		}
	}
}

func mapTarEntryTarget(name string, destDir string) (string, bool) {
	cleanName := name
	if len(cleanName) >= 2 && cleanName[:2] == "./" {
		cleanName = cleanName[2:]
	}
	parts := splitFirst(cleanName, '/')
	if len(parts) < 2 {
		return "", false
	}
	rel := parts[1]
	if rel == "" {
		return "", false
	}
	return filepath.Join(destDir, rel), true
}

func splitFirst(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func extractTarEntry(ctx context.Context, tr *tar.Reader, hdr *tar.Header, target string) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0755)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			logging.Close(ctx, f)
			return err
		}
		return f.Close()
	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.Symlink(hdr.Linkname, target); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}
