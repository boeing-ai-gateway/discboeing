package thread

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Info is the persisted metadata view for a thread.
type Info struct {
	ID              string
	Name            string
	CWD             string
	LastMessage     string
	ErrorMessage    string
	Model           string
	Reasoning       string
	ServiceTier     string
	State           State
	PendingQuestion bool
	ActiveCommand   string
	Metadata        json.RawMessage
}

// CreateThreadRequest describes initial thread metadata.
type CreateThreadRequest struct {
	ID          string
	Name        string
	CWD         string
	LastMessage string
	Metadata    json.RawMessage
}

// UpdateThreadRequest describes editable thread metadata fields.
type UpdateThreadRequest struct {
	Name              *string
	CWD               *string
	LastMessage       *string
	ErrorMessage      *string
	ClearErrorMessage bool
	Metadata          json.RawMessage
}

// ListThreadInfos returns metadata for all threads in the store.
func (s *Store) ListThreadInfos() ([]Info, error) {
	threadIDs, err := s.ListThreads()
	if err != nil {
		return nil, err
	}
	infos := make([]Info, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		info, err := s.GetThreadInfo(threadID)
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// GetThreadInfo returns persisted metadata for one thread.
func (s *Store) GetThreadInfo(threadID string) (Info, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return Info{}, fmt.Errorf("thread ID is required")
	}
	exists, err := s.ThreadExists(threadID)
	if err != nil {
		return Info{}, err
	}
	if !exists {
		return Info{}, os.ErrNotExist
	}
	cfg, err := s.LoadConfig(threadID)
	if err != nil {
		return Info{}, err
	}
	return s.ThreadInfoFromConfig(threadID, cfg), nil
}

// CreateThreadInfo creates a thread and returns its persisted metadata.
func (s *Store) CreateThreadInfo(defaultCWD string, req CreateThreadRequest) (Info, error) {
	threadID := strings.TrimSpace(req.ID)
	if threadID == "" {
		return Info{}, fmt.Errorf("id is required")
	}
	exists, err := s.ThreadExists(threadID)
	if err != nil {
		return Info{}, err
	}
	if exists {
		return Info{}, fmt.Errorf("thread already exists")
	}
	if err := s.CreateThread(threadID); err != nil {
		return Info{}, err
	}
	cfg, err := s.LoadConfig(threadID)
	if err != nil {
		return Info{}, err
	}
	if trimmedCWD := strings.TrimSpace(req.CWD); trimmedCWD != "" {
		cfg.CWD = trimmedCWD
	} else if strings.TrimSpace(cfg.CWD) == "" {
		cfg.CWD = strings.TrimSpace(defaultCWD)
	}
	if trimmedName := strings.TrimSpace(req.Name); trimmedName != "" {
		cfg.Name = trimmedName
		cfg.NameSource = ThreadNameSourceUser
	}
	if req.LastMessage != "" {
		cfg.LastMessage = req.LastMessage
	}
	if len(req.Metadata) > 0 {
		var metadata ConfigMetadata
		if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
			return Info{}, fmt.Errorf("decode thread metadata: %w", err)
		}
		cfg.Metadata = metadata
	}
	if err := s.SaveConfig(threadID, cfg); err != nil {
		return Info{}, err
	}
	return s.ThreadInfoFromConfig(threadID, cfg), nil
}

