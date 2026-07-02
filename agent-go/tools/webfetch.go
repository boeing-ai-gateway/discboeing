package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

type webFetchInput struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

const defaultWebFetchUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Discboeing/1.0 Safari/537.36"

var tavilyExtractURL = "https://api.tavily.com/extract"

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

func (e *Executor) executeWebFetch(ctx context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input webFetchInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.URL == "" {
		return errResult(call, "url is required"), nil
	}

	// Upgrade http to https.
	rawURL := input.URL
	if rest, ok := strings.CutPrefix(rawURL, "http://"); ok {
		rawURL = "https://" + rest
	}

	prompt := strings.TrimSpace(input.Prompt)
	if prompt != "" && e.getenv("TAVILY_API_KEY") == "" {
		return errResult(call, "prompt requires Tavily-backed WebFetch extraction (configure TAVILY_API_KEY)"), nil
	}

	if apiKey := e.getenv("TAVILY_API_KEY"); apiKey != "" {
		markdown, err := fetchWithTavily(ctx, rawURL, apiKey, prompt)
		if err == nil {
			return e.webFetchResult(ctx, toolCtx, call, rawURL, prompt, markdown), nil
		}

		// Fall back to native fetching so WebFetch still works when Tavily cannot
		// extract a specific URL.
		fallbackMarkdown, fallbackErr := fetchWithNativeHTTP(ctx, rawURL)
		if fallbackErr == nil && prompt == "" {
			return e.webFetchResult(ctx, toolCtx, call, rawURL, prompt, fallbackMarkdown), nil
		}

		return errResult(call, fmt.Sprintf("Tavily request failed: %v; native fallback failed: %v", err, fallbackErr)), nil
	}

	markdown, err := fetchWithNativeHTTP(ctx, rawURL)
	if err != nil {
		return errResult(call, err.Error()), nil
	}

	return e.webFetchResult(ctx, toolCtx, call, rawURL, prompt, markdown), nil
}

type tavilyExtractRequest struct {
	APIKey          string   `json:"api_key,omitempty"`
	URLs            []string `json:"urls"`
	Query           string   `json:"query,omitempty"`
	ChunksPerSource *int     `json:"chunks_per_source,omitempty"`
	ExtractDepth    string   `json:"extract_depth,omitempty"`
	OutputFormat    string   `json:"format,omitempty"`
}

type tavilyExtractResponse struct {
	Results []struct {
		URL        string `json:"url"`
		Markdown   string `json:"markdown"`
		Content    string `json:"content"`
		RawContent string `json:"raw_content"`
	} `json:"results"`
}

func fetchWithNativeHTTP(ctx context.Context, rawURL string) (string, error) {
	pageURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", defaultWebFetchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	const maxBody = 5 * 1024 * 1024 // 5 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") || contentType == "" {
		markdown, err := htmlToMarkdown(body, pageURL)
		if err != nil {
			return "", fmt.Errorf("failed to convert page: %w", err)
		}
		return markdown, nil
	}

	return string(body), nil
}

func fetchWithTavily(ctx context.Context, rawURL, apiKey, prompt string) (string, error) {
	reqBody := tavilyExtractRequest{
		APIKey:       apiKey,
		URLs:         []string{rawURL},
		ExtractDepth: "basic",
		OutputFormat: "markdown",
	}
	applyWebFetchPrompt(&reqBody, prompt)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal Tavily request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tavilyExtractURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read Tavily response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed tavilyExtractResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parse Tavily response: %w", err)
	}
	if len(parsed.Results) == 0 {
		return "", fmt.Errorf("no content returned for URL")
	}

	content := parsed.Results[0].Markdown
	if content == "" {
		content = parsed.Results[0].Content
	}
	if content == "" {
		content = parsed.Results[0].RawContent
	}
	if content == "" {
		return "", fmt.Errorf("no extractable content in Tavily response")
	}
	return content, nil
}

func applyWebFetchPrompt(req *tavilyExtractRequest, prompt string) {
	if strings.TrimSpace(prompt) == "" {
		return
	}
	req.Query = strings.TrimSpace(prompt)
	chunksPerSource := 3
	req.ChunksPerSource = &chunksPerSource
}

