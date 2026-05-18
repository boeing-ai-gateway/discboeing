package parts

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func queuedPromptText(entry viewmodel.QueuedPrompt) string {
	if strings.TrimSpace(entry.Text) != "" {
		return entry.Text
	}
	return "Queued prompt"
}

func queuedPromptAttachmentLabel(count int) string {
	if count == 1 {
		return "1 attachment"
	}
	return strconv.Itoa(count) + " attachments"
}

func queuedPromptRunAfterLabel(entry viewmodel.QueuedPrompt) string {
	if entry.RunAfterLabel != "" {
		return entry.RunAfterLabel
	}
	if entry.RunAfter != "" {
		return entry.RunAfter
	}
	return ""
}

func queuedPromptEditTitle(entry viewmodel.QueuedPrompt) string {
	if entry.Editing {
		return "Cancel editing queued prompt"
	}
	return "Edit queued prompt"
}

func queuedPromptCommand(id string, action string) string {
	values := url.Values{}
	values.Set("prompt_id", id)
	values.Set("action", action)
	return "@post('/ui/commands/prompt-queue/action?" + values.Encode() + "')"
}

func queuedPromptScheduleCommand(id string, action string, offsetMinutes string) string {
	values := url.Values{}
	values.Set("prompt_id", id)
	values.Set("action", action)
	if offsetMinutes != "" {
		values.Set("offset_minutes", offsetMinutes)
	}
	return "@post('/ui/commands/prompt-queue/action?" + values.Encode() + "')"
}

func queuedPromptSchedulePopoverClass(entry viewmodel.QueuedPrompt) string {
	base := "absolute right-0 top-full z-50 mt-1 w-72 rounded-md border border-border bg-popover p-3 text-popover-foreground shadow-md"
	if !entry.ScheduleOpen {
		return base + " hidden"
	}
	return base
}
