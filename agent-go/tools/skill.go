package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/scriptexec"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type skillInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

// executeSkill handles the Skill tool. It looks up the named skill file,
// expands $ARGUMENTS substitutions, and returns the body wrapped in a
// <skill-name> tag so Claude knows to follow the embedded instructions.
func (e *Executor) executeSkill(ctx context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input skillInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.Skill == "" {
		return errResult(call, "skill name is required"), nil
	}

	result, err := runSkill(ctx, e.cwd, e.bashEnv(), input.Skill, input.Args)
	if err != nil {
		if strings.HasPrefix(err.Error(), "<script_execution ") {
			return errResult(call, err.Error()), nil
		}
		return errResult(call, fmt.Sprintf("skill %q: %v", input.Skill, err)), nil
	}
	return textResult(call, result), nil
}

// runSkill looks up a skill-like command by name.
// Skills and legacy commands return wrapped prompt text; visible scripts are
// executed directly and return their stdout or formatted failure details.
func runSkill(ctx context.Context, cwd string, env []string, skillName, args string) (string, error) {
	projectRoot := sessionconfig.FindProjectRoot(cwd)

	cfg, found, err := sessionconfig.LookupSkillLike(projectRoot, skillName, true)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("skill %q not found in configured skill, command, or visible script directories", skillName)
	}

	switch cfg.Kind {
	case sessionconfig.SkillLikeKindSkill, sessionconfig.SkillLikeKindCommand:
		body, err := cfg.Expand(args)
		if err != nil {
			return "", err
		}

		// Wrap in a tag matching the skill name. The Skill tool description tells
		// Claude: "If you see a <command-name> tag in the current conversation
		// turn, the skill has ALREADY been loaded — follow the instructions
		// directly instead of calling this tool again."
		return fmt.Sprintf("<%s>\n%s\n</%s>", skillName, body, skillName), nil
	case sessionconfig.SkillLikeKindScript:
		result, err := scriptexec.RunDiscovered(ctx, *cfg.Script, cwd, env, args)
		if err != nil {
			return "", err
		}
		if result.Success {
			return result.TrimmedStdout(), nil
		}
		return "", fmt.Errorf("%s", result.FormatForLLM())
	default:
		return "", fmt.Errorf("unsupported skill-like kind %q", cfg.Kind)
	}
}
