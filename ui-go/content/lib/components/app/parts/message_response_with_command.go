package parts

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func messageCommandPresent(command viewmodel.OriginalCommandSnapshot) bool {
	return command.Command != "" || command.Kind != ""
}

func messageCommandKindLabel(kind string) string {
	switch kind {
	case "skill":
		return "Skill"
	case "script":
		return "Script"
	default:
		return "Command"
	}
}

func messageGeneratedSectionLabel(kind string) string {
	if kind == "skill" {
		return "Skill text"
	}
	return "Generated text"
}

func messageGeneratedToggleLabel(kind string, expanded bool) string {
	prefix := "Show"
	if expanded {
		prefix = "Hide"
	}
	if kind == "skill" {
		return prefix + " skill text"
	}
	return prefix + " generated text"
}

func messageShouldShowGenerated(snapshot viewmodel.MessageResponseWithCommandSnapshot) bool {
	if !messageCommandPresent(snapshot.OriginalCommand) {
		return snapshot.OriginalText != "" && len(snapshot.TextParts) > 0
	}
	if snapshot.OriginalCommand.Kind == "skill" || snapshot.OriginalCommand.Kind == "script" {
		return snapshot.OriginalCommand.Text != ""
	}
	return len(snapshot.TextParts) > 0
}

func messageCommandChevronClass(expanded bool) string {
	base := "size-3 transition-all group-hover:opacity-100"
	if expanded {
		return base + " rotate-180 opacity-100"
	}
	return base + " opacity-0"
}
