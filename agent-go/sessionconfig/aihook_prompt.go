package sessionconfig

import (
	"fmt"
	"strings"
)

// AIHookPromptData contains the hook-specific values used to build an AI hook
// review prompt.
type AIHookPromptData struct {
	HookName        string
	Instructions    string
	Pattern         string
	ChangedFiles    []string
	ContextFilePath string
	Diff            string
	DiffTruncated   bool
}

// FormatAIHookPrompt builds the packaged user prompt for an AI-powered hook.
func FormatAIHookPrompt(data AIHookPromptData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are running the Discobot hook %q.\n\n", data.HookName)
	if strings.TrimSpace(data.Instructions) != "" {
		b.WriteString("Hook instructions:\n")
		b.WriteString(strings.TrimSpace(data.Instructions))
		b.WriteString("\n\n")
	}
	b.WriteString("Review what changed for this hook run. Respond with exactly one of:\n")
	b.WriteString("- `SUCCESS` if the changes satisfy the hook and you have no feedback.\n")
	b.WriteString("- `FEEDBACK: <actionable feedback>` if the changes need attention.\n\n")
	if data.ContextFilePath != "" {
		fmt.Fprintf(&b, "Full hook run context was written to `%s`. ", data.ContextFilePath)
		b.WriteString("Read that file if you need the complete changed-file list or diff")
		if data.DiffTruncated {
			b.WriteString(", including the omitted part of the truncated inline diff")
		}
		b.WriteString(".\n\n")
	}
	if data.Pattern != "" {
		fmt.Fprintf(&b, "Pattern: `%s`\n", data.Pattern)
	}
	if len(data.ChangedFiles) > 0 {
		b.WriteString("Changed files:\n")
		for _, file := range data.ChangedFiles {
			fmt.Fprintf(&b, "- %s\n", file)
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(data.Diff) != "" {
		b.WriteString("Diff:")
		if data.DiffTruncated {
			b.WriteString(" (truncated; read the context file above for the complete diff)")
		}
		b.WriteString("\n```diff\n")
		b.WriteString(data.Diff)
		if !strings.HasSuffix(data.Diff, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	return b.String()
}

// FormatAIHookContext builds the full context file content for an AI hook run.
func FormatAIHookContext(data AIHookPromptData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Discobot hook context: %s\n\n", data.HookName)
	if strings.TrimSpace(data.Instructions) != "" {
		b.WriteString("## Hook instructions\n\n")
		b.WriteString(strings.TrimSpace(data.Instructions))
		b.WriteString("\n\n")
	}
	if data.Pattern != "" {
		b.WriteString("## Pattern\n\n")
		fmt.Fprintf(&b, "`%s`\n\n", data.Pattern)
	}
	if len(data.ChangedFiles) > 0 {
		b.WriteString("## Changed files\n\n")
		for _, file := range data.ChangedFiles {
			fmt.Fprintf(&b, "- %s\n", file)
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(data.Diff) != "" {
		b.WriteString("## Diff\n\n```diff\n")
		b.WriteString(data.Diff)
		if !strings.HasSuffix(data.Diff, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	return b.String()
}
