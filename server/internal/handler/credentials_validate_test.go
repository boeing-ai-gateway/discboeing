package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/keyvalidator"
	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/service"
)

type stubHandlerKeyValidator struct {
	err error
}

func (v *stubHandlerKeyValidator) Validate(_ context.Context, _ string) error {
	return v.err
}

func TestValidateCredentials_ReturnsValidationResults(t *testing.T) {
	t.Parallel()

	st := setupChatTestStore(t)
	credSvc, err := service.NewCredentialServiceWithValidators(
		st,
		&config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789")},
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{
			service.ProviderAnthropic: &stubHandlerKeyValidator{err: &keyvalidator.ValidationError{
				Provider: "Anthropic",
				Message:  "Anthropic rejected the API key: invalid x-api-key",
			}},
		}),
	)
	if err != nil {
		t.Fatalf("failed to create credential service: %v", err)
	}

	_, err = credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		testProjectID,
		service.ProviderAnthropic,
		"Anthropic",
		"",
		"sk-ant-test-123",
		service.CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("failed to create credential: %v", err)
	}

	h := &Handler{credentialService: credSvc}
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+testProjectID+"/credentials/validate", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ProjectIDKey, testProjectID))
	w := httptest.NewRecorder()

	h.ValidateCredentials(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Validations []service.CredentialValidationInfo `json:"validations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Validations) != 1 {
		t.Fatalf("expected 1 validation result, got %d", len(response.Validations))
	}
	if response.Validations[0].Status != service.CredentialValidationStatusInvalid {
		t.Fatalf("expected invalid status, got %s", response.Validations[0].Status)
	}
}

func TestValidateCredential_ByIDReturnsValidationResult(t *testing.T) {
	t.Parallel()

	st := setupChatTestStore(t)
	credSvc, err := service.NewCredentialServiceWithValidators(
		st,
		&config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789")},
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{
			service.ProviderAnthropic: &stubHandlerKeyValidator{},
		}),
	)
	if err != nil {
		t.Fatalf("failed to create credential service: %v", err)
	}

	info, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		testProjectID,
		service.ProviderAnthropic,
		"Anthropic",
		"",
		"sk-ant-test-123",
		service.CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("failed to create credential: %v", err)
	}

	h := &Handler{credentialService: credSvc}
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+testProjectID+"/credentials/"+info.ID+"/validate", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", info.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ProjectIDKey, testProjectID))
	w := httptest.NewRecorder()

	h.ValidateCredential(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var validation service.CredentialValidationInfo
	if err := json.Unmarshal(w.Body.Bytes(), &validation); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if validation.CredentialID != info.ID {
		t.Fatalf("expected credential %q, got %q", info.ID, validation.CredentialID)
	}
	if validation.Status != service.CredentialValidationStatusValid {
		t.Fatalf("expected valid status, got %s", validation.Status)
	}
}
