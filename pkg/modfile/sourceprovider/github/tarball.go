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

	libfetchurl "github.com/lucasew/fetchurl"
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

func fetchAndExtractTarballURL(ctx context.Context, url string, destDir string, expectedHash string) (string, error) {
	archivePath := filepath.Join(destDir, ".source.tar.gz")

	if expectedHash != "" {
		fetcher := libfetchurl.NewFetcher(githubHTTPClient)
		out, err := os.Create(archivePath)
		if err != nil {
			return "", err
		}
		err = fetcher.Fetch(ctx, libfetchurl.FetchOptions{
			URLs: []string{url},
			Algo: "sha256",
			Hash: expectedHash,
			Out:  out,
		})
		closeErr := out.Close()
		if err != nil {
			_ = os.Remove(archivePath)
			return "", err
		}
		if closeErr != nil {
			_ = os.Remove(archivePath)
			return "", closeErr
		}
		if err := extractTarGzFromPath(archivePath, destDir); err != nil {
			_ = os.Remove(archivePath)
			return "", err
		}
		_ = os.Remove(archivePath)
		return expectedHash, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	h := sha256.New()
	if err := extractTarGz(io.TeeReader(resp.Body, h), destDir); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractTarGzFromPath(path string, destDir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTarGz(f, destDir)
}

func extractTarGz(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
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

		target, ok := mapTarEntryTarget(hdr.Name, destDir)
		if !ok {
			continue
		}
		if err := extractTarEntry(tr, hdr, target); err != nil {
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

func extractTarEntry(tr *tar.Reader, hdr *tar.Header, target string) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0755)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
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
