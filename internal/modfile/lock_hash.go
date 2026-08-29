package modfile

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lucasew/workspaced/pkg/logging"
	"github.com/lucasew/workspaced/pkg/taskgroup"
)

type sourceLockHashUpdate struct {
	alias string
	entry LockedSource
}

func PopulateSourceLockHashes(ctx context.Context, modFile *ModFile, modulesBaseDir string, entries map[string]LockedSource) error {
	if modFile == nil || len(entries) == 0 {
		return nil
	}
	logger := logging.GetLogger(ctx)

	needsWork := make([]string, 0, len(entries))
	for alias, entry := range entries {
		if strings.TrimSpace(entry.Hash) != "" {
			logger.Debug("source lock hash already present, skipping re-resolution", "source", alias)
			continue
		}
		needsWork = append(needsWork, alias)
	}
	sort.Strings(needsWork)
	if len(needsWork) == 0 {
		logger.Info("source lock hashes computed", "sources", len(entries), "resolved", 0)
		return nil
	}

	logger.Info("computing source lock hashes", "sources", len(entries), "pending", len(needsWork))

	// Map owns the aggregate bar; httpclient may add per-fetch bars under children.
	// Session root always carries a Group (MustFromContext).
	updates, err := taskgroup.Map[string, sourceLockHashUpdate]{
		Name:     "source-locks",
		Items:    needsWork,
		PoolKind: taskgroup.Internet,
		TaskName: func(_ int, alias string) string { return "source:" + alias },
		Fn: func(ctx context.Context, s *taskgroup.Status, alias string) (sourceLockHashUpdate, error) {
			entry := entries[alias]
			s.Update(alias)
			logger := logging.GetLogger(ctx)
			logger.Info("computing source lock hash", "source", alias, "provider", entry.Provider)

			src, ok := modFile.Sources[alias]
			if !ok {
				return sourceLockHashUpdate{}, fmt.Errorf("source %q: missing from module file", alias)
			}
			providerID := strings.TrimSpace(entry.Provider)
			if providerID == "" {
				providerID = strings.TrimSpace(src.Provider)
			}
			if providerID == "" {
				providerID = strings.TrimSpace(alias)
			}

			provider, ok := getSourceProvider(providerID)
			if !ok {
				return sourceLockHashUpdate{}, fmt.Errorf("source %q: %w: %q", alias, ErrUnsupportedProvider, providerID)
			}
			normalized := provider.Normalize(src)
			if strings.TrimSpace(normalized.URL) != "" {
				entry.URL = strings.TrimSpace(normalized.URL)
			}
			hash, resolved, err := provider.LockHash(ctx, alias, normalized, modulesBaseDir)
			if err != nil {
				return sourceLockHashUpdate{}, fmt.Errorf("source %q: failed to compute hash: %w", alias, err)
			}
			hash = strings.TrimSpace(hash)
			if hash == "" {
				return sourceLockHashUpdate{}, fmt.Errorf("source %q: %w: provider %q", alias, ErrEmptyHash, providerID)
			}
			if strings.TrimSpace(resolved.URL) != "" {
				entry.URL = strings.TrimSpace(resolved.URL)
			}
			if r := strings.TrimSpace(resolved.Ref); r != "" {
				entry.Ref = r
			}
			entry.Hash = hash
			logger.Info("computed source lock hash", "source", alias)
			return sourceLockHashUpdate{alias: alias, entry: entry}, nil
		},
	}.Run(ctx)
	if err != nil {
		return err
	}

	for _, u := range updates {
		entries[u.alias] = u.entry
	}
	logger.Info("source lock hashes computed", "sources", len(entries), "resolved", len(updates))
	return nil
}
