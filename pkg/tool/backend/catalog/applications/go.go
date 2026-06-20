package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"workspaced/pkg/driver"
	"workspaced/pkg/driver/httpclient"
	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/tool/backend"
	"workspaced/pkg/tool/backend/catalog"
	providerinstall "workspaced/pkg/tool/backend/install"
)

func init() {
	catalog.RegisterTool("golang", newGo)
}

type goTool struct{}

func newGo() (backend.Tool, error) {
	return &goTool{}, nil
}

func (t *goTool) ListVersions(ctx context.Context) ([]string, error) {
	return t.listVersions(ctx)
}

func (t *goTool) Install(ctx context.Context, version string, destDir string) error {
	v := normalizeGoVersion(version)
	if v == "" || v == "latest" {
		vers, err := t.listVersions(ctx)
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
	return t.InstallArtifact(ctx, arts[0], destDir)
}

func (t *goTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	// No standard renovate datasource for the custom go.dev tarballs.
	if strings.TrimSpace(entry.CurrentValue) == "" {
		// caller pre-populates CurrentValue
	}
}

func (t *goTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	relVer := goVersionForIndex(version)
	if relVer == "" || relVer == "latest" {
		vers, err := t.listVersions(ctx)
		if err != nil {
			return nil, err
		}
		if len(vers) == 0 {
			return nil, ErrNoVersions
		}
		relVer = goVersionForIndex(vers[0])
	}

	u := "https://go.dev/dl/?mode=json"
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
		return nil, fmt.Errorf("go dl index: %s", resp.Status)
	}

	var releases []goRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	osName := runtime.GOOS
	archName := runtime.GOARCH

	for _, r := range releases {
		if r.Version != relVer {
			continue
		}
		for _, f := range r.Files {
			if f.Kind != "archive" {
				continue
			}
			if f.OS == osName && f.Arch == archName {
				return []backend.Artifact{{
					OS:   f.OS,
					Arch: f.Arch,
					URL:  "https://go.dev/dl/" + f.Filename,
					Hash: "sha256:" + f.SHA256,
					Size: f.Size,
				}}, nil
			}
		}
	}
	return nil, ErrNoPlatformArtifact
}

func (t *goTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	return providerinstall.InstallArtifact(ctx, artifact, destDir, providerinstall.DownloadOptions{})
}

func (t *goTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	if err := t.Install(ctx, version, destDir); err != nil {
		return "", err
	}

	// After StripTopLevelDir the Go tarball/zip layout gives us bin/go, bin/gofmt, etc.
	candidates := []string{
		filepath.Join(destDir, "bin", cmdName),
		filepath.Join(destDir, "bin", cmdName+".exe"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("binary %q not found in Go installation at %s", cmdName, destDir)
}

// --- helpers ---

type goRelease struct {
	Version string   `json:"version"`
	Stable  bool     `json:"stable"`
	Files   []goFile `json:"files"`
}

type goFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Kind     string `json:"kind"`
}

func (t *goTool) listVersions(ctx context.Context) ([]string, error) {
	u := "https://go.dev/dl/?mode=json"
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
		return nil, fmt.Errorf("go dl index: %s", resp.Status)
	}

	var releases []goRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	out := []string{}
	seen := map[string]bool{}
	osName := runtime.GOOS
	archName := runtime.GOARCH
	for _, r := range releases {
		for _, f := range r.Files {
			if f.Kind == "archive" && f.OS == osName && f.Arch == archName {
				ver := strings.TrimPrefix(r.Version, "go")
				if ver != "" && !seen[ver] {
					seen[ver] = true
					out = append(out, ver)
				}
				break
			}
		}
	}
	return out, nil
}

func normalizeGoVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "go")
	v = strings.TrimPrefix(v, "v")
	if v == "" || v == "latest" {
		return v
	}
	return v
}

func goVersionForIndex(version string) string {
	v := normalizeGoVersion(version)
	if v == "" || v == "latest" {
		return v
	}
	return "go" + v
}
