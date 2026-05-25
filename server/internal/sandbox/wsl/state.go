package wsl

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// StateStore provides the path shared with the WSL startup script for runtime
// metadata.
type StateStore struct {
	statePath string
}

// RuntimeState contains metadata written by the WSL startup script.
type RuntimeState struct {
	Version    int    `json:"version,omitempty"`
	DistroName string `json:"distro_name,omitempty"`
	ImageRef   string `json:"image_ref,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
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

func (s *StateStore) Read() (RuntimeState, bool, error) {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RuntimeState{}, false, nil
		}
		return RuntimeState{}, false, err
	}

	var state RuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return RuntimeState{}, false, err
	}
	return state, true, nil
}
