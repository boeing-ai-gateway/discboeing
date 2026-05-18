package app

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func supportInfoCloseCommand() string {
	return "@post('/ui/commands/settings/action?action=support-close')"
}

func supportInfoCanUse(snapshot viewmodel.SupportInfoSnapshot) bool {
	return snapshot.JSON != "" && snapshot.Status != "loading" && snapshot.Status != "error"
}

func supportInfoError(snapshot viewmodel.SupportInfoSnapshot) string {
	if snapshot.Error != "" {
		return snapshot.Error
	}
	return "Failed to load support information."
}
