package thread

import (
	"context"
	"errors"
	"log"
	"strings"
)

// ErrorStore is the thread metadata persistence boundary needed for turn error
// bookkeeping.
type ErrorStore interface {
	GetThreadInfo(threadID string) (Info, error)
	UpdateThreadInfo(threadID string, req UpdateThreadRequest) (Info, error)
}

// ClearError clears a persisted thread error when a new turn starts.
func ClearError(store ErrorStore, threadID string) (Info, bool) {
	info, err := store.GetThreadInfo(threadID)
	if err != nil {
		log.Printf("thread error: failed to load config for %s: %v", threadID, err)
		return Info{}, false
	}
	if strings.TrimSpace(info.ErrorMessage) == "" {
		return Info{}, false
	}
	info, err = store.UpdateThreadInfo(threadID, UpdateThreadRequest{ClearErrorMessage: true})
	if err != nil {
		log.Printf("thread error: failed to clear config for %s: %v", threadID, err)
		return Info{}, false
	}
	return info, true
}

// PersistError saves a non-cancellation turn error on the thread.
func PersistError(store ErrorStore, threadID string, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}

	errMessage := strings.TrimSpace(err.Error())
	if errMessage == "" {
		return false
	}
	if _, saveErr := store.UpdateThreadInfo(threadID, UpdateThreadRequest{ErrorMessage: &errMessage}); saveErr != nil {
		log.Printf("thread error: failed to save config for %s: %v", threadID, saveErr)
		return false
	}
	return true
}
