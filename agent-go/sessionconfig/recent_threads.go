package sessionconfig

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// RuntimeEnvironmentSnapshot describes the runtime details surfaced at startup.
type RuntimeEnvironmentSnapshot struct {
	CurrentWorkingDirectory string
	CurrentModel            string
	CurrentDateTime         time.Time
	GitState                string
}

// FormatRuntimeEnvironmentReminder formats the runtime snapshot as a
// <system-reminder> block.
func FormatRuntimeEnvironmentReminder(snapshot RuntimeEnvironmentSnapshot) string {
	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("Runtime environment snapshot:\n")
	fmt.Fprintf(&b, "- Current working directory: %s\n", snapshot.CurrentWorkingDirectory)
	fmt.Fprintf(&b, "- OS/platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "- Current date/time: %s\n", snapshot.CurrentDateTime.Format(time.RFC3339))
	if snapshot.CurrentModel != "" {
		fmt.Fprintf(&b, "- Current model: %s\n", snapshot.CurrentModel)
	}
	if runtime.GOOS == "windows" {
		fmt.Fprintf(&b, "- Shell tool note: the %q-configured command tool is exposed in this runtime as %q and runs commands with PowerShell.\n", "Bash", windowsShellToolName)
	}
	fmt.Fprintf(&b, "- Git state (captured at the current time of this reminder; this may change throughout the conversation): %s\n", snapshot.GitState)
	b.WriteString("</system-reminder>")
	return b.String()
}

// RecentThreadReference describes one prior thread the model can inspect on disk.
type RecentThreadReference struct {
	ThreadID string
	Label    string
}

// FormatWorkspaceChangeReminder formats a changed-file snapshot as a
// <system-reminder> block. Returns empty string if files is empty.
func FormatWorkspaceChangeReminder(fullListPath string, files []string, limit int) string {
	if len(files) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = len(files)
	}
	if limit > len(files) {
		limit = len(files)
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("The following files have changed in the workspace since the end of the last turn:\n")
	for _, file := range files[:limit] {
		fmt.Fprintf(&b, "- %s\n", file)
	}
	if remaining := len(files) - limit; remaining > 0 {
		fmt.Fprintf(&b, "and %d more, read file %s for the full list.\n", remaining, fullListPath)
	}
	b.WriteString("</system-reminder>")
	return b.String()
}

// FormatRecentThreadsReminder formats recent thread references as a
// <system-reminder> block. Returns empty string if refs is empty.
func FormatRecentThreadsReminder(currentThreadID, readerScriptPath, listScriptPath string, refs []RecentThreadReference) string {
	if len(refs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("Recent threads from this session are available if you need to read prior conversation context.\n")
	if strings.TrimSpace(currentThreadID) != "" {
		fmt.Fprintf(&b, "Current thread ID: %s\n", currentThreadID)
	}
	if strings.TrimSpace(readerScriptPath) != "" {
		fmt.Fprintf(&b, "Use %s <thread-id> to print a thread transcript.\n", readerScriptPath)
	}
	if strings.TrimSpace(listScriptPath) != "" {
		fmt.Fprintf(&b, "Use %s to list available thread IDs and names. It skips the current thread automatically when DISCOBOT_SESSION_ID is set.\n", listScriptPath)
	}
	b.WriteString("\n")
	for _, ref := range refs {
		fmt.Fprintf(&b, "- %s (thread ID: %s)\n", ref.Label, ref.ThreadID)
	}
	b.WriteString("</system-reminder>")
	return b.String()
}
