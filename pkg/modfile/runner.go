package modfile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"workspaced/pkg/configcue"
)

var (
	ErrNilConfig = errors.New("config is nil")
)

type LockResult struct {
	Sources int
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
		// Pre-load hashes from the existing sumfile for sources that are already
		// resolved/pinned. This avoids re-resolving (and re-hitting the network or
		// even the "resolving github source" logs) on every refresh, mirroring the
		// "skip if already locked with version" logic used for lazy tools.
		// Only skip when not force (i.e. mod lock forces re-pinning).
		if cursum, lerr := ws.LoadSumFile(); lerr == nil {
			for name, entry := range sourceEntries {
				if locked, ok := cursum.FindSource(name); ok && strings.TrimSpace(locked.Hash) != "" {
					desiredRef := strings.TrimSpace(entry.Ref)
					lockedRef := strings.TrimSpace(locked.Ref)
					if desiredRef == "" || desiredRef == "HEAD" || desiredRef == lockedRef {
						entry.Hash = locked.Hash
						if strings.TrimSpace(locked.URL) != "" {
							entry.URL = locked.URL
						}
						if lockedRef != "" {
							entry.Ref = lockedRef
						}
						sourceEntries[name] = entry
					}
				}
			}
		}
	}

	if err := PopulateSourceLockHashes(ctx, mod, ws.ModulesBaseDir(), sourceEntries); err != nil {
		return LockResult{}, err
	}
	_, err = ws.UpdateSumFile(ctx, func(sum *SumFile) (bool, error) {
		beforeSources := len(sum.SourceLocks())
		changed := false
		for name, entry := range sourceEntries {
			if sum.UpsertSource(name, entry) {
				changed = true
			}
		}
		afterSources := len(sum.SourceLocks())
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
	}, nil
}
