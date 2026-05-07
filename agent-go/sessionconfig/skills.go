package sessionconfig

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	fmparser "github.com/obot-platform/discobot/agent-go/frontmatter"
)

// SkillConfig represents a discovered skill (user-invocable prompt template).
type SkillConfig struct {
	// Name is the skill's slash-command name (e.g., "commit").
	Name string

	// Description describes what the skill does. Claude uses this to decide
	// when to auto-invoke the skill and for autocomplete.
	Description string

	// Body is the markdown prompt content (after frontmatter is stripped).
	Body string

	// SourcePath is the absolute path to the markdown file that defined the
	// skill or command.
	SourcePath string

	// Kind is "skill" for entries from skills/ directories and "command" for
	// entries from commands/ directories.
	Kind string

	// Discobot contains optional Discobot-specific metadata parsed from
	// frontmatter.
	Discobot DiscobotCommandMetadata
}

type DiscobotCommandMetadata struct {
	UI                bool
	Label             string
	ActiveLabel       string
	Icon              string
	Group             string
	Order             int
	CredentialRequest []DiscobotCredentialRequest
}

type DiscobotCredentialRequest struct {
	EnvVar        string
	Name          string
	Justification string
	ApprovedUses  []DiscobotCredentialApprovedUse
}

type DiscobotCredentialApprovedUse struct {
	Description string
}

// discoverSkills loads skill configs from Discobot-native skill directories and
// the shared .agents skill directories first. Provider-specific skill
// directories are compatibility fallbacks and only win for names that were not
// already defined by .discobot or .agents. Later entries with a duplicate name
// are ignored.
func discoverSkills(projectRoot string) ([]SkillConfig, []string, error) {
	home, _ := os.UserHomeDir()
	return discoverSkillsWithHome(projectRoot, home)
}

func discoverSkillsWithHome(projectRoot, home string) ([]SkillConfig, []string, error) {
	var skills []SkillConfig
	var warnings []string
	seen := make(map[string]bool)

	add := func(s SkillConfig) {
		if !seen[s.Name] {
			seen[s.Name] = true
			skills = append(skills, s)
		}
	}

	addFrom := func(list []SkillConfig, listWarnings []string, err error) error {
		if err != nil {
			warnings = append(warnings, err.Error())
			return nil
		}
		warnings = append(warnings, listWarnings...)
		for _, s := range list {
			add(s)
		}
		return nil
	}

	// 1. Project skills: .discobot/skills/*/SKILL.md then
	// .agents/skills/*/SKILL.md.
	for _, dir := range []string{".discobot", ".agents"} {
		if err := addFrom(loadSkillsDir(filepath.Join(projectRoot, dir, "skills"))); err != nil {
			return nil, nil, err
		}
	}

	// 2. User skills: ~/.discobot/skills/*/SKILL.md then
	// ~/.agents/skills/*/SKILL.md.
	if home != "" {
		for _, dir := range []string{".discobot", ".agents"} {
			if err := addFrom(loadSkillsDir(filepath.Join(home, dir, "skills"))); err != nil {
				return nil, nil, err
			}
		}
	}

	// 3. System skills installed with the image.
	for _, dir := range discobotSystemPaths("skills") {
		if err := addFrom(loadSkillsDir(dir)); err != nil {
			return nil, nil, err
		}
	}

	// 4. Provider-specific project skills are fallback compatibility sources.
	for _, dir := range []string{".claude", ".gemini", ".opencode"} {
		if err := addFrom(loadSkillsDir(filepath.Join(projectRoot, dir, "skills"))); err != nil {
			return nil, nil, err
		}
	}

	// 5. Provider-specific user skills are fallback compatibility sources.
	if home != "" {
		for _, dir := range []string{".claude", ".gemini", filepath.Join(".config", "opencode")} {
			if err := addFrom(loadSkillsDir(filepath.Join(home, dir, "skills"))); err != nil {
				return nil, nil, err
			}
		}
	}

	// 6. Project commands: .claude/commands/ then .discobot/commands/ (both formats).
	for _, dir := range []string{".claude", ".discobot"} {
		if err := addFrom(loadCommandsDir(filepath.Join(projectRoot, dir, "commands"))); err != nil {
			return nil, nil, err
		}
	}

	// 7. User commands: ~/.claude/commands/ then ~/.discobot/commands/ then
	// ~/.agents/commands/ (both formats).
	if home != "" {
		for _, dir := range []string{".claude", ".discobot", ".agents"} {
			if err := addFrom(loadCommandsDir(filepath.Join(home, dir, "commands"))); err != nil {
				return nil, nil, err
			}
		}
	}

	// 8. System commands installed with the image.
	for _, dir := range discobotSystemPaths("commands") {
		if err := addFrom(loadCommandsDir(dir)); err != nil {
			return nil, nil, err
		}
	}

	return skills, warnings, nil
}

