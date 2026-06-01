package sync

import (
	"strings"

	agentmessage "github.com/obot-platform/discobot/agent-go/message"
	serverapi "github.com/obot-platform/discobot/server/api"
)

func messageText(message serverapi.Message) string {
	var parts []string
	for _, part := range message.Parts {
		switch part := part.(type) {
		case agentmessage.UITextPart:
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		case agentmessage.UIReasoningPart:
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}
