package modfile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/workspaced/pkg/logging"
)

func IsLockableProvider(provider string) bool {
	switch strings.TrimSpace(provider) {
	case "self", "core":
		return false
	default:
		return true
	}
}

func BuildSourceLockEntries(modFile *ModFile) map[string]LockedSource {
	out := map[string]LockedSource{}
	if modFile == nil {
		return out
	}
	for name, src := range modFile.Sources {
		provider := strings.TrimSpace(src.Provider)
		if provider == "" {
			provider = strings.TrimSpace(name)
		}
		if !IsLockableProvider(provider) {
			continue
		}
		if p, ok := getSourceProvider(provider); ok {
			src = p.Normalize(src)
		}
		out[name] = LockedSource{
			Provider: provider,
			Path:     strings.TrimSpace(src.Path),
			Repo:     strings.TrimSpace(src.Repo),
			Ref:      strings.TrimSpace(src.Ref),
			URL:      strings.TrimSpace(src.URL),
		}
	}
	return out
}

func writeSumFile(ctx context.Context, path string, sum *SumFile) error {
	if sum == nil {
		sum = &SumFile{}
	}
	if err := normalizeDependencies(sum.Dependencies); err != nil {
		return err
	}

	onDisk := sumFileDisk{
		Dependencies: sum.Dependencies,
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Write via temp + rename so a crash mid-write cannot leave a truncated
	// workspaced.lock.json. On any post-create failure, remove the temp so it
	// does not linger next to the live lockfile.
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(onDisk); err != nil {
		logging.Close(ctx, f)
		_ = os.Remove(tmpPath)
		return err
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
