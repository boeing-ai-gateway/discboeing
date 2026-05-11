package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/service"
	"github.com/obot-platform/discobot/server/internal/store"
)

type sandboxProviderTypeResponse struct {
	ID           string                               `json:"id"`
	Name         string                               `json:"name"`
	Icon         string                               `json:"icon,omitempty"`
	Description  string                               `json:"description,omitempty"`
	Available    bool                                 `json:"available"`
	BuiltIn      bool                                 `json:"builtIn"`
	Capabilities sandboxProviderCapabilitiesResponse  `json:"capabilities"`
	ConfigFields []sandboxProviderConfigFieldResponse `json:"configFields,omitempty"`
}

type sandboxProviderInstanceRequest struct {
	Type     string          `json:"type"`
	Name     string          `json:"name"`
	Icon     string          `json:"icon,omitempty"`
	Config   json.RawMessage `json:"config,omitempty"`
	Disabled *bool           `json:"disabled,omitempty"`
}

type sandboxProviderDefaultRequest struct {
	ProviderID string `json:"providerId"`
}

type sandboxProviderInstanceResponse struct {
	ID           string                              `json:"id"`
	ProjectID    string                              `json:"projectId"`
	Type         string                              `json:"type"`
	Name         string                              `json:"name"`
	Icon         string                              `json:"icon,omitempty"`
	Config       json.RawMessage                     `json:"config,omitempty"`
	BuiltIn      bool                                `json:"builtIn"`
	Disabled     bool                                `json:"disabled"`
	Available    bool                                `json:"available"`
	Default      bool                                `json:"default"`
	Capabilities sandboxProviderCapabilitiesResponse `json:"capabilities"`
	CreatedAt    string                              `json:"createdAt,omitempty"`
	UpdatedAt    string                              `json:"updatedAt,omitempty"`
}

type sandboxProviderCapabilitiesResponse struct {
	Resources  bool `json:"resources"`
	Inspection bool `json:"inspection"`
	ClearCache bool `json:"clearCache"`
}

type sandboxProviderConfigFieldResponse = sandbox.ProviderConfigField

func sandboxProviderConfigValue(config json.RawMessage, key string) string {
	if len(config) == 0 || string(config) == "null" {
		return ""
	}
	var configMap map[string]any
	if err := json.Unmarshal(config, &configMap); err != nil {
		return ""
	}
	value, ok := configMap[key].(string)
	if !ok {
		return ""
	}
	return value
}

func resolveSandboxProviderIcon(instance *model.SandboxProviderInstance, driverIcon string) string {
	if instance == nil {
		return ""
	}
	if icon := sandboxProviderConfigValue(instance.Config, "icon"); icon != "" {
		return icon
	}
	if instance.BuiltIn {
		return driverIcon
	}
	return ""
}

func sandboxProviderCapabilitiesFromStatus(status sandbox.ProviderStatus) sandboxProviderCapabilitiesResponse {
	return sandboxProviderCapabilitiesResponse{
		Resources:  status.SupportsResources,
		Inspection: status.SupportsInspection,
		ClearCache: status.SupportsClearCache,
	}
}

func (h *Handler) sandboxProviderCapabilities(providerType string) sandboxProviderCapabilitiesResponse {
	if h.sandboxService == nil {
		return sandboxProviderCapabilitiesResponse{}
	}
	return sandboxProviderCapabilitiesFromStatus(h.sandboxService.ProviderCapabilities(providerType))
}

func (h *Handler) sandboxProviderDefinition(providerType string) sandbox.ProviderDefinition {
	if h.sandboxService == nil {
		return sandbox.ProviderDefinition{}
	}
	return h.sandboxService.ProviderDefinition(providerType)
}

func (h *Handler) registeredSandboxProviderTypes() []sandboxProviderTypeResponse {
	if h.sandboxService == nil {
		return nil
	}

	items := h.sandboxService.ListProviderCatalog()
	providerTypes := make([]sandboxProviderTypeResponse, 0, len(items))
	for _, item := range items {
		providerTypes = append(providerTypes, sandboxProviderTypeResponse{
			ID:           item.ID,
			Name:         item.Name,
			Icon:         item.Icon,
			Description:  item.Description,
			Available:    item.Available,
			BuiltIn:      item.BuiltIn,
			Capabilities: sandboxProviderCapabilitiesFromStatus(item.Capabilities),
			ConfigFields: item.ConfigFields,
		})
	}
	return providerTypes
}

