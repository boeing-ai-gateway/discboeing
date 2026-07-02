package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/middleware"
	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/exedev"
	mocksandbox "github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/mock"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
)

func newSandboxProviderTestHandler(t *testing.T) (*Handler, *service.CredentialService) {
	t.Helper()

	st := setupChatTestStore(t)
	if err := st.CreateProject(context.Background(), &model.Project{
		ID:   testProjectID,
		Name: "Test Project",
		Slug: "test-project",
	}); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	if err := st.CreateProjectMember(context.Background(), &model.ProjectMember{
		ProjectID: testProjectID,
		UserID:    "test-user",
		Role:      "owner",
	}); err != nil {
		t.Fatalf("failed to create project member: %v", err)
	}

	cfg := &config.Config{EncryptionKey: []byte("01234567890123456789012345678901")}
	credSvc, err := service.NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("failed to create credential service: %v", err)
	}

	manager := sandbox.NewProviderManager()
	mockProvider := mocksandbox.NewProvider()
	manager.RegisterProvider("mock", mockProvider)
	manager.SetDefault("mock")

	sandboxSvc := service.NewSandboxService(st, mockProvider, cfg, nil, nil, nil, nil)
	sandboxSvc.SetProviderManager(manager)
	sandboxSvc.SetCredentialService(credSvc)

	return &Handler{
		store:             st,
		cfg:               cfg,
		credentialService: credSvc,
		projectService:    service.NewProjectService(st, sandboxSvc),
		sandboxService:    sandboxSvc,
	}, credSvc
}

func sandboxProviderRequest(t *testing.T, method, target string, body any) *http.Request {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, target, reader)
	ctx := context.WithValue(req.Context(), middleware.ProjectIDKey, testProjectID)
	ctx = context.WithValue(ctx, middleware.UserIDKey, "test-user")
	req = req.WithContext(ctx)
	return req
}

type exeDevTestCommandClient struct{}

func (exeDevTestCommandClient) Exec(context.Context, string) ([]byte, error) {
	return nil, nil
}