// LookupSkill searches for a skill by name in skills/ directories only.
// It does NOT search commands/ — use LookupCommand for legacy commands.
// Project-level .discobot and .agents directories are checked first, followed by
// user-level ~/.discobot/skills and ~/.agents/skills, Discobot system skills,
// then provider-specific compatibility fallbacks.
// Returns (zero, false, nil) if the skill is not found.
func LookupSkill(projectRoot, skillName string) (SkillConfig, bool, error) {
	home, _ := os.UserHomeDir()
	return lookupSkillWithHome(projectRoot, skillName, home)
}

func lookupSkillWithHome(projectRoot, skillName, home string) (SkillConfig, bool, error) {
	dirs := []string{
		filepath.Join(projectRoot, ".discobot", "skills"),
		filepath.Join(projectRoot, ".agents", "skills"),
	}
	if home != "" {
		for _, dir := range []string{".discobot", ".agents"} {
			dirs = append(dirs, filepath.Join(home, dir, "skills"))
		}
	}
	dirs = append(dirs, discobotSystemPaths("skills")...)
	for _, dir := range []string{".claude", ".gemini", ".opencode"} {
		dirs = append(dirs, filepath.Join(projectRoot, dir, "skills"))
	}
	if home != "" {
		for _, dir := range []string{".claude", ".gemini", filepath.Join(".config", "opencode")} {
			dirs = append(dirs, filepath.Join(home, dir, "skills"))
		}
	}

	return lookupInSkillIndex(skillName, dirs, "skill", false)
}

// LookupCommand searches for a legacy command by name in commands/ directories.
// Commands are expanded programmatically when a user message starts with /name,
// unlike skills which are invoked via the Skill tool by the LLM.
// Project-level .claude and .discobot directory styles are checked, along with
// user-level ~/.claude/commands, ~/.discobot/commands, ~/.agents/commands, and
// the Discobot system commands directories.
// Returns (zero, false, nil) if the command is not found.
func LookupCommand(projectRoot, cmdName string) (SkillConfig, bool, error) {
	home, _ := os.UserHomeDir()
	return lookupCommandWithHome(projectRoot, cmdName, home)
}

func lookupCommandWithHome(projectRoot, cmdName, home string) (SkillConfig, bool, error) {
	dirs := []string{
		filepath.Join(projectRoot, ".claude", "commands"),
		filepath.Join(projectRoot, ".discobot", "commands"),
	}
	if home != "" {
		for _, dir := range []string{".claude", ".discobot", ".agents"} {
			dirs = append(dirs, filepath.Join(home, dir, "commands"))
		}
	}
	dirs = append(dirs, discobotSystemPaths("commands")...)

	return lookupInSkillIndex(cmdName, dirs, "command", true)
}

// lookupInSkillIndex builds a fresh in-memory index from parsed skill names for
// each lookup. Skill files are small, and rebuilding keeps tool execution in
// sync with files edited after the initial session reminder was generated.
func lookupInSkillIndex(name string, dirs []string, kind string, includeTopLevelMarkdown bool) (SkillConfig, bool, error) {
	index, err := buildSkillIndex(dirs, kind, includeTopLevelMarkdown)
	if err != nil {
		return SkillConfig{}, false, err
	}
	skill, ok := index[name]
	return skill, ok, nil
}

func buildSkillIndex(dirs []string, kind string, includeTopLevelMarkdown bool) (map[string]SkillConfig, error) {
	index := make(map[string]SkillConfig)
	for _, dir := range dirs {
		var (
			skills []SkillConfig
			err    error
		)
		if kind == "skill" {
			skills, _, err = loadSkillsDir(dir)
		} else {
			skills, _, err = loadSkillTree(dir, kind, includeTopLevelMarkdown)
		}
		if err != nil {
			return nil, err
		}
		for _, skill := range skills {
			if _, ok := index[skill.Name]; !ok {
				index[skill.Name] = skill
			}
		}
	}
	return index, nil
}

