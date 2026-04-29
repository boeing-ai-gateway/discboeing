package keyvalidator

import "net/http"

func newAnthropicValidator(client *http.Client) Validator {
	return &listModelsValidator{
		provider:    "anthropic",
		displayName: "Anthropic",
		url:         "https://api.anthropic.com/v1/models",
		client:      client,
		buildHeaders: func(apiKey string) http.Header {
			headers := make(http.Header)
			headers.Set("x-api-key", apiKey)
			headers.Set("anthropic-version", "2023-06-01")
			return headers
		},
		accept429Rate: true,
	}
}
