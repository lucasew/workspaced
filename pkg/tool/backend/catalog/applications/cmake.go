package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/semver"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	"workspaced/pkg/tool/backend/github"
	providerinstall "workspaced/pkg/tool/backend/install"
	"workspaced/pkg/tool/checks"
)

func init() {
	catalog.RegisterTool("cmake", newCMake)
}

type cmakeTool struct {
	inner backend.Tool
}

type cmakeFilesManifest struct {
	Version cmakeVersionInfo `json:"version"`
	Files   []cmakeFileEntry `json:"files"`
}

type cmakeVersionInfo struct {
	String string `json:"string"`
}

type cmakeFileEntry struct {
	OS           []string `json:"os"`
	Architecture []string `json:"architecture"`
	Class        string   `json:"class"`
	Name         string   `json:"name"`
}

func newCMake() (backend.Tool, error) {
	inner, err := github.NewTool("Kitware/CMake")
	if err != nil {
		return nil, err
	}
	return &cmakeTool{inner: inner}, nil
}

func (t *cmakeTool) ListVersions(ctx context.Context) ([]string, error) {
	vers, err := t.inner.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(vers))
	seen := map[string]bool{}
	for _, v := range vers {
		v = strings.TrimSpace(v)
		v = strings.TrimPrefix(v, "v")
		v = strings.TrimPrefix(v, "V")
		if v == "" {
			continue
		}
		// Skip prereleases (e.g. 4.4.0-rc2). Final releases have no "-".
		if strings.Contains(v, "-") {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, ErrNoVersions
	}

	// Sort descending so [0] is the newest (robust "latest").
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

func (t *cmakeTool) Install(ctx context.Context, version string, destDir string) error {
	v := normalizeCMakeVersion(version)
	if v == "" || v == "latest" {
		vers, err := t.ListVersions(ctx)
		if err != nil {
			return err
		}
		if len(vers) == 0 {
			return ErrNoVersions
		}
		v = vers[0]
	}
	arts, err := t.ListArtifacts(ctx, v)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return ErrNoPlatformArtifact
	}
	artifact := backend.SelectArtifact(arts, runtime.GOOS, runtime.GOARCH, "cmake")
	if artifact == nil {
		return fmt.Errorf("no suitable cmake artifact found for %s/%s @ %s", runtime.GOOS, runtime.GOARCH, v)
	}
	return t.InstallArtifact(ctx, *artifact, destDir)
}

func (t *cmakeTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.Versioning = "semver"
	// Official downloads via cmake.org; files-v1.json + SHA-256.txt sidecar.
}

func (t *cmakeTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v := normalizeCMakeVersion(version)
	if v == "" || v == "latest" {
		vers, err := t.ListVersions(ctx)
		if err != nil {
			return nil, err
		}
		if len(vers) == 0 {
			return nil, ErrNoVersions
		}
		v = vers[0]
	}

	dir := cmakeDirForVersion(v)
	manifest, err := t.fetchManifest(ctx, dir, v)
	if err != nil {
		return nil, err
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	wantOS := cmakeOS(goos)
	wantArch := cmakeArch(goos, goarch)

	var candidates []backend.Artifact
	for _, f := range manifest.Files {
		if f.Class != "archive" {
			continue
		}
		if !containsCI(f.OS, wantOS) {
			continue
		}
		if !containsCI(f.Architecture, wantArch) {
			continue
		}
		// Skip legacy macos10.10 if a regular macos one is present (we'll pick best below)
		url := fmt.Sprintf("https://cmake.org/files/%s/%s", dir, f.Name)
		candidates = append(candidates, backend.Artifact{
			OS:   goos,
			Arch: goarch,
			URL:  url,
			// hash filled below per chosen
		})
	}

	if len(candidates) == 0 {
		return nil, ErrNoPlatformArtifact
	}

	// Prefer non-legacy macOS tarball when both exist.
	best := pickBestCMakeArchive(candidates)

	// Attach hash from SHA-256.txt sidecar (official at cmake.org)
	filename := filepath.Base(best.URL)
	if h, err := t.fetchSHA256(ctx, dir, v, filename); err == nil && h != "" {
		best.Hash = "sha256:" + h
	}

	return []backend.Artifact{best}, nil
}

func (t *cmakeTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *cmakeTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	if err := t.Install(ctx, version, destDir); err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(destDir, "bin", cmdName),
		filepath.Join(destDir, "bin", cmdName+".exe"),
		filepath.Join(destDir, cmdName),
		filepath.Join(destDir, cmdName+".exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("binary %q not found in CMake installation at %s", cmdName, destDir)
}

// --- helpers ---

func normalizeCMakeVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" || v == "latest" {
		return v
	}
	return v
}

func cmakeDirForVersion(ver string) string {
	// ver is already normalized like "4.3.4"
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		return "v" + ver // fallback
	}
	return "v" + parts[0] + "." + parts[1]
}

func cmakeOS(goos string) string {
	switch goos {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return goos // linux etc
	}
}

func cmakeArch(goos, goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "arm64":
		if goos == "linux" {
			return "aarch64"
		}
		return "arm64"
	case "386":
		return "i386"
	default:
		return goarch
	}
}

func containsCI(haystack []string, needle string) bool {
	n := strings.ToLower(needle)
	for _, h := range haystack {
		if strings.ToLower(h) == n {
			return true
		}
	}
	return false
}

func pickBestCMakeArchive(cands []backend.Artifact) backend.Artifact {
	if len(cands) == 1 {
		return cands[0]
	}
	// Prefer entries whose URL basename does not contain "10.10" (legacy mac)
	for _, c := range cands {
		base := strings.ToLower(filepath.Base(c.URL))
		if !strings.Contains(base, "10.10") {
			return c
		}
	}
	return cands[0]
}

func (t *cmakeTool) fetchManifest(ctx context.Context, dir, ver string) (cmakeFilesManifest, error) {
	u := fmt.Sprintf("https://cmake.org/files/%s/cmake-%s-files-v1.json", dir, ver)
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return cmakeFilesManifest{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return cmakeFilesManifest{}, err
	}
	resp, err := hc.Client().Do(req)
	if err != nil {
		return cmakeFilesManifest{}, err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return cmakeFilesManifest{}, fmt.Errorf("cmake files manifest: %s", resp.Status)
	}
	var m cmakeFilesManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return cmakeFilesManifest{}, err
	}
	return m, nil
}

func (t *cmakeTool) fetchSHA256(ctx context.Context, dir, ver, filename string) (string, error) {
	u := fmt.Sprintf("https://cmake.org/files/%s/cmake-%s-SHA-256.txt", dir, ver)
	hc, err := driver.Get[httpclient.Driver](ctx)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := hc.Client().Do(req)
	if err != nil {
		return "", err
	}
	defer logging.Close(ctx, resp.Body)
	if resp.StatusCode != http.StatusOK {
		// Some very old releases may lack it; non-fatal.
		return "", nil
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		fs := strings.Fields(line)
		if len(fs) >= 2 && fs[1] == filename {
			return fs[0], nil
		}
	}
	return "", nil
}

func (t *cmakeTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("cmake"))
}
