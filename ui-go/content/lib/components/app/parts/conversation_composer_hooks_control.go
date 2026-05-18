package parts

import (
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func composerHookPassedCount(hooks []viewmodel.HookStatus) int {
	count := 0
	for _, hook := range hooks {
		if hook.DisplayState == "success" {
			count++
		}
	}
	return count
}

func composerHookHasState(hooks []viewmodel.HookStatus, state string) bool {
	for _, hook := range hooks {
		if hook.DisplayState == state {
			return true
		}
	}
	return false
}

func composerHooksControlState(hooks []viewmodel.HookStatus) string {
	if composerHookHasState(hooks, "running") {
		return "running"
	}
	if composerHookHasState(hooks, "failure") {
		return "failure"
	}
	return "success"
}

func composerHooksControlLabel(snapshot viewmodel.ConversationHooksPanelSnapshot) string {
	return "Hooks: " + strconv.Itoa(composerHookPassedCount(snapshot.Hooks)) + " of " + strconv.Itoa(len(snapshot.Hooks)) + " passed"
}
