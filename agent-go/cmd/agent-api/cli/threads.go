package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/clisession"
	"github.com/obot-platform/discobot/agent-go/internal/config"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// threadSummary holds display metadata for a single thread.
type threadSummary struct {
	id      string
	modTime time.Time
	preview string
	pending bool
}

func normalizeCWD(path string) string {
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return path
}

func threadExists(ctx context.Context, session clisession.Session, threadID string) bool {
	if strings.TrimSpace(threadID) == "" {
		return false
	}
	_, err := session.GetThread(ctx, threadID)
	return err == nil
}

func startupCommandHints(ctx context.Context, session clisession.Session, threadID string) (showResume bool, showHistory bool) {
	if threadExists(ctx, session, threadID) {
		showHistory = true
	}

	threads, err := session.ListThreads(ctx)
	if err != nil || len(threads) == 0 {
		return false, showHistory
	}

	targetCWD := normalizeCWD(session.WorkspaceRoot())
	matchingCWD := 0
	currentMatchesCWD := false
	for _, item := range threads {
		threadCWD := normalizeCWD(item.CWD)
		if threadCWD == "" || threadCWD != targetCWD {
			continue
		}
		matchingCWD++
		if item.ID == threadID {
			currentMatchesCWD = true
		}
	}

	if currentMatchesCWD {
		showResume = matchingCWD > 1
	} else {
		showResume = matchingCWD > 0
	}
	return showResume, showHistory
}

func selectInitialThreadID(_ *thread.Store, cfg *config.Config, forceNew bool, resumeID string) string {
	if forceNew {
		return "thread-" + agent.GenerateID()
	}
	if strings.TrimSpace(resumeID) != "" {
		return resumeID
	}
	if cfg.SessionID != "" && cfg.SessionID != "default" {
		return cfg.SessionID
	}
	return "thread-" + agent.GenerateID()
}

func handleResumeCommand(ctx context.Context, session clisession.Session, currentThreadID string) string {
	threads, err := session.ListThreads(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing threads: %v\n", err)
		return currentThreadID
	}
	if len(threads) == 0 {
		fmt.Fprintln(os.Stderr, "No threads found.")
		return currentThreadID
	}

	targetCWD := normalizeCWD(session.WorkspaceRoot())
	summaries := make([]threadSummary, 0, len(threads))
	otherDirCounts := map[string]int{}
	otherUnknown := 0
	threadsDir := filepath.Join(os.Getenv("HOME"), ".discobot", "threads")

	for _, item := range threads {
		threadCWD := normalizeCWD(item.CWD)
		if threadCWD != "" && threadCWD != targetCWD {
			otherDirCounts[threadCWD]++
			continue
		}
		if threadCWD == "" {
			otherUnknown++
		}

		s := threadSummary{id: item.ID, pending: item.PendingQuestion}
		if fi, err := os.Stat(filepath.Join(threadsDir, item.ID)); err == nil {
			s.modTime = fi.ModTime()
		}
		if msgs, err := session.Messages(ctx, item.ID); err == nil {
			s.preview = lastUserPreview(msgs)
		}
		summaries = append(summaries, s)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].modTime.After(summaries[j].modTime)
	})

	if len(summaries) == 0 {
		fmt.Fprintf(os.Stderr, "No threads found for current directory: %s\n", targetCWD)
	} else {
		fmt.Fprintf(os.Stderr, "\nAvailable threads for %s:\n", targetCWD)
		for i, s := range summaries {
			marker := ""
			if s.id == currentThreadID {
				marker = " (current)"
			}
			if s.pending {
				marker += " [pending approval]"
			}
			age := formatAge(time.Since(s.modTime))
			fmt.Fprintf(os.Stderr, "  %d. %s  %s%s\n", i+1, s.id, age, marker)
			if s.preview != "" {
				fmt.Fprintf(os.Stderr, "     \"%s\"\n", s.preview)
			}
		}
	}

	if len(otherDirCounts) > 0 {
		fmt.Fprintln(os.Stderr)
		totalOther := 0
		for _, n := range otherDirCounts {
			totalOther += n
		}
		fmt.Fprintf(os.Stderr, "%d thread(s) belong to other directories.\n", totalOther)
		dirs := make([]string, 0, len(otherDirCounts))
		for dir := range otherDirCounts {
			dirs = append(dirs, dir)
		}
		sort.Strings(dirs)
		for _, dir := range dirs {
			fmt.Fprintf(os.Stderr, "  - %s (%d)\n", dir, otherDirCounts[dir])
		}
		fmt.Fprintln(os.Stderr, "To resume those threads, cd into that directory and run /resume.")
	}
	if otherUnknown > 0 {
		fmt.Fprintf(os.Stderr, "\nIncluding %d legacy thread(s) with unknown cwd.\n", otherUnknown)
	}
	if len(summaries) == 0 {
		return currentThreadID
	}

	for {
		input, err := readLine("\nSelect thread (number, or Enter to cancel): ", nil)
		if err != nil {
			return currentThreadID
		}
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			return currentThreadID
		}
		n, err := strconv.Atoi(input)
		if err != nil || n < 1 || n > len(summaries) {
			fmt.Fprintf(os.Stderr, "Please enter a number between 1 and %d.\n", len(summaries))
			continue
		}
		return summaries[n-1].id
	}
}

func lastUserPreview(messages []message.UIMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "user" && !isInjectedMessageID(msg.ID) {
			if text := extractMessageText(msg.Parts); text != "" && !isInjectedText(text) {
				return abbreviate(text, 80)
			}
		}
	}
	return ""
}

func isInjectedMessageID(id string) bool {
	return strings.HasPrefix(id, "system-") ||
		strings.HasPrefix(id, "instructions-") ||
		strings.HasPrefix(id, "skills-")
}

func isInjectedText(text string) bool {
	return strings.HasPrefix(text, "<system-reminder>") ||
		strings.HasPrefix(text, "<skills-reminder>")
}

func extractMessageText(parts []message.UIPart) string {
	for _, p := range parts {
		if tp, ok := p.(message.UITextPart); ok && tp.Text != "" {
			return tp.Text
		}
	}
	return ""
}

func extractAllText(parts []message.UIPart) string {
	var b strings.Builder
	for _, p := range parts {
		if tp, ok := p.(message.UITextPart); ok {
			b.WriteString(tp.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func printThreadHistory(ctx context.Context, session clisession.Session, threadID string) bool {
	history, err := session.Messages(ctx, threadID)
	if err != nil || len(history) == 0 {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading thread history: %v\n", err)
		}
		return false
	}

	printed := false
	for _, entry := range history {
		if isInjectedMessageID(entry.ID) {
			continue
		}
		if entry.Role != "user" && entry.Role != "assistant" {
			continue
		}
		text := extractAllText(entry.Parts)
		if text == "" {
			continue
		}
		fmt.Fprintln(os.Stdout)
		if entry.Role == "user" {
			if !noColor && term.IsTerminal(int(os.Stdout.Fd())) {
				fmt.Fprint(os.Stdout, "\033[1;36m>\033[0m ")
			} else {
				fmt.Fprint(os.Stdout, "> ")
			}
		}
		md := newMarkdownRenderer(os.Stdout, term.IsTerminal(int(os.Stdout.Fd())), !noColor)
		md.WriteText(text)
		md.Finish()
		fmt.Fprintln(os.Stdout)
		printed = true
	}
	if printed {
		fmt.Fprintln(os.Stdout)
	}
	return printed
}

func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
