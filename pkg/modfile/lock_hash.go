package modfile

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
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
			if pinnedRef := refFromCodeloadTarballURL(entry.URL); pinnedRef != "" {
				entry.Ref = pinnedRef
			}
		}
		if strings.TrimSpace(entry.Ref) == "" && strings.TrimSpace(resolved.Ref) != "" {
			entry.Ref = strings.TrimSpace(resolved.Ref)
		}
		entry.Hash = hash
		entries[alias] = entry
		slog.Info("computed source lock hash", "source", alias)
	}
	slog.Info("source lock hashes computed", "sources", len(entries))
	return nil
}

func refFromCodeloadTarballURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if !strings.EqualFold(parsed.Hostname(), "codeload.github.com") {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	// codeload.github.com/<owner>/<repo>/tar.gz/<ref>
	if len(parts) != 4 || parts[2] != "tar.gz" {
		return ""
	}
	return strings.TrimSpace(parts[3])
}
