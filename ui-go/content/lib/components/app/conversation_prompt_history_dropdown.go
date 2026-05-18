package app

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func hasPromptHistoryItems(snapshot viewmodel.ConversationPromptHistorySnapshot) bool {
	return len(snapshot.PinnedPrompts) > 0 || len(snapshot.RecentPrompts) > 0
}

func promptHistoryItemClass(selected bool) string {
	base := "group flex items-start gap-2 px-3 py-2 transition-colors"
	if selected {
		return base + " bg-accent"
	}
	return base + " hover:bg-accent"
}

func promptHistoryPinnedSelected(snapshot viewmodel.ConversationPromptHistorySnapshot, index int) bool {
	return snapshot.HasSelection && snapshot.SelectionPinned && snapshot.SelectedIndex == index
}

func promptHistoryRecentSelected(snapshot viewmodel.ConversationPromptHistorySnapshot, index int) bool {
	return snapshot.HasSelection && !snapshot.SelectionPinned && snapshot.SelectedIndex == index
}
