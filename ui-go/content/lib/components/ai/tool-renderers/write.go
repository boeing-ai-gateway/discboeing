package toolrenderers

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

type WriteView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type WriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type WriteOutput struct {
	Success      *bool  `json:"success"`
	BytesWritten *int   `json:"bytes_written"`
	Error        string `json:"error"`
}

func parseWriteInput(input string) (WriteInput, bool) {
	if strings.TrimSpace(input) == "" {
		return WriteInput{}, false
	}
	var parsed WriteInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return WriteInput{}, false
	}
	return parsed, parsed.FilePath != ""
}

func parseWriteOutput(output string) (WriteOutput, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return WriteOutput{}, false
	}
	var parsed WriteOutput
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return WriteOutput{}, false
	}
	return parsed, true
}

func writeHeader(input WriteInput, inputOK bool, state string) string {
	if inputOK && input.FilePath != "" {
		return filepath.Base(input.FilePath)
	}
	if isStreamingState(state) {
		return "Loading write details..."
	}
	return "Write file"
}

func writePreview(content string) string {
	if len(content) <= 400 {
		return content
	}
	return content[:400]
}

func writeLineCount(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(normalizeNewlines(content), "\n") + 1
}

func writeError(view WriteView, output WriteOutput) string {
	if view.ErrorText != "" {
		return view.ErrorText
	}
	return output.Error
}
