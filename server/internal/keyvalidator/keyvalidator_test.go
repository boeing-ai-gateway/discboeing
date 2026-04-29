package keyvalidator

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListModelsValidator_OpenAIUsesBearerToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("expected /v1/models path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-openai-test" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	validator := &listModelsValidator{
		provider:    "openai",
		displayName: "OpenAI",
		url:         server.URL + "/v1/models",
		client:      server.Client(),
		buildHeaders: func(apiKey string) http.Header {
			headers := make(http.Header)
			headers.Set("Authorization", "Bearer "+apiKey)
			return headers
		},
	}

	if err := validator.Validate(context.Background(), "sk-openai-test"); err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
}

func TestListModelsValidator_AnthropicUsesExpectedHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("expected /v1/models path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "sk-ant-test" {
			t.Fatalf("expected x-api-key header, got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("expected anthropic-version header, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	validator := &listModelsValidator{
		provider:    "anthropic",
		displayName: "Anthropic",
		url:         server.URL + "/v1/models",
		client:      server.Client(),
		buildHeaders: func(apiKey string) http.Header {
			headers := make(http.Header)
			headers.Set("x-api-key", apiKey)
			headers.Set("anthropic-version", "2023-06-01")
			return headers
		},
	}

	if err := validator.Validate(context.Background(), "sk-ant-test"); err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
}

func TestListModelsValidator_ReturnsValidationErrorForRejectedKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad API key"}}`))
	}))
	defer server.Close()

	validator := &listModelsValidator{
		provider:    "openai",
		displayName: "OpenAI",
		url:         server.URL,
		client:      server.Client(),
		buildHeaders: func(apiKey string) http.Header {
			headers := make(http.Header)
			headers.Set("Authorization", "Bearer "+apiKey)
			return headers
		},
	}

	err := validator.Validate(context.Background(), "sk-openai-test")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("expected validation failure, got %v", err)
	}
	if got := err.Error(); got != "OpenAI rejected the API key: bad API key" {
		t.Fatalf("unexpected error message %q", got)
	}
}
