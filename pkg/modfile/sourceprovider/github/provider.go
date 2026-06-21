package github

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"workspaced/pkg/logging"
	"workspaced/pkg/modfile"
	"workspaced/pkg/modfile/sourceprovider/common"
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
	cv := strings.TrimSpace(dep.CurrentValue)
	if cv == "" {
		cv = strings.TrimSpace(src.Ref)
	}
	// Only attach renovate info if we have a usable (non-HEAD or with URL)
	// per the best-effort semantics.
	hasUsable := !strings.EqualFold(cv, "HEAD") || strings.TrimSpace(src.URL) != ""
	if !hasUsable {
		return
	}
	// Fill renovate fields for this github source. The stable "ref" is the
	// source identifier "github:owner/repo" (like tool refs), depName and
	// datasource for renovate to manage it.
	//
	// Default tracking is latest commit on the default branch (github-commits),
	// not latest tag (github-tags). The pinned commit SHA lives in currentValue
	// only — never currentDigest, which renovate cannot update via our manager.
	dep.Ref = "github:" + repo
	dep.DepName = repo
	dep.Datasource = "github-commits"
	// Prefer the resolved commit SHA from the tarball URL over a symbolic ref
	// (HEAD/branch/tag) so renovate sees a concrete, updatable pin.
	if u := modfile.RefFromTarballURL(src.URL); u != "" {
		dep.CurrentValue = u
	} else if strings.TrimSpace(dep.CurrentValue) == "" {
		if r := strings.TrimSpace(src.Ref); r != "" && !strings.EqualFold(r, "HEAD") {
			dep.CurrentValue = r
		}
	} else if strings.EqualFold(strings.TrimSpace(dep.CurrentValue), "HEAD") {
		// No URL/sha yet; leave empty so we don't attach unusable renovate fields.
		dep.CurrentValue = ""
	}
	dep.CurrentDigest = ""
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