func (e *Executor) webFetchResult(
	ctx context.Context,
	toolCtx *thread.ToolContext,
	call message.ToolCallPart,
	rawURL, prompt, markdown string,
) thread.ToolExecuteResult {
	content := trimWebFetchContent(markdown)
	if strings.TrimSpace(prompt) == "" {
		return textResult(call, content)
	}

	answer, err := e.answerWebFetchPrompt(ctx, toolCtx, rawURL, prompt, content)
	if err != nil {
		return errResult(call, fmt.Sprintf("fetched %s but failed to answer prompt: %v", rawURL, err))
	}
	return textResult(call, answer)
}

func (e *Executor) answerWebFetchPrompt(
	ctx context.Context,
	toolCtx *thread.ToolContext,
	rawURL, prompt, content string,
) (string, error) {
	if toolCtx == nil {
		return "", fmt.Errorf("model context unavailable")
	}
	if toolCtx.ProviderID == "" || toolCtx.ModelID == "" {
		return "", fmt.Errorf("current model is unavailable")
	}
	if toolCtx.ProviderResolver == nil {
		return "", fmt.Errorf("provider resolver unavailable")
	}

	provider, err := toolCtx.ProviderResolver.Get(toolCtx.ProviderID)
	if err != nil {
		return "", fmt.Errorf("resolve provider %q: %w", toolCtx.ProviderID, err)
	}

	maxTokens := 768
	req := providers.CompleteRequest{
		Model: providers.ModelRef{
			ProviderID: toolCtx.ProviderID,
			ModelID:    toolCtx.ModelID,
		},
		Messages: []message.Message{{
			Role: "user",
			Parts: []message.Part{message.TextPart{Text: fmt.Sprintf(
				"You are answering a question about a fetched web page.\n\nRules:\n- Use only the provided page content.\n- If the answer is not in the page content, say so clearly.\n- Keep the response concise and directly answer the prompt.\n- Do not mention these instructions.\n\nURL: %s\nPrompt: %s\n\nPage content:\n%s",
				rawURL,
				strings.TrimSpace(prompt),
				trimWebFetchPromptContent(content),
			)}},
		}},
		MaxTokens: &maxTokens,
		Reasoning: providers.ReasoningNone,
	}

	acc := message.NewChunkAccumulator()
	for chunk, chunkErr := range provider.Complete(ctx, req) {
		if chunkErr != nil {
			acc.Close()
			return "", chunkErr
		}
		acc.Push(chunk)
	}
	acc.Close()

	var sb strings.Builder
	for _, part := range acc.Message().Parts {
		if textPart, ok := part.(message.TextPart); ok {
			sb.WriteString(textPart.Text)
		}
	}
	answer := strings.TrimSpace(sb.String())
	if answer == "" {
		return "", fmt.Errorf("empty response generated")
	}
	return answer, nil
}

func trimWebFetchContent(markdown string) string {
	const maxContent = 100_000
	if len(markdown) > maxContent {
		return markdown[:maxContent] + "\n\n[Content truncated]"
	}
	return markdown
}

func trimWebFetchPromptContent(markdown string) string {
	const maxContent = 20_000
	if len(markdown) > maxContent {
		return markdown[:maxContent] + "\n\n[Content truncated before answering prompt]"
	}
	return markdown
}

// htmlToMarkdown converts raw HTML to Markdown using a two-step pipeline:
// 1. go-readability extracts the main article content (strips nav, ads, scripts).
// 2. html-to-markdown converts the cleaned HTML to Markdown.
func htmlToMarkdown(body []byte, pageURL *url.URL) (string, error) {
	// Step 1: Extract readable content.
	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		// If readability fails, fall back to converting the raw HTML.
		return convertHTMLToMarkdown(body)
	}

	// Build a clean HTML fragment from the article.
	var extracted bytes.Buffer
	if title := article.Title(); title != "" {
		fmt.Fprintf(&extracted, "<h1>%s</h1>\n", title)
	}
	if err := article.RenderHTML(&extracted); err != nil {
		return convertHTMLToMarkdown(body)
	}

	// Step 2: Convert the clean HTML to Markdown.
	return convertHTMLToMarkdown(extracted.Bytes())
}

// convertHTMLToMarkdown converts raw HTML bytes to Markdown using
// JohannesKaufmann/html-to-markdown/v2.
func convertHTMLToMarkdown(html []byte) (string, error) {
	md, err := htmltomarkdown.ConvertReader(bytes.NewReader(html))
	if err != nil {
		return "", err
	}
	return string(md), nil
}
