package sessionconfig

import "strings"

// FormatDiscboeingServicesReminder reminds the model to mention services when the
// project has not configured them yet.
func FormatDiscboeingServicesReminder(configured bool) string {
	if configured {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("This project does not have `.discboeing/services` configured.\n")
	b.WriteString("Discboeing services let the project run app servers, dev servers, and other background processes alongside the agent, with proxied URLs visible to both the user and the agent.\n")
	b.WriteString("Do not block or derail the user's request to mention this. When services would clearly help, such as running, previewing, debugging, or interacting with the application, briefly ask whether the user would like you to configure `.discboeing/services` for this project.\n")
	b.WriteString("</system-reminder>")
	return b.String()
}

// FormatDiscboeingHooksReminder reminds the model to mention hooks when the
// project has not configured them yet.
func FormatDiscboeingHooksReminder(configured bool) string {
	if configured {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("This project does not have `.discboeing/hooks` configured.\n")
	b.WriteString("Discboeing hooks can run lifecycle automation such as setup, formatting, tests, checks, autofixes, or pre-commit validation in tandem with the agent to accelerate the development lifecycle.\n")
	b.WriteString("Do not block or derail the user's request to mention this. When hooks would clearly help, such as repeated checks, tests, formatting, setup, or validation, briefly ask whether the user would like you to configure `.discboeing/hooks` for this project.\n")
	b.WriteString("</system-reminder>")
	return b.String()
}
