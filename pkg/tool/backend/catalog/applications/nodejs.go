package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	providerinstall "workspaced/pkg/tool/backend/install"
	"workspaced/pkg/tool/checks"
)

var (
	// ErrNoVersions is the shared sentinel for "this catalog SDK tool has no
	// versions available for the current platform".
	// The older per-tool names are aliases to the same value for compatibility.
	ErrNoVersions         = errors.New("no versions found")
	ErrNoGoVersions       = ErrNoVersions
	ErrNoNodeVersions     = ErrNoVersions
	ErrNoPlatformArtifact = errors.New("no artifact for current platform")
)

func init() {
	catalog.RegisterTool("nodejs", newNodejs)
}

type nodejsTool struct{}

func newNodejs() (backend.Tool, error) {
	return &nodejsTool{}, nil
}

func (t *nodejsTool) ListVersions(ctx context.Context) ([]string, error) {
	return t.listVersions(ctx)
}

func (t *nodejsTool) Install(ctx context.Context, version string, destDir string) error {
	return installFirstArtifact(ctx, version, destDir, normalizeNodejsVersion, t.listVersions, t.ListArtifacts, t.InstallArtifact)
}

func (t *nodejsTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	// No standard renovate datasource for direct nodejs.org; shasums give us
	// verification at install time via fetchurl.
}

func (t *nodejsTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v := normalizeNodejsVersion(version)
	if v == "" || v == "latest" {
		vers, err := t.listVersions(ctx)
		if err != nil {
			return nil, err
		}
		if len(vers) == 0 {
			return nil, ErrNoNodeVersions
		}
		v = vers[0]
	}

	osPart, archPart, ext := t.nodePlatformAndExt()
	filename := fmt.Sprintf("node-%s-%s-%s%s", v, osPart, archPart, ext)
	url := fmt.Sprintf("https://nodejs.org/dist/%s/%s", v, filename)

	// Fetch SHASUMS256.txt so we can attach hash and use fetchurl backend.
	sums, _ := t.fetchShasums(ctx, v)
	hash := ""
	if h, ok := sums[filename]; ok && h != "" {
		hash = "sha256:" + h
	}

	return []backend.Artifact{{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		URL:  url,
		Hash: hash,
	}}, nil
}

func (t *nodejsTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *nodejsTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	p, err := checks.EnsureBinary(destDir, cmdName, "Node.js", func() error {
		return t.Install(ctx, version, destDir)
	})
	if err == nil {
		return p, nil
	}
	if fallback := checks.FindBinary(destDir, "node"); fallback != "" {
		return fallback, nil
	}
	return "", err
}

// --- helpers ---

func (t *nodejsTool) listVersions(ctx context.Context) ([]string, error) {
	u := "https://nodejs.org/dist/index.json"
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := hc.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index.json: %s", resp.Status)
	}
	var infos []struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return nil, err
	}
	out := make([]string, len(infos))
	for i, v := range infos {
		out[i] = v.Version
	}
	return out, nil
}

func (t *nodejsTool) fetchShasums(ctx context.Context, ver string) (map[string]string, error) {
	ver = normalizeNodejsVersion(ver)
	u := fmt.Sprintf("https://nodejs.org/dist/%s/SHASUMS256.txt", ver)
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := hc.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SHASUMS: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		fs := strings.Fields(line)
		if len(fs) >= 2 {
			m[fs[1]] = fs[0]
		}
	}
	return m, nil
}

func (t *nodejsTool) nodePlatformAndExt() (osPart, archPart, ext string) {
	osPart = runtime.GOOS
	archPart = runtime.GOARCH
	ext = ".tar.gz"

	switch osPart {
	case "darwin":
		osPart = "darwin"
	case "linux":
		osPart = "linux"
	case "windows":
		osPart = "win"
		ext = ".zip"
	}

	switch archPart {
	case "amd64":
		archPart = "x64"
	case "arm64":
		archPart = "arm64"
	case "386":
		archPart = "x86"
	}

	return osPart, archPart, ext
}

func normalizeNodejsVersion(version string) string {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" || strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

func (t *nodejsTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("node"))
}
