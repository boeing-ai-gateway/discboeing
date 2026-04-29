package keyvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type listModelsValidator struct {
	provider      string
	displayName   string
	url           string
	client        *http.Client
	buildHeaders  func(apiKey string) http.Header
	accept429Rate bool
}

func (v *listModelsValidator) Validate(ctx context.Context, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return &ValidationError{
			Provider: v.displayName,
			Message:  fmt.Sprintf("%s API key is required", v.displayName),
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return fmt.Errorf("build %s API key validation request: %w", v.displayName, err)
	}
	for key, values := range v.buildHeaders(apiKey) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("validate %s API key: %w", v.displayName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	if v.accept429Rate && resp.StatusCode == http.StatusTooManyRequests {
		return nil
	}

	message := extractProviderErrorMessage(resp.Body)
	if isRejectedStatus(resp.StatusCode) {
		if message == "" {
			message = fmt.Sprintf("%s rejected the API key", v.displayName)
		} else {
			message = fmt.Sprintf("%s rejected the API key: %s", v.displayName, message)
		}
		return &ValidationError{
			Provider: v.displayName,
			Message:  message,
		}
	}

	if message != "" {
		return fmt.Errorf("%s API key validation returned %s: %s", v.displayName, resp.Status, message)
	}
	return fmt.Errorf("%s API key validation returned %s", v.displayName, resp.Status)
}

func isRejectedStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
		return true
	default:
		return false
	}
}

func extractProviderErrorMessage(body io.Reader) string {
	const maxBodyBytes = 8192
	payload, err := io.ReadAll(io.LimitReader(body, maxBodyBytes))
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return ""
	}

	var nested struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &nested); err == nil && strings.TrimSpace(nested.Error.Message) != "" {
		return strings.TrimSpace(nested.Error.Message)
	}

	var flat struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(payload, &flat); err == nil && strings.TrimSpace(flat.Error) != "" {
		return strings.TrimSpace(flat.Error)
	}

	return trimmed
}
