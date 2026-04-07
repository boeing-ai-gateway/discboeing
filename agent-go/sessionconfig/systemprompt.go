package sessionconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type systemConfig struct {
	PromptBody   string
	AllowedTools []string
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
	fm, body, err := parseFrontmatter(content)
	if err != nil {
		return systemConfig{}, fmt.Errorf("parse frontmatter in %s: %w", source, err)
	}

	allowedTools, err := frontmatterStringList(fm, "allowedTools")
	if err != nil {
		return systemConfig{}, fmt.Errorf("%s: %w", source, err)
	}
	if len(allowedTools) == 0 {
		return systemConfig{}, fmt.Errorf("%s: allowedTools is required", source)
	}

	promptBody := strings.TrimSpace(body)
	if promptBody == "" {
		return systemConfig{}, fmt.Errorf("%s: system prompt body is empty", source)
	}

	return systemConfig{
		PromptBody:   promptBody,
		AllowedTools: allowedTools,
	}, nil
}

func frontmatterStringList(fm map[string]any, key string) ([]string, error) {
	if fm == nil {
		return nil, nil
	}
	raw, ok := fm[key]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a list of strings", key)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a list of strings", key)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("%s cannot contain empty strings", key)
		}
		result = append(result, value)
	}
	return result, nil
}
