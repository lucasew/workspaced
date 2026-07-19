package modfile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lucasew/workspaced/internal/configcue"
)

var (
	ErrNilConfig = errors.New("config is nil")
)

type LockResult struct {
	Sources int
	Changed bool
}

func GenerateLock(ctx context.Context, ws *Workspace) (LockResult, error) {
	cfg, err := configcue.LoadForWorkspace(ctx, ws.Root)
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}
	return GenerateLockWithConfig(ctx, ws, cfg, true)
}

func GenerateLockWithConfig(ctx context.Context, ws *Workspace, cfg *configcue.Config, force bool) (LockResult, error) {
	if cfg == nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", ErrNilConfig)
	}

	if err := ws.EnsureFiles(ctx); err != nil {
		return LockResult{}, err
	}

	mod, err := ModFileFromConfig(cfg)
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}
	sourceEntries := BuildSourceLockEntries(mod)

	if !force {
		// Pre-load pins from the existing sumfile for sources that are already
		// resolved with a tracking ref + digest. Avoids re-resolving on every
		// apply refresh. mod lock uses force=true and always re-pins.
		if cursum, lerr := ws.LoadSumFile(); lerr == nil {
			for name, entry := range sourceEntries {
				locked, ok := findExistingSourceLock(cursum, name, entry)
				if !ok || !sourceLockReusable(locked) || !sourceLockMatchesDesired(entry, locked) {
					continue
				}
				entry.Hash = locked.Hash
				if strings.TrimSpace(locked.URL) != "" {
					entry.URL = locked.URL
				}
				if ref := strings.TrimSpace(locked.Ref); ref != "" {
					entry.Ref = ref
				}
				sourceEntries[name] = entry
			}
		}
	}

	if err := PopulateSourceLockHashes(ctx, mod, ws.ModulesBaseDir(), sourceEntries); err != nil {
		return LockResult{}, err
	}
	changed, err := ws.UpdateSumFile(ctx, func(sum *SumFile) (bool, error) {
		beforeSources := countSourceDependencies(sum)
		changed := false
		for name, entry := range sourceEntries {
			if sum.UpsertSource(name, entry) {
				changed = true
			}
		}
		afterSources := countSourceDependencies(sum)
		if afterSources < beforeSources {
			return false, fmt.Errorf("%w: before=%d after=%d", ErrLockEntryShrunk, beforeSources, afterSources)
		}
		return changed, nil
	})
	if err != nil {
		return LockResult{}, err
	}

	return LockResult{
		Sources: len(sourceEntries),
		Changed: changed,
	}, nil
}

func countSourceDependencies(sum *SumFile) int {
	if sum == nil {
		return 0
	}
	n := 0
	for _, dep := range sum.Dependencies {
		if strings.TrimSpace(dep.Kind) == "source" {
			n++
		}
	}
	return n
}

func findExistingSourceLock(sum *SumFile, alias string, entry LockedSource) (LockedSource, bool) {
	if sum == nil {
		return LockedSource{}, false
	}
	if locked, ok := sum.FindSource(alias); ok {
		return locked, true
	}
	// Lock deps are keyed by stable provider identity, not cue alias.
	for _, key := range sourceLockLookupKeys(entry) {
		if locked, ok := sum.FindSource(key); ok {
			return locked, true
		}
	}
	return LockedSource{}, false
}
