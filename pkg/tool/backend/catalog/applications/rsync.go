package apps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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
	catalog.RegisterTool("rsync", newRsync)
}

type rsyncTool struct{}

func newRsync() (backend.Tool, error) {
	return &rsyncTool{}, nil
}

func (t *rsyncTool) ListVersions(ctx context.Context) ([]string, error) {
	return t.listVersions(ctx)
}

func (t *rsyncTool) Install(ctx context.Context, version string, destDir string) error {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		vers, err := t.listVersions(ctx)
		if err != nil {
			return err
		}
		if len(vers) == 0 {
			return fmt.Errorf("no rsync versions found")
		}
		v = vers[0]
	}
	arts, err := t.ListArtifacts(ctx, v)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return fmt.Errorf("no artifact for current platform")
	}
	return t.InstallArtifact(ctx, arts[0], destDir)
}

func (t *rsyncTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	entry.Provider = "registry"
	if strings.TrimSpace(entry.CurrentValue) == "" {
		entry.CurrentValue = entry.Version
	}
}

func (t *rsyncTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v := strings.TrimSpace(version)
	isLatest := false
	if v == "" || v == "latest" {
		isLatest = true
		vers, err := t.listVersions(ctx)
		if err != nil {
			return nil, err
		}
		if len(vers) == 0 {
			return nil, fmt.Errorf("no rsync versions")
		}
		v = vers[0]
	}

	platformFolder, err := t.rsyncPlatformFolder()
	if err != nil {
		return nil, err
	}

	urls := []string{
		fmt.Sprintf("https://download.samba.org/pub/rsync/binaries/%s/rsync-%s.tar.gz", platformFolder, v),
	}
	if isLatest {
		urls = append(urls, fmt.Sprintf("https://download.samba.org/pub/rsync/binaries/%s/latest.tar.gz", platformFolder))
	}

	return []backend.Artifact{{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		URL:  strings.Join(urls, ","), // We encode multiple URLs in the URL string, separated by commas.
	}}, nil
}

func (t *rsyncTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	urls := strings.Split(artifact.URL, ",")
	if len(urls) == 0 {
		return fmt.Errorf("no URL provided")
	}
	// We use DownloadFirst which isn't directly exposed for tarball extraction. Wait, providerinstall.InstallArtifact takes an artifact.
	// We'll try each URL until one succeeds.
	var lastErr error
	for _, u := range urls {
		art := artifact
		art.URL = u
		err := providerinstall.InstallArtifact(ctx, art, destDir, providerinstall.DownloadOptions{})
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("failed to install rsync artifact: %w", lastErr)
}

func (t *rsyncTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	if err := t.Install(ctx, version, destDir); err != nil {
		return "", err
	}

	// Try usual paths, since the tar file includes usr/local/bin/rsync
	candidates := []string{
		filepath.Join(destDir, "usr", "local", "bin", cmdName),
		filepath.Join(destDir, "usr", "local", "bin", cmdName+".exe"),
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
	// fallback
	return filepath.Join(destDir, "usr", "local", "bin", "rsync"), nil
}

// --- helpers ---

var versionRegex = regexp.MustCompile(`rsync-([0-9]+\.[0-9]+\.[0-9]+)\.tar\.gz`)

func (t *rsyncTool) listVersions(ctx context.Context) ([]string, error) {
	u := "https://download.samba.org/pub/rsync/src/"
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
		return nil, fmt.Errorf("listing src/: %s", resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	versionMap := map[string]bool{}
	matches := versionRegex.FindAllStringSubmatch(string(b), -1)
	for _, match := range matches {
		if len(match) > 1 {
			versionMap[match[1]] = true
		}
	}

	var versions []string
	for v := range versionMap {
		versions = append(versions, v)
	}

	// Simple semver sorting without importing semver package
	sort.Slice(versions, func(i, j int) bool {
		return t.compareVersions(versions[i], versions[j]) > 0 // Descending
	})

	return versions, nil
}

func (t *rsyncTool) rsyncPlatformFolder() (string, error) {
	osn := runtime.GOOS
	arch := runtime.GOARCH

	if osn == "linux" {
		if arch == "amd64" {
			return "debian-11-x86_64", nil
		} else if arch == "arm64" {
			return "centos-8-aarch64", nil
		}
	} else if osn == "darwin" {
		if arch == "arm64" {
			return "macos-12.6-arm64", nil
		}
	} else if osn == "windows" {
		return "", fmt.Errorf("rsync precompiled binaries not available for windows")
	}

	return "", fmt.Errorf("unsupported platform for rsync precompiled binaries: %s/%s", osn, arch)
}

func (t *rsyncTool) compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		p1 := 0
		if i < len(parts1) {
			_, err := fmt.Sscanf(parts1[i], "%d", &p1)
			if err != nil {
				// fallback if not a number
				p1 = 0
			}
		}
		p2 := 0
		if i < len(parts2) {
			_, err := fmt.Sscanf(parts2[i], "%d", &p2)
			if err != nil {
				// fallback if not a number
				p2 = 0
			}
		}

		if p1 > p2 {
			return 1
		} else if p1 < p2 {
			return -1
		}
	}
	return 0
}
