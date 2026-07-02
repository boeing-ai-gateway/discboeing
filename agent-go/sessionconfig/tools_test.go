package sessionconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
)

func TestBuiltinTools_AllDefined(t *testing.T) {
	toolMap, err := builtinToolDefinitions()
	if err != nil {
		t.Fatal(err)
	}

	expectedNames := []string{
		// Execution
		"Bash",
		// File operations
		"Read", "Write", "Edit", "apply_patch",
		// Search
		"Glob", "Grep",
		// Web
		"WebFetch", "WebSearch",
		// Agent orchestration
		"Task",
		// Task management
		"TodoWrite",
		// Background tasks
		"TaskOutput", "TaskStop",
		// User interaction
		"AskUserQuestion", "RequestUserCredential", "RequestCommitPull",
		// Skills
		"Skill",
		// Phase transitions
		"ReadyForReview",
	}

	if len(toolMap) != len(expectedNames) {
		t.Fatalf("expected %d tools, got %d", len(expectedNames), len(toolMap))
	}

	for _, name := range expectedNames {
		if _, ok := toolMap[name]; !ok {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestBuiltinTool_LoadsReadyForReview(t *testing.T) {
	tool, ok := BuiltinTool("ReadyForReview")
	if !ok {
		t.Fatal("expected ReadyForReview builtin tool")
	}
	if tool.Name != "ReadyForReview" {
		t.Fatalf("tool name = %q, want ReadyForReview", tool.Name)
	}
}

func TestBuiltinTools_DefaultSelectionMatchesSystemConfig(t *testing.T) {
	cfg, err := defaultSystemConfig()
	if err != nil {
		t.Fatal(err)
	}
	tools := BuiltinTools("")
	if len(tools) != len(cfg.AllowedTools) {
		t.Fatalf("expected %d tools, got %d", len(cfg.AllowedTools), len(tools))
	}
	for i, tool := range tools {
		if tool.Name != cfg.AllowedTools[i] {
			t.Errorf("tool[%d] = %q, want %q", i, tool.Name, cfg.AllowedTools[i])
		}
	}
}

func TestBuiltinTools_ValidSchemas(t *testing.T) {
	tools := BuiltinTools("")

	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("tool %s has empty description", tool.Name)
		}

		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("tool %s has invalid JSON schema: %v", tool.Name, err)
			continue
		}

		if schema["type"] != "object" {
			t.Errorf("tool %s schema type = %v, want object", tool.Name, schema["type"])
		}
		if _, ok := schema["properties"]; !ok {
			t.Errorf("tool %s schema missing properties", tool.Name)
		}
	}
}

func TestBuiltinTools_BashSchema(t *testing.T) {
	schema := findToolSchema(t, "Bash")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"command", "description", "timeout", "run_in_background", "credentialUses"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Bash schema missing '%s' property", field)
		}
	}

	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "command" {
		t.Errorf("Bash required = %v, want [command]", required)
	}
}

