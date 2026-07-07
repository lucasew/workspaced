package local

import (
	"context"
	"path/filepath"
	"strings"
	"workspaced/internal/modfile"
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

func (p Provider) LockHash(ctx context.Context, alias string, src modfile.SourceConfig, modulesBaseDir string) (string, modfile.SourceConfig, error) {
	_ = ctx
	_ = alias
	_ = src
	_ = modulesBaseDir
	return "", src, nil
}

func (p Provider) Normalize(src modfile.SourceConfig) modfile.SourceConfig {
	src.Provider = "local"
	src.Path = strings.TrimSpace(src.Path)
	src.Repo = strings.TrimSpace(src.Repo)
	src.URL = strings.TrimSpace(src.URL)
	return src
}

func (p Provider) EnrichRenovateDependency(dep *modfile.RenovateDependency, src modfile.LockedSource) {
	// local sources don't get renovate metadata
}

func (p Provider) ConfigureFromSpec(cfg *modfile.SourceConfig, target string) {
	if cfg == nil {
		return
	}
	cfg.Path = strings.TrimSpace(target)
	cfg.Repo = ""
}

func (p Provider) ResolveModuleRef(src modfile.SourceConfig, pathAndVersion string) (fullRef, version string, err error, handled bool) {
	_ = src
	_ = pathAndVersion
	return "", "", nil, false
}

func (p Provider) RehydrateLockedSource(dep modfile.RenovateDependency) (modfile.LockedSource, bool) {
	_ = dep
	return modfile.LockedSource{}, false
}

func (p Provider) LockLookupKeys(lock modfile.LockedSource) []string {
	_ = lock
	return nil
}

func (p Provider) CanPersistLock(dep modfile.RenovateDependency, lock modfile.LockedSource) bool {
	_ = dep
	_ = lock
	return true
}

func (p Provider) LockReusable(locked modfile.LockedSource) bool {
	return strings.TrimSpace(locked.Hash) != ""
}

func (p Provider) LockMatchesDesired(desired, locked modfile.LockedSource) bool {
	desiredRef := strings.TrimSpace(desired.Ref)
	if desiredRef == "" || strings.EqualFold(desiredRef, "HEAD") {
		return true
	}
	return desiredRef == strings.TrimSpace(locked.Ref) || desiredRef == strings.TrimSpace(locked.Hash)
}

func init() {
	modfile.RegisterSourceProvider(Provider{})
}
