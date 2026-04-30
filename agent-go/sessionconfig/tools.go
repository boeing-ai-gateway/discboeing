package sessionconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/obot-platform/discobot/agent-go/providers"
)

type embeddedToolDefinition struct {
	Type        string                `yaml:"type"`
	Name        string                `yaml:"name"`
	Description string                `yaml:"description"`
	InputSchema any                   `yaml:"inputSchema"`
	Format      *providers.ToolFormat `yaml:"format"`
}

var (
	builtinToolMapOnce sync.Once
	builtinToolMap     map[string]providers.ToolDefinition
	builtinToolMapErr  error
)

const windowsShellToolName = "PowerShell"

// BuiltinTools returns the default embedded tool set in SYSTEM.md order.
func BuiltinTools(_ string) []providers.ToolDefinition {
	cfg, err := defaultSystemConfig()
	if err != nil {
		panic("sessionconfig: load default system config: " + err.Error())
	}
	tools, err := toolsForNames(cfg.AllowedTools)
	if err != nil {
		panic("sessionconfig: load builtin tools: " + err.Error())
	}
	return tools
}

// AdaptToolsForRuntime renames or rewrites tool definitions for the current
// runtime while keeping configuration names stable on disk.
func AdaptToolsForRuntime(goos string, tools []providers.ToolDefinition) []providers.ToolDefinition {
	if strings.TrimSpace(goos) == "" {
		goos = runtime.GOOS
	}
	if len(tools) == 0 {
		return nil
	}

	adapted := make([]providers.ToolDefinition, len(tools))
	for i, tool := range tools {
		adapted[i] = adaptToolForRuntime(goos, tool)
	}
	return adapted
}

func toolsForNames(names []string) ([]providers.ToolDefinition, error) {
	toolMap, err := builtinToolDefinitions()
	if err != nil {
		return nil, err
	}

	tools := make([]providers.ToolDefinition, 0, len(names))
	for _, name := range names {
		tool, ok := toolMap[name]
		if !ok {
			return nil, fmt.Errorf("unknown tool %q", name)
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func builtinToolDefinitions() (map[string]providers.ToolDefinition, error) {
	builtinToolMapOnce.Do(func() {
		builtinToolMap, builtinToolMapErr = loadBuiltinToolDefinitions()
	})
	if builtinToolMapErr != nil {
		return nil, builtinToolMapErr
	}

	clone := make(map[string]providers.ToolDefinition, len(builtinToolMap))
	maps.Copy(clone, builtinToolMap)
	return clone, nil
}

func adaptToolForRuntime(goos string, tool providers.ToolDefinition) providers.ToolDefinition {
	if strings.TrimSpace(goos) == "" || strings.TrimSpace(tool.Name) == "" {
		return tool
	}
	adapted, err := applyRuntimeToolOverride(goos, tool)
	if err != nil {
		panic("sessionconfig: apply runtime tool override: " + err.Error())
	}
	return adapted
}

func loadBuiltinToolDefinitions() (map[string]providers.ToolDefinition, error) {
	entries, err := embeddedConfigFiles.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read embedded config dir: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		baseName, goos, ok := parseToolDefinitionFileName(name)
		if !ok {
			continue
		}
		if strings.TrimSpace(baseName) == "" || strings.TrimSpace(goos) != "" {
			continue
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)

	if len(paths) == 0 {
		return nil, fmt.Errorf("no embedded tool definitions found")
	}

	tools := make(map[string]providers.ToolDefinition, len(paths))
	for _, path := range paths {
		tool, err := loadEmbeddedToolDefinition(path)
		if err != nil {
			return nil, err
		}
		if _, exists := tools[tool.Name]; exists {
			return nil, fmt.Errorf("duplicate tool name %q", tool.Name)
		}
		tools[tool.Name] = tool
	}

	return tools, nil
}

func applyRuntimeToolOverride(goos string, tool providers.ToolDefinition) (providers.ToolDefinition, error) {
	overridePath := runtimeToolOverridePath(tool.Name, goos)
	if overridePath == "" {
		return tool, nil
	}

	overrideMap, err := loadEmbeddedToolDefinitionMap(overridePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return tool, nil
		}
		return providers.ToolDefinition{}, err
	}

	baseMap, err := toolDefinitionToMap(tool)
	if err != nil {
		return providers.ToolDefinition{}, err
	}
	merged := mergeToolDefinitionMaps(baseMap, overrideMap)
	return toolDefinitionFromMap(overridePath, merged)
}

func loadEmbeddedToolDefinition(path string) (providers.ToolDefinition, error) {
	raw, err := loadEmbeddedToolDefinitionMap(path)
	if err != nil {
		return providers.ToolDefinition{}, err
	}
	return toolDefinitionFromMap(path, raw)
}

func loadEmbeddedToolDefinitionMap(path string) (map[string]any, error) {
	data, err := embeddedConfigFiles.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	return raw, nil
}

func toolDefinitionFromMap(path string, raw map[string]any) (providers.ToolDefinition, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return providers.ToolDefinition{}, fmt.Errorf("marshal %s: %w", path, err)
	}

	var decoded embeddedToolDefinition
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		return providers.ToolDefinition{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if decoded.Name == "" {
		return providers.ToolDefinition{}, fmt.Errorf("%s: missing tool name", path)
	}
	if decoded.InputSchema == nil {
		return providers.ToolDefinition{}, fmt.Errorf("%s: missing inputSchema", path)
	}

	schema, err := json.Marshal(decoded.InputSchema)
	if err != nil {
		return providers.ToolDefinition{}, fmt.Errorf("marshal inputSchema for %s: %w", decoded.Name, err)
	}

	return providers.ToolDefinition{
		Type:        decoded.Type,
		Name:        decoded.Name,
		Description: decoded.Description,
		InputSchema: schema,
		Format:      decoded.Format,
	}, nil
}

func toolDefinitionToMap(tool providers.ToolDefinition) (map[string]any, error) {
	raw := map[string]any{
		"name":        tool.Name,
		"description": tool.Description,
	}
	if strings.TrimSpace(tool.Type) != "" {
		raw["type"] = tool.Type
	}
	if len(tool.InputSchema) > 0 {
		var schema any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("unmarshal input schema for %s: %w", tool.Name, err)
		}
		raw["inputSchema"] = schema
	}
	if tool.Format != nil {
		var format any
		data, err := json.Marshal(tool.Format)
		if err != nil {
			return nil, fmt.Errorf("marshal format for %s: %w", tool.Name, err)
		}
		if err := json.Unmarshal(data, &format); err != nil {
			return nil, fmt.Errorf("unmarshal format for %s: %w", tool.Name, err)
		}
		raw["format"] = format
	}
	return raw, nil
}

func mergeToolDefinitionMaps(base, override map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(override))
	maps.Copy(merged, base)
	for key, value := range override {
		baseMap, baseOK := merged[key].(map[string]any)
		overrideMap, overrideOK := value.(map[string]any)
		if baseOK && overrideOK {
			merged[key] = mergeToolDefinitionMaps(baseMap, overrideMap)
			continue
		}
		merged[key] = value
	}
	return merged
}

