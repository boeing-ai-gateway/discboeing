package parts

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func composerScheduleDisabled(snapshot viewmodel.ConversationComposerSnapshot) bool {
	return strings.TrimSpace(snapshot.DisabledMessage) != ""
}

func composerScheduleCommand(action string) string {
	return "@post('" + composerScheduleURL(action) + "', {contentType: 'form'})"
}

func composerScheduleURL(action string) string {
	values := url.Values{}
	values.Set("action", action)
	return "/ui/commands/composer-schedule?" + values.Encode()
}

func composerScheduleOffsetCommand(minutes int) string {
	values := url.Values{}
	values.Set("action", "later")
	values.Set("offset_minutes", strconv.Itoa(minutes))
	return "@post('/ui/commands/composer-schedule?" + values.Encode() + "', {contentType: 'form'})"
}

func composerScheduleTitle(snapshot viewmodel.ConversationComposerSnapshot) string {
	if snapshot.ScheduledRunAfter == "" {
		return "Schedule prompt"
	}
	if composerSchedulePaused(snapshot.ScheduledRunAfter) {
		return "Submit paused prompt"
	}
	if parsed, ok := parseScheduledRunAfter(snapshot.ScheduledRunAfter); ok {
		return "Submit scheduled prompt for " + parsed.Format("1/2/2006, 3:04:05 PM")
	}
	return "Schedule prompt"
}

func composerScheduleButtonClass(snapshot viewmodel.ConversationComposerSnapshot) string {
	base := "focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:ring-[3px] disabled:pointer-events-none disabled:opacity-50 size-8"
	if snapshot.ScheduledRunAfter != "" {
		return base + " bg-primary text-primary-foreground hover:bg-primary/90"
	}
	return base + " hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50"
}

func composerScheduleCustomValue(snapshot viewmodel.ConversationComposerSnapshot) string {
	if parsed, ok := parseScheduledRunAfter(snapshot.ScheduledRunAfter); ok {
		return parsed.Format("2006-01-02T15:04")
	}
	return time.Now().Add(time.Hour).Format("2006-01-02T15:04")
}

func parseScheduledRunAfter(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed, true
	}
	parsed, err = time.Parse("2006-01-02T15:04", value)
	return parsed, err == nil
}

func composerSchedulePaused(value string) bool {
	parsed, ok := parseScheduledRunAfter(value)
	return ok && parsed.After(time.Now().AddDate(25, 0, 0))
}
