package apps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"workspaced/pkg/driver"
	execdriver "workspaced/pkg/driver/exec"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	providerinstall "workspaced/pkg/tool/backend/install"
	"workspaced/pkg/tool/checks"
)

var ErrGrokBuildProbeFailure = errors.New("failed to probe grok-build latest from x.ai channels")

func init() {
	catalog.RegisterTool("grok-build", newGrokBuild)
}

type grokBuildTool struct{}

func newGrokBuild() (backend.Tool, error) {
	return &grokBuildTool{}, nil
}

func (t *grokBuildTool) ListVersions(ctx context.Context) ([]string, error) {
	v, err := t.probeLatest(ctx)
	if err != nil {
		return nil, err
	}
	return []string{v}, nil
}

func (t *grokBuildTool) Install(ctx context.Context, version string, destDir string) error {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		var err error
		v, err = t.probeLatest(ctx)
		if err != nil {
			return err
		}
	}
	plat := t.grokPlatform()
	url := "https://x.ai/cli/grok-" + v + "-" + plat
	if runtime.GOOS == "windows" {
		url += ".exe"
	}
	return t.InstallArtifact(ctx, backend.Artifact{URL: url, OS: runtime.GOOS, Arch: runtime.GOARCH}, destDir)
}

func (t *grokBuildTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	// no renovate datasource metadata for this internal tool
}

func (t *grokBuildTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		var err error
		v, err = t.probeLatest(ctx)
		if err != nil {
			return nil, err
		}
	}
	plat := t.grokPlatform()
	u := "https://x.ai/cli/grok-" + v + "-" + plat
	if runtime.GOOS == "windows" {
		u += ".exe"
	}
	return []backend.Artifact{{OS: runtime.GOOS, Arch: runtime.GOARCH, URL: u}}, nil
}

func (t *grokBuildTool) InstallArtifact(ctx context.Context, art backend.Artifact, destDir string) error {
	bin := "grok"
	if runtime.GOOS == "windows" {
		bin = "grok.exe"
	}
	path := filepath.Join(destDir, bin)

	urls := []string{art.URL}
	fallback := strings.Replace(art.URL, "https://x.ai/cli/", "https://storage.googleapis.com/grok-build-public-artifacts/cli/", 1)
	if fallback != art.URL {
		urls = append(urls, fallback)
	}
	if err := providerinstall.DownloadFirst(ctx, urls, path, providerinstall.DownloadOptions{Mode: 0o755}); err != nil {
		return err
	}

	if cmd, err := execdriver.Run(ctx, path, "--version"); err == nil {
		cmd.Stdin = strings.NewReader("")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			_ = os.Remove(path)
			return fmt.Errorf("smoke test failed: %w", err)
		}
	}

	agent := "agent"
	if runtime.GOOS == "windows" {
		agent = "agent.exe"
	}
	_ = os.Remove(filepath.Join(destDir, agent))
	_ = os.Symlink(bin, filepath.Join(destDir, agent))
	return nil
}

func (t *grokBuildTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	p, err := checks.EnsureBinary(destDir, cmdName, "grok", func() error {
		return t.Install(ctx, version, destDir)
	})
	if err == nil {
		return p, nil
	}
	if fallback := checks.FindBinary(destDir, "grok"); fallback != "" {
		return fallback, nil
	}
	return "", err
}

// --- grok-specific bits (follows https://x.ai/cli/install.sh artifact layout) ---

func (t *grokBuildTool) probeLatest(ctx context.Context) (string, error) {
	for _, base := range []string{
		"https://x.ai/cli",
		"https://storage.googleapis.com/grok-build-public-artifacts/cli",
	} {
		u := base + "/stable"
		hc, err := driver.Get[httpclient.Driver](ctx)
		if err != nil {
			continue
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		resp, err := hc.Client().Do(req)
		if err != nil {
			continue
		}
		defer logging.Close(ctx, resp.Body)
		if resp.StatusCode == http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			if s := strings.TrimSpace(string(b)); s != "" {
				return s, nil
			}
		}
	}
	return "", ErrGrokBuildProbeFailure
}

func (t *grokBuildTool) grokPlatform() string {
	osn := runtime.GOOS
	if osn == "darwin" {
		osn = "macos"
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	} else if arch == "arm64" {
		arch = "aarch64"
	}
	return osn + "-" + arch
}

func (t *grokBuildTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("grok"))
}
