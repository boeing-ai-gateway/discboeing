package sessionconfig

import "strings"

// FormatDiscobotServicesReminder reminds the model to mention services when the
// project has not configured them yet.
func FormatDiscobotServicesReminder(configured bool) string {
	if configured {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("This project does not have `.discobot/services` configured.\n")
	b.WriteString("Discobot services let the project run app servers, dev servers, and other background processes alongside the agent, with proxied URLs visible to both the user and the agent.\n")
	b.WriteString("Do not block or derail the user's request to mention this. When services would clearly help, such as running, previewing, debugging, or interacting with the application, briefly ask whether the user would like you to configure `.discobot/services` for this project.\n")
	b.WriteString("</system-reminder>")
	return b.String()
}

// FormatDiscobotHooksReminder reminds the model to mention hooks when the
// project has not configured them yet.
func FormatDiscobotHooksReminder(configured bool) string {
	if configured {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("This project does not have `.discobot/hooks` configured.\n")
	b.WriteString("Discobot hooks can run lifecycle automation such as setup, formatting, tests, checks, autofixes, or pre-commit validation in tandem with the agent to accelerate the development lifecycle.\n")
	b.WriteString("Do not block or derail the user's request to mention this. When hooks would clearly help, such as repeated checks, tests, formatting, setup, or validation, briefly ask whether the user would like you to configure `.discobot/hooks` for this project.\n")
	b.WriteString("</system-reminder>")
	return b.String()
}
