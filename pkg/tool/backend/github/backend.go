package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/githubutil"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/backend"
	providerinstall "workspaced/pkg/tool/backend/install"
)

var (
	ErrEmptyGitHubRef   = errors.New("github ref cannot be empty (expected owner/repo)")
	ErrInvalidGitHubRef = errors.New("invalid github ref (expected owner/repo)")
	ErrAPIError         = errors.New("github api error")
	ErrNoArtifact       = errors.New("no suitable artifact")
)

func init() {
	tool.Register("github", &Backend{})
}

type Backend struct{}

func (p *Backend) Name() string { return "GitHub Releases" }

// Tool returns a first-class Tool for the given ref (owner/repo).
func (p *Backend) Tool(ref string) (backend.Tool, error) {
	return NewTool(ref)
}

// ParsePackage is kept for transitional use by code that still talks to the
// old detailed surface on the concrete backend.
func (p *Backend) ParsePackage(spec string) (backend.PackageConfig, error) {
	parts := strings.Split(spec, "/")
	if len(parts) != 2 {
		return backend.PackageConfig{}, fmt.Errorf("invalid GitHub spec: %s: %w", spec, ErrInvalidGitHubRef)
	}

	return backend.PackageConfig{
		Backend: "github",
		Spec:    spec,
		Repo:    spec,
	}, nil
}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
	APIURL             string `json:"url"` // api.github.com url for the asset
}

func (p *Backend) ListVersions(ctx context.Context, pkg backend.PackageConfig) ([]string, error) {
	logger := logging.GetLogger(ctx)
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", pkg.Repo)
	logger.Debug("fetching versions", "url", url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "workspaced (+https://github.com/lucasew/.dotfiles)")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	githubutil.ApplyAuth(ctx, req)

	// Use httpclient driver
	httpClient, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get http client: %w", err)
	}

	resp, err := httpClient.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusForbidden && strings.Contains(msg, "rate limit") {
			return nil, fmt.Errorf("github api rate limit exceeded for %s (consider setting GITHUB_TOKEN or logging in with 'gh auth login'): %s: %w", url, resp.Status, ErrAPIError)
		}
		if msg != "" {
			return nil, fmt.Errorf("github api error for %s: %s: %s: %w", url, resp.Status, msg, ErrAPIError)
		}
		return nil, fmt.Errorf("github api error for %s: %s: %w", url, resp.Status, ErrAPIError)
	}

	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	var versions []string
	for _, r := range releases {
		versions = append(versions, r.TagName)
	}
	logger.Debug("found versions", "count", len(versions))
	return versions, nil
}

func (p *Backend) GetArtifacts(ctx context.Context, pkg backend.PackageConfig, version string) ([]backend.Artifact, error) {
	logger := logging.GetLogger(ctx)
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", pkg.Repo, version)
	if version == "latest" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", pkg.Repo)
	}
	logger.Debug("fetching release info", "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "workspaced (+https://github.com/lucasew/.dotfiles)")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	githubutil.ApplyAuth(ctx, req)

	// Use httpclient driver
	httpClient, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get http client: %w", err)
	}

	resp, err := httpClient.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusForbidden && strings.Contains(msg, "rate limit") {
			return nil, fmt.Errorf("github api rate limit exceeded for %s (consider setting GITHUB_TOKEN or logging in with 'gh auth login'): %s: %w", url, resp.Status, ErrAPIError)
		}
		if msg != "" {
			return nil, fmt.Errorf("github api error for %s: %s: %s: %w", url, resp.Status, msg, ErrAPIError)
		}
		return nil, fmt.Errorf("github api error for %s: %s: %w", url, resp.Status, ErrAPIError)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var artifacts []backend.Artifact
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
				logger.Debug("found checksum", "asset", a.Name, "algo", parts[0], "hash", parts[1][:16]+"...")
			}
		}

		artifacts = append(artifacts, backend.Artifact{
			OS:                osName,
			Arch:              arch,
			URL:               a.BrowserDownloadURL,
			Hash:              hash,
			Size:              a.Size,
			GitHubAssetID:     a.ID,
			GitHubAssetAPIURL: a.APIURL,
		})
	}
	logger.Debug("found assets", "total_assets", len(r.Assets), "matched_artifacts", len(artifacts))

	return artifacts, nil
}

