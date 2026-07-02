package sessionconfig

import (
	"log"
	"os"
	"path/filepath"

	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
)

const DefaultMaxSubagentDepth = 4

// InstructionEntry represents a discovered instruction file with metadata.
type InstructionEntry struct {
	// Path is the display path (e.g., "CLAUDE.md", "~/.claude/CLAUDE.md").
	Path string

	// Description describes the source (e.g., "project instructions, checked into the codebase").
	Description string

	// Content is the file content.
	Content string
}

// SessionConfig holds the discovered session configuration.
type SessionConfig struct {
	// SystemPrompt is the default base system prompt (behavioral instructions).
	SystemPrompt string

	// UserInstructions are discovered AGENTS.md, provider fallback instruction
	// files, and rules files.
	// These are delivered separately from the system prompt in <system-reminder> tags.
	UserInstructions []InstructionEntry

	// Tools are the built-in tool definitions sent to the LLM provider.
	Tools []providers.ToolDefinition

	// MCPServers are parsed MCP server definitions from .mcp.json files.
	// These are not connected yet — just parsed for future use.
	MCPServers []MCPServerConfig

	// SubAgents are sub-agent configurations from project agent directories.
	SubAgents []SubAgentConfig

	// MaxSubagentDepth limits how many nested Task/Agent hops may run beneath a
	// top-level thread. Top-level threads are depth 0.
	MaxSubagentDepth int

	// Skills are discovered skill configurations from Discboeing-native skill
	// directories, shared .agents skill directories, provider fallback skill
	// directories, Discboeing system directories, and legacy command directories.
	// They are listed in the system-reminder so the model knows which slash
	// commands are available.
	Skills []SkillConfig

	// SkillDiscoveryWarnings are non-fatal skill loading issues, such as
	// malformed frontmatter in SKILL.md files. These are surfaced to the model in
	// a system reminder so it can tell the user to fix them.
	SkillDiscoveryWarnings []string

	// Scripts are discovered executable slash-command scripts from project-level
	// .discboeing/scripts, user-level ~/.discboeing/scripts, and Discboeing system
	// script directories. Visible scripts are listed in the system reminders so
	// the model knows which executable slash commands are available.
	Scripts []ScriptConfig

	// ScriptDiscoveryWarnings are non-fatal script loading issues.
	ScriptDiscoveryWarnings []string

	// DiscboeingServicesConfigured indicates whether project-level
	// .discboeing/services exists.
	DiscboeingServicesConfigured bool

	// DiscboeingHooksConfigured indicates whether project-level .discboeing/hooks
	// exists.
	DiscboeingHooksConfigured bool
}

// Load discovers and loads session configuration from the given working directory.
// Non-critical errors (missing optional files) are logged as warnings.
// Returns an error only for I/O failures on files that do exist.
func Load(cwd string) (*SessionConfig, error) {
	cfg := &SessionConfig{}

	// 1. Set the system prompt and default tool set.
	projectRoot := findProjectRoot(cwd)
	systemCfg, err := loadSystemConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	cfg.SystemPrompt = systemCfg.PromptBody
	cfg.Tools, err = toolsForNames(systemCfg.AllowedTools)
	if err != nil {
		return nil, err
	}
	cfg.MaxSubagentDepth = DefaultMaxSubagentDepth
	cfg.DiscboeingServicesConfigured = dirExists(filepath.Join(projectRoot, ".discboeing", "services"))
	cfg.DiscboeingHooksConfigured = dirExists(filepath.Join(projectRoot, ".discboeing", "hooks"))

	// 2. Discover user instruction files (CLAUDE.md, AGENTS.md, rules).
	entries, err := discoverInstructions(cwd)
	if err != nil {
		return nil, err
	}
	cfg.UserInstructions = entries

	// 3. Discover MCP server configs.
	mcpServers, err := discoverMCPServers(projectRoot)
	if err != nil {
		log.Printf("sessionconfig: warning: MCP discovery: %v", err)
		// Non-fatal — continue without MCP servers.
	} else {
		cfg.MCPServers = mcpServers
	}

	// 4. Discover sub-agent configs.
	subAgents, err := discoverSubAgents(projectRoot)
	if err != nil {
		log.Printf("sessionconfig: warning: sub-agent discovery: %v", err)
		// Non-fatal — continue without sub-agents.
	} else {
		cfg.SubAgents = subAgents
	}

	// 5. Discover skills.
	skills, warnings, err := discoverSkills(projectRoot)
	if err != nil {
		log.Printf("sessionconfig: warning: skill discovery: %v", err)
		// Non-fatal — continue without skills.
	} else {
		cfg.Skills = skills
		cfg.SkillDiscoveryWarnings = warnings
		for _, warning := range warnings {
			log.Printf("sessionconfig: warning: skill discovery: %s", warning)
		}
	}

	// 6. Discover scripts.
	scripts, warnings, err := discoverScripts(projectRoot)
	if err != nil {
		log.Printf("sessionconfig: warning: script discovery: %v", err)
		// Non-fatal — continue without scripts.
	} else {
		cfg.Scripts = scripts
		cfg.ScriptDiscoveryWarnings = warnings
		for _, warning := range warnings {
			log.Printf("sessionconfig: warning: script discovery: %s", warning)
		}
	}

	return cfg, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
