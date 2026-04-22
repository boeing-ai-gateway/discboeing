package agentimpl

import (
	"context"
	"strings"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/scriptexec"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type parsedSlashCommand struct {
	original string
	name     string
	args     string
}

func parseSlashCommand(parts []message.UIPart) (message.UITextPart, parsedSlashCommand, bool) {
	if len(parts) == 0 {
		return message.UITextPart{}, parsedSlashCommand{}, false
	}
	first, ok := parts[0].(message.UITextPart)
	if !ok {
		return message.UITextPart{}, parsedSlashCommand{}, false
	}
	text := strings.TrimLeft(first.Text, " \t")
	if !strings.HasPrefix(text, "/") {
		return message.UITextPart{}, parsedSlashCommand{}, false
	}

	rest := text[1:]
	var cmdName, args string
	if idx := strings.IndexAny(rest, " \t\n"); idx >= 0 {
		cmdName = rest[:idx]
		args = strings.TrimLeft(rest[idx:], " \t")
	} else {
		cmdName = rest
	}
	if cmdName == "" {
		return message.UITextPart{}, parsedSlashCommand{}, false
	}

	return first, parsedSlashCommand{
		original: text,
		name:     strings.TrimSpace(cmdName),
		args:     args,
	}, true
}

func annotateSlashCommandParts(parts []message.UIPart, first message.UITextPart, original string, kind agent.CommandKind, replacement string) []message.UIPart {
	annotated := make([]message.UIPart, len(parts))
	copy(annotated, parts)
	annotated[0] = message.UITextPart{
		Text:  replacement,
		State: first.State,
		ProviderMetadata: message.MarshalProviderMetadata(message.DiscobotPartMetadata{
			OriginalCommand: original,
			CommandKind:     string(kind),
		}),
	}
	return annotated
}

func resolveSlashCommandMatch(projectRoot string, parsed parsedSlashCommand) (sessionconfig.SkillLikeConfig, bool, error) {
	return sessionconfig.LookupSkillLike(projectRoot, parsed.name, false)
}

func skillLikeKindToCommandKind(kind sessionconfig.SkillLikeKind) agent.CommandKind {
	switch kind {
	case sessionconfig.SkillLikeKindSkill:
		return agent.CommandKindSkill
	case sessionconfig.SkillLikeKindCommand:
		return agent.CommandKindCommand
	case sessionconfig.SkillLikeKindScript:
		return agent.CommandKindScript
	default:
		return ""
	}
}

func slashCommandMetadata(parsed parsedSlashCommand, kind agent.CommandKind, cfg sessionconfig.SkillLikeConfig) *thread.UserSlashCommandMetadata {
	metadata := &thread.UserSlashCommandMetadata{
		Name: parsed.name,
		Kind: kind,
	}
	if kind == agent.CommandKindSkill && cfg.Skill != nil {
		metadata.Text = cfg.Skill.Body
	}
	return metadata
}

func (a *DefaultAgent) executeScriptSlashCommand(
	ctx context.Context,
	userParts []message.UIPart,
	originalText string,
	slashCommand *thread.UserSlashCommandMetadata,
) ([]message.UIPart, *thread.UserSlashCommandMetadata, error) {
	if slashCommand == nil || slashCommand.Kind != agent.CommandKindScript {
		return userParts, slashCommand, nil
	}

	execution, err := scriptexec.RunNamed(
		ctx,
		sessionconfig.FindProjectRoot(a.cwd),
		a.cwd,
		nil,
		slashCommand.Name,
		scriptCommandArgs(originalText),
		false,
	)
	if err != nil {
		return nil, nil, err
	}

	slashCommand.Script = scriptExecutionMetadata(execution)
	formatted := execution.FormatForLLM()
	if execution.Success && formatted == "" {
		slashCommand.Script.SuppressedLLM = true
		return []message.UIPart{message.UITextPart{Text: firstUserText(userParts)}}, slashCommand, nil
	}

	slashCommand.Text = formatted
	return []message.UIPart{message.UITextPart{Text: formatted}}, slashCommand, nil
}
