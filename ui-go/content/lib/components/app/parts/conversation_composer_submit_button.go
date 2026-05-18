package parts

func composerSubmitButtonType(status string) string {
	if composerSubmitIsGenerating(status) {
		return "button"
	}
	return "submit"
}

func composerSubmitIsGenerating(status string) bool {
	return status == "submitted" || status == "streaming"
}

func composerSubmitShowPlus(status string, inputEmpty bool, isPending bool) bool {
	return isPending && inputEmpty && !composerSubmitIsGenerating(status)
}

func composerSubmitAriaLabel(status string, inputEmpty bool, isPending bool) string {
	if composerSubmitShowPlus(status, inputEmpty, isPending) {
		return "New session"
	}
	if status == "submitted" || status == "streaming" {
		return "Stop"
	}
	return "Submit"
}

func composerSubmitAction(status string, inputEmpty bool, isPending bool) string {
	if composerSubmitShowPlus(status, inputEmpty, isPending) {
		return "new-session"
	}
	if composerSubmitIsGenerating(status) {
		return "stop"
	}
	return "submit"
}

func composerSubmitClickCommand(status string) string {
	if composerSubmitIsGenerating(status) {
		return "@post('/ui/commands/composer-stop')"
	}
	return "true"
}
