package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"workspaced/pkg/modfile"
	"workspaced/pkg/modfile/sourceprovider/common"
)

type Provider struct{}

func (p Provider) ID() string { return "github" }

func (p Provider) ResolvePath(ctx context.Context, alias string, src modfile.SourceConfig, rel string, modulesBaseDir string) (string, error) {
	_ = modulesBaseDir
	root, err := ensureGithubSource(ctx, alias, src)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, rel), nil
}

func (p Provider) Normalize(src modfile.SourceConfig) modfile.SourceConfig {
	src.Provider = "github"
	src.Path = strings.TrimSpace(src.Path)
	src.Repo = normalizeGitHubRepo(src.Repo)
	src.Ref = strings.TrimSpace(src.Ref)
	src.URL = resolvedTarballURL(src)
	return src
}

func (p Provider) LockHash(ctx context.Context, alias string, src modfile.SourceConfig, modulesBaseDir string) (string, error) {
	_ = alias
	_ = modulesBaseDir
	url := resolvedTarballURL(src)
	if url == "" {
		return "", fmt.Errorf("github source requires repo or url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func init() {
	modfile.RegisterSourceProvider(Provider{})
}

func ensureGithubSource(ctx context.Context, alias string, src modfile.SourceConfig) (string, error) {
	repo := normalizeGitHubRepo(src.Repo)
	if repo == "" {
		return "", fmt.Errorf("source alias %q (github) requires repo", alias)
	}

	tarballURL := resolvedTarballURL(src)
	key := repo + "|" + tarballURL
	slog.Info("resolving github source", "alias", alias, "repo", repo, "url", tarballURL)
	return common.EnsureCachedDir("github", key, func(tmpDir string) error {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return err
		}
		if err := downloadAndExtractTarball(ctx, tarballURL, tmpDir); err != nil {
			return fmt.Errorf("failed to fetch source %q (%s): %w", alias, tarballURL, err)
		}
		return nil
	})
}

func normalizeGitHubRepo(in string) string {
	repo := strings.Trim(strings.TrimSpace(in), "/")
	repo = strings.TrimPrefix(repo, "github:")
	repo = strings.Trim(repo, "/")
	return repo
}

func resolvedTarballURL(src modfile.SourceConfig) string {
	if u := strings.TrimSpace(src.URL); u != "" {
		return u
	}
	repo := normalizeGitHubRepo(src.Repo)
	if repo == "" {
		return ""
	}
	ref := strings.TrimSpace(src.Ref)
	if ref == "" {
		ref = "main"
	}
	return fmt.Sprintf("https://codeload.github.com/%s/tar.gz/%s", repo, ref)
}

func downloadAndExtractTarball(ctx context.Context, url string, destDir string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		name := strings.TrimPrefix(hdr.Name, "./")
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		rel := parts[1]
		if strings.TrimSpace(rel) == "" {
			continue
		}
		target := filepath.Join(destDir, rel)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
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
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil && !os.IsExist(err) {
				return err
			}
		}
	}

	return nil
}
