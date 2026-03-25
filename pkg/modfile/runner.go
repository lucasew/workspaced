package modfile

import (
	"context"
	"fmt"
	"workspaced/pkg/config"
)

type LockResult struct {
	Sources int
}

func GenerateLock(ctx context.Context, ws *Workspace) (LockResult, error) {
	cfg, err := config.LoadConfigForWorkspace(ws.Root)
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}
	return GenerateLockWithConfig(ctx, ws, cfg)
}

func GenerateLockWithConfig(ctx context.Context, ws *Workspace, cfg *config.GlobalConfig) (LockResult, error) {
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
	sourceEntries := BuildSourceLockEntries(mod)
	if err := PopulateSourceLockHashes(ctx, mod, ws.ModulesBaseDir(), sourceEntries); err != nil {
		return LockResult{}, err
	}
	if err := WriteSumFile(ws.SumPath(), &SumFile{
		Sources: sourceEntries,
	}); err != nil {
		return LockResult{}, err
	}

	return LockResult{
		Sources: len(sourceEntries),
	}, nil
}
