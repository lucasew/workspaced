package modfile

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

func PopulateSourceLockHashes(ctx context.Context, modFile *ModFile, modulesBaseDir string, entries map[string]LockedSource) error {
	if modFile == nil {
		return nil
	}
	slog.Info("computing source lock hashes", "sources", len(entries))
	for alias, entry := range entries {
		slog.Info("computing source lock hash", "source", alias, "provider", entry.Provider)
		src, ok := modFile.Sources[alias]
		if !ok {
			continue
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
			return fmt.Errorf("source %q provider %q not supported for lock hash", alias, providerID)
		}
		normalized := provider.Normalize(src)
		if strings.TrimSpace(normalized.URL) != "" {
			entry.URL = strings.TrimSpace(normalized.URL)
		}
		hash, resolved, err := provider.LockHash(ctx, alias, normalized, modulesBaseDir)
		if err != nil {
			return fmt.Errorf("source %q: failed to compute hash: %w", alias, err)
		}
		hash = strings.TrimSpace(hash)
		if hash == "" {
			return fmt.Errorf("source %q: provider %q returned empty hash", alias, providerID)
		}
		if strings.TrimSpace(resolved.URL) != "" {
			entry.URL = strings.TrimSpace(resolved.URL)
		}
		entry.Hash = hash
		entries[alias] = entry
		slog.Info("computed source lock hash", "source", alias)
	}
	slog.Info("source lock hashes computed", "sources", len(entries))
	return nil
}
