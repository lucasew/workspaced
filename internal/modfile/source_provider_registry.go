package modfile

import (
	"context"
	"sort"
	"strings"
)

type SourceProvider interface {
	ID() string
	ResolvePath(ctx context.Context, alias string, src SourceConfig, rel string, modulesBaseDir string) (string, error)
	LockHash(ctx context.Context, alias string, src SourceConfig, modulesBaseDir string) (string, SourceConfig, error)
	Normalize(src SourceConfig) SourceConfig
	EnrichRenovateDependency(dep *RenovateDependency, src LockedSource)

	// ConfigureFromSpec fills provider-specific SourceConfig fields from the
	// parsed spec target (the part after "provider:").
	ConfigureFromSpec(cfg *SourceConfig, target string)

	// ResolveModuleRef maps an input alias config plus "path[@version]" into
	// module coordinates. handled=false means the caller should use the
	// generic fallback.
	ResolveModuleRef(src SourceConfig, pathAndVersion string) (fullRef, version string, err error, handled bool)

	// RehydrateLockedSource reconstructs runtime lock fields from a persisted
	// renovate row. ok=false if this provider does not own the dependency.
	RehydrateLockedSource(dep RenovateDependency) (LockedSource, bool)

	// LockLookupKeys returns alternate FindSource keys for a lock entry
	// (stable ref, repo id, …). Core indexes these without knowing provider shape.
	LockLookupKeys(lock LockedSource) []string

	// CanPersistLock reports whether an enriched renovate row is complete
	// enough to write for this provider.
	CanPersistLock(dep RenovateDependency, lock LockedSource) bool

	// LockReusable reports whether a rehydrated lock has enough pin metadata
	// to skip re-resolution on non-force refresh.
	LockReusable(locked LockedSource) bool

	// LockMatchesDesired reports whether locked still satisfies the cue-declared ref.
	LockMatchesDesired(desired, locked LockedSource) bool
}

var sourceProviders = map[string]SourceProvider{}

func RegisterSourceProvider(p SourceProvider) {
	sourceProviders[p.ID()] = p
}

func getSourceProvider(id string) (SourceProvider, bool) {
	p, ok := sourceProviders[strings.TrimSpace(id)]
	return p, ok
}

func allSourceProviders() []SourceProvider {
	out := make([]SourceProvider, 0, len(sourceProviders))
	for _, p := range sourceProviders {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

func sourceProviderForLock(lock LockedSource) (SourceProvider, bool) {
	if id := strings.TrimSpace(lock.Provider); id != "" {
		return getSourceProvider(id)
	}
	return SourceProvider(nil), false
}

func rehydrateSourceLock(dep RenovateDependency) (LockedSource, string, bool) {
	if strings.TrimSpace(dep.Kind) != "source" {
		return LockedSource{}, "", false
	}
	key := strings.TrimSpace(dep.Ref)
	if key == "" {
		key = strings.TrimSpace(dep.DepName)
	}
	if key == "" {
		return LockedSource{}, "", false
	}
	for _, p := range allSourceProviders() {
		if lock, ok := p.RehydrateLockedSource(dep); ok {
			return lock, key, true
		}
	}
	return genericRehydrateSourceLock(dep, key), key, true
}

func genericRehydrateSourceLock(dep RenovateDependency, key string) LockedSource {
	out := LockedSource{
		Ref:  strings.TrimSpace(dep.CurrentValue),
		Hash: strings.TrimSpace(dep.CurrentDigest),
	}
	if out.Ref == "" {
		out.Ref = key
	}
	if out.Hash == "" {
		out.Hash = strings.TrimSpace(dep.CurrentValue)
	}
	return out
}

func sourceLockLookupKeys(lock LockedSource) []string {
	if p, ok := sourceProviderForLock(lock); ok {
		return p.LockLookupKeys(lock)
	}
	var keys []string
	if r := strings.TrimSpace(lock.Ref); r != "" {
		keys = append(keys, r)
	}
	return keys
}

func sourceLockReusable(locked LockedSource) bool {
	if p, ok := sourceProviderForLock(locked); ok {
		return p.LockReusable(locked)
	}
	return strings.TrimSpace(locked.Hash) != ""
}

func sourceLockMatchesDesired(desired, locked LockedSource) bool {
	if p, ok := sourceProviderForLock(locked); ok {
		return p.LockMatchesDesired(desired, locked)
	}
	desiredRef := strings.TrimSpace(desired.Ref)
	if desiredRef == "" || strings.EqualFold(desiredRef, "HEAD") {
		return true
	}
	return desiredRef == strings.TrimSpace(locked.Ref) || desiredRef == strings.TrimSpace(locked.Hash)
}

func isRegisteredSourceProviderID(id string) bool {
	_, ok := getSourceProvider(id)
	return ok
}

func isDirectModuleSourcePrefix(left string) bool {
	switch strings.TrimSpace(left) {
	case "self", "core", "registry", "http", "https":
		return true
	}
	return isRegisteredSourceProviderID(left)
}
