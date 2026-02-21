package github

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"workspaced/pkg/modfile"
	"workspaced/pkg/modfile/sourceprovider/common"
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
		return "", src, fmt.Errorf("missing cached source hash metadata")
	}

	normalized.Config.URL = strings.TrimSpace(meta.URL)
	return strings.TrimSpace(meta.Hash), normalized.Config, nil
}

func init() {
	modfile.RegisterSourceProvider(Provider{})
}

func ensureGithubSource(ctx context.Context, alias string, src modfile.SourceConfig) (string, error) {
	s := newSource(alias, src)
	if s.Repo() == "" {
		return "", fmt.Errorf("source alias %q (github) requires repo", alias)
	}

	slog.Info("resolving github source", "alias", s.Alias, "repo", s.Repo(), "ref", s.Ref(), "url", s.Config.URL)
	return common.EnsureCachedDir("github", s.CacheKey(), func(tmpDir string) error {
		meta, err := downloadAndExtractTarball(ctx, s, tmpDir, "")
		if err != nil {
			return fmt.Errorf("failed to fetch source %q: %w", alias, err)
		}
		if err := s.WriteMeta(tmpDir, meta); err != nil {
			return fmt.Errorf("failed to write source metadata: %w", err)
		}
		slog.Info("fetched github source", "alias", s.Alias, "url", meta.URL, "sha256", meta.Hash)
		return nil
	})
}
