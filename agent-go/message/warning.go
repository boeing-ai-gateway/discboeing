package message

import "time"

// Warning is a non-fatal issue reported by the provider.
type Warning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// FinishReason describes why generation stopped.
type FinishReason struct {
	// Unified is the normalized reason: "stop", "length", "content-filter",
	// "tool-calls", "error", or "other".
	Unified string `json:"unified"`
	// Raw is the provider-specific reason string, if available.
	Raw string `json:"raw,omitempty"`
}

// Usage contains token usage statistics from a completion.
type Usage struct {
	InputTokens  InputTokens  `json:"inputTokens"`
	OutputTokens OutputTokens `json:"outputTokens"`
}

func (u Usage) IsZero() bool {
	return usageIsZero(u)
}

func usageIsZero(u Usage) bool {
	return u.InputTokens.Total == 0 &&
		u.InputTokens.NoCache == 0 &&
		u.InputTokens.CacheRead == 0 &&
		u.InputTokens.CacheWrite == 0 &&
		u.OutputTokens.Total == 0 &&
		u.OutputTokens.Text == 0 &&
		u.OutputTokens.Reasoning == 0
}

// TokenPrices contains model token prices in USD per million tokens.
type TokenPrices struct {
	Input  float64 `json:"input,omitempty"`
	Output float64 `json:"output,omitempty"`
}

func (p TokenPrices) IsZero() bool {
	return p.Input == 0 && p.Output == 0
}

// InputTokens breaks down input token usage.
type InputTokens struct {
	Total      int `json:"total"`
	NoCache    int `json:"noCache,omitempty"`
	CacheRead  int `json:"cacheRead,omitempty"`
	CacheWrite int `json:"cacheWrite,omitempty"`
}

// OutputTokens breaks down output token usage.
type OutputTokens struct {
	Total     int `json:"total"`
	Text      int `json:"text,omitempty"`
	Reasoning int `json:"reasoning,omitempty"`
}

// ResponseMetadata provides metadata about a provider response.
type ResponseMetadata struct {
	ID        string     `json:"id,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
	ModelID   string     `json:"modelId,omitempty"`
}
