package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"workspaced/pkg/modfile"
	"workspaced/pkg/modfile/sourceprovider/common"
)

type Provider struct{}

func (p Provider) ID() string { return "github" }

var githubHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

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
	src.URL = strings.TrimSpace(src.URL)
	return src
}

func (p Provider) LockHash(ctx context.Context, alias string, src modfile.SourceConfig, modulesBaseDir string) (string, modfile.SourceConfig, error) {
	root, err := ensureGithubSource(ctx, alias, src)
	if err != nil {
		return "", src, err
	}
	meta, err := readMeta(root)
	if err != nil {
		return "", src, fmt.Errorf("failed to read source metadata: %w", err)
	}
	if strings.TrimSpace(meta.Hash) == "" {
		return "", src, fmt.Errorf("missing cached source hash metadata")
	}
	_ = modulesBaseDir
	src.URL = strings.TrimSpace(meta.URL)
	return strings.TrimSpace(meta.Hash), src, nil
}

func init() {
	modfile.RegisterSourceProvider(Provider{})
}

func ensureGithubSource(ctx context.Context, alias string, src modfile.SourceConfig) (string, error) {
	repo := normalizeGitHubRepo(src.Repo)
	if repo == "" {
		return "", fmt.Errorf("source alias %q (github) requires repo", alias)
	}

	key := sourceCacheKey(src)
	slog.Info("resolving github source", "alias", alias, "repo", repo, "ref", strings.TrimSpace(src.Ref), "url", strings.TrimSpace(src.URL))
	return common.EnsureCachedDir("github", key, func(tmpDir string) error {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return err
		}
		meta, err := downloadAndExtractTarball(ctx, src, tmpDir)
		if err != nil {
			return fmt.Errorf("failed to fetch source %q: %w", alias, err)
		}
		if err := writeMeta(tmpDir, meta); err != nil {
			return fmt.Errorf("failed to write source metadata: %w", err)
		}
		slog.Info("fetched github source", "alias", alias, "url", meta.URL, "sha256", meta.Hash)
		return nil
	})
}

func sourceCacheKey(src modfile.SourceConfig) string {
	if u := strings.TrimSpace(src.URL); u != "" {
		return "v3:url:" + u
	}
	repo := normalizeGitHubRepo(src.Repo)
	ref := strings.TrimSpace(src.Ref)
	if ref == "" {
		ref = "HEAD"
	}
	return "v3:repo:" + repo + "@" + ref
}

func normalizeGitHubRepo(in string) string {
	repo := strings.Trim(strings.TrimSpace(in), "/")
	repo = strings.TrimPrefix(repo, "github:")
	repo = strings.Trim(repo, "/")
	return repo
}

func tarballURLCandidates(src modfile.SourceConfig) []string {
	if u := strings.TrimSpace(src.URL); u != "" {
		return []string{u}
	}
	repo := normalizeGitHubRepo(src.Repo)
	if repo == "" {
		return nil
	}
	ref := strings.TrimSpace(src.Ref)
	if ref != "" {
		out := []string{
			fmt.Sprintf("https://codeload.github.com/%s/tar.gz/%s", repo, ref),
		}
		if !strings.HasPrefix(ref, "refs/") {
			out = append(out,
				fmt.Sprintf("https://codeload.github.com/%s/tar.gz/refs/heads/%s", repo, ref),
				fmt.Sprintf("https://codeload.github.com/%s/tar.gz/refs/tags/%s", repo, ref),
			)
		}
		return dedupeStrings(out)
	}
	return []string{
		fmt.Sprintf("https://codeload.github.com/%s/tar.gz/refs/heads/main", repo),
		fmt.Sprintf("https://codeload.github.com/%s/tar.gz/refs/heads/master", repo),
	}
}

func dedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func downloadAndExtractTarball(ctx context.Context, src modfile.SourceConfig, destDir string) (sourceMeta, error) {
	candidates := tarballURLCandidates(src)
	if pinned, err := resolvePinnedTarballURL(ctx, src); err == nil && strings.TrimSpace(pinned) != "" {
		candidates = []string{pinned}
	} else if err != nil {
		slog.Warn("failed to resolve github ref to commit, using fallback candidates", "error", err)
	}
	if len(candidates) == 0 {
		return sourceMeta{}, fmt.Errorf("github source requires repo or url")
	}
	var lastStatus string
	for _, url := range candidates {
		ok, status, hash, err := tryDownloadAndExtractTarballURL(ctx, url, destDir)
		if err != nil {
			return sourceMeta{}, err
		}
		if ok {
			return sourceMeta{
				URL:  url,
				Hash: hash,
			}, nil
		}
		lastStatus = status
	}
	if lastStatus == "" {
		lastStatus = "no candidates"
	}
	return sourceMeta{}, fmt.Errorf("unexpected status: %s", lastStatus)
}

func tryDownloadAndExtractTarballURL(ctx context.Context, url string, destDir string) (bool, string, string, error) {
	slog.Info("trying github source candidate", "url", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, "", "", err
	}
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return false, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, resp.Status, "", nil
	}

	h := sha256.New()
	gzr, err := gzip.NewReader(io.TeeReader(resp.Body, h))
	if err != nil {
		return false, "", "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, "", "", err
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
				return false, "", "", err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return false, "", "", err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return false, "", "", err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return false, "", "", err
			}
			if err := f.Close(); err != nil {
				return false, "", "", err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return false, "", "", err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil && !os.IsExist(err) {
				return false, "", "", err
			}
		}
	}

	return true, resp.Status, hex.EncodeToString(h.Sum(nil)), nil
}

type sourceMeta struct {
	URL  string `json:"url"`
	Hash string `json:"hash"`
}

func writeMeta(dir string, meta sourceMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".workspaced-source-meta.json"), b, 0644)
}

func readMeta(dir string) (sourceMeta, error) {
	b, err := os.ReadFile(filepath.Join(dir, ".workspaced-source-meta.json"))
	if err != nil {
		return sourceMeta{}, err
	}
	var meta sourceMeta
	if err := json.Unmarshal(b, &meta); err != nil {
		return sourceMeta{}, err
	}
	return meta, nil
}

var shaRefRe = regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`)

func resolvePinnedTarballURL(ctx context.Context, src modfile.SourceConfig) (string, error) {
	if u := strings.TrimSpace(src.URL); u != "" {
		return u, nil
	}
	repo := normalizeGitHubRepo(src.Repo)
	if repo == "" {
		return "", fmt.Errorf("github source requires repo")
	}
	ref := strings.TrimSpace(src.Ref)
	if ref == "" {
		defaultBranch, err := resolveDefaultBranch(ctx, repo)
		if err != nil {
			return "", err
		}
		ref = defaultBranch
	}
	if shaRefRe.MatchString(ref) {
		return fmt.Sprintf("https://codeload.github.com/%s/tar.gz/%s", repo, ref), nil
	}
	sha, err := resolveCommitSHA(ctx, repo, ref)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://codeload.github.com/%s/tar.gz/%s", repo, sha), nil
}

func resolveDefaultBranch(ctx context.Context, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "workspaced")
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from repo metadata: %s", resp.Status)
	}
	var payload struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.DefaultBranch) == "" {
		return "", fmt.Errorf("missing default_branch in github response")
	}
	return strings.TrimSpace(payload.DefaultBranch), nil
}

func resolveCommitSHA(ctx context.Context, repo string, ref string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "workspaced")
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from commit lookup: %s", resp.Status)
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.SHA) == "" {
		return "", fmt.Errorf("missing sha in github response")
	}
	return strings.TrimSpace(payload.SHA), nil
}
