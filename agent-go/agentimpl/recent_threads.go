package agentimpl

import (
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/obot-platform/discobot/agent-go/sessionconfig"
)

func (a *DefaultAgent) formatRecentThreadsReminder(currentThreadID string) string {
	return sessionconfig.FormatRecentThreadsReminder(
		currentThreadID,
		readThreadScriptPath(),
		listThreadsScriptPath(),
		a.recentThreads(currentThreadID, 5),
	)
}

type recentThreadReference struct {
	sessionconfig.RecentThreadReference
	ActivityTime time.Time
}

func (a *DefaultAgent) recentThreads(currentThreadID string, limit int) []sessionconfig.RecentThreadReference {
	threadIDs, err := a.store.ListThreads()
	if err != nil {
		log.Printf("agent: warning: list recent threads: %v", err)
		return nil
	}

	threads := make([]recentThreadReference, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		if threadID == "" || threadID == currentThreadID {
			continue
		}

		threadDir := a.store.ThreadDir(threadID)
		activityTime, ok := latestThreadActivityTime(threadDir)
		if !ok {
			continue
		}

		threads = append(threads, recentThreadReference{
			RecentThreadReference: sessionconfig.RecentThreadReference{
				ThreadID: threadID,
				Label:    a.recentThreadLabel(threadID),
			},
			ActivityTime: activityTime,
		})
	}

	sort.Slice(threads, func(i, j int) bool {
		if !threads[i].ActivityTime.Equal(threads[j].ActivityTime) {
			return threads[i].ActivityTime.After(threads[j].ActivityTime)
		}
		return threads[i].ThreadID < threads[j].ThreadID
	})

	if limit > 0 && len(threads) > limit {
		threads = threads[:limit]
	}
	refs := make([]sessionconfig.RecentThreadReference, 0, len(threads))
	for _, threadRef := range threads {
		refs = append(refs, threadRef.RecentThreadReference)
	}
	return refs
}

func latestThreadActivityTime(threadDir string) (time.Time, bool) {
	var latest time.Time
	err := filepath.WalkDir(threadDir, func(_ string, d fs.DirEntry, err error) error {
		reachable := err == nil
		if !reachable {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			reachable = false
		}
		if !reachable {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	if err != nil || latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}

func (a *DefaultAgent) recentThreadLabel(threadID string) string {
	cfg, err := a.store.LoadConfig(threadID)
	if err == nil {
		if name := summarizeRecentThreadText(cfg.Name); name != "" {
			return name
		}
	}
	return threadID
}

func summarizeRecentThreadText(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= 120 {
		return text
	}
	return strings.TrimSpace(string(runes[:119])) + "…"
}
