package toolrenderers

import (
	"encoding/json"
	"strings"
)

type WebSearchView struct {
	Input     string
	Output    string
	ErrorText string
	State     string
	Open      bool
	Raw       bool
	Queued    bool
}

type WebSearchInput struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
}

type WebSearchOutput struct {
	Results []WebSearchResult `json:"results"`
	Content string            `json:"content"`
}

type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Favicon string `json:"favicon"`
}

func parseWebSearchInput(input string) (WebSearchInput, bool) {
	if strings.TrimSpace(input) == "" {
		return WebSearchInput{}, false
	}
	var parsed WebSearchInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return WebSearchInput{}, false
	}
	return parsed, parsed.Query != ""
}

func parseWebSearchOutput(output string) (WebSearchOutput, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return WebSearchOutput{}, false
	}
	var parsed WebSearchOutput
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed, true
	}
	var content string
	if err := json.Unmarshal([]byte(trimmed), &content); err == nil {
		return WebSearchOutput{Content: content}, true
	}
	return WebSearchOutput{}, false
}

func webSearchHeader(input WebSearchInput, inputOK bool, state string) string {
	if inputOK && input.Query != "" {
		return input.Query
	}
	if isStreamingState(state) {
		return "Loading web search..."
	}
	return "Web search"
}
