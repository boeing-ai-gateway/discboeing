package sessionconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	fmparser "github.com/boeing-ai-gateway/discboeing/agent-go/frontmatter"
	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
)

// SubAgentConfig represents a sub-agent defined in an agents/*.md directory.
type SubAgentConfig struct {
	Name             string                     `yaml:"name" json:"name"`
	Description      string                     `yaml:"description" json:"description"`
	Model            string                     `yaml:"model,omitempty" json:"model,omitempty"`
	SupportingModels providers.SupportingModels `yaml:"supportingModels,omitempty" json:"supportingModels,omitempty"`
	AllowedTools     []string                   `yaml:"allowedTools,omitempty" json:"allowedTools,omitempty"`
	DisallowedTools  []string                   `yaml:"disallowedTools,omitempty" json:"disallowedTools,omitempty"`
	MaxTurns         int                        `yaml:"maxTurns,omitempty" json:"maxTurns,omitempty"`
	Prompt           string                     `yaml:"-" json:"prompt"` // Markdown body
}

// discoverSubAgents loads sub-agent configs from project agent directories plus
// built-in embedded agents. Project agents override built-in agents with the
// same name.
func discoverSubAgents(projectRoot string) ([]SubAgentConfig, error) {
	projectAgents, err := discoverProjectSubAgents(projectRoot)
	if err != nil {
		return nil, err
	}
	builtinAgents, err := discoverBuiltinSubAgents()
	if err != nil {
		return nil, err
	}
	return mergeSubAgents(projectAgents, builtinAgents), nil
}

func discoverProjectSubAgents(projectRoot string) ([]SubAgentConfig, error) {
	dirs := []string{
		filepath.Join(projectRoot, ".discboeing", "agents"),
		filepath.Join(projectRoot, ".agents", "agents"),
		filepath.Join(projectRoot, ".claude", "agents"),
		filepath.Join(projectRoot, ".gemini", "agents"),
		filepath.Join(projectRoot, ".opencode", "agents"),
	}

	var agents []SubAgentConfig
	seen := make(map[string]struct{})
	for _, agentsDir := range dirs {
		dirAgents, err := loadSubAgentsDir(agentsDir)
		if err != nil {
			return nil, err
		}
		for _, agent := range dirAgents {
			if _, ok := seen[agent.Name]; ok {
				continue
			}
			seen[agent.Name] = struct{}{}
			agents = append(agents, agent)
		}
	}
	return agents, nil
}

func loadSubAgentsDir(agentsDir string) ([]SubAgentConfig, error) {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read agents dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var agents []SubAgentConfig
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p := filepath.Join(agentsDir, e.Name())
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read agent %s: %w", p, err)
		}

		agent, err := parseSubAgent(e.Name(), string(content))
		if err != nil {
			return nil, fmt.Errorf("parse agent %s: %w", e.Name(), err)
		}
		agents = append(agents, agent)
	}

	return agents, nil
}

func discoverBuiltinSubAgents() ([]SubAgentConfig, error) {
	entries, err := embeddedConfigFiles.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read embedded config dir: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		matched, err := filepath.Match("agent-*.md", name)
		if err != nil {
			return nil, fmt.Errorf("match embedded agent file %s: %w", name, err)
		}
		if matched {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var agents []SubAgentConfig
	for _, name := range names {
		data, err := embeddedConfigFiles.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		agent, err := parseSubAgent(name, string(data))
		if err != nil {
			return nil, fmt.Errorf("parse built-in agent %s: %w", name, err)
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func mergeSubAgents(projectAgents, builtinAgents []SubAgentConfig) []SubAgentConfig {
	seen := make(map[string]struct{}, len(projectAgents))
	merged := make([]SubAgentConfig, 0, len(projectAgents)+len(builtinAgents))
	for _, agent := range projectAgents {
		merged = append(merged, agent)
		seen[agent.Name] = struct{}{}
	}
	for _, agent := range builtinAgents {
		if _, ok := seen[agent.Name]; ok {
			continue
		}
		merged = append(merged, agent)
	}
	return merged
}

// parseSubAgent parses a single sub-agent markdown file.
// The file may have YAML frontmatter followed by the agent's prompt.
func parseSubAgent(filename, content string) (SubAgentConfig, error) {
	doc, err := fmparser.ParseMarkdown[subAgentFrontmatter](content)
	if err != nil {
		return SubAgentConfig{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	agent := SubAgentConfig{
		Name:             doc.Metadata.Name,
		Description:      doc.Metadata.Description,
		Model:            doc.Metadata.Model,
		SupportingModels: doc.Metadata.SupportingModels,
		AllowedTools:     doc.Metadata.AllowedTools,
		DisallowedTools:  doc.Metadata.DisallowedTools,
		MaxTurns:         doc.Metadata.MaxTurns,
		Prompt:           strings.TrimSpace(doc.Body),
	}
	if agent.Name == "" {
		agent.Name = strings.TrimSuffix(filename, ".md")
	}
	return agent, nil
}

// FormatSubAgentReminder formats the discovered Task sub-agent types as a
// startup reminder. It returns an empty string when no sub-agents are available.
func FormatSubAgentReminder(agents []SubAgentConfig) string {
	if len(agents) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("The following sub-agent types are available for use with the Task tool:\n\n")
	wroteAgent := false
	for _, agent := range agents {
		name := strings.TrimSpace(agent.Name)
		if name == "" {
			continue
		}
		wroteAgent = true
		b.WriteString("- ")
		b.WriteString(name)
		description := strings.TrimSpace(agent.Description)
		if description != "" {
			b.WriteString(": ")
			b.WriteString(description)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nUse only one of these exact values for Task.subagent_type. Do not guess or invent other sub-agent types.")
	b.WriteString("\n</system-reminder>")

	if !wroteAgent {
		return ""
	}
	return b.String()
}
