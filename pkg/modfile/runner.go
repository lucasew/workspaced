package modfile

import (
	"context"
	"fmt"
	"workspaced/pkg/configcue"
)

type LockResult struct {
	Sources int
}

func GenerateLock(ctx context.Context, ws *Workspace) (LockResult, error) {
	cfg, err := configcue.LoadForWorkspace(ws.Root)
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}
	return GenerateLockWithConfig(ctx, ws, cfg)
}

func GenerateLockWithConfig(ctx context.Context, ws *Workspace, cfg *configcue.Config) (LockResult, error) {
	if cfg == nil {
		return LockResult{}, fmt.Errorf("failed to load config: config is nil")
	}

	if err := ws.EnsureFiles(); err != nil {
		return LockResult{}, err
	}

	mod, err := ModFileFromConfig(cfg)
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}
	sum, err := LoadSumFile(ws.SumPath())
	if err != nil {
		return LockResult{}, err
	}
	beforeSources := len(sum.Sources)
	sourceEntries := BuildSourceLockEntries(mod)
	if err := PopulateSourceLockHashes(ctx, mod, ws.ModulesBaseDir(), sourceEntries); err != nil {
		return LockResult{}, err
	}
	if sum.Sources == nil {
		sum.Sources = map[string]LockedSource{}
	}
	for name, entry := range sourceEntries {
		sum.Sources[name] = entry
	}
	if len(sum.Sources) < beforeSources {
		return LockResult{}, fmt.Errorf("refusing to shrink source lock entries: before=%d after=%d", beforeSources, len(sum.Sources))
	}
	if err := WriteSumFile(ws.SumPath(), sum); err != nil {
		return LockResult{}, err
	}

	return LockResult{
		Sources: len(sourceEntries),
	}, nil
}
