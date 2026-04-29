package keyvalidator

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"
)

// ErrValidationFailed marks errors where a provider rejected a supplied API key.
var ErrValidationFailed = errors.New("api key validation failed")

// Validator validates a provider-specific API key.
type Validator interface {
	Validate(ctx context.Context, apiKey string) error
}

// Registry stores provider-specific key validators.
type Registry struct {
	validators map[string]Validator
}

// ValidationError is returned when a provider rejects a key during validation.
type ValidationError struct {
	Provider string
	Message  string
	Err      error
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "api key validation failed"
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if strings.TrimSpace(e.Provider) != "" {
		return fmt.Sprintf("%s API key validation failed", e.Provider)
	}
	return "api key validation failed"
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return ErrValidationFailed
	}
	if e.Err != nil {
		return errors.Join(ErrValidationFailed, e.Err)
	}
	return ErrValidationFailed
}

// NewRegistry creates a validator registry from an explicit provider map.
func NewRegistry(validators map[string]Validator) *Registry {
	cloned := make(map[string]Validator, len(validators))
	maps.Copy(cloned, validators)
	return &Registry{validators: cloned}
}

// DefaultRegistry creates the built-in validator set.
func DefaultRegistry(client *http.Client) *Registry {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return NewRegistry(map[string]Validator{
		"anthropic": newAnthropicValidator(client),
		"openai":    newOpenAIValidator(client),
	})
}

// ValidateAPIKey validates the provider key when a validator is registered.
// Providers without validators are treated as success.
func (r *Registry) ValidateAPIKey(ctx context.Context, provider, apiKey string) error {
	if r == nil {
		return nil
	}
	validator, ok := r.validators[provider]
	if !ok || validator == nil {
		return nil
	}
	return validator.Validate(ctx, apiKey)
}

// HasValidator reports whether the registry has a validator for the provider.
func (r *Registry) HasValidator(provider string) bool {
	if r == nil {
		return false
	}
	validator, ok := r.validators[provider]
	return ok && validator != nil
}
