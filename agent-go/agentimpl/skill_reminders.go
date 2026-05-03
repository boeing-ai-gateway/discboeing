package agentimpl

import (
	"fmt"
	"strings"

	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func currentVisibleSkillLikeEntries(sessionCfg *sessionconfig.SessionConfig, tools []providers.ToolDefinition) []thread.CommunicatedSkillLikeEntry {
	if sessionCfg == nil || !hasNamedTool(tools, "Skill") {
		return nil
	}
	entries := make([]thread.CommunicatedSkillLikeEntry, 0, len(sessionCfg.Skills)+len(sessionCfg.Scripts))
	for _, skill := range sessionCfg.Skills {
		entries = append(entries, thread.CommunicatedSkillLikeEntry{
			Name:        skill.Name,
			Description: skill.Description,
		})
	}
	for _, script := range sessionCfg.Scripts {
		if !script.Visible {
			continue
		}
		entries = append(entries, thread.CommunicatedSkillLikeEntry{
			Name:        script.Name,
			Description: script.Description,
		})
	}
	return thread.NormalizeCommunicatedSkillLikeEntries(entries)
}

func buildSkillLikeChangeReminder(added, removed, changed []thread.CommunicatedSkillLikeEntry) string {
	if len(added) == 0 && len(removed) == 0 && len(changed) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("Available skills for the Skill tool changed since they were last communicated:\n\n")
	writeSkillLikeChangeSection(&b, "Added", added)
	writeSkillLikeChangeSection(&b, "Removed", removed)
	writeSkillLikeChangeSection(&b, "Changed descriptions", changed)
	b.WriteString("\nUse the Skill tool only for currently available skills listed here or in earlier skill reminders.")
	b.WriteString("\n</system-reminder>")
	return b.String()
}

func writeSkillLikeChangeSection(b *strings.Builder, title string, entries []thread.CommunicatedSkillLikeEntry) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", title)
	for _, entry := range entries {
		fmt.Fprintf(b, "- %s", entry.Name)
		if entry.Description != "" {
			fmt.Fprintf(b, ": %s", entry.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
