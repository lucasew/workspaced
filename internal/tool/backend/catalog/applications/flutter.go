package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"workspaced/internal/modfile"
	"workspaced/internal/tool/backend"
	"workspaced/internal/tool/backend/catalog"
	providerinstall "workspaced/internal/tool/backend/install"
	"workspaced/internal/tool/checks"
	"workspaced/pkg/driver"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
)

func init() {
	catalog.RegisterTool("flutter", newFlutter)
}

type flutterTool struct{}

func newFlutter() (backend.Tool, error) {
	return &flutterTool{}, nil
}

func (t *flutterTool) ListVersions(ctx context.Context) ([]string, error) {
	return t.listVersions(ctx)
}

func (t *flutterTool) Install(ctx context.Context, version string, destDir string) error {
	return installFirstArtifact(ctx, version, destDir, normalizeFlutterVersion, t.listVersions, t.ListArtifacts, t.InstallArtifact)
}

func (t *flutterTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	// Flutter releases come from Google Cloud Storage; no standard Renovate
	// datasource matches the official release index + shas.
}

func (t *flutterTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v, err := resolveToolVersion(ctx, version, normalizeFlutterVersion, t.listVersions)
	if err != nil {
		return nil, err
	}

	u := t.releasesURL()
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flutter releases index: %s", resp.Status)
	}

	var idx flutterReleasesIndex
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, err
	}

	for _, r := range idx.Releases {
		if r.Version != v {
			continue
		}
		if !t.archiveMatchesPlatform(r.Archive) {
			continue
		}
		base := strings.TrimRight(idx.BaseURL, "/")
		fullURL := base + "/" + strings.TrimLeft(r.Archive, "/")
		hash := ""
		if r.SHA256 != "" {
			hash = "sha256:" + r.SHA256
		}
		return []backend.Artifact{{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
			URL:  fullURL,
			Hash: hash,
		}}, nil
	}
	return nil, ErrNoPlatformArtifact
}

func (t *flutterTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *flutterTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	return checks.EnsureBinary(destDir, cmdName, "Flutter", func() error {
		return t.Install(ctx, version, destDir)
	})
}

// --- helpers ---

type flutterReleasesIndex struct {
	BaseURL  string           `json:"base_url"`
	Releases []flutterRelease `json:"releases"`
}

type flutterRelease struct {
	Hash    string `json:"hash"`
	Channel string `json:"channel"`
	Version string `json:"version"`
	Archive string `json:"archive"`
	SHA256  string `json:"sha256"`
}

func (t *flutterTool) listVersions(ctx context.Context) ([]string, error) {
	u := t.releasesURL()
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flutter releases index: %s", resp.Status)
	}

	var idx flutterReleasesIndex
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, err
	}

	out := []string{}
	seen := map[string]bool{}
	for _, r := range idx.Releases {
		ver := strings.TrimSpace(r.Version)
		if ver == "" || seen[ver] {
			continue
		}
		if t.archiveMatchesPlatform(r.Archive) {
			seen[ver] = true
			out = append(out, ver)
		}
	}
	if len(out) == 0 {
		return nil, ErrNoVersions
	}
	return out, nil
}

func (t *flutterTool) releasesURL() string {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	return fmt.Sprintf("https://storage.googleapis.com/flutter_infra_release/releases/releases_%s.json", platform)
}

func (t *flutterTool) archiveMatchesPlatform(archive string) bool {
	if archive == "" {
		return false
	}
	base := filepath.Base(archive)
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "linux":
		// The linux index primarily lists x64 entries; arm64 linux archives use
		// a flutter_linux_arm64_... name but are not separately enumerated in
		// the index today. Accept any linux_ entry so versions list and resolve
		// on linux (arm64 linux will currently receive the x64 SDK tarball).
		return strings.HasPrefix(base, "flutter_linux_")
	case "darwin":
		if goarch == "arm64" {
			return strings.Contains(base, "macos_arm64_")
		}
		return strings.HasPrefix(base, "flutter_macos_") && !strings.Contains(base, "_arm64_")
	case "windows":
		if goarch == "arm64" {
			return strings.Contains(base, "windows_arm64_")
		}
		return strings.HasPrefix(base, "flutter_windows_") && !strings.Contains(base, "_arm64_")
	default:
		return false
	}
}

func normalizeFlutterVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" || v == "latest" {
		return v
	}
	return v
}

func (t *flutterTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("flutter"))
}
