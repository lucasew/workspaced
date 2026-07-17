package deployer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	envdriver "workspaced/pkg/driver/env"
)

// StateStore is the interface for state persistence.
type StateStore interface {
	// Load loads the current state.
	Load() (*State, error)

	// Save persists the state.
	Save(state *State) error

	// Path returns the path or identifier of the store (for logging).
	Path() string
}

// FileStateStore implements StateStore using a JSON file.
// On disk, file keys are stored relative to Root (home for home apply,
// workspace/git root for codebase apply). In memory, Load returns absolute
// paths so planner/executor can use them directly.
type FileStateStore struct {
	path string
	root string
}

// NewFileStateStore creates a FileStateStore.
// root is the apply target base (e.g. $HOME or the workspace root); paths in
// the state file are stored relative to it. Empty root keeps absolute keys.
func NewFileStateStore(path, root string) (*FileStateStore, error) {
	expanded := envdriver.ExpandPath(path)

	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	root = filepath.Clean(envdriver.ExpandPath(root))
	return &FileStateStore{path: expanded, root: root}, nil
}

func (s *FileStateStore) Load() (*State, error) {
	state := &State{Files: make(map[string]ManagedInfo)}

	// If the file does not exist, return an empty state
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return state, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var disk State
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	if disk.Files == nil {
		return state, nil
	}

	// Expand on-disk keys (relative to root, ~/…, or legacy absolute) to abs.
	for key, info := range disk.Files {
		abs := AbsFromRoot(key, s.root)
		state.Files[abs] = info
	}

	return state, nil
}

func (s *FileStateStore) Save(state *State) error {
	disk := &State{Files: make(map[string]ManagedInfo)}
	if state != nil && state.Files != nil {
		// Deterministic key order for stable JSON (map range is random).
		keys := make([]string, 0, len(state.Files))
		for k := range state.Files {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, abs := range keys {
			rel := RelToRoot(abs, s.root)
			disk.Files[rel] = state.Files[abs]
		}
	}

	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write via temp + rename so a crash mid-write cannot leave a truncated
	// state.json that later Load cannot parse (same pattern as lockfile).
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write state temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace state file: %w", err)
	}

	return nil
}

func (s *FileStateStore) Path() string {
	return s.path
}

// MemoryStateStore implements StateStore in memory (useful for tests).
type MemoryStateStore struct {
	state *State
	id    string
}

// NewMemoryStateStore creates a MemoryStateStore.
func NewMemoryStateStore(id string) *MemoryStateStore {
	return &MemoryStateStore{
		state: &State{Files: make(map[string]ManagedInfo)},
		id:    id,
	}
}

func (s *MemoryStateStore) Load() (*State, error) {
	if s.state == nil {
		s.state = &State{Files: make(map[string]ManagedInfo)}
	}
	return s.state, nil
}

func (s *MemoryStateStore) Save(state *State) error {
	s.state = state
	return nil
}

func (s *MemoryStateStore) Path() string {
	return fmt.Sprintf("memory:%s", s.id)
}
