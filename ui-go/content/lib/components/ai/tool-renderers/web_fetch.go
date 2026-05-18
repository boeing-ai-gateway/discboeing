package toolrenderers

import (
	"encoding/json"
	"strings"
)

type WebFetchView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type WebFetchInput struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

type WebFetchOutput struct {
	Content string `json:"content"`
	Error   string `json:"error"`
}

func parseWebFetchInput(input string) (WebFetchInput, bool) {
	if strings.TrimSpace(input) == "" {
		return WebFetchInput{}, false
	}
	var parsed WebFetchInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return WebFetchInput{}, false
	}
	return parsed, parsed.URL != ""
}

func parseWebFetchOutput(output string) (WebFetchOutput, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return WebFetchOutput{}, false
	}
	var parsed WebFetchOutput
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed, true
	}
	var content string
	if err := json.Unmarshal([]byte(trimmed), &content); err == nil {
		return WebFetchOutput{Content: content}, true
	}
	return WebFetchOutput{}, false
}

func webFetchHeader(input WebFetchInput, inputOK bool, state string) string {
	if isStreamingState(state) {
		return "Loading web fetch..."
	}
	if inputOK && input.URL != "" {
		return "Web fetch"
	}
	return "Web fetch"
}

func webFetchError(view WebFetchView, output WebFetchOutput) string {
	if view.ErrorText != "" {
		return view.ErrorText
	}
	return output.Error
}
