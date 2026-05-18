package toolrenderers

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type OptimizedToolView struct {
	ToolName    string
	Input       string
	Output      string
	ErrorText   string
	State       string
	Title       string
	Queued      bool
	ForceRaw    bool
	DefaultOpen bool
}

func optimizedToolOpen(view OptimizedToolView) bool {
	return view.DefaultOpen || view.ToolName == "AskUserQuestion" || ((view.ToolName == "RequestCommitPull" || view.ToolName == "RequestUserCredential") && view.State == "approval-requested")
}

func optimizedAlwaysExpanded(view OptimizedToolView) bool {
	return view.ToolName == "AskUserQuestion" || ((view.ToolName == "RequestCommitPull" || view.ToolName == "RequestUserCredential") && view.State == "approval-requested")
}

func hasOptimizedRenderer(toolName string) bool {
	switch toolName {
	case "AskUserQuestion", "Bash", "PowerShell", "Read", "read", "Edit", "Glob", "Grep", "WebFetch", "WebSearch", "Write", "apply_patch", "RequestCommitPull", "RequestUserCredential", "Skill", "Task", "TodoWrite":
		return true
	default:
		return false
	}
}

func optimizedToolTitle(view OptimizedToolView) string {
	if view.Title != "" {
		return view.Title
	}
	input := map[string]any{}
	_ = json.Unmarshal([]byte(view.Input), &input)
	switch view.ToolName {
	case "apply_patch":
		if title := summarizeApplyPatchTitle(view.Input); title != "" {
			return title
		}
		return "Apply patch"
	case "Bash", "PowerShell":
		if command, ok := input["command"].(string); ok && command != "" {
			return "Run: " + truncateTitle(command, 60)
		}
	case "Read", "read", "Write", "Edit":
		if filePath, ok := input["file_path"].(string); ok && filePath != "" {
			name := filepath.Base(filePath)
			if view.ToolName == "read" {
				return "Read: " + name
			}
			return view.ToolName + ": " + name
		}
	case "Grep", "Glob":
		if pattern, ok := input["pattern"].(string); ok && pattern != "" {
			prefix := "Find"
			if view.ToolName == "Grep" {
				prefix = "Search"
			}
			return prefix + ": " + truncateTitle(pattern, 50)
		}
	case "WebSearch":
		if query, ok := input["query"].(string); ok && query != "" {
			return "Search: " + truncateTitle(query, 50)
		}
	case "WebFetch":
		if url, ok := input["url"].(string); ok && url != "" {
			return "Fetch: " + truncateTitle(url, 50)
		}
	case "TodoWrite":
		if todos, ok := input["todos"].([]any); ok {
			return fmt.Sprintf("Track: %d %s", len(todos), plural(len(todos), "task", "tasks"))
		}
	case "Task":
		if description, ok := input["description"].(string); ok && description != "" {
			return "Launch: " + truncateTitle(description, 50)
		}
	case "Skill":
		if skill, ok := input["skill"].(string); ok && skill != "" {
			return "Run: " + skill
		}
	case "AskUserQuestion":
		return "Agent Question"
	case "RequestUserCredential":
		return "Credential Request"
	case "RequestCommitPull":
		return "Pull Sandbox Commit"
	}
	return ""
}

func truncateTitle(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