func mapSandboxProviderInstance(instance *model.SandboxProviderInstance, available, isDefault bool, capabilities sandboxProviderCapabilitiesResponse, driverIcon string) sandboxProviderInstanceResponse {
	if instance == nil {
		return sandboxProviderInstanceResponse{}
	}
	return sandboxProviderInstanceResponse{
		ID:           instance.ID,
		ProjectID:    instance.ProjectID,
		Type:         instance.Type,
		Name:         instance.Name,
		Icon:         resolveSandboxProviderIcon(instance, driverIcon),
		Config:       instance.Config,
		BuiltIn:      instance.BuiltIn,
		Disabled:     instance.Disabled,
		Available:    available,
		Default:      isDefault,
		Capabilities: capabilities,
		CreatedAt:    formatSandboxProviderTime(instance.CreatedAt),
		UpdatedAt:    formatSandboxProviderTime(instance.UpdatedAt),
	}
}

func formatSandboxProviderTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ListSandboxProviderTypes returns the hard-coded provider capability catalog.
func (h *Handler) ListSandboxProviderTypes(w http.ResponseWriter, _ *http.Request) {
	h.JSON(w, http.StatusOK, map[string]any{"providerTypes": h.registeredSandboxProviderTypes()})
}

// ListSandboxProviders returns built-in and user-configured provider instances.
func (h *Handler) ListSandboxProviders(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	providerList, err := h.sandboxService.ListProjectProviders(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Project not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to list sandbox providers")
		return
	}

	result := make([]sandboxProviderInstanceResponse, 0, len(providerList.Statuses)+len(providerList.Instances))
	builtinOverrides := map[string]*model.SandboxProviderInstance{}
	customInstances := make([]*model.SandboxProviderInstance, 0, len(providerList.Instances))
	for _, instance := range providerList.Instances {
		if instance.BuiltIn {
			builtinOverrides[instance.ID] = instance
			continue
		}
		customInstances = append(customInstances, instance)
	}
	for providerType, status := range providerList.Statuses {
		definition := h.sandboxProviderDefinition(providerType)
		name := definition.Name
		if name == "" {
			name = providerType
		}
		if override := builtinOverrides[providerType]; override != nil {
			result = append(result, mapSandboxProviderInstance(
				override,
				status.Available && !override.Disabled,
				providerType == providerList.Default && !override.Disabled,
				sandboxProviderCapabilitiesFromStatus(status),
				definition.Icon,
			))
			continue
		}
		result = append(result, sandboxProviderInstanceResponse{
			ID:           providerType,
			ProjectID:    projectID,
			Type:         providerType,
			Name:         name,
			Icon:         definition.Icon,
			BuiltIn:      true,
			Disabled:     false,
			Available:    status.Available,
			Default:      providerType == providerList.Default,
			Capabilities: sandboxProviderCapabilitiesFromStatus(status),
		})
	}
	for _, instance := range customInstances {
		status := providerList.Statuses[instance.Type]
		result = append(result, mapSandboxProviderInstance(
			instance,
			h.sandboxService.ProviderInstanceAvailable(r.Context(), projectID, instance, status),
			instance.ID == providerList.Default,
			sandboxProviderCapabilitiesFromStatus(status),
			h.sandboxProviderDefinition(instance.Type).Icon,
		))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].BuiltIn != result[j].BuiltIn {
			return result[i].BuiltIn
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	h.JSON(w, http.StatusOK, map[string]any{
		"providers":      result,
		"default":        providerList.Default,
		"projectDefault": providerList.ProjectDefault,
		"systemDefault":  providerList.SystemDefault,
	})
}

// UpdateSandboxProviderDefault updates the project-level default sandbox provider.
func (h *Handler) UpdateSandboxProviderDefault(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if !h.requireSandboxProviderAdmin(w, r, projectID) {
		return
	}

	var req sandboxProviderDefaultRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	providerID := strings.TrimSpace(req.ProviderID)
	if err := h.sandboxService.ValidateSandboxProviderID(r.Context(), projectID, providerID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusBadRequest, "Sandbox provider not found")
			return
		}
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	project, err := h.store.GetProjectByID(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusNotFound, "Project not found")
		return
	}
	project.DefaultSandboxProviderID = providerID
	if err := h.store.UpdateProject(r.Context(), project); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to update default sandbox provider")
		return
	}

	systemDefaultProvider := h.sandboxService.DefaultProviderName()
	defaultProvider := providerID
	if defaultProvider == "" {
		defaultProvider = systemDefaultProvider
	}
	h.JSON(w, http.StatusOK, map[string]any{
		"default":        defaultProvider,
		"projectDefault": providerID,
		"systemDefault":  systemDefaultProvider,
	})
}

