package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// FileManager loads renderable Discobot data from a local JSON file. It is
// intended for deterministic UI/e2e fixtures rather than live backend sync.
type FileManager struct {
	path   string
	store  Store
	logger *slog.Logger
}

// NewFileManager creates a sync manager backed by a file:// URL.
func NewFileManager(fileURL string, store Store, logger *slog.Logger) (*FileManager, error) {
	parsed, err := url.Parse(fileURL)
	if err != nil {
		return nil, fmt.Errorf("parse file sync URL: %w", err)
	}
	if parsed.Scheme != "file" {
		return nil, fmt.Errorf("file sync URL must use file scheme: %s", parsed.Scheme)
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		return nil, fmt.Errorf("file sync URL host must be empty or localhost: %s", parsed.Host)
	}
	path := parsed.Path
	if path == "" {
		return nil, fmt.Errorf("file sync URL path is required")
	}
	return &FileManager{path: path, store: store, logger: logger}, nil
}

// Run loads the file once, publishes it, then waits for cancellation.
func (m *FileManager) Run(ctx context.Context) {
	data, err := m.load()
	if err != nil {
		m.logger.Warn("failed to load discobot file sync data", "path", m.path, "error", err)
	} else {
		m.store.SaveData(ctx, func(current *state.Data) {
			mergeFileData(current, data)
		})
	}

	<-ctx.Done()
}

func (m *FileManager) load() (state.Data, error) {
	contents, err := os.ReadFile(m.path)
	if err != nil {
		return state.Data{}, err
	}
	var data state.Data
	if err := json.Unmarshal(contents, &data); err != nil {
		return state.Data{}, err
	}
	return data, nil
}

func mergeFileData(current *state.Data, fixture state.Data) {
	if fixture.Title != "" {
		current.Title = fixture.Title
	}
	if fixture.App.Name != "" || fixture.App.Description != "" {
		current.App = fixture.App
	}
	if len(fixture.Projects) > 0 {
		current.Projects = fixture.Projects
	}
	if len(fixture.Project) > 0 {
		if current.Project == nil {
			current.Project = map[string]state.ProjectData{}
		}
		for projectID, project := range fixture.Project {
			current.Project[projectID] = project
		}
	}
}
