package sessionconfig

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"sort"
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
		matched, err := filepath.Match("tool-*.yaml", name)
		if err != nil {
			return nil, fmt.Errorf("match embedded tool file %s: %w", name, err)
		}
		if matched {
			paths = append(paths, name)
		}
	}
	sort.Strings(paths)

	if len(paths) == 0 {
		return nil, fmt.Errorf("no embedded tool definitions found")
	}

	tools := make(map[string]providers.ToolDefinition, len(paths))
	for _, path := range paths {
		data, err := embeddedConfigFiles.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		var raw embeddedToolDefinition
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w", path, err)
		}
		if raw.Name == "" {
			return nil, fmt.Errorf("%s: missing tool name", path)
		}
		if raw.InputSchema == nil {
			return nil, fmt.Errorf("%s: missing inputSchema", path)
		}
		if _, exists := tools[raw.Name]; exists {
			return nil, fmt.Errorf("duplicate tool name %q", raw.Name)
		}

		schema, err := json.Marshal(raw.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("marshal inputSchema for %s: %w", raw.Name, err)
		}

		tools[raw.Name] = providers.ToolDefinition{
			Type:        raw.Type,
			Name:        raw.Name,
			Description: raw.Description,
			InputSchema: schema,
			Format:      raw.Format,
		}
	}

	return tools, nil
}
