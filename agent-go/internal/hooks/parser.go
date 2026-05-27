// Package hooks provides the hook system for post-completion evaluation.
// Hooks are executable scripts in .discobot/hooks/ with YAML front matter.
package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// HooksDir is the directory within the workspace where hooks are defined.
const HooksDir = ".discobot/hooks"

// HookType is the type of hook.
type HookType string

const (
	HookTypeSession   HookType = "session"
	HookTypeFile      HookType = "file"
	HookTypePreCommit HookType = "pre-commit"
)

// HookEngine is the runtime used to execute a hook.
type HookEngine string

const (
	HookEngineScript HookEngine = "script"
	HookEngineAI     HookEngine = "ai"
)

// Hook represents a discovered hook definition.
type Hook struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        HookType   `json:"type"`
	Description string     `json:"description,omitempty"`
	Engine      HookEngine `json:"engine"`
	Path        string     `json:"path"`
	RunAs       string     `json:"runAs"` // "root" or "user"
	Blocking    bool       `json:"blocking"`
	Pattern     string     `json:"pattern,omitempty"`
	NotifyLLM   bool       `json:"notifyLlm"`
	Prompt      string     `json:"prompt,omitempty"`
	Subagent    string     `json:"subagent,omitempty"`
}

// hookConfig is the raw front matter config parsed from a hook file.
type hookConfig struct {
	Name        string
	Type        string
	Description string
	Engine      string
	AI          bool
	RunAs       string
	Blocking    bool
	Pattern     string
	Subagent    string
	NotifyLLM   *bool // nil = default (true)
}

// Script extensions to strip when normalizing IDs.
var scriptExtensions = []string{
	".sh", ".bash", ".zsh", ".py", ".js", ".ts", ".rb", ".pl", ".php",
}

var nonAlphanumericRe = regexp.MustCompile(`[^a-z0-9_-]`)
var leadingTrailingHyphens = regexp.MustCompile(`^-+|-+$`)

// normalizeID converts a filename to a hook ID.
func normalizeID(filename string) string {
	id := filename
	lower := strings.ToLower(filename)
	for _, ext := range scriptExtensions {
		if strings.HasSuffix(lower, ext) {
			id = id[:len(id)-len(ext)]
			break
		}
	}
	id = strings.ReplaceAll(id, ".", "-")
	id = strings.ToLower(id)
	id = nonAlphanumericRe.ReplaceAllString(id, "")
	id = leadingTrailingHyphens.ReplaceAllString(id, "")
	return id
}

// delimiterStyle describes the front matter delimiter format.
type delimiterStyle struct {
	prefix    string // "" for plain, "#" for hash, "//" for slash
	delimiter string // "---", "#---", or "//---"
}

// detectDelimiter checks if a line is a front matter delimiter.
func detectDelimiter(line string) *delimiterStyle {
	trimmed := strings.TrimSpace(line)
	switch trimmed {
	case "---":
		return &delimiterStyle{prefix: "", delimiter: "---"}
	case "#---":
		return &delimiterStyle{prefix: "#", delimiter: "#---"}
	case "//---":
		return &delimiterStyle{prefix: "//", delimiter: "//---"}
	}
	return nil
}

// stripPrefix removes the comment prefix and leading whitespace from a content line.
func stripPrefix(line, prefix string) string {
	if prefix == "" {
		return line
	}
	_, content, found := strings.Cut(line, prefix)
	if !found {
		return line
	}
	return strings.TrimLeft(content, " \t")
}

// parseHookFrontMatter parses the front matter from a hook file's content.
func parseHookFrontMatter(content string) *hookConfig {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	// Check for shebang
	startLine := 0
	if strings.HasPrefix(lines[0], "#!") {
		startLine = 1
	}

	if len(lines) <= startLine {
		return nil
	}

	delim := detectDelimiter(lines[startLine])
	if delim == nil {
		return nil
	}

	// Find closing delimiter
	var yamlLines []string
	found := false
	for i := startLine + 1; i < len(lines); i++ {
		if detectDelimiter(lines[i]) != nil {
			found = true
			break
		}
		yamlLines = append(yamlLines, stripPrefix(lines[i], delim.prefix))
	}

	if !found {
		return nil
	}

	// Parse simple YAML key-value pairs
	cfg := &hookConfig{}
	for _, line := range yamlLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Remove quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		switch key {
		case "name":
			cfg.Name = value
		case "type":
			cfg.Type = value
		case "description":
			cfg.Description = value
		case "engine":
			cfg.Engine = value
		case "ai":
			cfg.AI = strings.EqualFold(value, "true") || strings.EqualFold(value, "yes") || value == "1"
		case "run_as":
			cfg.RunAs = value
		case "blocking":
			cfg.Blocking = strings.EqualFold(value, "true")
		case "pattern":
			cfg.Pattern = value
		case "subagent", "subagent_type", "sub_agent":
			cfg.Subagent = value
		case "notify_llm":
			v := strings.ToLower(value)
			b := v != "false" && v != "no" && v != "0"
			cfg.NotifyLLM = &b
		}
	}

	return cfg
}

func hookBodyAfterFrontMatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return ""
	}

	startLine := 0
	if strings.HasPrefix(lines[0], "#!") {
		startLine = 1
	}
	if len(lines) <= startLine || detectDelimiter(lines[startLine]) == nil {
		return strings.TrimSpace(content)
	}

	for i := startLine + 1; i < len(lines); i++ {
		if detectDelimiter(lines[i]) != nil {
			return strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
		}
	}

	return ""
}

// DiscoverHooks finds all valid hooks in the given hooks directory.
func DiscoverHooks(hooksDir string) ([]Hook, error) {
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var hooks []Hook
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(hooksDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		contentStr := string(content)
		cfg := parseHookFrontMatter(contentStr)
		if cfg == nil {
			continue
		}

		engine := HookEngineScript
		if strings.EqualFold(cfg.Engine, string(HookEngineAI)) || cfg.AI {
			engine = HookEngineAI
		}

		if engine == HookEngineScript {
			// Must be executable (Windows has no Unix-style execute bits, so all files qualify).
			if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
				continue
			}
			lines := strings.Split(contentStr, "\n")
			if len(lines) == 0 || !strings.HasPrefix(lines[0], "#!") {
				continue
			}
		}

		// Must have a valid type
		hookType := HookType(cfg.Type)
		switch hookType {
		case HookTypeSession, HookTypeFile, HookTypePreCommit:
			// valid
		default:
			continue
		}
		if engine == HookEngineAI && hookType == HookTypePreCommit {
			continue
		}
		prompt := ""
		if engine == HookEngineAI {
			prompt = hookBodyAfterFrontMatter(contentStr)
		}

		// File hooks must have a pattern
		if hookType == HookTypeFile && cfg.Pattern == "" {
			continue
		}

		id := normalizeID(entry.Name())
		name := cfg.Name
		if name == "" {
			name = id
		}
		runAs := cfg.RunAs
		if runAs == "" {
			runAs = "user"
		}
		notifyLLM := true
		if cfg.NotifyLLM != nil {
			notifyLLM = *cfg.NotifyLLM
		}

		hooks = append(hooks, Hook{
			ID:          id,
			Name:        name,
			Type:        hookType,
			Description: cfg.Description,
			Engine:      engine,
			Path:        filePath,
			RunAs:       runAs,
			Blocking:    cfg.Blocking,
			Pattern:     cfg.Pattern,
			NotifyLLM:   notifyLLM,
			Prompt:      prompt,
			Subagent:    cfg.Subagent,
		})
	}

	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Name < hooks[j].Name
	})

	return hooks, nil
}
