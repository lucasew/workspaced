package apps

import (
	"context"
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
	"workspaced/pkg/tool/provider"
	"workspaced/pkg/tool/provider/registry"

	"github.com/schollz/progressbar/v3"
)

func init() {
	registry.RegisterRegistryTool("grok-build", WrapNewTool(newGrokBuild, ""))
}

type grokBuildTool struct{}

func newGrokBuild(_ string) (provider.Tool, error) {
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
	return t.InstallArtifact(ctx, provider.Artifact{URL: url, OS: runtime.GOOS, Arch: runtime.GOARCH}, destDir)
}

func (t *grokBuildTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.Provider = "registry"
	if strings.TrimSpace(entry.CurrentValue) == "" {
		entry.CurrentValue = entry.Version
	}
}

func (t *grokBuildTool) ListArtifacts(ctx context.Context, version string) ([]provider.Artifact, error) {
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
	return []provider.Artifact{{OS: runtime.GOOS, Arch: runtime.GOARCH, URL: u}}, nil
}

func (t *grokBuildTool) InstallArtifact(ctx context.Context, art provider.Artifact, destDir string) error {
	bin := "grok"
	if runtime.GOOS == "windows" {
		bin = "grok.exe"
	}
	p := filepath.Join(destDir, bin)
	if err := t.download(ctx, art.URL, p); err != nil {
		fb := strings.Replace(art.URL, "https://x.ai/cli/", "https://storage.googleapis.com/grok-build-public-artifacts/cli/", 1)
		if fb != art.URL {
			if err = t.download(ctx, fb, p); err != nil {
				return fmt.Errorf("primary and fallback failed: %w", err)
			}
		} else {
			return err
		}
	}
	// Also expose "agent" (official installer creates both)
	agent := "agent"
	if runtime.GOOS == "windows" {
		agent = "agent.exe"
	}
	_ = os.Remove(filepath.Join(destDir, agent))
	_ = os.Symlink(bin, filepath.Join(destDir, agent))
	return nil
}

func (t *grokBuildTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	if err := t.Install(ctx, version, destDir); err != nil {
		return "", err
	}
	name := cmdName
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}
	p := filepath.Join(destDir, name)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	// default to grok binary
	g := "grok"
	if runtime.GOOS == "windows" {
		g = "grok.exe"
	}
	return filepath.Join(destDir, g), nil
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
	return "", fmt.Errorf("failed to probe grok-build latest from x.ai channels")
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

func (t *grokBuildTool) download(ctx context.Context, url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	tmp := dest + ".tmp"
	outFile, err := os.Create(tmp)
	if err != nil {
		return err
	}

	// Determine size for progress (ContentLength preferred)
	size := int64(0)
	// We'll set progress after we have the response

	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return err
	}

	resp, err := hc.Client().Do(req)
	if err != nil {
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return err
	}
	defer logging.Close(ctx, resp.Body)

	if resp.StatusCode != http.StatusOK {
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	if resp.ContentLength > 0 {
		size = resp.ContentLength
	}

	progress := t.newDownloadProgressBar(filepath.Base(url), size)
	outWriter := io.Writer(outFile)
	if progress != nil {
		outWriter = io.MultiWriter(outFile, progress)
	}

	if _, err := io.Copy(outWriter, resp.Body); err != nil {
		logging.Close(ctx, outFile)
		_ = os.Remove(tmp)
		return err
	}

	if progress != nil {
		_ = progress.Finish()
	}

	logging.Close(ctx, outFile)

	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if err := os.Chmod(dest, 0o755); err != nil {
		return err
	}

	// smoke test using driver (per guidelines)
	if c, err := execdriver.Run(ctx, dest, "--version"); err == nil {
		c.Stdin = strings.NewReader("")
		c.Stdout = io.Discard
		c.Stderr = io.Discard
		if verr := c.Run(); verr != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("smoke test failed: %w", verr)
		}
	}
	return nil
}

func (t *grokBuildTool) newDownloadProgressBar(name string, size int64) *progressbar.ProgressBar {
	description := fmt.Sprintf("downloading %s", name)
	if size > 0 {
		return progressbar.NewOptions64(
			size,
			progressbar.OptionSetDescription(description),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(24),
			progressbar.OptionThrottle(65),
			progressbar.OptionShowCount(),
			progressbar.OptionClearOnFinish(),
		)
	}

	return progressbar.NewOptions64(
		-1,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetWidth(24),
		progressbar.OptionThrottle(65),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionClearOnFinish(),
	)
}
