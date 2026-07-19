package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"

	"github.com/lucasew/workspaced/internal/modfile"
	"github.com/lucasew/workspaced/internal/semver"
	"github.com/lucasew/workspaced/internal/tool/backend"
	"github.com/lucasew/workspaced/internal/tool/backend/catalog"
	providerinstall "github.com/lucasew/workspaced/internal/tool/backend/install"
	"github.com/lucasew/workspaced/internal/tool/checks"
	"github.com/lucasew/workspaced/pkg/driver"
	"github.com/lucasew/workspaced/pkg/driver/httpclient"
	"github.com/lucasew/workspaced/pkg/logging"
)

// esbuild is installed from the @esbuild/* platform packages on the npm
// registry (same layout mise uses for http:esbuild). No Node runtime is
// required: each package is a tarball with bin/esbuild (or esbuild.exe).

const esbuildRegistryBase = "https://registry.npmjs.org"

func init() {
	catalog.RegisterTool("esbuild", newEsbuild)
}

type esbuildTool struct{}

func newEsbuild() (backend.Tool, error) {
	return &esbuildTool{}, nil
}

func (t *esbuildTool) ListVersions(ctx context.Context) ([]string, error) {
	return t.listVersions(ctx)
}

func (t *esbuildTool) Install(ctx context.Context, version string, destDir string) error {
	return installFirstArtifact(ctx, version, destDir, normalizeEsbuildVersion, t.listVersions, t.ListArtifacts, t.InstallArtifact)
}

func (t *esbuildTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.DepName = "esbuild"
	entry.Datasource = "npm"
	entry.Versioning = "semver"
}

func (t *esbuildTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v, err := resolveToolVersion(ctx, version, normalizeEsbuildVersion, t.listVersions)
	if err != nil {
		return nil, err
	}

	plat, ok := esbuildPlatform(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrNoPlatformArtifact, runtime.GOOS, runtime.GOARCH)
	}

	url := esbuildArtifactURL(plat, v)
	hash := t.fetchTarballHash(ctx, plat, v)

	return []backend.Artifact{{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		URL:  url,
		Hash: hash,
	}}, nil
}

func (t *esbuildTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *esbuildTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	name := cmdName
	if name == "" {
		name = "esbuild"
	}
	return checks.EnsureBinary(destDir, name, "esbuild", func() error {
		return t.Install(ctx, version, destDir)
	})
}

func (t *esbuildTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("esbuild"))
}

func (t *esbuildTool) listVersions(ctx context.Context) ([]string, error) {
	body, err := esbuildGET(ctx, esbuildRegistryBase+"/esbuild")
	if err != nil {
		return nil, err
	}

	var packument struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err := json.Unmarshal(body, &packument); err != nil {
		return nil, err
	}
	if len(packument.Versions) == 0 {
		return nil, ErrNoVersions
	}

	out := make([]string, 0, len(packument.Versions))
	for ver := range packument.Versions {
		ver = strings.TrimSpace(ver)
		// Stable releases only (skip 0.19.0-beta, etc.).
		if ver == "" || strings.Contains(ver, "-") {
			continue
		}
		out = append(out, ver)
	}
	if len(out) == 0 {
		return nil, ErrNoVersions
	}

	svs := make(semver.SemVers, len(out))
	for i, s := range out {
		svs[i] = semver.Parse(s)
	}
	sort.Sort(sort.Reverse(svs))
	for i, s := range svs {
		out[i] = s.Original
	}
	return out, nil
}

// fetchTarballHash returns "sha1:<shasum>" from the platform package document.
// Failures are non-fatal: install still works without a hash (direct download).
func (t *esbuildTool) fetchTarballHash(ctx context.Context, plat, version string) string {
	u := fmt.Sprintf("%s/@esbuild/%s/%s", esbuildRegistryBase, plat, version)
	body, err := esbuildGET(ctx, u)
	if err != nil {
		return ""
	}
	var doc struct {
		Dist struct {
			Shasum string `json:"shasum"`
		} `json:"dist"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return ""
	}
	if doc.Dist.Shasum == "" {
		return ""
	}
	return "sha1:" + doc.Dist.Shasum
}

func esbuildGET(ctx context.Context, u string) ([]byte, error) {
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	// Prefer install-v1 packument (smaller) when the registry supports it.
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json; q=1.0, application/json; q=0.8")
	resp, err := hc.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func esbuildArtifactURL(plat, version string) string {
	// https://registry.npmjs.org/@esbuild/linux-x64/-/linux-x64-0.28.1.tgz
	return fmt.Sprintf("%s/@esbuild/%s/-/%s-%s.tgz", esbuildRegistryBase, plat, plat, version)
}

func normalizeEsbuildVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	if v == "" || v == "latest" {
		return v
	}
	return v
}

// esbuildPlatform maps GOOS/GOARCH to the @esbuild/<platform> package suffix
// (npm/esbuild naming: darwin not macos, win32 not windows, x64 not amd64).
func esbuildPlatform(goos, goarch string) (plat string, ok bool) {
	var osPart string
	switch goos {
	case "linux":
		osPart = "linux"
	case "darwin":
		osPart = "darwin"
	case "windows":
		osPart = "win32"
	case "freebsd":
		osPart = "freebsd"
	case "netbsd":
		osPart = "netbsd"
	case "openbsd":
		osPart = "openbsd"
	default:
		return "", false
	}

	var archPart string
	switch goarch {
	case "amd64":
		archPart = "x64"
	case "arm64":
		archPart = "arm64"
	case "arm":
		archPart = "arm"
	case "386":
		archPart = "ia32"
	case "ppc64":
		archPart = "ppc64"
	case "riscv64":
		archPart = "riscv64"
	case "s390x":
		archPart = "s390x"
	case "loong64":
		archPart = "loong64"
	case "mips64le":
		archPart = "mips64el"
	default:
		return "", false
	}

	return osPart + "-" + archPart, true
}
