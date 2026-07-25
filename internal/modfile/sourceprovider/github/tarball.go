package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/lucasew/workspaced/internal/githubutil"
	"github.com/lucasew/workspaced/pkg/driver"
	httpclientdriver "github.com/lucasew/workspaced/pkg/driver/httpclient"
	"github.com/lucasew/workspaced/pkg/logging"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	// filepath.Join discards prior segments after an absolute element on some OSes.
	if filepath.IsAbs(rel) {
		return "", false, fmt.Errorf("illegal file path: %s", name)
	}
	target = filepath.Join(destDir, rel)
	if !isPathWithinDest(destDir, target) {
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

// isPathWithinDest reports whether target is dest or a path under dest after Clean.
// Same rule as internal/tool/backend/install (kept local so this package stays free of that import).
func isPathWithinDest(dest, target string) bool {
	cleanDest := filepath.Clean(dest)
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(cleanDest, cleanTarget)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// resolveTargetPath ensures target's parent directory resolves within destDir.
// It evaluates symlinks in the parent path to prevent writes escaping destDir.
func resolveTargetPath(destDir, target string) (string, error) {
	parent := filepath.Dir(target)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !isPathWithinDest(destDir, realParent) {
		return "", fmt.Errorf("illegal file path: %s", target)
	}
	return filepath.Join(realParent, filepath.Base(target)), nil
}

// symlinkTargetWithinDest ensures a symlink's linkname resolves under destDir
// when evaluated from the link's parent directory, including existing symlinks.
func symlinkTargetWithinDest(destDir, linkPath, linkname string) bool {
	if linkname == "" {
		return false
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(linkPath))
	if err != nil {
		return false
	}
	if !isPathWithinDest(destDir, realParent) {
		return false
	}
	var resolved string
	if filepath.IsAbs(linkname) {
		resolved = filepath.Clean(linkname)
	} else {
		resolved = filepath.Join(realParent, linkname)
	}
	return isPathWithinDest(destDir, resolved)
}

func extractTarEntry(ctx context.Context, tr *tar.Reader, hdr *tar.Header, destDir, target string) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0755)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		resolvedTarget, err := resolveTargetPath(destDir, target)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(resolvedTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			logging.Close(ctx, f)
			_ = os.Remove(resolvedTarget)
			return err
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(resolvedTarget)
			return err
		}
		return nil
	case tar.TypeSymlink:
		if !symlinkTargetWithinDest(destDir, target, hdr.Linkname) {
			return fmt.Errorf("illegal symlink target: %s -> %s", hdr.Name, hdr.Linkname)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		resolvedTarget, err := resolveTargetPath(destDir, target)
		if err != nil {
			return err
		}
		if err := os.Symlink(hdr.Linkname, resolvedTarget); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}
