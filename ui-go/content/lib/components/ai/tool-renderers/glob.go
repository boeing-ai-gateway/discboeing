package toolrenderers

import (
	"encoding/json"
	"strings"
)

type GlobView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type GlobInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

type GlobOutput struct {
	Files   []string `json:"files"`
	Content string   `json:"content"`
}

func parseGlobInput(input string) (GlobInput, bool) {
	if strings.TrimSpace(input) == "" {
		return GlobInput{}, false
	}
	var parsed GlobInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return GlobInput{}, false
	}
	return parsed, parsed.Pattern != ""
}

func parseGlobOutput(output string) (GlobOutput, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return GlobOutput{}, false
	}
	var object GlobOutput
	if err := json.Unmarshal([]byte(trimmed), &object); err == nil {
		return object, true
	}
	var files []string
	if err := json.Unmarshal([]byte(trimmed), &files); err == nil {
		return GlobOutput{Files: files}, true
	}
	var content string
	if err := json.Unmarshal([]byte(trimmed), &content); err == nil {
		return GlobOutput{Content: content}, true
	}
	return GlobOutput{Content: output}, true
}

func globHeader(input GlobInput, inputOK bool, state string) string {
	if inputOK && input.Pattern != "" {
		return input.Pattern
	}
	if isStreamingState(state) {
		return "Loading file search..."
	}
	return "Glob"
}
