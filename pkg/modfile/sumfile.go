package modfile

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type LockedSource struct {
	Provider string `json:"provider"`
	Path     string `json:"path"`
	Repo     string `json:"repo"`
	URL      string `json:"url"`
	Hash     string `json:"hash"`
}

type LockedTool struct {
	Ref     string `json:"ref"`
	Version string `json:"version"`
}

type SumFile struct {
	Sources map[string]LockedSource `json:"sources"`
	Tools   map[string]LockedTool   `json:"tools"`
}

func LoadSumFile(path string) (*SumFile, error) {
	out := &SumFile{
		Sources: map[string]LockedSource{},
		Tools:   map[string]LockedTool{},
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return out, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if out.Sources == nil {
		out.Sources = map[string]LockedSource{}
	}
	if out.Tools == nil {
		out.Tools = map[string]LockedTool{}
	}
	for name, lock := range out.Sources {
		lock.Provider = strings.TrimSpace(lock.Provider)
		lock.Path = strings.TrimSpace(lock.Path)
		lock.Repo = strings.TrimSpace(lock.Repo)
		lock.URL = strings.TrimSpace(lock.URL)
		lock.Hash = strings.TrimSpace(lock.Hash)
		if lock.Provider == "" {
			return nil, fmt.Errorf("invalid lock entry for source %q: provider is required", name)
		}
		if lock.Hash == "" {
			return nil, fmt.Errorf("invalid lock entry for source %q: hash is required", name)
		}
		out.Sources[name] = lock
	}
	for name, lock := range out.Tools {
		lock.Ref = strings.TrimSpace(lock.Ref)
		lock.Version = strings.TrimSpace(lock.Version)
		if lock.Ref == "" {
			return nil, fmt.Errorf("invalid lock entry for tool %q: ref is required", name)
		}
		if lock.Version == "" {
			return nil, fmt.Errorf("invalid lock entry for tool %q: version is required", name)
		}
		out.Tools[name] = lock
	}
	return out, nil
}