func pathToName(rel string) string {
	rel = strings.Trim(rel, string(filepath.Separator))
	if rel == "" {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return strings.Join(parts, ":")
}

type skillFileCandidate struct {
	name     string
	path     string
	priority int
}

// loadSkillsDir loads skills from one-level skill directories.
// Supports <skill-name>/SKILL.md, matched case-insensitively.
func loadSkillsDir(dir string) ([]SkillConfig, []string, error) {
	return loadSkillsTree(dir)
}

// loadCommandsDir loads skills from a .claude/commands directory recursively.
// Supports subdirectory format (name/SKILL.md), flat format (name.md), and nested markdown files.
func loadCommandsDir(dir string) ([]SkillConfig, []string, error) {
	return loadSkillTree(dir, "command", true)
}

func loadSkillsTree(dir string) ([]SkillConfig, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read skills dir %s: %w", dir, err)
	}

	skills := make([]SkillConfig, 0, len(entries))
	warnings := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path, ok, err := findSkillMarkdownFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		content, err := readFileIfExists(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read skill %s: %w", path, err)
		}
		if content == "" {
			continue
		}
		skill, err := parseSkill(entry.Name(), content)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("parse skill %s: %v", path, err))
			continue
		}
		skill.SourcePath = path
		skill.Kind = "skill"
		skills = append(skills, skill)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, warnings, nil
}

func findSkillMarkdownFile(dir string) (string, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, fmt.Errorf("read skill dir %s: %w", dir, err)
	}

	var fallback string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Name() == "SKILL.md" {
			return filepath.Join(dir, entry.Name()), true, nil
		}
		if fallback == "" && strings.EqualFold(entry.Name(), "SKILL.md") {
			fallback = filepath.Join(dir, entry.Name())
		}
	}
	if fallback == "" {
		return "", false, nil
	}
	return fallback, true, nil
}

func loadSkillTree(dir, kind string, includeTopLevelMarkdown bool) ([]SkillConfig, []string, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("stat skills dir %s: %w", dir, err)
	}

	var candidates []skillFileCandidate
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		name, priority, ok, err := skillCandidateName(dir, path, includeTopLevelMarkdown)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		candidates = append(candidates, skillFileCandidate{
			name:     name,
			path:     path,
			priority: priority,
		})
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk skills dir %s: %w", dir, err)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].name != candidates[j].name {
			return candidates[i].name < candidates[j].name
		}
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		return candidates[i].path < candidates[j].path
	})

	skills := make([]SkillConfig, 0, len(candidates))
	warnings := make([]string, 0)
	for _, candidate := range candidates {
		content, err := readFileIfExists(candidate.path)
		if err != nil {
			return nil, nil, fmt.Errorf("read skill %s: %w", candidate.path, err)
		}
		if content == "" {
			continue
		}
		skill, err := parseSkill(candidate.name, content)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("parse skill %s: %v", candidate.path, err))
			continue
		}
		skill.SourcePath = candidate.path
		skill.Kind = kind
		skills = append(skills, skill)
	}
	return skills, warnings, nil
}

func skillCandidateName(rootDir, path string, includeTopLevelMarkdown bool) (string, int, bool, error) {
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		return "", 0, false, fmt.Errorf("get relative path for %s: %w", path, err)
	}

	if strings.EqualFold(filepath.Base(rel), "SKILL.md") {
		relDir := filepath.Dir(rel)
		if relDir == "." {
			return "", 0, false, nil
		}
		return pathToName(relDir), 0, true, nil
	}

	if !strings.HasSuffix(rel, ".md") {
		return "", 0, false, nil
	}

	relWithoutExt := strings.TrimSuffix(rel, ".md")
	if relWithoutExt == "" || relWithoutExt == "." {
		return "", 0, false, nil
	}
	if !includeTopLevelMarkdown && !strings.Contains(relWithoutExt, string(filepath.Separator)) {
		return "", 0, false, nil
	}

	return pathToName(relWithoutExt), 1, true, nil
}

