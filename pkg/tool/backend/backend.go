// Package backend defines the tool backends (previously called "providers").
//
// A Backend (e.g. GitHub Releases, mise, or the internal catalog) knows how to
// turn a reference string into a concrete Tool.
//
// A Tool is the minimum viable handle for one specific external package/binary:
//   - ListVersions
//   - Install(version, destDir)
//   - EnrichLockfile (mutates the RenovateDependency entry in the lockfile)
//
// Optional richer interfaces: ArtifactTool and BinaryTool.
//
// Concrete backends live in sibling directories: github, mise, catalog.
package backend

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"workspaced/pkg/modfile"
)

// Backend is the thin handler registered for one tool source/registry
// (e.g. "github", "mise", or the internal catalog for short names).
// It is registered under an ID and is what appears before the ':' in specs
// like "github:cli/cli" or bare "uv".
type Backend interface {
	// Name is a human-friendly description ("GitHub Releases", "mise").
	Name() string

	// Tool performs the ref lookup and returns the Tool for that package.
	// ref is backend-specific:
	//   - "owner/repo" for github
	//   - "node", "deno", "python" for mise
	Tool(ref string) (Tool, error)
}

// Tool represents one specific tool/package obtained via a backend.
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
// artifacts (primarily useful for GitHub-style backends and tools like selfupdate
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

// InstallFixer is an optional extension for Tools that need to perform
// post-extraction repairs on the destDir (e.g. rewriting hashbangs in
// scripts that were baked with CI paths by prebuilt tarballs).
// Fix should be idempotent and cheap when no changes are needed.
type InstallFixer interface {
	Fix(ctx context.Context, destDir string) error
}

// --- Transitional / compatibility surface ---
// These are kept while we migrate internal call sites to the Tool-based path.
// New code should prefer Backend.Tool(ref) + the Tool interface and its extensions.

// PackageConfig is the old package reference shape. It is still used by some
// transitional code paths and by the current low-level methods on concrete backends.
type PackageConfig struct {
	Backend string
	Spec    string
	Repo    string
}

// Artifact describes a downloadable platform-specific artifact for a release.
// Used by ArtifactTool and by code that still talks to the old detailed surface.
type Artifact struct {
	OS   string
	Arch string
	URL  string
	Hash string
	Size int64

	// GitHubAssetID and GitHubAssetAPIURL are populated for GitHub-sourced
	// artifacts. They allow Install to use the proper authenticated asset
	// download endpoint (https://api.github.com/.../assets/ID with
	// Accept: application/octet-stream) instead of the browser_download_url.
	// This avoids 403 errors that can occur when sending Authorization on
	// direct release asset download URLs.
	GitHubAssetID     int64
	GitHubAssetAPIURL string
}

// ContainsAnyOf reports whether any of the needles is a substring of haystack.
// It short-circuits on the first match.
func ContainsAnyOf(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

// ScoreArtifact returns a score indicating how well the artifact matches the
// requested platform (osName + arch) and optional binaryHint.
//
// A return value of 0 means the artifact is ineligible (wrong OS/arch after
// android→linux fallback, or a .deb/.rpm package). Eligible artifacts always
// receive a positive score.
//
// Higher scores are better. SelectArtifact (and custom selection logic) can
// use this to filter (score > 0) and to rank.
func ScoreArtifact(a Artifact, osName, arch, binaryHint string) int {
	// Acceptable OSes (android falls back to linux for many projects).
	oses := []string{osName}
	if osName == "android" {
		oses = append(oses, "linux")
	}

	// Platform match?
	matchesPlatform := false
	for _, o := range oses {
		if a.OS == o && a.Arch == arch {
			matchesPlatform = true
			break
		}
	}
	if !matchesPlatform {
		return 0
	}

	// Package types we never want as direct artifacts.
	if ContainsAnyOf(a.URL, ".deb", ".rpm") {
		return 0
	}

	hint := strings.ToLower(strings.TrimSpace(binaryHint))
	base := strings.ToLower(filepath.Base(a.URL))

	score := 0

	if hint != "" {
		// Strong tokenized match (e.g. "resvg-linux", "foo_resvg_bar", "x.resvg.")
		for _, sep := range []string{"-", "_", "."} {
			if strings.Contains(base, hint+sep) || strings.Contains(base, sep+hint+sep) || strings.Contains(base, sep+hint+".") {
				score += 120
				break
			}
		}
		// Generic substring fallback
		if strings.Contains(base, hint) {
			score += 60
		}
	} else {
		// No hint provided: reserve 0 strictly for ineligibility by giving
		// eligible artifacts a minimal positive baseline.
		score = 1
	}

	// Mild preference for common CLI archive formats.
	if strings.HasSuffix(base, ".tar.gz") || strings.HasSuffix(base, ".tgz") || strings.HasSuffix(base, ".zip") {
		score += 10
	}

	// Soft penalty for obvious debug/minimal builds.
	if strings.Contains(base, "debug") {
		score -= 20
	}

	// Never let penalties turn an otherwise-eligible artifact into 0.
	if score <= 0 {
		score = 1
	}

	return score
}

// SelectArtifact chooses the best artifact for the given OS/arch from the list
// using ScoreArtifact for both eligibility (score > 0) and ranking.
//
// For android, if no android-specific artifact is found for the arch, it
// falls back to trying linux artifacts (many projects do not publish
// dedicated Android builds).
func SelectArtifact(artifacts []Artifact, osName, arch, binaryHint string) *Artifact {
	var candidates []Artifact
	for _, a := range artifacts {
		if ScoreArtifact(a, osName, arch, binaryHint) > 0 {
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		si := ScoreArtifact(candidates[i], osName, arch, binaryHint)
		sj := ScoreArtifact(candidates[j], osName, arch, binaryHint)
		if si != sj {
			return si > sj
		}
		return len(candidates[i].URL) < len(candidates[j].URL)
	})

	return &candidates[0]
}