func TestListSandboxProviderTypes_IncludesCapabilities(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)
	w := httptest.NewRecorder()

	h.ListSandboxProviderTypes(w, sandboxProviderRequest(t, http.MethodGet, "/sandbox-provider-types", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		ProviderTypes []sandboxProviderTypeResponse `json:"providerTypes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.ProviderTypes) != 1 {
		t.Fatalf("expected one provider type, got %#v", response.ProviderTypes)
	}
	providerType := response.ProviderTypes[0]
	if providerType.ID != "mock" || !providerType.Capabilities.Resources || !providerType.Capabilities.ClearCache || providerType.Capabilities.Inspection {
		t.Fatalf("unexpected provider type capabilities: %#v", providerType)
	}
}

func TestSandboxProviderConfigFields_ExeDev(t *testing.T) {
	provider, err := exedev.NewProviderWithClient(exedev.Config{SandboxImage: "discboeing-agent-api:latest"}, exeDevTestCommandClient{})
	if err != nil {
		t.Fatalf("failed to create exe.dev provider: %v", err)
	}
	fields := provider.Definition().ConfigFields
	if len(fields) < 6 {
		t.Fatalf("expected exe.dev config fields, got %#v", fields)
	}

	byKey := map[string]sandbox.ProviderConfigField{}
	for _, field := range fields {
		byKey[field.Key] = field
	}
	for _, key := range []string{"endpoint", "credentialId", "vmHostSuffix", "vmNamePrefix", "stopCommand", "sandboxImage"} {
		if byKey[key].Key == "" {
			t.Fatalf("expected exe.dev field %q in %#v", key, fields)
		}
	}
	if byKey["credentialId"].Type != "credential" || byKey["stopCommand"].Type != "textarea" {
		t.Fatalf("unexpected exe.dev field types: %#v", byKey)
	}
	if byKey["credentialId"].CredentialProvider != "exedev" || byKey["credentialId"].CredentialAuthType != "api_key" {
		t.Fatalf("expected exe.dev credential metadata, got %#v", byKey["credentialId"])
	}
	if byKey["endpoint"].Required || !byKey["credentialId"].Required {
		t.Fatalf("expected exe.dev endpoint to be optional and credential to be required: %#v", byKey)
	}
	for _, key := range []string{"endpoint", "vmHostSuffix", "vmNamePrefix", "stopCommand", "sandboxImage"} {
		if !byKey[key].Advanced {
			t.Fatalf("expected exe.dev field %q to be advanced: %#v", key, byKey[key])
		}
	}
}

func TestCreateSandboxProvider_NameOptional(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)
	w := httptest.NewRecorder()

	h.CreateSandboxProvider(w, sandboxProviderRequest(t, http.MethodPost, "/sandbox-providers", map[string]any{
		"type": "mock",
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var response sandboxProviderInstanceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Name != "Mock" {
		t.Fatalf("expected provider name to default to driver name, got %q", response.Name)
	}
}

func TestSandboxProviderMutationsRequireAdmin(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)
	instance := &model.SandboxProviderInstance{
		ProjectID: testProjectID,
		Type:      "mock",
		Name:      "Mock Instance",
	}
	if err := h.store.CreateSandboxProviderInstance(context.Background(), instance); err != nil {
		t.Fatalf("failed to create provider instance: %v", err)
	}

	tests := []struct {
		name    string
		method  string
		target  string
		body    any
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "create", method: http.MethodPost, target: "/sandbox-providers", body: map[string]any{"type": "mock"}, handler: h.CreateSandboxProvider},
		{name: "update", method: http.MethodPatch, target: "/sandbox-providers/" + instance.ID, body: map[string]any{"disabled": true}, handler: h.UpdateSandboxProvider},
		{name: "delete", method: http.MethodDelete, target: "/sandbox-providers/" + instance.ID, handler: h.DeleteSandboxProvider},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := sandboxProviderRequest(t, tt.method, tt.target, tt.body)
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, "viewer-user")
			if tt.name != "create" {
				rctx := chi.NewRouteContext()
				rctx.URLParams.Add("providerId", instance.ID)
				ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
			}
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			tt.handler(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestUpdateSandboxProvider_DisabledOnlyDoesNotRequireType(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)
	instance := &model.SandboxProviderInstance{
		ProjectID: testProjectID,
		Type:      "mock",
		Name:      "Mock Instance",
	}
	if err := h.store.CreateSandboxProviderInstance(context.Background(), instance); err != nil {
		t.Fatalf("failed to create provider instance: %v", err)
	}

	req := sandboxProviderRequest(t, http.MethodPatch, "/sandbox-providers/"+instance.ID, map[string]any{
		"disabled": true,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerId", instance.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateSandboxProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	updated, err := h.store.GetSandboxProviderInstance(context.Background(), testProjectID, instance.ID)
	if err != nil {
		t.Fatalf("failed to load provider instance: %v", err)
	}
	if !updated.Disabled {
		t.Fatal("expected provider instance to be disabled")
	}
	if updated.Type != instance.Type || updated.Name != instance.Name {
		t.Fatalf("expected type/name to be preserved, got %#v", updated)
	}
}

func TestDeleteSandboxProvider_RejectsDefaultProvider(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)
	instance := &model.SandboxProviderInstance{
		ProjectID: testProjectID,
		Type:      "mock",
		Name:      "Mock Instance",
	}
	if err := h.store.CreateSandboxProviderInstance(context.Background(), instance); err != nil {
		t.Fatalf("failed to create provider instance: %v", err)
	}
	project, err := h.store.GetProjectByID(context.Background(), testProjectID)
	if err != nil {
		t.Fatalf("failed to load project: %v", err)
	}
	project.DefaultSandboxProviderID = instance.ID
	if err := h.store.UpdateProject(context.Background(), project); err != nil {
		t.Fatalf("failed to update project: %v", err)
	}

	req := sandboxProviderRequest(t, http.MethodDelete, "/sandbox-providers/"+instance.ID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerId", instance.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteSandboxProvider(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteSandboxProvider_RejectsProviderUsedBySession(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)
	instance := &model.SandboxProviderInstance{
		ProjectID: testProjectID,
		Type:      "mock",
		Name:      "Mock Instance",
	}
	if err := h.store.CreateSandboxProviderInstance(context.Background(), instance); err != nil {
		t.Fatalf("failed to create provider instance: %v", err)
	}
	if err := h.store.CreateWorkspace(context.Background(), &model.Workspace{
		ID:         "workspace-1",
		ProjectID:  testProjectID,
		Path:       "/tmp/workspace",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	if err := h.store.CreateSession(context.Background(), &model.Session{
		ID:                "session-1",
		ProjectID:         testProjectID,
		WorkspaceID:       "workspace-1",
		SandboxProviderID: instance.ID,
		Name:              "Session",
		SandboxStatus:     model.SessionStatusReady,
		ThreadStatus:      model.SessionActivityStatusIdle,
	}); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := sandboxProviderRequest(t, http.MethodDelete, "/sandbox-providers/"+instance.ID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerId", instance.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteSandboxProvider(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSandboxProviderResources_BuiltinAndInstance(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)

	req := sandboxProviderRequest(t, http.MethodGet, "/sandbox-providers/mock/resources", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerId", "mock")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetSandboxProviderResources(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for built-in provider resources, got %d: %s", w.Code, w.Body.String())
	}
	var builtinResources service.ProviderResources
	if err := json.Unmarshal(w.Body.Bytes(), &builtinResources); err != nil {
		t.Fatalf("failed to decode built-in resources: %v", err)
	}
	if builtinResources.Provider != "mock" || builtinResources.VM.MemoryMB != 4096 {
		t.Fatalf("unexpected built-in resources: %#v", builtinResources)
	}

	instance := &model.SandboxProviderInstance{
		ProjectID: testProjectID,
		Type:      "mock",
		Name:      "Mock Remote",
	}
	if err := h.store.CreateSandboxProviderInstance(context.Background(), instance); err != nil {
		t.Fatalf("failed to create provider instance: %v", err)
	}

	req = sandboxProviderRequest(t, http.MethodGet, "/sandbox-providers/"+instance.ID+"/resources", nil)
	rctx = chi.NewRouteContext()
	rctx.URLParams.Add("providerId", instance.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w = httptest.NewRecorder()

	h.GetSandboxProviderResources(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for provider instance resources, got %d: %s", w.Code, w.Body.String())
	}
	var instanceResources service.ProviderResources
	if err := json.Unmarshal(w.Body.Bytes(), &instanceResources); err != nil {
		t.Fatalf("failed to decode instance resources: %v", err)
	}
	if instanceResources.Provider != "mock" || instanceResources.VM.DataDiskGB != 100 {
		t.Fatalf("unexpected instance resources: %#v", instanceResources)
	}
}

func TestUpdateSandboxProviderResources(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)
	req := sandboxProviderRequest(t, http.MethodPatch, "/sandbox-providers/mock/resources", map[string]any{
		"memoryMB": 8192,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerId", "mock")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateSandboxProviderResources(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var response service.ProviderResourcesUpdateResult
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Current.MemoryMB != 8192 || !response.RestartRequired {
		t.Fatalf("unexpected update response: %#v", response)
	}
}

func TestCreateSandboxProvider_WithCredentialReference(t *testing.T) {
	h, credSvc := newSandboxProviderTestHandler(t)
	credential, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		testProjectID,
		service.ProviderAnthropic,
		"Provider API key",
		"",
		"test-token",
		service.CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("failed to create credential: %v", err)
	}

	w := httptest.NewRecorder()
	h.CreateSandboxProvider(w, sandboxProviderRequest(t, http.MethodPost, "/sandbox-providers", map[string]any{
		"icon": "simple-icons:docker",
		"type": "mock",
		"name": "Mock Remote",
		"config": map[string]any{
			"endpoint":     "https://example.test",
			"credentialId": credential.ID,
		},
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var response sandboxProviderInstanceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Type != "mock" || response.Name != "Mock Remote" {
		t.Fatalf("unexpected provider response: %#v", response)
	}
	if response.Icon != "simple-icons:docker" {
		t.Fatalf("expected provider icon, got %q", response.Icon)
	}
	if response.BuiltIn {
		t.Fatal("created provider instance should not be built in")
	}
	if !response.Available {
		t.Fatal("created provider instance should report available provider type")
	}

	stored, err := h.store.GetSandboxProviderInstance(context.Background(), testProjectID, response.ID)
	if err != nil {
		t.Fatalf("failed to load stored provider: %v", err)
	}
	var config map[string]string
	if err := json.Unmarshal(stored.Config, &config); err != nil {
		t.Fatalf("failed to decode stored config: %v", err)
	}
	if config["credentialId"] != credential.ID {
		t.Fatalf("expected credential reference %q, got %q", credential.ID, config["credentialId"])
	}
	if config["icon"] != "simple-icons:docker" {
		t.Fatalf("expected icon reference, got %q", config["icon"])
	}
}

func TestCreateSandboxProvider_ValidatesCredentialMetadata(t *testing.T) {
	h, credSvc := newSandboxProviderTestHandler(t)
	provider, err := exedev.NewProviderWithClient(exedev.Config{SandboxImage: "discboeing-agent-api:latest"}, exeDevTestCommandClient{})
	if err != nil {
		t.Fatalf("failed to create exe.dev provider: %v", err)
	}
	h.sandboxService.RegisterProvider("exedev", provider)

	credential, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		testProjectID,
		service.ProviderAnthropic,
		"Wrong API key",
		"",
		"test-token",
		service.CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("failed to create credential: %v", err)
	}

	w := httptest.NewRecorder()
	h.CreateSandboxProvider(w, sandboxProviderRequest(t, http.MethodPost, "/sandbox-providers", map[string]any{
		"type": "exedev",
		"config": map[string]any{
			"credentialId": credential.ID,
		},
	}))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	credential, err = credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		testProjectID,
		service.ProviderExeDev,
		"exe.dev API key",
		"",
		"test-token",
		service.CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("failed to create exe.dev credential: %v", err)
	}

	w = httptest.NewRecorder()
	h.CreateSandboxProvider(w, sandboxProviderRequest(t, http.MethodPost, "/sandbox-providers", map[string]any{
		"type": "exedev",
		"config": map[string]any{
			"credentialId": credential.ID,
		},
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListSandboxProviders_ExeDevInstanceAvailableWithCredential(t *testing.T) {
	h, credSvc := newSandboxProviderTestHandler(t)
	h.sandboxService.RegisterProviderDefinition("exedev", exedev.Definition())

	credential, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		testProjectID,
		service.ProviderExeDev,
		"exe.dev API key",
		"",
		"test-token",
		service.CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("failed to create exe.dev credential: %v", err)
	}

	instance := &model.SandboxProviderInstance{
		ProjectID: testProjectID,
		Type:      "exedev",
		Name:      "exe.dev",
		Config:    json.RawMessage(`{"credentialId":"` + credential.ID + `"}`),
	}
	if err := h.store.CreateSandboxProviderInstance(context.Background(), instance); err != nil {
		t.Fatalf("failed to create provider instance: %v", err)
	}

	w := httptest.NewRecorder()
	h.ListSandboxProviders(w, sandboxProviderRequest(t, http.MethodGet, "/sandbox-providers", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Providers []sandboxProviderInstanceResponse `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	var found *sandboxProviderInstanceResponse
	for i := range response.Providers {
		if response.Providers[i].ID == instance.ID {
			found = &response.Providers[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected exe.dev instance in response: %#v", response.Providers)
	}
	if !found.Available {
		t.Fatalf("expected exe.dev instance with credential to be available: %#v", found)
	}
}

func TestDeleteCredential_UsedBySandboxProviderReturnsConflict(t *testing.T) {
	h, credSvc := newSandboxProviderTestHandler(t)
	credential, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		testProjectID,
		service.ProviderAnthropic,
		"Provider API key",
		"",
		"test-token",
		service.CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("failed to create credential: %v", err)
	}

	if err := h.store.CreateSandboxProviderInstance(context.Background(), &model.SandboxProviderInstance{
		ProjectID: testProjectID,
		Type:      "mock",
		Name:      "Mock Remote",
		Config:    json.RawMessage(`{"credentialId":"` + credential.ID + `"}`),
	}); err != nil {
		t.Fatalf("failed to create provider instance: %v", err)
	}

	req := sandboxProviderRequest(t, http.MethodDelete, "/credentials/"+credential.ID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", credential.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteCredential(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSandboxProvider_DisablesBuiltinProvider(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)

	req := sandboxProviderRequest(t, http.MethodPatch, "/sandbox-providers/mock", map[string]any{
		"disabled": true,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerId", "mock")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateSandboxProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var response sandboxProviderInstanceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.BuiltIn || !response.Disabled || response.Available {
		t.Fatalf("expected disabled built-in provider, got %#v", response)
	}
	if err := h.sandboxService.ValidateSandboxProviderID(req.Context(), testProjectID, "mock"); err == nil {
		t.Fatal("expected disabled built-in provider validation to fail")
	}

	req = sandboxProviderRequest(t, http.MethodPatch, "/sandbox-providers/mock", map[string]any{
		"disabled": false,
	})
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w = httptest.NewRecorder()

	h.UpdateSandboxProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if err := h.sandboxService.ValidateSandboxProviderID(req.Context(), testProjectID, "mock"); err != nil {
		t.Fatalf("expected enabled built-in provider validation to pass: %v", err)
	}
}

func TestUpdateSandboxProvider_EditsBuiltinProviderOverride(t *testing.T) {
	h, _ := newSandboxProviderTestHandler(t)

	req := sandboxProviderRequest(t, http.MethodPatch, "/sandbox-providers/mock", map[string]any{
		"type": "mock",
		"name": "Local Mock",
		"icon": "simple-icons:docker",
		"config": map[string]any{
			"endpoint": "https://mock.example.test",
		},
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("providerId", "mock")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateSandboxProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var response sandboxProviderInstanceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.BuiltIn || response.Name != "Local Mock" || response.Icon != "simple-icons:docker" {
		t.Fatalf("expected edited built-in provider, got %#v", response)
	}

	stored, err := h.store.GetSandboxProviderInstance(context.Background(), testProjectID, "mock")
	if err != nil {
		t.Fatalf("failed to load built-in provider override: %v", err)
	}
	var config map[string]string
	if err := json.Unmarshal(stored.Config, &config); err != nil {
		t.Fatalf("failed to decode stored config: %v", err)
	}
	if config["endpoint"] != "https://mock.example.test" || config["icon"] != "simple-icons:docker" {
		t.Fatalf("expected stored config override, got %#v", config)
	}

	w = httptest.NewRecorder()
	h.ListSandboxProviders(w, sandboxProviderRequest(t, http.MethodGet, "/sandbox-providers", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResponse struct {
		Providers []sandboxProviderInstanceResponse `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listResponse.Providers) != 1 || listResponse.Providers[0].Name != "Local Mock" {
		t.Fatalf("expected edited built-in provider in list, got %#v", listResponse.Providers)
	}
}
