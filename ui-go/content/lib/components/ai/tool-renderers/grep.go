package toolrenderers

import (
	"encoding/json"
	"strings"
)

type GrepView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type GrepInput struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path"`
	Glob       string `json:"glob"`
	Type       string `json:"type"`
	OutputMode string `json:"output_mode"`
}

type GrepOutput struct {
	Content string      `json:"content"`
	Files   []string    `json:"files"`
	Count   *int        `json:"count"`
	Matches []GrepMatch `json:"matches"`
}

type GrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func parseGrepInput(input string) (GrepInput, bool) {
	if strings.TrimSpace(input) == "" {
		return GrepInput{}, false
	}
	var parsed GrepInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return GrepInput{}, false
	}
	return parsed, parsed.Pattern != ""
}

func parseGrepOutput(output string) (GrepOutput, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return GrepOutput{}, false
	}
	var parsed GrepOutput
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed, true
	}
	var content string
	if err := json.Unmarshal([]byte(trimmed), &content); err == nil {
		return GrepOutput{Content: content}, true
	}
	return GrepOutput{Content: output}, true
}

func grepHeader(input GrepInput, inputOK bool, state string) string {
	if inputOK && input.Pattern != "" {
		return input.Pattern
	}
	if isStreamingState(state) {
		return "Loading search..."
	}
	return "Grep"
}
