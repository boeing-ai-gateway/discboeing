package sessionconfig

import "fmt"

type SkillLikeKind string

const (
	SkillLikeKindSkill   SkillLikeKind = "skill"
	SkillLikeKindCommand SkillLikeKind = "command"
	SkillLikeKindScript  SkillLikeKind = "script"
)

// SkillLikeConfig represents a slash-command target resolved from a skill,
// legacy command, or executable script.
type SkillLikeConfig struct {
	Name   string
	Kind   SkillLikeKind
	Skill  *SkillConfig
	Script *ScriptConfig
}

// LookupSkillLike resolves a slash-command name using the same precedence the
// agent uses elsewhere: skill, then legacy command, then executable script.
func LookupSkillLike(projectRoot, name string, visibleScriptsOnly bool) (SkillLikeConfig, bool, error) {
	cfg, found, err := LookupSkill(projectRoot, name)
	if err != nil {
		return SkillLikeConfig{}, false, err
	}
	if found {
		return SkillLikeConfig{Name: cfg.Name, Kind: SkillLikeKindSkill, Skill: &cfg}, true, nil
	}

	cfg, found, err = LookupCommand(projectRoot, name)
	if err != nil {
		return SkillLikeConfig{}, false, err
	}
	if found {
		return SkillLikeConfig{Name: cfg.Name, Kind: SkillLikeKindCommand, Skill: &cfg}, true, nil
	}

	script, found, err := LookupScript(projectRoot, name, visibleScriptsOnly)
	if err != nil {
		return SkillLikeConfig{}, false, err
	}
	if found {
		return SkillLikeConfig{Name: script.Name, Kind: SkillLikeKindScript, Script: &script}, true, nil
	}

	return SkillLikeConfig{}, false, nil
}

func (c SkillLikeConfig) Expand(args string) (string, error) {
	if c.Skill == nil {
		return "", fmt.Errorf("skill-like config %q has no prompt body", c.Name)
	}
	return c.Skill.Expand(args), nil
}
