package toolrenderers

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

type ReadView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type ReadInput struct {
	FilePath string `json:"file_path"`
	Limit    *int   `json:"limit"`
	Offset   *int   `json:"offset"`
	Pages    string `json:"pages"`
}

type ReadOutput struct {
	Content string            `json:"content"`
	Lines   []string          `json:"lines"`
	Type    string            `json:"type"`
	Value   []ReadContentItem `json:"value"`
	Error   string            `json:"error"`
}

type ReadContentItem struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Data      string `json:"data"`
	URL       string `json:"url"`
	MediaType string `json:"mediaType"`
	Filename  string `json:"filename"`
}

func parseReadInput(input string) (ReadInput, bool) {
	if strings.TrimSpace(input) == "" {
		return ReadInput{}, false
	}
	var parsed ReadInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return ReadInput{}, false
	}
	return parsed, parsed.FilePath != ""
}

func parseReadOutput(output string) (ReadOutput, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ReadOutput{}, false
	}
	var parsed ReadOutput
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed, true
	}
	var content string
	if err := json.Unmarshal([]byte(trimmed), &content); err == nil {
		return ReadOutput{Content: content}, true
	}
	return ReadOutput{Content: output}, true
}

func readHeader(input ReadInput, inputOK bool, state string) string {
	if inputOK && input.FilePath != "" {
		return filepath.Base(input.FilePath)
	}
	if isStreamingState(state) {
		return "Loading file details..."
	}
	return "Reading file"
}

func readFileName(input ReadInput) string {
	if input.FilePath == "" {
		return ""
	}
	return filepath.Base(input.FilePath)
}

func readContent(output ReadOutput) string {
	if output.Content != "" {
		return output.Content
	}
	if len(output.Lines) > 0 {
		return strings.Join(output.Lines, "\n")
	}
	var parts []string
	for _, item := range output.Value {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func readImageItems(output ReadOutput) []ReadContentItem {
	var images []ReadContentItem
	for _, item := range output.Value {
		if item.Type == "image-data" || item.Type == "image-url" || ((item.Type == "file-data" || item.Type == "media") && strings.HasPrefix(item.MediaType, "image/")) {
			images = append(images, item)
		}
	}
	return images
}

func readImageSrc(item ReadContentItem) string {
	if item.Type == "image-url" {
		return item.URL
	}
	if item.Data == "" {
		return ""
	}
	if strings.HasPrefix(item.Data, "data:") || strings.HasPrefix(item.Data, "http") {
		return item.Data
	}
	mediaType := item.MediaType
	if mediaType == "" {
		mediaType = "image/png"
	}
	return "data:" + mediaType + ";base64," + item.Data
}

func readImageLabel(item ReadContentItem) string {
	if item.Filename != "" {
		return item.Filename
	}
	return item.MediaType
}
