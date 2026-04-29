package cli

import (
	"context"
	"strings"

	"github.com/obot-platform/discobot/agent-go/internal/clisession"
)

func getThreadPlanMode(ctx context.Context, session clisession.Session, threadID string) bool {
	thread, err := session.GetThread(ctx, threadID)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(thread.Mode), "plan")
}

func planModeRequest(enabled bool) string {
	if enabled {
		return "plan"
	}
	return "build"
}
