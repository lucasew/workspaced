package github

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"workspaced/internal/modfile"
	"workspaced/internal/modfile/sourceprovider/common"
	"workspaced/pkg/logging"
)

var (
	ErrMissingCachedHash = errors.New("missing cached source hash metadata")
)

type Provider struct{}

func (p Provider) ID() string { return "github" }

func (p Provider) ResolvePath(ctx context.Context, alias string, src modfile.SourceConfig, rel string, modulesBaseDir string) (string, error) {
	_ = modulesBaseDir
	root, err := ensureGithubSource(ctx, alias, src)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, rel), nil
}

func (p Provider) Normalize(src modfile.SourceConfig) modfile.SourceConfig {
	s := newSource("", src)
	return s.Config
}

func (p Provider) LockHash(ctx context.Context, alias string, src modfile.SourceConfig, modulesBaseDir string) (string, modfile.SourceConfig, error) {
	_ = modulesBaseDir
	normalized := newSource(alias, src)
	root, err := ensureGithubSource(ctx, alias, src)
	if err != nil {
		return "", src, err
	}

	meta, err := normalized.ReadMeta(root)
	if err != nil {
		return "", src, fmt.Errorf("failed to read source metadata: %w", err)
	}
	if strings.TrimSpace(meta.Hash) == "" {
		return "", src, ErrMissingCachedHash
	}

	normalized.Config.URL = strings.TrimSpace(meta.URL)
	// Renovate git-refs needs a real branch/tag in currentValue — never HEAD and
	// never a commit SHA (those live in the tarball URL / currentDigest).
	// Empty, HEAD, or SHA config => track the repo default branch for updates.
	trackRef := strings.TrimSpace(normalized.Config.Ref)
	if trackRef == "" || strings.EqualFold(trackRef, "HEAD") || shaRefRe.MatchString(trackRef) {
		branch, berr := normalized.resolveDefaultBranch(ctx, normalized.Repo())
		if berr != nil {
			return "", src, fmt.Errorf("failed to resolve default branch for renovate tracking: %w", berr)
		}
		trackRef = branch
	}
	normalized.Config.Ref = trackRef
	return strings.TrimSpace(meta.Hash), normalized.Config, nil
}

func (p Provider) EnrichRenovateDependency(dep *modfile.RenovateDependency, src modfile.LockedSource) {
	if dep == nil {
		return
	}
	repo := strings.TrimSpace(src.Repo)
	if repo == "" {
		return
	}

	pinnedSHA := ""
	if u := refFromCodeloadTarballURL(src.URL); shaRefRe.MatchString(u) {
		pinnedSHA = u
	} else if shaRefRe.MatchString(strings.TrimSpace(src.Ref)) {
		pinnedSHA = strings.TrimSpace(src.Ref)
	} else if shaRefRe.MatchString(strings.TrimSpace(dep.CurrentDigest)) {
		pinnedSHA = strings.TrimSpace(dep.CurrentDigest)
	} else if shaRefRe.MatchString(strings.TrimSpace(dep.CurrentValue)) {
		// Legacy lock entries stored the SHA in currentValue (pre git-refs).
		pinnedSHA = strings.TrimSpace(dep.CurrentValue)
	}

	// Prefer explicit branch/tag; never emit HEAD or a commit as currentValue.
	trackRef := trackingGitRef(src.Ref, dep.CurrentValue)
	if trackRef == "" {
		// Incomplete lock row (SHA-only / HEAD-only). Do not partially write
		// renovate fields — UpsertSource must not persist lock.Ref as identity.
		return
	}

	// Stable lock identity (kind+ref), independent of the tracked git ref.
	dep.Ref = "github:" + repo
	dep.DepName = repo
	// Renovate tracks latest commit on a git ref via git-refs: symbolic ref in
	// currentValue, commit SHA in currentDigest, full clone URL in packageName.
	dep.Datasource = "git-refs"
	dep.PackageName = "https://github.com/" + repo
	dep.CurrentValue = trackRef
	dep.CurrentDigest = pinnedSHA
}

// trackingGitRef is the symbolic git ref Renovate should follow (branch/tag).
// Commit SHAs and HEAD are not usable as git-refs currentValue — SHAs belong
// in currentDigest; HEAD is only Renovate's fallback when currentValue is empty.
func trackingGitRef(srcRef, currentValue string) string {
	for _, candidate := range []string{srcRef, currentValue} {
		c := strings.TrimSpace(candidate)
		if c == "" || strings.EqualFold(c, "HEAD") || shaRefRe.MatchString(c) {
			continue
		}
		return c
	}
	return ""
}

func init() {
	modfile.RegisterSourceProvider(Provider{})
}

func ensureGithubSource(ctx context.Context, alias string, src modfile.SourceConfig) (string, error) {
	s := newSource(alias, src)
	if s.Repo() == "" {
		return "", fmt.Errorf("source alias %q (github) requires repo", alias)
	}

	logger := logging.GetLogger(ctx)
	logger.Info("resolving github source", "alias", s.Alias, "repo", s.Repo(), "ref", s.Ref(), "url", s.Config.URL)
	return common.EnsureCachedDir(ctx, "github", s.CacheKey(), func(tmpDir string) error {
		meta, err := downloadAndExtractTarball(ctx, s, tmpDir, "")
		if err != nil {
			return fmt.Errorf("failed to fetch source %q: %w", alias, err)
		}
		if err := s.WriteMeta(tmpDir, meta); err != nil {
			return fmt.Errorf("failed to write source metadata: %w", err)
		}
		logger.Info("fetched github source", "alias", s.Alias, "url", meta.URL, "sha256", meta.Hash)
		return nil
	})
}
