package wsl

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// StateStore provides the path shared with the WSL startup script for runtime
// metadata.
type StateStore struct {
	statePath string
}

// RuntimeState contains metadata written by the WSL startup script.
type RuntimeState struct {
	Version                  int    `json:"version,omitempty"`
	DistroName               string `json:"distro_name,omitempty"`
	ImageRef                 string `json:"image_ref,omitempty"`
	VarDiskSizeGB            int    `json:"var_disk_size_gb,omitempty"`
	DesiredVarDiskSizeGB     int    `json:"desired_var_disk_size_gb,omitempty"`
	VarDiskResizeRequestedBy string `json:"var_disk_resize_requested_by,omitempty"`
	UpdatedAt                string `json:"updated_at,omitempty"`
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

func (s *StateStore) Write(state RuntimeState) error {
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.statePath, append(data, '\n'), 0600)
}

func (s *StateStore) RequestVarDiskSize(sizeGB int, currentSizeGB int, runtimeID string) error {
	state, found, err := s.Read()
	if err != nil {
		return err
	}
	if !found {
		state.Version = 1
	}
	if state.VarDiskSizeGB <= 0 && currentSizeGB > 0 {
		state.VarDiskSizeGB = currentSizeGB
	}
	state.DesiredVarDiskSizeGB = sizeGB
	state.VarDiskResizeRequestedBy = runtimeID
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.Write(state)
}
