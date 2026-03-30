package modfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func WriteSumFile(path string, sum *SumFile) error {
	if sum == nil {
		sum = &SumFile{Sources: map[string]LockedSource{}, Tools: map[string]LockedTool{}}
	}
	if sum.Sources == nil {
		sum.Sources = map[string]LockedSource{}
	}
	if sum.Tools == nil {
		sum.Tools = map[string]LockedTool{}
	}
	generated := BuildRenovateDependencies(sum)
	sum.Dependencies = MergeRenovateDependencies(sum.Dependencies, generated)
	onDisk := sumFileDisk{
		Dependencies: sum.Dependencies,
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(onDisk); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
