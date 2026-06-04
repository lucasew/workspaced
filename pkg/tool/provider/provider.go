package provider

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"workspaced/pkg/modfile"
)

// Provider is the thin handler registered for one tool registry (e.g. "github", "mise").
// It is looked up by the ID used at registration time (the "provider" part of "provider:ref").
// The handler's only job for the MVA is to produce a Tool for a given ref string.
type Provider interface {
	// Name is a human-friendly description of the registry ("GitHub Releases", "mise").
	Name() string

	// Tool performs the simple ref lookup and returns the Tool for that package.
	// ref is the provider-specific package identifier:
	//   - "owner/repo" for github
	//   - "node", "deno", "python" for mise
	Tool(ref string) (Tool, error)
}

// Tool represents one specific tool/package obtained via a provider ref.
// It is the minimum viable "one tool":
// - it can list versions (the version lookuper)
// - it can install itself into a caller-provided directory (the install handler)
type Tool interface {
	ListVersions(ctx context.Context) ([]string, error)
	Install(ctx context.Context, version string, destDir string) error

	// EnrichLockfile receives a pointer to the exact RenovateDependency that
	// will be stored in the lockfile for this tool.
	//
	// The Tool can inspect the current state (Name, Ref, Version, any
	// previously written CurrentValue/DepName etc.) and mutate attributes
	// directly. The ref is the key the item is referenced by in the lock.
	//
	// Because it is a reference to the live structure that gets persisted,
	// any logic changes or migrations inside the Tool's EnrichLockfile
	// implementation are applied to the lockfile entry by default on the
	// next refresh/update of that tool.
	EnrichLockfile(entry *modfile.RenovateDependency)
}

// ArtifactTool is an optional extension for Tools that can expose raw
// artifacts (primarily useful for GitHub-style providers and tools like selfupdate
// that need to do custom platform selection or install to non-standard locations).
type ArtifactTool interface {
	Tool
	ListArtifacts(ctx context.Context, version string) ([]Artifact, error)
	InstallArtifact(ctx context.Context, artifact Artifact, destDir string) error
}

// BinaryTool is an optional extension for Tools that support direct
// "ensure this named binary exists for the given version" handling
// (e.g. the mise backend).
type BinaryTool interface {
	Tool
	EnsureBinary(ctx context.Context, version string, cmdName string, destDir string) (string, error)
}

// --- Transitional / compatibility surface ---
// These are kept while we migrate internal call sites to the Tool-based path.
// New code should prefer Provider.Tool(ref) + the Tool interface and its extensions.

// PackageConfig is the old package reference shape. It is still used by some
// transitional code paths and by the current low-level methods on concrete providers.
type PackageConfig struct {
	Provider string
	Spec     string
	Repo     string
}

// BinaryProvider is the old extension interface. Code is being migrated to
// check for BinaryTool on the value returned by Provider.Tool(ref) instead.
type BinaryProvider interface {
	EnsureBinary(ctx context.Context, pkg PackageConfig, version string, cmdName string, destPath string) (string, error)
}

// Artifact describes a downloadable platform-specific artifact for a release.
// Used by ArtifactTool and by code that still talks to the old detailed surface.
type Artifact struct {
	OS   string
	Arch string
	URL  string
	Hash string
	Size int64
}

// SelectArtifact chooses the best artifact for the given OS/arch from the list,
// applying optional binary hint scoring (used by both the manager and
// provider Tool implementations during the migration).
func SelectArtifact(artifacts []Artifact, osName, arch, binaryHint string) *Artifact {
	var candidates []Artifact
	for _, a := range artifacts {
		if a.OS == osName && a.Arch == arch {
			if strings.HasSuffix(a.URL, ".deb") {
				continue
			}
			if strings.HasSuffix(a.URL, ".rpm") {
				continue
			}
			candidates = append(candidates, a)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	hint := strings.ToLower(strings.TrimSpace(binaryHint))
	sort.Slice(candidates, func(i, j int) bool {
		si := scoreArtifactForHint(candidates[i].URL, hint)
		sj := scoreArtifactForHint(candidates[j].URL, hint)
		if si != sj {
			return si > sj
		}
		return len(candidates[i].URL) < len(candidates[j].URL)
	})

	return &candidates[0]
}

func scoreArtifactForHint(url string, hint string) int {
	if hint == "" {
		return 0
	}

	base := strings.ToLower(filepath.Base(url))
	score := 0

	// Strong matches for tokenized binary names (resvg-*, *_resvg_*, etc.)
	for _, sep := range []string{"-", "_", "."} {
		if strings.Contains(base, hint+sep) || strings.Contains(base, sep+hint+sep) || strings.Contains(base, sep+hint+".") {
			score += 120
			break
		}
	}

	// Generic match fallback
	if strings.Contains(base, hint) {
		score += 60
	}

	// Slightly prefer common distributable archives for CLI tools
	if strings.HasSuffix(base, ".tar.gz") || strings.HasSuffix(base, ".tgz") || strings.HasSuffix(base, ".zip") {
		score += 10
	}

	// Avoid obvious debug/minimal artifacts when possible
	if strings.Contains(base, "debug") {
		score -= 20
	}

	return score
}