func TestBuiltinTools_ReadSchema(t *testing.T) {
	schema := findToolSchema(t, "Read")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"file_path", "offset", "limit", "pages"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Read schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_EditSchema(t *testing.T) {
	schema := findToolSchema(t, "Edit")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"file_path", "old_string", "new_string", "replace_all"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Edit schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_ApplyPatchSchema(t *testing.T) {
	schema := findToolSchema(t, "apply_patch")
	props := schema["properties"].(map[string]any)

	if _, ok := props["input"]; !ok {
		t.Error("apply_patch schema missing 'input' property")
	}

	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "input" {
		t.Errorf("apply_patch required = %v, want [input]", required)
	}
}

func TestBuiltinTools_ApplyPatchCustomFormat(t *testing.T) {
	tools := BuiltinTools("")
	var applyPatch *providers.ToolDefinition
	for i := range tools {
		if tools[i].Name == "apply_patch" {
			applyPatch = &tools[i]
			break
		}
	}
	if applyPatch == nil {
		t.Fatal("apply_patch tool not found")
	}
	if applyPatch.Type != "custom" {
		t.Fatalf("apply_patch type = %q, want custom", applyPatch.Type)
	}
	if applyPatch.Format == nil {
		t.Fatal("apply_patch format is nil")
	}
	if applyPatch.Format.Type != "grammar" {
		t.Fatalf("apply_patch format.type = %q, want grammar", applyPatch.Format.Type)
	}
	if applyPatch.Format.Syntax != "lark" {
		t.Fatalf("apply_patch format.syntax = %q, want lark", applyPatch.Format.Syntax)
	}
	if strings.TrimSpace(applyPatch.Format.Definition) == "" {
		t.Fatal("apply_patch format.definition is empty")
	}
}

func TestBuiltinTools_GrepSchema(t *testing.T) {
	schema := findToolSchema(t, "Grep")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{
		"pattern", "path", "glob", "type", "output_mode",
		"-i", "-n", "-A", "-B", "-C", "context",
		"multiline", "head_limit", "offset",
	} {
		if _, ok := props[field]; !ok {
			t.Errorf("Grep schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_TaskSchema(t *testing.T) {
	schema := findToolSchema(t, "Task")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"description", "prompt", "subagent_type", "resume", "run_in_background"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Task schema missing '%s' property", field)
		}
	}
	for _, field := range []string{"allowed_tools", "model", "max_turns"} {
		if _, ok := props[field]; ok {
			t.Errorf("Task schema should not expose '%s' property", field)
		}
	}

	required := schema["required"].([]any)
	reqSet := make(map[string]bool)
	for _, r := range required {
		reqSet[r.(string)] = true
	}
	for _, r := range []string{"description", "prompt", "subagent_type"} {
		if !reqSet[r] {
			t.Errorf("Task missing required field: %s", r)
		}
	}
}

func TestBuiltinTools_TodoWriteSchema(t *testing.T) {
	schema := findToolSchema(t, "TodoWrite")
	props := schema["properties"].(map[string]any)

	if _, ok := props["todos"]; !ok {
		t.Error("TodoWrite schema missing 'todos' property")
	}

	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "todos" {
		t.Errorf("TodoWrite required = %v, want [todos]", required)
	}
}

func TestBuiltinTools_AskUserQuestionSchema(t *testing.T) {
	schema := findToolSchema(t, "AskUserQuestion")
	props := schema["properties"].(map[string]any)

	if _, ok := props["questions"]; !ok {
		t.Error("AskUserQuestion schema missing 'questions' property")
	}
}

func TestBuiltinTools_RequestUserCredentialSchema(t *testing.T) {
	schema := findToolSchema(t, "RequestUserCredential")
	props := schema["properties"].(map[string]any)

	credentials, ok := props["credentials"]
	if !ok {
		t.Error("RequestUserCredential schema missing 'credentials' property")
		return
	}
	items := credentials.(map[string]any)["items"].(map[string]any)
	itemProps := items["properties"].(map[string]any)
	approvedUses := itemProps["approvedUses"].(map[string]any)
	if approvedUses["type"] != "array" {
		t.Errorf("RequestUserCredential approvedUses type = %v, want array", approvedUses["type"])
	}
}
func TestBuiltinTools_WebSearchSchema(t *testing.T) {
	schema := findToolSchema(t, "WebSearch")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"query", "allowed_domains", "blocked_domains"} {
		if _, ok := props[field]; !ok {
			t.Errorf("WebSearch schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_SkillSchema(t *testing.T) {
	schema := findToolSchema(t, "Skill")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"skill", "args"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Skill schema missing '%s' property", field)
		}
	}

	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "skill" {
		t.Errorf("Skill required = %v, want [skill]", required)
	}
}

func TestBuiltinTools_WebSearchDescriptionMentionsSources(t *testing.T) {
	tools := BuiltinTools("")

	var description string
	for _, tool := range tools {
		if tool.Name == "WebSearch" {
			description = tool.Description
			break
		}
	}
	if description == "" {
		t.Fatal("WebSearch tool not found")
	}
	if !strings.Contains(description, "Sources:") {
		t.Error("WebSearch description should mention the required Sources section")
	}
}

func TestAdaptToolsForRuntime_WindowsAppliesEmbeddedOverride(t *testing.T) {
	tools := AdaptToolsForRuntime("windows", []providers.ToolDefinition{findTool(t, "Bash")})
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != windowsShellToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, windowsShellToolName)
	}
	if !strings.Contains(tools[0].Description, "Run a PowerShell command and return its output.") {
		t.Fatalf("expected Windows override description, got %q", tools[0].Description)
	}
	if strings.Contains(tools[0].Description, "Run a bash command and return its output.") {
		t.Fatalf("did not expect bash wording after Windows override, got %q", tools[0].Description)
	}
}

func TestAdaptToolsForRuntime_AppendsBashSudoGuidanceWithDiscboeingSudo(t *testing.T) {
	oldStatDiscboeingRealSudo := statDiscboeingRealSudo
	t.Cleanup(func() { statDiscboeingRealSudo = oldStatDiscboeingRealSudo })

	statDiscboeingRealSudo = func() bool { return false }
	tools := AdaptToolsForRuntime("linux", []providers.ToolDefinition{findTool(t, "Bash")})
	if strings.Contains(tools[0].Description, sudoGuidance) {
		t.Fatalf("did not expect sudo guidance without %s", discboeingRealSudoPath)
	}

	statDiscboeingRealSudo = func() bool { return true }
	tools = AdaptToolsForRuntime("linux", []providers.ToolDefinition{findTool(t, "Bash")})
	if !strings.Contains(tools[0].Description, sudoGuidance) {
		t.Fatalf("expected sudo guidance with %s", discboeingRealSudoPath)
	}
}

func TestAdaptToolsForRuntime_OverlayMergesNestedMaps(t *testing.T) {
	merged := mergeToolDefinitionMaps(
		map[string]any{
			"description": "base",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "base description",
					},
					"timeout": map[string]any{
						"type": "number",
					},
				},
			},
		},
		map[string]any{
			"description": "override",
			"inputSchema": map[string]any{
				"properties": map[string]any{
					"command": map[string]any{
						"description": "override description",
					},
				},
			},
		},
	)

	if merged["description"] != "override" {
		t.Fatalf("description = %v, want override", merged["description"])
	}
	schema := merged["inputSchema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	command := props["command"].(map[string]any)
	if command["type"] != "string" {
		t.Fatalf("command.type = %v, want string", command["type"])
	}
	if command["description"] != "override description" {
		t.Fatalf("command.description = %v, want override description", command["description"])
	}
	if _, ok := props["timeout"]; !ok {
		t.Fatal("expected timeout property to be preserved during overlay merge")
	}
}

func findToolSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	tool := findTool(t, name)
	var schema map[string]any
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		t.Fatalf("%s schema: %v", name, err)
	}
	return schema
}

func findTool(t *testing.T, name string) providers.ToolDefinition {
	t.Helper()
	tools := BuiltinTools("")
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("%s tool not found", name)
	return providers.ToolDefinition{}
}

func TestFormatToolAvailabilityChangeReminder(t *testing.T) {
	got := FormatToolAvailabilityChangeReminder(
		[]providers.ToolDefinition{{Name: "Read"}, {Name: "Write"}},
		[]providers.ToolDefinition{{Name: "Read"}, {Name: "server__search"}},
	)
	if !strings.Contains(got, "<system-reminder>") {
		t.Fatalf("expected system reminder, got %q", got)
	}
	if !strings.Contains(got, "Newly available tools: server__search") {
		t.Fatalf("expected added tool in reminder, got %q", got)
	}
	if !strings.Contains(got, "No longer available tools: Write") {
		t.Fatalf("expected removed tool in reminder, got %q", got)
	}
}

func TestFormatToolAvailabilityChangeReminder_Unchanged(t *testing.T) {
	got := FormatToolAvailabilityChangeReminder(
		[]providers.ToolDefinition{{Name: "Read"}, {Name: "Write"}},
		[]providers.ToolDefinition{{Name: "Write"}, {Name: "Read"}},
	)
	if got != "" {
		t.Fatalf("expected empty reminder for unchanged tool names, got %q", got)
	}
}

func TestFormatMaxStepsReminder(t *testing.T) {
	got := FormatMaxStepsReminder(3)
	if !strings.Contains(got, "<system-reminder>") {
		t.Fatalf("expected system reminder, got %q", got)
	}
	if !strings.Contains(got, "maximum number of agent steps (3)") {
		t.Fatalf("expected max steps count in reminder, got %q", got)
	}
	if !strings.Contains(got, "Do not call any more tools") {
		t.Fatalf("expected no-tools instruction in reminder, got %q", got)
	}
}
