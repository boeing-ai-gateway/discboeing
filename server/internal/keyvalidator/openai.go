package keyvalidator

import "net/http"

func newOpenAIValidator(client *http.Client) Validator {
	return &listModelsValidator{
		provider:    "openai",
		displayName: "OpenAI",
		url:         "https://api.openai.com/v1/models",
		client:      client,
		buildHeaders: func(apiKey string) http.Header {
			headers := make(http.Header)
			headers.Set("Authorization", "Bearer "+apiKey)
			return headers
		},
		accept429Rate: true,
	}
}
