package sessionconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScriptConfig represents a discovered executable slash-command script.
type ScriptConfig struct {
	Name         string
	Description  string
	Path         string
	Visible      bool
	ArgumentHint string
	Discobot     DiscobotCommandMetadata
}

// discoverScripts loads executable scripts from project and user script directories.
func discoverScripts(projectRoot string) ([]ScriptConfig, []string, error) {
	home, _ := os.UserHomeDir()
	return discoverScriptsWithHome(projectRoot, home)
}

func discoverScriptsWithHome(projectRoot, home string) ([]ScriptConfig, []string, error) {
	var scripts []ScriptConfig
	var warnings []string
	seen := make(map[string]bool)

	add := func(script ScriptConfig) {
		if !seen[script.Name] {
			seen[script.Name] = true
			scripts = append(scripts, script)
		}
	}

	addFrom := func(list []ScriptConfig, listWarnings []string, err error) error {
		if err != nil {
			warnings = append(warnings, err.Error())
			return nil
		}
		warnings = append(warnings, listWarnings...)
		for _, script := range list {
			add(script)
		}
		return nil
	}

	if err := addFrom(loadScriptsDir(filepath.Join(projectRoot, ".discobot", "scripts"))); err != nil {
		return nil, nil, err
	}
	if home != "" {
		for _, dir := range []string{".discobot", ".agents"} {
			if err := addFrom(loadScriptsDir(filepath.Join(home, dir, "scripts"))); err != nil {
				return nil, nil, err
			}
		}
	}

	return scripts, warnings, nil
}

// LookupScript searches for an executable script by name. Hidden scripts are
// returned unless visibleOnly is true.
func LookupScript(projectRoot, name string, visibleOnly bool) (ScriptConfig, bool, error) {
	home, _ := os.UserHomeDir()
	return lookupScriptWithHome(projectRoot, name, home, visibleOnly)
}

func lookupScriptWithHome(projectRoot, name, home string, visibleOnly bool) (ScriptConfig, bool, error) {
	var dirs []string
	dirs = append(dirs, filepath.Join(projectRoot, ".discobot", "scripts"))
	if home != "" {
		for _, dir := range []string{".discobot", ".agents"} {
			dirs = append(dirs, filepath.Join(home, dir, "scripts"))
		}
	}

	for _, dir := range dirs {
		cfg, ok, err := lookupScriptInDir(dir, name)
		if err != nil {
			return ScriptConfig{}, false, err
		}
		if !ok {
			continue
		}
		if visibleOnly && !cfg.Visible {
			continue
		}
		return cfg, true, nil
	}
	return ScriptConfig{}, false, nil
}

func lookupScriptInDir(dir, name string) (ScriptConfig, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return ScriptConfig{}, false, nil
		}
		return ScriptConfig{}, false, fmt.Errorf("read scripts dir %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if normalizeScriptName(entry.Name()) != normalizeScriptName(name) {
			continue
		}
		return loadScriptFile(filepath.Join(dir, entry.Name()), entry.Name())
	}
	return ScriptConfig{}, false, nil
}

func loadScriptsDir(dir string) ([]ScriptConfig, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read scripts dir %s: %w", dir, err)
	}

	var scripts []ScriptConfig
	var warnings []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		cfg, ok, err := loadScriptFile(path, entry.Name())
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("load script %s: %v", path, err))
			continue
		}
		if !ok {
			continue
		}
		scripts = append(scripts, cfg)
	}

	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Name < scripts[j].Name
	})
	return scripts, warnings, nil
}

