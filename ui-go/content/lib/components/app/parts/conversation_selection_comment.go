package parts

import (
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func selectionCommentRootClass(snapshot viewmodel.ConversationSelectionCommentSnapshot) string {
	if snapshot.Enabled {
		return "contents"
	}

	return "hidden"
}

func selectionCommentButtonStyle(snapshot viewmodel.ConversationSelectionCommentSnapshot) string {
	return "left: " + strconv.Itoa(snapshot.Left) + "px; top: " + strconv.Itoa(snapshot.Top) + "px;"
}

func selectionCommentEditorStyle(snapshot viewmodel.ConversationSelectionCommentSnapshot) string {
	left := snapshot.Left
	if left > 0 {
		left = max(left, 12)
	}

	return "left: min(" + strconv.Itoa(left) + "px, calc(100vw - 340px)); top: " + strconv.Itoa(snapshot.Top) + "px;"
}

func selectionCommentSubmitLabel(submitting bool) string {
	if submitting {
		return "Submitting…"
	}

	return "Submit"
}