func (p *Backend) Install(ctx context.Context, artifact backend.Artifact, destPath string) error {
	// Always pass the browser_download_url as the Artifact.URL to downstream
	// install logic. This ensures filepath.Base(artifact.URL) yields a filename
	// with the proper extension (e.g. .tar.gz, .zip) so InstallArtifact can
	// pick the right Extract path and the temp download file is named sensibly.
	// When a GitHub token is available we rewrite the *request* (not the
	// artifact metadata) to the authenticated API asset endpoint.
	browserURL := artifact.URL
	configure := func(req *http.Request) {
		githubutil.ApplyAuth(ctx, req)
		req.Header.Set("User-Agent", "workspaced (+https://github.com/lucasew/.dotfiles)")
	}

	// For GitHub release assets, when we have a token, rewrite the outgoing
	// request inside ConfigureRequest to the API endpoint + Accept header.
	// Sending Authorization directly on browser_download_url can 403 for some
	// token types/redirects; the asset API with octet-stream is the documented
	// way.
	if artifact.GitHubAssetID != 0 && artifact.GitHubAssetAPIURL != "" {
		if token := githubutil.Token(ctx); token != "" {
			if apiURL, err := url.Parse(artifact.GitHubAssetAPIURL); err == nil {
				configure = func(req *http.Request) {
					githubutil.ApplyAuth(ctx, req)
					req.URL = apiURL
					req.Host = apiURL.Host
					req.Header.Set("Accept", "application/octet-stream")
					req.Header.Set("User-Agent", "workspaced (+https://github.com/lucasew/.dotfiles)")
					req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
				}
			}
		}
	}

	return providerinstall.InstallArtifact(ctx, backend.Artifact{
		OS:   artifact.OS,
		Arch: artifact.Arch,
		URL:  browserURL,
		Hash: artifact.Hash,
		Size: artifact.Size,
	}, destPath, providerinstall.DownloadOptions{
		ConfigureRequest: configure,
	})
}

func parseAssetName(name string) (osName, arch string, ok bool) {
	name = strings.ToLower(name)

	// OS Detection
	if backend.ContainsAnyOf(name, "android") {
		osName = "android"
	} else if backend.ContainsAnyOf(name, "linux", "ubuntu") {
		osName = "linux"
	} else if backend.ContainsAnyOf(name, "darwin", "macos", "apple") {
		osName = "darwin"
	} else if backend.ContainsAnyOf(name, "windows") {
		osName = "windows"
	} else {
		return "", "", false
	}

	// Arch Detection
	if backend.ContainsAnyOf(name, "amd64", "x86_64", "x64") {
		arch = "amd64"
	} else if backend.ContainsAnyOf(name, "arm64", "aarch64") {
		arch = "arm64"
	} else if backend.ContainsAnyOf(name, "386", "x86") {
		arch = "386"
	} else if backend.ContainsAnyOf(name, "riscv") {
		arch = "riscv"
	} else {
		return "", "", false
	}

	return osName, arch, true
}

// ============================================================================
// GitHubTool - exported Tool implementation for the github backend
// ============================================================================

// GitHubTool is the concrete Tool for packages distributed via GitHub Releases.
// It is exported (along with NewTool) so that a future central "registry" backend
// can construct, wrap, or delegate to GitHub-based tools.
type GitHubTool struct {
	repo string
	p    *Backend // delegate to existing backend logic during migration
}

// NewTool constructs a GitHubTool for the given ref ("owner/repo").
// This is the preferred way for external code (including a future registry backend)
// to obtain a github-backed Tool without going through the handler registration.
func NewTool(ref string) (backend.Tool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, ErrEmptyGitHubRef
	}
	// Basic validation to match old ParsePackage behavior
	parts := strings.Split(ref, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidGitHubRef, ref)
	}
	return &GitHubTool{repo: ref, p: &Backend{}}, nil
}

func (t *GitHubTool) ListVersions(ctx context.Context) ([]string, error) {
	pkg := backend.PackageConfig{
		Spec: t.repo,
		Repo: t.repo,
	}
	return t.p.ListVersions(ctx, pkg)
}

func (t *GitHubTool) Install(ctx context.Context, version string, destDir string) error {
	pkg := backend.PackageConfig{
		Spec: t.repo,
		Repo: t.repo,
	}
	artifacts, err := t.p.GetArtifacts(ctx, pkg, version)
	if err != nil {
		return err
	}

	artifact := backend.SelectArtifact(artifacts, runtime.GOOS, runtime.GOARCH, "")
	if artifact == nil {
		return fmt.Errorf("no suitable artifact found for %s/%s for github:%s@%s: %w", runtime.GOOS, runtime.GOARCH, t.repo, version, ErrNoArtifact)
	}

	return t.p.Install(ctx, *artifact, destDir)
}

// ArtifactTool implementation (for selfupdate and other custom artifact needs)
func (t *GitHubTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	pkg := backend.PackageConfig{
		Spec: t.repo,
		Repo: t.repo,
	}
	return t.p.GetArtifacts(ctx, pkg, version)
}

func (t *GitHubTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return t.p.Install(ctx, artifact, destDir)
}

// EnrichLockfile sets Renovate metadata for a GitHub Releases tool entry.
// CurrentValue is left to the caller (resolved version at lock time).
func (t *GitHubTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.DepName = t.repo
	entry.Datasource = "github-releases"
}
