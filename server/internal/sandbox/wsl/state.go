package wsl

import "path/filepath"

// StateStore provides the path shared with the WSL startup script for bootstrap
// metadata. The script owns the file contents; Go only needs to pass the path.
type StateStore struct {
	statePath string
}

// NewStateStore creates a store rooted in the configured WSL state directory.
func NewStateStore(stateDir string) *StateStore {
	return &StateStore{
		statePath: filepath.Join(stateDir, "runtime-state.json"),
	}
}

// Path returns the runtime state file path.
func (s *StateStore) Path() string {
	return s.statePath
}
