package sessionconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	fmparser "github.com/obot-platform/discobot/agent-go/frontmatter"
)

type CredentialReminderUse struct {
	ID          string
	Description string
}

type CredentialReminderBinding struct {
	CredentialID string
	EnvVar       string
	Uses         []CredentialReminderUse
}

type systemConfig struct {
	PromptBody   string
	AllowedTools []string
}

const credentialUseAuthorizerSystemPrompt = "You validate whether a Bash command is within one or more previously approved credential uses. Be strict and reject on ambiguity. Allow only when the command is clearly consistent with at least one approved use description and does not appear to perform broader credentialed actions than necessary. Evaluate the approved uses together. Respond with exactly one minified JSON object matching this shape and no surrounding text: {\"allow\":true|false,\"reason\":\"short reason\"}."

func CredentialUseAuthorizerSystemPrompt() string {
	return credentialUseAuthorizerSystemPrompt
}

func FormatCredentialChangeReminder(
	added []CredentialReminderBinding,
	removed []CredentialReminderBinding,
) string {
	if len(added) == 0 && len(removed) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("Credential ID update: the set of session-scoped credential and approved use IDs available to the agent has changed since the last report.\n")
	b.WriteString("Use the current credentialId/useId values below in future tool calls.\n")

	writeSection := func(title string, bindings []CredentialReminderBinding) {
		if len(bindings) == 0 {
			return
		}
		b.WriteString("\n")
		b.WriteString(title)
		b.WriteString(":\n")
		for _, binding := range bindings {
			fmt.Fprintf(&b, "- %s: credentialId=%s\n", binding.EnvVar, binding.CredentialID)
			for _, use := range binding.Uses {
				fmt.Fprintf(&b, "  - useId=%s", use.ID)
				if use.Description != "" {
					fmt.Fprintf(&b, ": %s", use.Description)
				}
				b.WriteString("\n")
			}
		}
	}

	writeSection("Added", added)
	writeSection("Removed", removed)
	b.WriteString("</system-reminder>")
	return b.String()
}

func FormatModeChangeReminder(planMode bool) string {
	if planMode {
		return "<system-reminder>\nMode update: the current mode is now plan. This change was triggered by the current prompt request.\n</system-reminder>"
	}
	return "<system-reminder>\nMode update: the current mode is now build. Plan mode has been exited. This change was triggered by the current prompt request.\n</system-reminder>"
}

// FormatUserInstructions formats discovered instruction entries into a
// <system-reminder> tagged block. Returns empty string if entries is empty.
func FormatUserInstructions(entries []InstructionEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("Codebase and user instructions are shown below. Be sure to adhere to these instructions. ")
	b.WriteString("IMPORTANT: These instructions OVERRIDE any default behavior and you MUST follow them exactly as written.\n")

	for _, entry := range entries {
		b.WriteString("\n")
		fmt.Fprintf(&b, "Contents of %s (%s):\n\n", entry.Path, entry.Description)
		b.WriteString(entry.Content)
		b.WriteString("\n")
	}

	b.WriteString("</system-reminder>")
	return b.String()
}

// defaultSystemPrompt returns the embedded base system prompt body.
func defaultSystemPrompt() string {
	cfg, err := defaultSystemConfig()
	if err != nil {
		panic("sessionconfig: load default system prompt: " + err.Error())
	}
	return cfg.PromptBody
}

func defaultSystemConfig() (systemConfig, error) {
	data, err := embeddedConfigFiles.ReadFile("SYSTEM.md")
	if err != nil {
		return systemConfig{}, fmt.Errorf("read embedded SYSTEM.md: %w", err)
	}
	return parseSystemConfig(string(data), "SYSTEM.md")
}

func loadSystemConfig(projectRoot string) (systemConfig, error) {
	overridePath := filepath.Join(projectRoot, ".discobot", "SYSTEM.md")
	data, err := os.ReadFile(overridePath)
	if err == nil {
		return parseSystemConfig(string(data), overridePath)
	}
	if err != nil && !os.IsNotExist(err) {
		return systemConfig{}, fmt.Errorf("read %s: %w", overridePath, err)
	}
	return defaultSystemConfig()
}

func parseSystemConfig(content, source string) (systemConfig, error) {
	doc, err := fmparser.ParseMarkdown[systemPromptFrontmatter](content)
	if err != nil {
		return systemConfig{}, fmt.Errorf("parse frontmatter in %s: %w", source, err)
	}
	allowedTools := doc.Metadata.AllowedTools
	if len(allowedTools) == 0 {
		return systemConfig{}, fmt.Errorf("%s: allowedTools is required", source)
	}

	promptBody := strings.TrimSpace(doc.Body)
	if promptBody == "" {
		return systemConfig{}, fmt.Errorf("%s: system prompt body is empty", source)
	}

	return systemConfig{
		PromptBody:   promptBody,
		AllowedTools: allowedTools,
	}, nil
}