// Expand substitutes $ARGUMENTS (and the $0 shorthand) in the skill body with
// args. If args is non-empty and neither placeholder appears in the body, the
// args are appended as "ARGUMENTS: <args>".
func (s SkillConfig) Expand(args string) string {
	body := s.Body
	if args == "" {
		body = strings.ReplaceAll(body, "$ARGUMENTS", "")
		body = strings.ReplaceAll(body, "$0", "")
		return strings.TrimSpace(body)
	}

	replaced := false
	if strings.Contains(body, "$ARGUMENTS") {
		body = strings.ReplaceAll(body, "$ARGUMENTS", args)
		replaced = true
	}
	if strings.Contains(body, "$0") {
		body = strings.ReplaceAll(body, "$0", args)
		replaced = true
	}
	if !replaced {
		body = body + "\n\nARGUMENTS: " + args
	}
	return body
}

// parseSkill parses a skill markdown file (with optional YAML frontmatter)
// into a SkillConfig. defaultName is used when the frontmatter omits a name.
func parseSkill(defaultName, content string) (SkillConfig, error) {
	doc, err := fmparser.ParseMarkdown[skillFrontmatter](content)
	if err != nil {
		return SkillConfig{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	skill := SkillConfig{
		Name: defaultName,
		Body: strings.TrimSpace(doc.Body),
	}
	if doc.Metadata.Name != "" {
		skill.Name = doc.Metadata.Name
	}
	if doc.Metadata.Description != "" {
		skill.Description = doc.Metadata.Description
	}
	skill.Discobot = doc.Metadata.discobotMetadata()

	return skill, nil
}

// FormatSkillsReminder formats the list of available skills as a
// <system-reminder> block. Returns empty string if skills is empty.
func FormatSkillsReminder(skills []SkillConfig) string {
	if len(skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("The following skills are available for use with the Skill tool:\n\n")

	for _, s := range skills {
		fmt.Fprintf(&b, "- %s", s.Name)
		if s.Description != "" {
			fmt.Fprintf(&b, ": %s", s.Description)
		}
		b.WriteString("\n")
	}

	b.WriteString("\nWhen users reference a slash command or `/<something>`, it may refer to one of these skills or commands. Use the Skill tool for the entries listed here.")
	b.WriteString("\n</system-reminder>")
	return b.String()
}

// FormatSkillLikeReminder formats the skill-like commands available through the
// Skill tool. This intentionally presents markdown skills, legacy commands, and
// visible executable scripts as one unified capability set to the model.
func FormatSkillLikeReminder(skills []SkillConfig, scripts []ScriptConfig) string {
	type item struct {
		name        string
		description string
	}

	items := make([]item, 0, len(skills)+len(scripts))
	for _, skill := range skills {
		items = append(items, item{name: skill.Name, description: skill.Description})
	}
	for _, script := range scripts {
		if !script.Visible {
			continue
		}
		items = append(items, item{name: script.Name, description: script.Description})
	}
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("The following skills are available for use with the Skill tool:\n\n")
	for _, item := range items {
		fmt.Fprintf(&b, "- %s", item.name)
		if item.description != "" {
			fmt.Fprintf(&b, ": %s", item.description)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nWhen users reference a slash command or `/<something>`, it may refer to one of these skills or commands. Use the Skill tool for the entries listed here.")
	b.WriteString("\n</system-reminder>")
	return b.String()
}

// FormatSkillDiscoveryWarningsReminder formats non-fatal skill parse warnings as
// a <system-reminder> block. Returns empty string if warnings is empty.
func FormatSkillDiscoveryWarningsReminder(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("Some skills or slash commands could not be loaded. Do not try to use them. Tell the user that these skill files need to be fixed:\n\n")
	for _, warning := range warnings {
		fmt.Fprintf(&b, "- %s\n", warning)
	}
	b.WriteString("\nIf the user asks about one of these skills, explain that it is malformed and include the error above.\n")
	b.WriteString("</system-reminder>")
	return b.String()
}

// FormatSkillLikeDiscoveryWarningsReminder formats non-fatal loading warnings
// for any skill-like slash command source surfaced through the Skill tool.
func FormatSkillLikeDiscoveryWarningsReminder(skillWarnings, scriptWarnings []string) string {
	warnings := make([]string, 0, len(skillWarnings)+len(scriptWarnings))
	warnings = append(warnings, skillWarnings...)
	warnings = append(warnings, scriptWarnings...)
	if len(warnings) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("Some skills or slash commands could not be loaded. Do not try to use them. Tell the user that these files need to be fixed:\n\n")
	for _, warning := range warnings {
		fmt.Fprintf(&b, "- %s\n", warning)
	}
	b.WriteString("\nIf the user asks about one of these skills, explain that it is malformed and include the error above.\n")
	b.WriteString("</system-reminder>")
	return b.String()
}
