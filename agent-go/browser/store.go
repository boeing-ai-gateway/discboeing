package browser

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/files"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

// Store owns thread-local browser event and artifact persistence.
type Store struct {
	baseDir         string
	browserEventsMu sync.Mutex
}

// NewStore creates a browser store rooted at the thread data directory.
func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func (s *Store) threadDir(threadID string) string {
	return filepath.Join(s.baseDir, threadID)
}

func (s *Store) turnsDir(threadID string) string {
	return filepath.Join(s.threadDir(threadID), "turns")
}

func (s *Store) browserEventsPath(threadID, turnID string, step int) string {
	return filepath.Join(s.turnsDir(threadID), turnID, fmt.Sprintf("step-%03d-browser.jsonl", step))
}

func (s *Store) stepResultPath(threadID, turnID string, step int) string {
	return filepath.Join(s.turnsDir(threadID), turnID, fmt.Sprintf("step-%03d-result.json", step))
}

func (s *Store) browserArtifactDir(threadID string) string {
	return filepath.Join(s.threadDir(threadID), "artifacts", "browser", "sha256")
}

// readThreadArtifact reads a browser artifact from a thread-local artifact URI path.
func (s *Store) readThreadArtifact(threadID, artifactPath string) (*files.ReadResult, *files.Error) {
	if s == nil {
		return nil, &files.Error{Message: ErrStoreUnavailable.Error(), Status: http.StatusServiceUnavailable}
	}
	return files.ReadFile(artifactPath, s.threadDir(threadID))
}

// appendBrowserEvent appends one browser event record to the per-step browser log.
func (s *Store) appendBrowserEvent(threadID, turnID string, step int, event thread.BrowserEvent) error {
	dir := filepath.Join(s.turnsDir(threadID), turnID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create turn dir: %w", err)
	}
	if event.EventID == "" {
		id, err := randomHex(16)
		if err != nil {
			return fmt.Errorf("generate browser event ID: %w", err)
		}
		event.EventID = id
	}
	event.StepIndex = step
	if event.RecordedAt == nil {
		now := time.Now().UTC()
		event.RecordedAt = &now
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal browser event: %w", err)
	}
	data = append(data, '\n')

	s.browserEventsMu.Lock()
	defer s.browserEventsMu.Unlock()

	f, err := os.OpenFile(s.browserEventsPath(threadID, turnID, step), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open browser event log: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("append browser event: %w", err)
	}
	return nil
}

// loadBrowserEvents loads append-only browser event records for a step.
func (s *Store) loadBrowserEvents(threadID, turnID string, step int) ([]thread.BrowserEvent, error) {
	f, err := os.Open(s.browserEventsPath(threadID, turnID, step))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open browser event log: %w", err)
	}
	defer f.Close()

	var events []thread.BrowserEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event thread.BrowserEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("scan browser event log: %w", err)
	}
	return events, nil
}

// loadAllBrowserEventEntries loads browser events across all persisted turns in
// a thread so stream reconnects can replay browser activity in the UI.
func (s *Store) loadAllBrowserEventEntries(threadID string) ([]thread.BrowserEventEntry, error) {
	turnsDir := s.turnsDir(threadID)
	entries, err := os.ReadDir(turnsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read turns dir: %w", err)
	}

	type turnDirInfo struct {
		name    string
		modTime time.Time
	}
	turnDirs := make([]turnDirInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat turn dir %s: %w", entry.Name(), err)
		}
		turnDirs = append(turnDirs, turnDirInfo{
			name:    entry.Name(),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(turnDirs, func(i, j int) bool {
		if turnDirs[i].modTime.Equal(turnDirs[j].modTime) {
			return turnDirs[i].name < turnDirs[j].name
		}
		return turnDirs[i].modTime.Before(turnDirs[j].modTime)
	})

	var out []thread.BrowserEventEntry
	for _, turnDir := range turnDirs {
		pattern := filepath.Join(turnsDir, turnDir.name, "step-*-browser.jsonl")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob browser event logs for turn %s: %w", turnDir.name, err)
		}
		sort.Strings(matches)
		for _, match := range matches {
			base := filepath.Base(match)
			var step int
			if _, err := fmt.Sscanf(base, "step-%03d-browser.jsonl", &step); err != nil {
				continue
			}
			events, err := s.loadBrowserEvents(threadID, turnDir.name, step)
			if err != nil {
				return nil, err
			}
			if len(events) == 0 {
				continue
			}
			assistantMessageID, err := s.assistantMessageID(threadID, turnDir.name, step)
			if err != nil {
				return nil, err
			}
			for _, event := range events {
				out = append(out, thread.BrowserEventEntry{
					TurnID:             turnDir.name,
					AssistantMessageID: assistantMessageID,
					StepIndex:          step,
					Event:              event,
				})
			}
		}
	}
	return out, nil
}

func (s *Store) assistantMessageID(threadID, turnID string, step int) (string, error) {
	data, err := os.ReadFile(s.stepResultPath(threadID, turnID, step))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read step result: %w", err)
	}
	var result struct {
		AssistantMessageID string `json:"assistantMessageId,omitempty"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("unmarshal step result: %w", err)
	}
	return strings.TrimSpace(result.AssistantMessageID), nil
}

// artifactURI returns the canonical artifacts:// URI for a thread-local
// browser artifact path.
func artifactURI(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "artifacts://"
	}
	return "artifacts://" + path
}

// saveBrowserScreenshot stores a screenshot file for one browser event and
// returns the persisted file reference relative to the thread directory.
func (s *Store) saveBrowserScreenshot(threadID, turnID string, step int, eventID string, png []byte) (thread.BrowserEventFile, error) {
	_ = turnID
	_ = step
	_ = eventID
	if len(png) == 0 {
		return thread.BrowserEventFile{}, fmt.Errorf("browser screenshot is empty")
	}
	dir := s.browserArtifactDir(threadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return thread.BrowserEventFile{}, fmt.Errorf("create browser artifact dir: %w", err)
	}
	sum := sha256.Sum256(png)
	filename := fmt.Sprintf("%x.png", sum)
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return thread.BrowserEventFile{}, fmt.Errorf("stat browser screenshot: %w", err)
		}
		if err := thread.WriteFileAtomic(path, png, 0o644); err != nil {
			return thread.BrowserEventFile{}, fmt.Errorf("write browser screenshot: %w", err)
		}
	}
	relPath, err := filepath.Rel(s.threadDir(threadID), path)
	if err != nil {
		return thread.BrowserEventFile{}, fmt.Errorf("make browser screenshot path relative: %w", err)
	}
	return thread.BrowserEventFile{
		Path:      filepath.ToSlash(relPath),
		URI:       artifactURI(relPath),
		MediaType: "image/png",
		Filename:  filename,
	}, nil
}