// CreateSandboxProvider creates a user-configured sandbox provider instance.
func (h *Handler) CreateSandboxProvider(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if !h.requireSandboxProviderAdmin(w, r, projectID) {
		return
	}

	var req sandboxProviderInstanceRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	instance, ok := h.buildSandboxProviderInstance(w, r, projectID, &req, true)
	if !ok {
		return
	}
	if err := h.store.CreateSandboxProviderInstance(r.Context(), instance); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to create sandbox provider")
		return
	}

	available := h.sandboxService.ProviderInstanceAvailable(r.Context(), projectID, instance, sandbox.ProviderStatus{})
	h.JSON(w, http.StatusCreated, mapSandboxProviderInstance(instance, available, false, h.sandboxProviderCapabilities(instance.Type), h.sandboxProviderDefinition(instance.Type).Icon))
}

// UpdateSandboxProvider updates a user-configured sandbox provider instance.
func (h *Handler) UpdateSandboxProvider(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if !h.requireSandboxProviderAdmin(w, r, projectID) {
		return
	}

	providerID := chi.URLParam(r, "providerId")

	var req sandboxProviderInstanceRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if h.sandboxService.HasRuntimeProvider(providerID) {
		if req.Type != "" && strings.TrimSpace(req.Type) != providerID {
			h.Error(w, http.StatusBadRequest, "Built-in sandbox provider type cannot be changed")
			return
		}
		name := strings.TrimSpace(req.Name)
		config := json.RawMessage(nil)
		if req.Name == "" && req.Type == "" && req.Icon == "" && len(req.Config) == 0 {
			if existing, err := h.store.GetSandboxProviderInstance(r.Context(), projectID, providerID); err == nil {
				name = existing.Name
				config = existing.Config
			} else if !errors.Is(err, store.ErrNotFound) {
				h.Error(w, http.StatusInternalServerError, "Failed to load sandbox provider")
				return
			}
		} else {
			var ok bool
			config, ok = h.buildSandboxProviderConfig(w, r, projectID, &req, false)
			if !ok {
				return
			}
		}
		instance, err := h.store.UpdateBuiltinSandboxProviderInstance(r.Context(), projectID, providerID, providerID, name, config, req.Disabled)
		if err != nil {
			h.Error(w, http.StatusInternalServerError, "Failed to update sandbox provider")
			return
		}
		available := h.sandboxService.ProviderTypeAvailable(providerID) && !instance.Disabled
		h.JSON(w, http.StatusOK, mapSandboxProviderInstance(instance, available, providerID == h.sandboxService.DefaultProviderName() && !instance.Disabled, h.sandboxProviderCapabilities(providerID), h.sandboxProviderDefinition(providerID).Icon))
		return
	}

	instance, err := h.store.GetSandboxProviderInstance(r.Context(), projectID, providerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Sandbox provider not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to load sandbox provider")
		return
	}
	if instance.BuiltIn {
		h.Error(w, http.StatusBadRequest, "Built-in sandbox providers cannot be updated")
		return
	}

	if req.Type != "" || req.Name != "" || req.Icon != "" || len(req.Config) > 0 {
		updated, ok := h.buildSandboxProviderInstance(w, r, projectID, &req, false)
		if !ok {
			return
		}
		instance.Type = updated.Type
		instance.Name = updated.Name
		instance.Config = updated.Config
	}
	if req.Disabled != nil {
		instance.Disabled = *req.Disabled
	}
	if err := h.store.UpdateSandboxProviderInstance(r.Context(), instance); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to update sandbox provider")
		return
	}

	available := h.sandboxService.ProviderInstanceAvailable(r.Context(), projectID, instance, sandbox.ProviderStatus{})
	h.JSON(w, http.StatusOK, mapSandboxProviderInstance(instance, available, false, h.sandboxProviderCapabilities(instance.Type), h.sandboxProviderDefinition(instance.Type).Icon))
}

