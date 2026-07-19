package apps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/lucasew/workspaced/internal/modfile"
	"github.com/lucasew/workspaced/internal/semver"
	"github.com/lucasew/workspaced/internal/tool/backend"
	"github.com/lucasew/workspaced/internal/tool/backend/catalog"
	"github.com/lucasew/workspaced/internal/tool/backend/github"
	"github.com/lucasew/workspaced/internal/tool/checks"
)

// biomeReleasePrefix is the changesets/monorepo tag prefix for the CLI package.
// Other packages in the same repo (e.g. @biomejs/js-api@…) are not installable CLIs.
const biomeReleasePrefix = "@biomejs/biome@"

func init() {
	catalog.RegisterTool("biome", newBiome)
}

type biomeTool struct {
	inner backend.Tool
}

func newBiome() (backend.Tool, error) {
	inner, err := github.NewTool("biomejs/biome")
	if err != nil {
		return nil, err
	}
	return &biomeTool{inner: inner}, nil
}

func (t *biomeTool) ListVersions(ctx context.Context) ([]string, error) {
	vers, err := t.inner.ListVersions(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(vers))
	seen := map[string]bool{}
	for _, v := range vers {
		ver, ok := biomeVersionFromTag(v)
		if !ok || seen[ver] {
			continue
		}
		seen[ver] = true
		out = append(out, ver)
	}
	if len(out) == 0 {
		return nil, ErrNoVersions
	}

	// Sort descending semver so [0] == latest.
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

func (t *biomeTool) Install(ctx context.Context, version string, destDir string) error {
	v, err := resolveToolVersion(ctx, version, normalizeBiomeVersion, t.ListVersions)
	if err != nil {
		return err
	}
	arts, err := t.ListArtifacts(ctx, v)
	if err != nil {
		return err
	}
	if len(arts) == 0 {
		return ErrNoPlatformArtifact
	}
	artifact := selectBiomeArtifact(arts, runtime.GOOS, runtime.GOARCH)
	if artifact == nil {
		return fmt.Errorf("no suitable artifact found for %s/%s for registry:biome@%s", runtime.GOOS, runtime.GOARCH, v)
	}
	return t.InstallArtifact(ctx, *artifact, destDir)
}

func (t *biomeTool) EnrichLockfile(entry *modfile.RenovateDependency) {
	t.inner.EnrichLockfile(entry)
	entry.Versioning = "semver"
	// Renovate sees the real GitHub tag; lock stores the semver we expose.
	// extractVersionTemplate lets Renovate match @biomejs/biome@2.5.0 tags.
	if entry.ExtractVersion == "" {
		entry.ExtractVersion = `^@biomejs/biome@(?<version>\d+\.\d+\.\d+)$`
	}
}

func (t *biomeTool) ListArtifacts(ctx context.Context, version string) ([]backend.Artifact, error) {
	v, err := resolveToolVersion(ctx, version, normalizeBiomeVersion, t.ListVersions)
	if err != nil {
		return nil, err
	}

	tag := biomeTagForVersion(v)
	at, ok := t.inner.(backend.ArtifactTool)
	if !ok {
		return nil, fmt.Errorf("github tool does not implement ArtifactTool")
	}
	arts, err := at.ListArtifacts(ctx, tag)
	if err != nil {
		return nil, err
	}

	// Re-parse assets: biome uses win32 (not windows) and musl variants.
	// The generic github parser misses win32; we annotate musl preference via URL only.
	out := make([]backend.Artifact, 0, len(arts))
	for _, a := range arts {
		osName, arch, ok := parseBiomeAssetURL(a.URL)
		if !ok {
			// Fall back to what github already set when it could parse the name.
			if a.OS != "" && a.Arch != "" {
				out = append(out, a)
			}
			continue
		}
		a.OS = osName
		a.Arch = arch
		out = append(out, a)
	}

	// If github parser returned nothing (e.g. only win32 assets were present
	// and we got empty), still try a direct pass — ListArtifacts above already
	// walked release assets; when github matched zero we'd get empty arts.
	// That case is rare on multi-platform releases; callers handle empty.
	return out, nil
}

func (t *biomeTool) InstallArtifact(ctx context.Context, artifact backend.Artifact, destDir string) error {
	at, ok := t.inner.(backend.ArtifactTool)
	if !ok {
		return fmt.Errorf("github tool does not implement ArtifactTool")
	}
	return at.InstallArtifact(ctx, artifact, destDir)
}

func (t *biomeTool) EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error) {
	name := cmdName
	if name == "" {
		name = "biome"
	}
	p, err := checks.EnsureBinary(destDir, name, "biome", func() error {
		return t.Install(ctx, version, destDir)
	})
	if err == nil {
		return p, nil
	}
	// Last resort: anything starting with biome after installBinary normalization.
	entries, readErr := os.ReadDir(destDir)
	if readErr == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if strings.HasPrefix(n, "biome") {
				return filepath.Join(destDir, n), nil
			}
		}
	}
	return "", err
}

