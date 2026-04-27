package wsl

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const runtimeStateVersion = 1

// RuntimeState stores persisted Windows-side WSL runtime metadata.
type RuntimeState struct {
	Version    int       `json:"version"`
	DistroName string    `json:"distro_name,omitempty"`
	BridgeType string    `json:"bridge_type,omitempty"`
	BridgePort int       `json:"bridge_port,omitempty"`
	ImageRef   string    `json:"image_ref,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// StateStore persists WSL runtime metadata needed across process restarts.
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

// Load reads persisted state. A missing file is treated as empty state.
func (s *StateStore) Load() (RuntimeState, error) {
	if strings.TrimSpace(s.statePath) == "" {
		return RuntimeState{}, fmt.Errorf("runtime state path is empty")
	}

	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RuntimeState{}, nil
		}
		return RuntimeState{}, fmt.Errorf("read runtime state %s: %w", s.statePath, err)
	}

	var state RuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return RuntimeState{}, fmt.Errorf("decode runtime state %s: %w", s.statePath, err)
	}
	return state, nil
}

// Save writes the provided runtime state atomically.
func (s *StateStore) Save(state RuntimeState) error {
	if strings.TrimSpace(s.statePath) == "" {
		return fmt.Errorf("runtime state path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0755); err != nil {
		return fmt.Errorf("create runtime state directory %s: %w", filepath.Dir(s.statePath), err)
	}

	state.Version = runtimeStateVersion
	state.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime state %s: %w", s.statePath, err)
	}
	data = append(data, '\n')

	tmpPath := s.statePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write runtime state temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, s.statePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace runtime state %s: %w", s.statePath, err)
	}
	return nil
}

// Clear removes persisted runtime state.
func (s *StateStore) Clear() error {
	if strings.TrimSpace(s.statePath) == "" {
		return fmt.Errorf("runtime state path is empty")
	}
	if err := os.Remove(s.statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove runtime state %s: %w", s.statePath, err)
	}
	return nil
}
