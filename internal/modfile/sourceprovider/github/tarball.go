package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lucasew/workspaced/internal/archive"
	"github.com/lucasew/workspaced/internal/githubutil"
	"github.com/lucasew/workspaced/pkg/driver"
	httpclientdriver "github.com/lucasew/workspaced/pkg/driver/httpclient"
	"github.com/lucasew/workspaced/pkg/logging"
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
		return "", fmt.Errorf("get http client driver: %w", err)
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
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		target, skip, err := mapTarEntryTarget(hdr.Name, destDir)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if err := extractTarEntry(ctx, tr, hdr, destDir, target); err != nil {
			return err
		}
	}
}

// mapTarEntryTarget strips the GitHub archive top-level prefix (repo-sha/) and
// joins the remainder under destDir. Entries that would escape destDir (zip/tar
// slip) return an error; prefix-only or malformed names are skipped.
func mapTarEntryTarget(name string, destDir string) (target string, skip bool, err error) {
	cleanName := name
	if len(cleanName) >= 2 && cleanName[:2] == "./" {
		cleanName = cleanName[2:]
	}
	parts := splitFirst(cleanName, '/')
	if len(parts) < 2 {
		return "", true, nil
	}
	rel := parts[1]
	if rel == "" {
		return "", true, nil
	}
	target, err = archive.JoinWithin(destDir, rel)
	if err != nil {
		return "", false, fmt.Errorf("illegal file path: %s", name)
	}
	return target, false, nil
}

func splitFirst(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func extractTarEntry(ctx context.Context, tr *tar.Reader, hdr *tar.Header, destDir, target string) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		resolvedTarget, err := archive.ResolveWithin(destDir, target)
		if err != nil {
			return err
		}
		return archive.WriteMember(resolvedTarget, os.FileMode(hdr.Mode), tr)
	case tar.TypeSymlink:
		if !archive.SymlinkTargetWithin(destDir, target, hdr.Linkname) {
			return fmt.Errorf("illegal symlink target: %s -> %s", hdr.Name, hdr.Linkname)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		resolvedTarget, err := archive.ResolveWithin(destDir, target)
		if err != nil {
			return err
		}
		if err := os.Symlink(hdr.Linkname, resolvedTarget); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}