// biomeVersionFromTag returns the semver part of an @biomejs/biome@X.Y.Z tag.
func biomeVersionFromTag(tag string) (string, bool) {
	tag = strings.TrimSpace(tag)
	if !strings.HasPrefix(tag, biomeReleasePrefix) {
		return "", false
	}
	ver := strings.TrimPrefix(tag, biomeReleasePrefix)
	if ver == "" || strings.Contains(ver, "@") {
		return "", false
	}
	// Skip prereleases (e.g. 2.5.0-beta.1) unless we decide to support them later.
	if strings.Contains(ver, "-") {
		return "", false
	}
	return ver, true
}

func biomeTagForVersion(version string) string {
	v := normalizeBiomeVersion(version)
	if strings.HasPrefix(v, biomeReleasePrefix) {
		return v
	}
	return biomeReleasePrefix + v
}

func normalizeBiomeVersion(version string) string {
	v := strings.TrimSpace(version)
	if v == "" || v == "latest" {
		return v
	}
	if ver, ok := biomeVersionFromTag(v); ok {
		return ver
	}
	v = strings.TrimPrefix(v, "v")
	return v
}

// parseBiomeAssetURL recovers OS/arch from biome release asset basenames such as
// biome-darwin-arm64, biome-linux-x64-musl, biome-win32-x64.exe.
func parseBiomeAssetURL(rawURL string) (osName, arch string, ok bool) {
	base := strings.ToLower(filepath.Base(rawURL))
	base = strings.TrimSuffix(base, ".exe")

	switch {
	case strings.Contains(base, "darwin"):
		osName = "darwin"
	case strings.Contains(base, "linux"):
		osName = "linux"
	case strings.Contains(base, "win32"), strings.Contains(base, "windows"):
		osName = "windows"
	default:
		return "", "", false
	}

	switch {
	case strings.Contains(base, "arm64"), strings.Contains(base, "aarch64"):
		arch = "arm64"
	case strings.Contains(base, "x64"), strings.Contains(base, "amd64"), strings.Contains(base, "x86_64"):
		arch = "amd64"
	case strings.Contains(base, "ia32"), strings.Contains(base, "x86"):
		arch = "386"
	default:
		return "", "", false
	}
	return osName, arch, true
}

// selectBiomeArtifact prefers non-musl builds when both exist for the platform.
func selectBiomeArtifact(arts []backend.Artifact, goos, goarch string) *backend.Artifact {
	var best *backend.Artifact
	bestScore := -1
	for i := range arts {
		a := &arts[i]
		osName, arch := a.OS, a.Arch
		if osName == "" || arch == "" {
			if o, ar, ok := parseBiomeAssetURL(a.URL); ok {
				osName, arch = o, ar
			}
		}
		if osName != goos || arch != goarch {
			continue
		}
		score := 100
		base := strings.ToLower(filepath.Base(a.URL))
		if strings.Contains(base, "musl") {
			score -= 40
		}
		if score > bestScore {
			bestScore = score
			best = a
		}
	}
	return best
}

func (t *biomeTool) InstallChecks() []checks.Check {
	return checks.Checks(checks.Binary("biome"))
}
