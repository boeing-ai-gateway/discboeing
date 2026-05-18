package parts

import (
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

const maxDevErrors = 20

func devErrorOverlayVisible(snapshot viewmodel.DevErrorOverlaySnapshot) bool {
	return snapshot.Enabled && len(snapshot.Errors) > 0
}

func devErrorID(id int) string {
	return strconv.Itoa(id)
}

func devErrorCopyLabel(snapshot viewmodel.DevErrorOverlaySnapshot, errorID int) string {
	if snapshot.CopiedID == errorID {
		return "Copied"
	}

	return "Copy"
}

func devErrorTitleID(id int) string {
	return "dev-error-title-" + devErrorID(id)
}