// DeleteSandboxProvider deletes a user-configured sandbox provider instance.
func (h *Handler) DeleteSandboxProvider(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if !h.requireSandboxProviderAdmin(w, r, projectID) {
		return
	}

	providerID := chi.URLParam(r, "providerId")
	if h.sandboxService.HasRuntimeProvider(providerID) {
		h.Error(w, http.StatusBadRequest, "Built-in sandbox providers cannot be deleted")
		return
	}
	project, err := h.store.GetProjectByID(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to load project")
		return
	}
	if strings.TrimSpace(project.DefaultSandboxProviderID) == providerID {
		h.Error(w, http.StatusConflict, "Sandbox provider is the project default")
		return
	}
	referenceCount, err := h.store.CountSessionsReferencingSandboxProvider(r.Context(), projectID, providerID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to check sandbox provider usage")
		return
	}
	if referenceCount > 0 {
		h.Error(w, http.StatusConflict, "Sandbox provider is used by existing sessions")
		return
	}
	if err := h.store.DeleteSandboxProviderInstance(r.Context(), projectID, providerID); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to delete sandbox provider")
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetSandboxProviderResources returns provider resources for a sandbox provider.
func (h *Handler) GetSandboxProviderResources(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	providerID := chi.URLParam(r, "providerId")
	resources, err := h.sandboxService.GetProviderResourcesForProvider(r.Context(), projectID, providerID)
	if err != nil {
		if errors.Is(err, sandbox.ErrProviderResourcesUnsupported) {
			h.Error(w, http.StatusNotImplemented, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get sandbox provider resources")
		return
	}

	h.JSON(w, http.StatusOK, resources)
}

// UpdateSandboxProviderResources updates provider resources for a sandbox provider.
func (h *Handler) UpdateSandboxProviderResources(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	providerID := chi.URLParam(r, "providerId")
	if !h.requireSandboxProviderAdmin(w, r, projectID) {
		return
	}

	var req service.UpdateProviderResourcesRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.sandboxService.UpdateProviderResourcesForProvider(r.Context(), projectID, providerID, req)
	if err != nil {
		var validationErr *service.RequestValidationError
		switch {
		case errors.As(err, &validationErr):
			h.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, sandbox.ErrProviderResourcesUnsupported):
			h.Error(w, http.StatusNotImplemented, err.Error())
		default:
			h.Error(w, http.StatusInternalServerError, "Failed to update sandbox provider resources")
		}
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// GetSandboxProviderInspection returns inspection-container details for a sandbox provider.
func (h *Handler) GetSandboxProviderInspection(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	providerID := chi.URLParam(r, "providerId")
	info, err := h.sandboxService.GetProjectInspectionForProvider(r.Context(), projectID, providerID)
	if err != nil {
		if errors.Is(err, sandbox.ErrProjectInspectionUnsupported) {
			h.Error(w, http.StatusNotImplemented, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get sandbox provider inspection info")
		return
	}

	h.JSON(w, http.StatusOK, info)
}

// SandboxProviderInspectionTerminalWebSocket attaches to a provider's inspection shell.
func (h *Handler) SandboxProviderInspectionTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	providerID := chi.URLParam(r, "providerId")
	if !h.requireSandboxProviderAdmin(w, r, projectID) {
		return
	}

	userID := middleware.GetUserID(r.Context())
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	if rows < minTermRows {
		rows = minTermRows
	}
	if cols < minTermCols {
		cols = minTermCols
	}

	termUserID := userID
	if termUserID == "" {
		termUserID = "anonymous"
	}

	ctx := r.Context()
	termKey := "sandbox-provider-inspection:" + projectID + ":" + providerID + ":" + termUserID
	termSession, err := h.terminalManager.GetOrCreate(ctx, termKey, func(ctx context.Context) (sandbox.PTY, error) {
		return h.sandboxService.AttachProjectInspectionForProvider(ctx, projectID, providerID, sandbox.AttachOptions{
			Cmd:  []string{"nsenter", "-m", "-t", "1", "--", "/bin/bash", "-lc", "cd /root && exec /bin/bash -l"},
			Rows: rows,
			Cols: cols,
			User: "root",
		})
	})
	if err != nil {
		if errors.Is(err, sandbox.ErrProjectInspectionUnsupported) {
			h.Error(w, http.StatusNotImplemented, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, "failed to attach to sandbox provider inspection terminal")
		return
	}

	if err := termSession.Resize(ctx, rows, cols); err != nil {
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	sub := termSession.Subscribe()
	defer termSession.Unsubscribe(sub)

	handlePersistentTerminalSession(ctx, termSession, sub, conn)
}

func (h *Handler) requireSandboxProviderAdmin(w http.ResponseWriter, r *http.Request, projectID string) bool {
	userID := middleware.GetUserID(r.Context())
	role, err := h.projectService.GetMemberRole(r.Context(), projectID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		h.Error(w, http.StatusForbidden, "Admin access required")
		return false
	}
	return true
}

func (h *Handler) buildSandboxProviderInstance(w http.ResponseWriter, r *http.Request, projectID string, req *sandboxProviderInstanceRequest, applyCreateDefaults bool) (*model.SandboxProviderInstance, bool) {
	providerType := strings.TrimSpace(req.Type)
	name := strings.TrimSpace(req.Name)
	if providerType == "" {
		h.Error(w, http.StatusBadRequest, "type is required")
		return nil, false
	}
	if !h.sandboxService.HasProviderDefinition(providerType) {
		h.Error(w, http.StatusBadRequest, "Unknown sandbox provider type")
		return nil, false
	}
	if name == "" && applyCreateDefaults {
		name = h.sandboxProviderDefinition(providerType).Name
		if name == "" {
			name = providerType
		}
	}

	config, ok := h.buildSandboxProviderConfig(w, r, projectID, req, applyCreateDefaults)
	if !ok {
		return nil, false
	}

	return &model.SandboxProviderInstance{
		ProjectID: projectID,
		Type:      providerType,
		Name:      name,
		Config:    config,
	}, true
}

func (h *Handler) buildSandboxProviderConfig(w http.ResponseWriter, r *http.Request, projectID string, req *sandboxProviderInstanceRequest, applyCreateDefaults bool) (json.RawMessage, bool) {
	configMap := map[string]any{}
	if len(req.Config) > 0 && string(req.Config) != "null" {
		if !json.Valid(req.Config) {
			h.Error(w, http.StatusBadRequest, "config must be valid JSON")
			return nil, false
		}
		if err := json.Unmarshal(req.Config, &configMap); err != nil {
			h.Error(w, http.StatusBadRequest, "config must be a JSON object")
			return nil, false
		}
	}
	if icon := strings.TrimSpace(req.Icon); icon != "" {
		configMap["icon"] = icon
	} else if applyCreateDefaults {
		if icon := strings.TrimSpace(h.sandboxProviderDefinition(strings.TrimSpace(req.Type)).Icon); icon != "" {
			configMap["icon"] = icon
		}
	}
	for _, field := range h.sandboxProviderDefinition(strings.TrimSpace(req.Type)).ConfigFields {
		if !field.Required {
			continue
		}
		value, ok := configMap[field.Key]
		if !ok || strings.TrimSpace(valueToString(value)) == "" {
			h.Error(w, http.StatusBadRequest, field.Label+" is required")
			return nil, false
		}
	}
	for _, field := range h.sandboxProviderDefinition(strings.TrimSpace(req.Type)).ConfigFields {
		if field.Type != "credential" {
			continue
		}
		credentialID := strings.TrimSpace(valueToString(configMap[field.Key]))
		if credentialID == "" {
			continue
		}
		credential, err := h.credentialService.GetByID(r.Context(), projectID, credentialID)
		if err != nil {
			h.Error(w, http.StatusBadRequest, field.Key+" does not reference an existing credential")
			return nil, false
		}
		if field.CredentialProvider != "" && credential.Provider != field.CredentialProvider {
			h.Error(w, http.StatusBadRequest, field.Label+" must reference a "+field.CredentialProvider+" credential")
			return nil, false
		}
		if field.CredentialAuthType != "" && credential.AuthType != field.CredentialAuthType {
			h.Error(w, http.StatusBadRequest, field.Label+" must reference a "+field.CredentialAuthType+" credential")
			return nil, false
		}
	}
	if len(configMap) == 0 {
		return nil, true
	}
	compact, err := json.Marshal(configMap)
	if err != nil {
		h.Error(w, http.StatusBadRequest, "config must be valid JSON")
		return nil, false
	}
	return compact, true
}

func valueToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
