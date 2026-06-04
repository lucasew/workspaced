package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/githubutil"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/tool"
	"workspaced/pkg/tool/provider"
	providerinstall "workspaced/pkg/tool/provider/install"
)

func init() {
	tool.Register("github", &Provider{})
}

type Provider struct{}

func (p *Provider) Name() string { return "GitHub Releases" }

// Tool returns a first-class Tool for the given ref (owner/repo).
func (p *Provider) Tool(ref string) (provider.Tool, error) {
	return NewTool(ref)
}

// ParsePackage is kept for transitional use by code that still talks to the
// old detailed surface on the concrete provider.
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
	Size               int64  `json:"size"`
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

	githubutil.ApplyAuth(ctx, req)

	// Use httpclient driver (handles Termux DNS/certs)
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

	githubutil.ApplyAuth(ctx, req)

	// Use httpclient driver (handles Termux DNS/certs)
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
			Size: a.Size,
		})
	}
	slog.Debug("found assets", "total_assets", len(r.Assets), "matched_artifacts", len(artifacts))

	return artifacts, nil
}

func (p *Provider) Install(ctx context.Context, artifact provider.Artifact, destPath string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destPath, providerinstall.DownloadOptions{
		ConfigureRequest: func(req *http.Request) {
			githubutil.ApplyAuth(ctx, req)
		},
	})
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

// ============================================================================
// GitHubTool - exported Tool implementation for the github provider
// ============================================================================

// GitHubTool is the concrete Tool for packages distributed via GitHub Releases.
// It is exported (along with NewTool) so that a future central "registry" provider
// can construct, wrap, or delegate to GitHub-based tools.
type GitHubTool struct {
	repo string
	p    *Provider // delegate to existing provider logic during migration
}

// NewTool constructs a GitHubTool for the given ref ("owner/repo").
// This is the preferred way for external code (including a future registry provider)
// to obtain a github-backed Tool without going through the handler registration.
func NewTool(ref string) (provider.Tool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("github ref cannot be empty (expected owner/repo)")
	}
	// Basic validation to match old ParsePackage behavior
	parts := strings.Split(ref, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid github ref %q (expected owner/repo)", ref)
	}
	return &GitHubTool{repo: ref, p: &Provider{}}, nil
}

func (t *GitHubTool) ListVersions(ctx context.Context) ([]string, error) {
	pkg := provider.PackageConfig{
		Spec: t.repo,
		Repo: t.repo,
	}
	return t.p.ListVersions(ctx, pkg)
}

func (t *GitHubTool) Install(ctx context.Context, version string, destDir string) error {
	pkg := provider.PackageConfig{
		Spec: t.repo,
		Repo: t.repo,
	}
	artifacts, err := t.p.GetArtifacts(ctx, pkg, version)
	if err != nil {
		return err
	}

	artifact := provider.SelectArtifact(artifacts, runtime.GOOS, runtime.GOARCH, "")
	if artifact == nil {
		return fmt.Errorf("no suitable artifact found for %s/%s for github:%s@%s", runtime.GOOS, runtime.GOARCH, t.repo, version)
	}

	return t.p.Install(ctx, *artifact, destDir)
}

// ArtifactTool implementation (for selfupdate and other custom artifact needs)
func (t *GitHubTool) ListArtifacts(ctx context.Context, version string) ([]provider.Artifact, error) {
	pkg := provider.PackageConfig{
		Spec: t.repo,
		Repo: t.repo,
	}
	return t.p.GetArtifacts(ctx, pkg, version)
}

func (t *GitHubTool) InstallArtifact(ctx context.Context, artifact provider.Artifact, destDir string) error {
	return t.p.Install(ctx, artifact, destDir)
}

// EnrichLockfile receives a pointer to the *actual* structure that will be
// written into the lockfile's dependencies array. The Tool can read the
// ref (the key the item is referenced by) and any existing values, then
// mutate fields (Provider, DepName, Datasource, CurrentValue, etc.).
//
// This gives the Tool struct complete ownership of its metadata. On next
// lock update the current version of this method runs and can migrate
// attributes if the Tool's logic has changed.
func (t *GitHubTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.Provider = "github"
	entry.DepName = t.repo
	entry.Datasource = "github-releases"

	if strings.TrimSpace(entry.CurrentValue) == "" {
		entry.CurrentValue = entry.Version
	}
}
