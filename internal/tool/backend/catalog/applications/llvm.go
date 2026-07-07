package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"workspaced/internal/githubutil"
	"workspaced/internal/modfile"
	"workspaced/internal/tool/backend"
	"workspaced/internal/tool/backend/catalog"
	"workspaced/internal/tool/backend/github"
	providerinstall "workspaced/internal/tool/backend/install"
	"workspaced/internal/tool/checks"
	"workspaced/pkg/driver"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
)

func init() {
	catalog.RegisterTool("llvm", newLLVM)
}

type llvmTool struct {
	inner backend.Tool
}

func newLLVM() (backend.Tool, error) {
	inner, err := github.NewTool("llvm/llvm-project")
	if err != nil {
		return nil, err
	}
	return &llvmTool{inner: inner}, nil
}

func (t *llvmTool) ListVersions(ctx context.Context) ([]string, error) {
	// Use a direct fetch with larger page size so older releases (e.g. 18.x)
	// remain visible in the catalog. The generic github backend only gets the
	// default first page (~30).
	u := "https://api.github.com/repos/llvm/llvm-project/releases?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "workspaced (+https://github.com/lucasew/.dotfiles)")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	githubutil.ApplyAuth(ctx, req)

	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get http client: %w", err)
	}
	resp, err := hc.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusForbidden && strings.Contains(msg, "rate limit") {
			return nil, fmt.Errorf("github api rate limit exceeded for llvm releases (consider setting GITHUB_TOKEN or 'gh auth login')")
		}
		return nil, fmt.Errorf("github releases for llvm: %s: %s", resp.Status, msg)
	}

	var rels []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(rels))
	for _, r := range rels {
		v := strings.TrimSpace(r.TagName)
		if !strings.HasPrefix(v, "llvmorg-") {
			continue
		}
		ver := strings.TrimPrefix(v, "llvmorg-")
		if ver == "" || strings.Contains(ver, "-") {
			continue
		}
		out = append(out, ver)
	}
	if len(out) == 0 {
		return nil, ErrNoVersions
	}
	return out, nil
}

func (t *llvmTool) Install(ctx context.Context, version string, destDir string) error {
	logger := logging.GetLogger(ctx)
	logger.Warn("llvm (registry backend) is experimental; backed by official clang+llvm prebuilts from llvm/llvm-project")

	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		return t.inner.Install(ctx, v, destDir)
	}
	tag := "llvmorg-" + normalizeLLVMVersion(v)
	return t.inner.Install(ctx, tag, destDir)
}

func (t *llvmTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	// LLVM prebuilts come from llvm/llvm-project GitHub releases under llvmorg-* tags.
}

func (t *llvmTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	tag := toLLVMSpecTag(version)
	if at, ok := t.inner.(interface {
		ListArtifacts(context.Context, string) ([]backend.Artifact, error)
	}); ok {
		return at.ListArtifacts(ctx, tag)
	}
	return nil, ErrNoPlatformArtifact
}

func (t *llvmTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	logger := logging.GetLogger(ctx)
	logger.Warn("llvm (registry backend) is experimental; backed by official clang+llvm prebuilts from llvm/llvm-project")

	// Delegate to inner when possible so GitHub auth/asset download logic is used.
	if at, ok := t.inner.(interface {
		InstallArtifact(context.Context, backend.Artifact, string) error
	}); ok {
		return at.InstallArtifact(ctx, artifact, destDir)
	}
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *llvmTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	return checks.EnsureBinary(destDir, cmdName, "LLVM", func() error {
		return t.Install(ctx, version, destDir)
	})
}

// --- helpers ---

func normalizeLLVMVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "llvmorg-")
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" || v == "latest" {
		return v
	}
	return v
}

func toLLVMSpecTag(version string) string {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		return v
	}
	n := normalizeLLVMVersion(v)
	if n == "" {
		return v
	}
	return "llvmorg-" + n
}

func (t *llvmTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("clang"))
}
