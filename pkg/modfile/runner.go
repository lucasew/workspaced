package modfile

import (
	"context"
	"fmt"
	"workspaced/pkg/config"
)

type LockResult struct {
	Sources int
	Modules int
}

func GenerateLock(ctx context.Context, ws *Workspace) (LockResult, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return LockResult{}, fmt.Errorf("failed to load config: %w", err)
	}

	if err := ws.EnsureFiles(); err != nil {
		return LockResult{}, err
	}

	mod, err := LoadModFile(ws.ModPath())
	if err != nil {
		return LockResult{}, err
	}
	moduleEntries, err := BuildLockEntries(cfg, mod, ws.ModulesBaseDir())
	if err != nil {
		return LockResult{}, err
	}
	sourceEntries := BuildSourceLockEntries(mod)
	if err := PopulateSourceLockHashes(ctx, mod, ws.ModulesBaseDir(), sourceEntries); err != nil {
		return LockResult{}, err
	}
	if err := WriteSumFile(ws.SumPath(), &SumFile{
		Sources: sourceEntries,
		Modules: moduleEntries,
	}); err != nil {
		return LockResult{}, err
	}

	return LockResult{
		Sources: len(sourceEntries),
		Modules: len(moduleEntries),
	}, nil
}
