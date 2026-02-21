package local

import (
	"context"
	"path/filepath"
	"strings"
	"workspaced/pkg/modfile"
)

type Provider struct{}

func (p Provider) ID() string { return "local" }

func (p Provider) ResolvePath(ctx context.Context, alias string, src modfile.SourceConfig, rel string, modulesBaseDir string) (string, error) {
	_ = ctx
	_ = alias
	base := strings.TrimSpace(src.Path)
	if base == "" {
		base = modulesBaseDir
	} else if !filepath.IsAbs(base) {
		base = filepath.Join(filepath.Dir(modulesBaseDir), base)
	}
	return filepath.Join(base, rel), nil
}

func (p Provider) LockHash(ctx context.Context, alias string, src modfile.SourceConfig, modulesBaseDir string) (string, error) {
	_ = ctx
	_ = alias
	_ = src
	_ = modulesBaseDir
	return "", nil
}

func (p Provider) Normalize(src modfile.SourceConfig) modfile.SourceConfig {
	src.Provider = "local"
	src.Path = strings.TrimSpace(src.Path)
	src.Repo = strings.TrimSpace(src.Repo)
	src.URL = strings.TrimSpace(src.URL)
	return src
}

func init() {
	modfile.RegisterSourceProvider(Provider{})
}