// UpdateThreadInfo updates a thread and returns its persisted metadata.
func (s *Store) UpdateThreadInfo(threadID string, req UpdateThreadRequest) (Info, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return Info{}, fmt.Errorf("id is required")
	}
	exists, err := s.ThreadExists(threadID)
	if err != nil {
		return Info{}, err
	}
	if !exists {
		return Info{}, os.ErrNotExist
	}
	cfg, err := s.LoadConfig(threadID)
	if err != nil {
		return Info{}, err
	}
	if req.Name != nil {
		if trimmedName := strings.TrimSpace(*req.Name); trimmedName != "" {
			cfg.Name = trimmedName
			cfg.NameSource = ThreadNameSourceUser
		}
	}
	if req.CWD != nil {
		if trimmedCWD := strings.TrimSpace(*req.CWD); trimmedCWD != "" {
			cfg.CWD = trimmedCWD
		}
	}
	if req.LastMessage != nil {
		cfg.LastMessage = *req.LastMessage
	}
	if req.ErrorMessage != nil {
		cfg.ErrorMessage = strings.TrimSpace(*req.ErrorMessage)
	}
	if req.ClearErrorMessage {
		cfg.ErrorMessage = ""
	}
	if len(req.Metadata) > 0 {
		var metadata ConfigMetadata
		if err := json.Unmarshal(req.Metadata, &metadata); err != nil {
			return Info{}, fmt.Errorf("decode thread metadata: %w", err)
		}
		cfg.Metadata = metadata
	}
	if err := s.SaveConfig(threadID, cfg); err != nil {
		return Info{}, err
	}
	return s.ThreadInfoFromConfig(threadID, cfg), nil
}

// DeleteThreadInfo deletes a thread after validating it exists.
func (s *Store) DeleteThreadInfo(threadID string) error {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return fmt.Errorf("id is required")
	}
	exists, err := s.ThreadExists(threadID)
	if err != nil {
		return err
	}
	if !exists {
		return os.ErrNotExist
	}
	return s.DeleteThread(threadID)
}

// ThreadInfoFromConfig projects a persisted config into the public thread view.
func (s *Store) ThreadInfoFromConfig(threadID string, cfg Config) Info {
	pendingQuestion := false
	if state, err := s.LoadTurnState(threadID); err == nil && state != nil {
		pendingQuestion = state.Phase == PhaseWaitingForAnswer
	}
	serviceTier := cfg.ServiceTier
	if strings.TrimSpace(serviceTier) == "" {
		serviceTier = s.serviceTierFromTurnHistory(threadID, cfg)
	}
	return Info{
		ID:              threadID,
		Name:            strings.TrimSpace(cfg.Name),
		CWD:             strings.TrimSpace(cfg.CWD),
		LastMessage:     strings.TrimSpace(cfg.LastMessage),
		ErrorMessage:    strings.TrimSpace(cfg.ErrorMessage),
		Model:           cfg.Model,
		Reasoning:       string(cfg.Reasoning),
		ServiceTier:     serviceTier,
		State:           cfg.LastTurnState,
		PendingQuestion: pendingQuestion,
		ActiveCommand:   strings.TrimSpace(cfg.ActiveCommand),
		Metadata:        cfg.Metadata.RawMessage(),
	}
}

func (s *Store) serviceTierFromTurnHistory(threadID string, cfg Config) string {
	if strings.TrimSpace(cfg.ActiveLeafID) != "" {
		if turnIDs, err := s.HistoryTurnIDs(threadID); err == nil {
			if turnID := turnIDs[cfg.ActiveLeafID]; strings.TrimSpace(turnID) != "" {
				if state, err := s.LoadTurnRecord(threadID, turnID); err == nil && state != nil {
					return strings.TrimSpace(state.Config.ServiceTier)
				}
			}
		}
	}

	entries, err := os.ReadDir(s.turnsDir(threadID))
	if err != nil {
		return ""
	}
	var latestTime time.Time
	latestTier := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		state, err := s.LoadTurnRecord(threadID, entry.Name())
		if err != nil || state == nil || strings.TrimSpace(state.Config.ServiceTier) == "" {
			continue
		}
		updatedAt := state.StartedAt
		if state.UpdatedAt != nil {
			updatedAt = state.UpdatedAt
		}
		if latestTier == "" || (updatedAt != nil && updatedAt.After(latestTime)) {
			if updatedAt != nil {
				latestTime = *updatedAt
			}
			latestTier = strings.TrimSpace(state.Config.ServiceTier)
		}
	}
	return latestTier
}
