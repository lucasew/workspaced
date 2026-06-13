package deployer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
type FileStateStore struct {
	path string
}

// NewFileStateStore creates a FileStateStore.
func NewFileStateStore(path string) (*FileStateStore, error) {
	// Expand env vars and ~
	expanded := os.ExpandEnv(path)
	if len(expanded) > 0 && expanded[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		expanded = filepath.Join(home, expanded[1:])
	}

	// Ensure parent directory exists
	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &FileStateStore{path: expanded}, nil
}

func (s *FileStateStore) Load() (*State, error) {
	state := &State{Files: make(map[string]ManagedInfo)}

	// If the file does not exist, return an empty state
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return state, nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	// Ensure map is not nil
	if state.Files == nil {
		state.Files = make(map[string]ManagedInfo)
	}

	return state, nil
}

func (s *FileStateStore) Save(state *State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
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