func loadScriptFile(path, defaultName string) (ScriptConfig, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ScriptConfig{}, false, nil
		}
		return ScriptConfig{}, false, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() || !isExecutableFile(info) {
		return ScriptConfig{}, false, nil
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return ScriptConfig{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	content := string(contentBytes)
	if !strings.HasPrefix(content, "#!") {
		return ScriptConfig{}, false, nil
	}

	fm, err := parseScriptFrontMatter(content)
	if err != nil {
		return ScriptConfig{}, false, err
	}
	if fm == nil {
		return ScriptConfig{}, false, nil
	}

	name := normalizeScriptName(defaultName)
	if value, ok := stringFrontmatterValue(fm, "name"); ok && value != "" {
		name = strings.TrimSpace(value)
	}
	if name == "" {
		return ScriptConfig{}, false, fmt.Errorf("missing script name")
	}

	cfg := ScriptConfig{
		Name:    name,
		Path:    path,
		Visible: true,
	}
	if value, ok := stringFrontmatterValue(fm, "description"); ok {
		cfg.Description = value
	}
	if value, ok := boolFrontmatterValue(fm, "visible"); ok {
		cfg.Visible = value
	}
	if value, ok := stringFrontmatterValue(fm, "argument-hint", "argumentHint"); ok {
		cfg.ArgumentHint = value
	}
	cfg.Discobot = parseDiscobotMetadata(fm)

	return cfg, true, nil
}

func isExecutableFile(info os.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func normalizeScriptName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".sh", ".bash", ".zsh", ".py", ".js", ".ts", ".rb", ".pl", ".php":
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return strings.TrimSpace(name)
}

func parseScriptFrontMatter(content string) (map[string]any, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return nil, nil
	}

	startLine := 0
	if strings.HasPrefix(lines[0], "#!") {
		startLine = 1
	}
	if len(lines) <= startLine {
		return nil, nil
	}

	delim := detectScriptDelimiter(lines[startLine])
	if delim == nil {
		return nil, nil
	}

	var yamlLines []string
	found := false
	for i := startLine + 1; i < len(lines); i++ {
		if matchesScriptDelimiter(lines[i], delim.delimiter) {
			found = true
			break
		}
		yamlLines = append(yamlLines, stripScriptFrontMatterPrefix(lines[i], delim.prefix))
	}
	if !found {
		return nil, nil
	}

	frontmatter := make(map[string]any)
	yamlContent := strings.Join(yamlLines, "\n")
	if err := yaml.Unmarshal([]byte(yamlContent), &frontmatter); err != nil {
		return nil, fmt.Errorf("parse script front matter: %w", err)
	}
	return frontmatter, nil
}

type scriptDelimiter struct {
	prefix    string
	delimiter string
}

func detectScriptDelimiter(line string) *scriptDelimiter {
	switch strings.TrimSpace(line) {
	case "---":
		return &scriptDelimiter{delimiter: "---"}
	case "#---":
		return &scriptDelimiter{prefix: "#", delimiter: "#---"}
	case "//---":
		return &scriptDelimiter{prefix: "//", delimiter: "//---"}
	default:
		return nil
	}
}

func matchesScriptDelimiter(line, delimiter string) bool {
	return strings.TrimSpace(line) == delimiter
}

func stripScriptFrontMatterPrefix(line, prefix string) string {
	if prefix == "" {
		return line
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, prefix) {
		return line
	}
	_, after, ok := strings.Cut(line, prefix)
	if !ok {
		return line
	}
	content := after
	if len(content) > 0 && (content[0] == ' ' || content[0] == '\t') {
		content = content[1:]
	}
	return content
}

// FormatScriptsReminder formats visible scripts as skill-like executable slash
// commands available through the Skill tool.
func FormatScriptsReminder(scripts []ScriptConfig) string {
	visible := make([]ScriptConfig, 0, len(scripts))
	for _, script := range scripts {
		if script.Visible {
			visible = append(visible, script)
		}
	}
	if len(visible) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("The following executable scripts are available through the Skill tool:\n\n")
	for _, script := range visible {
		fmt.Fprintf(&b, "- %s", script.Name)
		if script.Description != "" {
			fmt.Fprintf(&b, ": %s", script.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nWhen users reference a slash command or `/<something>`, it may refer to one of these executable scripts. Use the Skill tool for the visible scripts listed here.")
	b.WriteString("\n</system-reminder>")
	return b.String()
}

// FormatScriptDiscoveryWarningsReminder formats non-fatal script loading warnings.
func FormatScriptDiscoveryWarningsReminder(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("Some executable scripts could not be loaded. Do not try to use them. Tell the user that these script files need to be fixed:\n\n")
	for _, warning := range warnings {
		fmt.Fprintf(&b, "- %s\n", warning)
	}
	b.WriteString("\nIf the user asks about one of these scripts, explain that it is malformed and include the error above.\n")
	b.WriteString("</system-reminder>")
	return b.String()
}

// SplitScriptArgs tokenizes a raw argument string for executable scripts.
func SplitScriptArgs(raw string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}

	for _, r := range raw {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in args")
	}
	flush()
	return args, nil
}