func parseToolDefinitionFileName(name string) (baseName, goos string, ok bool) {
	if matched, err := filepath.Match("tool-*.yaml", name); err != nil || !matched {
		return "", "", false
	}
	stem := strings.TrimSuffix(strings.TrimPrefix(name, "tool-"), ".yaml")
	if stem == "" {
		return "", "", false
	}
	if idx := strings.LastIndex(stem, "."); idx > 0 {
		return stem[:idx], stem[idx+1:], true
	}
	return stem, "", true
}

func runtimeToolOverridePath(toolName, goos string) string {
	toolName = strings.TrimSpace(strings.ToLower(toolName))
	goos = strings.TrimSpace(strings.ToLower(goos))
	if toolName == "" || goos == "" {
		return ""
	}
	return fmt.Sprintf("tool-%s.%s.yaml", toolName, goos)
}

// FormatToolAvailabilityChangeReminder formats a mid-conversation tool change as
// a <system-reminder> block. It lists newly available and removed tool names.
// Returns empty string when the tool-name sets are unchanged.
func FormatToolAvailabilityChangeReminder(previous, current []providers.ToolDefinition) string {
	previousNames := sortedToolNames(previous)
	currentNames := sortedToolNames(current)
	added := diffSortedToolNames(currentNames, previousNames)
	removed := diffSortedToolNames(previousNames, currentNames)
	if len(added) == 0 && len(removed) == 0 {
		return ""
	}

	lines := []string{
		"<system-reminder>",
		"Tool availability changed at this point in the conversation.",
		"Treat any tools listed below as unavailable before this reminder.",
	}
	if len(added) > 0 {
		lines = append(lines, "Newly available tools: "+strings.Join(added, ", "))
	}
	if len(removed) > 0 {
		lines = append(lines, "No longer available tools: "+strings.Join(removed, ", "))
	}
	lines = append(lines, "</system-reminder>")
	return strings.Join(lines, "\n")
}

func sortedToolNames(tools []providers.ToolDefinition) []string {
	if len(tools) == 0 {
		return nil
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func diffSortedToolNames(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	other := make(map[string]struct{}, len(b))
	for _, name := range b {
		other[name] = struct{}{}
	}
	var diff []string
	for _, name := range a {
		if _, ok := other[name]; !ok {
			diff = append(diff, name)
		}
	}
	return diff
}
