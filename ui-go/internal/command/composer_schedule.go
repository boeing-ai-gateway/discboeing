package command

import (
	"net/http"
	"strconv"
	"time"

	"github.com/obot-platform/discobot/ui-go/content/lib/components/app"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
	"github.com/obot-platform/discobot/ui-go/internal/readmodel"
)

// ComposerSchedule handles the composer run-after popover state and selections.
func (h *Handler) ComposerSchedule(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.logger.Warn("failed to parse composer schedule form", "error", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	action := r.URL.Query().Get("action")
	if action == "" {
		http.Error(w, "invalid schedule command", http.StatusBadRequest)
		return
	}

	session.Save(func(view *viewmodel.ShellSnapshot) {
		composer := &view.Workspace.Composer
		switch action {
		case "toggle":
			composer.ScheduleOpen = !composer.ScheduleOpen
		case "later":
			minutes, err := strconv.Atoi(r.URL.Query().Get("offset_minutes"))
			if err != nil || minutes <= 0 {
				return
			}
			composer.ScheduledRunAfter = time.Now().Add(time.Duration(minutes) * time.Minute).Format(time.RFC3339)
			composer.ScheduleOpen = false
		case "pause":
			composer.ScheduledRunAfter = buildComposerPauseDate().Format(time.RFC3339)
			composer.ScheduleOpen = false
		case "run-now":
			composer.ScheduledRunAfter = ""
			composer.ScheduleOpen = false
		case "save-custom":
			parsed, ok := parseComposerCustomRunAfter(r.FormValue("schedule_custom_run_after"))
			if !ok {
				return
			}
			composer.ScheduledRunAfter = parsed.Format(time.RFC3339)
			composer.ScheduleOpen = false
		}
	})

	view := session.View()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := app.AppShell(readmodel.BuildShellFromBackend(view, h.live.Snapshot(readmodel.LiveScopeFromView(view)))).Render(r.Context(), w); err != nil {
		h.logger.Warn("failed to render composer schedule patch", "error", err)
	}
}

func buildComposerPauseDate() time.Time {
	now := time.Now()
	return now.AddDate(100, 0, 0)
}

func parseComposerCustomRunAfter(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.ParseInLocation("2006-01-02T15:04", value, time.Local)
	return parsed, err == nil
}
