package app

import (
	"strings"

	aimessage "github.com/obot-platform/discobot/ui-go/content/lib/components/ai/message"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func conversationMessages(snapshot viewmodel.SessionWorkspaceSnapshot) []viewmodel.ConversationMessage {
	if len(snapshot.Conversation.Messages) > 0 {
		return snapshot.Conversation.Messages
	}
	if strings.TrimSpace(snapshot.Message) == "" {
		return nil
	}
	return []viewmodel.ConversationMessage{
		{
			ID:      "workspace-summary",
			Role:    "assistant",
			Content: snapshot.Message,
		},
	}
}

func conversationMessageFrom(role string) string {
	if role == "user" {
		return "user"
	}
	return "assistant"
}

func conversationMessageContent(row viewmodel.ConversationMessage) string {
	if len(row.Branches) == 0 {
		return row.Content
	}
	if row.CurrentBranch < 0 || row.CurrentBranch >= len(row.Branches) {
		return row.Branches[0]
	}
	return row.Branches[row.CurrentBranch]
}

func conversationMessageBranch(row viewmodel.ConversationMessage) aimessage.BranchView {
	current := row.CurrentBranch
	if current < 0 || current >= len(row.Branches) {
		current = 0
	}
	return aimessage.BranchView{
		MessageID: row.ID,
		Current:   current,
		Total:     len(row.Branches),
	}
}

func conversationContentClass(chatWidthMode string) string {
	base := "w-full min-w-0 space-y-4"
	if chatWidthMode == "constrained" || chatWidthMode == "" {
		return base + " mx-auto max-w-3xl"
	}
	return base
}

func conversationPaneBodyClass(hasMessages bool) string {
	base := "flex min-h-0 flex-1 flex-col transition-all duration-300 ease-out"
	if hasMessages {
		return base
	}
	return base + " justify-end md:justify-center"
}

func showConversationComposer(snapshot viewmodel.SessionWorkspaceSnapshot) bool {
	if snapshot.Conversation.Status == "" {
		return true
	}
	return snapshot.Conversation.ShowComposer
}
