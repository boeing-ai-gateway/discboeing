package promptqueue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store owns prompt queue persistence independently from thread config.
type Store struct {
	baseDir string
}

// NewStore creates a prompt queue store rooted at the thread data directory.
func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func (s *Store) queuePath(threadID string) string {
	return filepath.Join(s.baseDir, threadID, "promptqueue.json")
}

// List returns all queued prompts for a thread.
func (s *Store) List(threadID string) ([]Prompt, error) {
	queue, err := s.load(threadID)
	if err != nil {
		return nil, err
	}
	return append([]Prompt{}, queue...), nil
}

// Append adds a queued prompt to the end of the thread queue.
func (s *Store) Append(threadID string, prompt Prompt) ([]Prompt, Prompt, error) {
	queue, err := s.load(threadID)
	if err != nil {
		return nil, Prompt{}, err
	}
	prompt = normalizePrompt(prompt)
	queue = append(append([]Prompt{}, queue...), prompt)
	if err := s.save(threadID, queue); err != nil {
		return nil, Prompt{}, err
	}
	return queue, prompt, nil
}

// Delete removes one queued prompt by ID.
func (s *Store) Delete(threadID, promptID string) ([]Prompt, bool, error) {
	queue, err := s.load(threadID)
	if err != nil {
		return nil, false, err
	}
	nextQueue := make([]Prompt, 0, len(queue))
	removed := false
	for _, prompt := range queue {
		if prompt.ID == promptID {
			removed = true
			continue
		}
		nextQueue = append(nextQueue, prompt)
	}
	if !removed {
		return queue, false, nil
	}
	if err := s.save(threadID, nextQueue); err != nil {
		return nil, false, err
	}
	return nextQueue, true, nil
}

// Pop removes and returns the next eligible queued prompt. Prompts with
// RunAfter in the future remain queued and are skipped.
func (s *Store) Pop(threadID string) ([]Prompt, *Prompt, error) {
	queue, err := s.load(threadID)
	if err != nil {
		return nil, nil, err
	}
	if len(queue) == 0 {
		return queue, nil, nil
	}

	now := time.Now().UTC()
	nextQueue := make([]Prompt, 0, len(queue)-1)
	var prompt *Prompt
	for _, queued := range queue {
		if prompt == nil && (queued.RunAfter.IsZero() || !queued.RunAfter.After(now)) {
			copyPrompt := queued
			prompt = &copyPrompt
			continue
		}
		nextQueue = append(nextQueue, queued)
	}
	if prompt == nil {
		return queue, nil, nil
	}
	if err := s.save(threadID, nextQueue); err != nil {
		return nil, nil, err
	}
	return nextQueue, prompt, nil
}

// Prepend pushes a prompt onto the front of the queue.
func (s *Store) Prepend(threadID string, prompt Prompt) ([]Prompt, error) {
	queue, err := s.load(threadID)
	if err != nil {
		return nil, err
	}
	prompt = normalizePrompt(prompt)
	queue = append([]Prompt{prompt}, queue...)
	if err := s.save(threadID, queue); err != nil {
		return nil, err
	}
	return queue, nil
}

// UpdatePrompt updates one queued prompt and optionally moves it within the
// queue. Position is clamped to the valid queue range.
func (s *Store) UpdatePrompt(threadID, promptID string, update Update) ([]Prompt, bool, error) {
	queue, err := s.load(threadID)
	if err != nil {
		return nil, false, err
	}

	nextQueue := make([]Prompt, 0, len(queue))
	var updatedPrompt Prompt
	updated := false
	for _, prompt := range queue {
		if prompt.ID != promptID {
			nextQueue = append(nextQueue, prompt)
			continue
		}
		if update.ClearRunAfter {
			prompt.RunAfter = time.Time{}
		} else if update.RunAfter != nil {
			prompt.RunAfter = update.RunAfter.UTC()
		}
		if update.Message != nil {
			prompt.Message = *update.Message
		}
		updatedPrompt = prompt
		updated = true
		if update.Position == nil {
			nextQueue = append(nextQueue, prompt)
		}
	}
	if !updated {
		return queue, false, nil
	}

	if update.Position != nil {
		position := min(max(*update.Position, 0), len(nextQueue))
		nextQueue = append(nextQueue, Prompt{})
		copy(nextQueue[position+1:], nextQueue[position:])
		nextQueue[position] = updatedPrompt
	}

	if err := s.save(threadID, nextQueue); err != nil {
		return nil, false, err
	}
	return nextQueue, true, nil
}

// NextRunAfter returns the earliest future RunAfter time, or nil if no queued
// prompt needs a future timer. If any prompt is eligible now, ready is true.
func (s *Store) NextRunAfter(threadID string, now time.Time) (runAfter *time.Time, ready bool, err error) {
	queue, err := s.load(threadID)
	if err != nil {
		return nil, false, err
	}
	for _, queued := range queue {
		if queued.RunAfter.IsZero() || !queued.RunAfter.After(now) {
			return nil, true, nil
		}
		if runAfter == nil || queued.RunAfter.Before(*runAfter) {
			value := queued.RunAfter
			runAfter = &value
		}
	}
	return runAfter, false, nil
}

func (s *Store) load(threadID string) ([]Prompt, error) {
	data, err := os.ReadFile(s.queuePath(threadID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read prompt queue: %w", err)
	}
	var queue []Prompt
	if err := json.Unmarshal(data, &queue); err != nil {
		return nil, fmt.Errorf("unmarshal prompt queue: %w", err)
	}
	return queue, nil
}

func (s *Store) save(threadID string, queue []Prompt) error {
	dir := filepath.Join(s.baseDir, threadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create thread dir: %w", err)
	}
	data, err := json.Marshal(queue)
	if err != nil {
		return fmt.Errorf("marshal prompt queue: %w", err)
	}
	return writeFileAtomic(s.queuePath(threadID), data, 0o644)
}

func normalizePrompt(prompt Prompt) Prompt {
	if prompt.ID == "" {
		prompt.ID = generateID()
	}
	if prompt.CreatedAt.IsZero() {
		prompt.CreatedAt = time.Now().UTC()
	}
	return prompt
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err = os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
