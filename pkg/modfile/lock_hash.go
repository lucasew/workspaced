package modfile

import (
	"context"
	"fmt"
	"strings"
)

func PopulateSourceLockHashes(ctx context.Context, modFile *ModFile, modulesBaseDir string, entries map[string]LockedSource) error {
	if modFile == nil {
		return nil
	}
	for alias, entry := range entries {
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
		hash, err := provider.LockHash(ctx, alias, normalized, modulesBaseDir)
		if err != nil {
			return fmt.Errorf("source %q: failed to compute hash: %w", alias, err)
		}
		hash = strings.TrimSpace(hash)
		if hash == "" {
			return fmt.Errorf("source %q: provider %q returned empty hash", alias, providerID)
		}
		entry.Hash = hash
		entries[alias] = entry
	}
	return nil
}
