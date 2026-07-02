package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"sort"
	"strings"

	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	"github.com/boeing-ai-gateway/discboeing/server/internal/store"
)

// ListProjectCacheVolumes lists provider cache volumes for a project when supported.
func (s *SandboxService) ListProjectCacheVolumes(ctx context.Context, projectID string) (any, error) {
	type cacheVolumeManager interface {
		ListCacheVolumes(ctx context.Context, projectID string) (any, error)
	}
	manager, ok := s.provider.(cacheVolumeManager)
	if !ok {
		return nil, sandbox.ErrProjectCacheUnsupported
	}
	return manager.ListCacheVolumes(ctx, projectID)
}

// ClearProjectCache clears provider cache for a project when supported.
func (s *SandboxService) ClearProjectCache(ctx context.Context, projectID string) error {
	cacheManager, ok := s.provider.(sandbox.ProjectCacheManager)
	if !ok {
		return sandbox.ErrProjectCacheUnsupported
	}
	return cacheManager.ClearCache(ctx, projectID)
}

// RemoveProject removes provider-managed resources for a deleted project.
func (s *SandboxService) RemoveProject(ctx context.Context, projectID string) error {
	return s.provider.RemoveProject(ctx, projectID)
}

// DockerProxyProvider returns a provider that can proxy Docker API requests.
func (s *SandboxService) DockerProxyProvider() (sandbox.DockerProxyProvider, error) {
	if s.providerManager != nil {
		for _, name := range s.providerManager.ListProviders() {
			provider, err := s.providerManager.GetProvider(name)
			if err != nil {
				continue
			}
			if proxyProvider, ok := provider.(sandbox.DockerProxyProvider); ok {
				return proxyProvider, nil
			}
		}
	}
	if proxyProvider, ok := s.provider.(sandbox.DockerProxyProvider); ok {
		return proxyProvider, nil
	}
	return nil, fmt.Errorf("no provider supports Docker proxying")
}

// ErrSandboxProviderTypeUnavailable indicates that a configured provider type has no runtime implementation.
var ErrSandboxProviderTypeUnavailable = errors.New("sandbox provider type is not available")

// SandboxProviderCatalogItem describes a registered sandbox provider type.
type SandboxProviderCatalogItem struct {
	ID           string
	Name         string
	Icon         string
	Description  string
	Available    bool
	BuiltIn      bool
	Capabilities sandbox.ProviderStatus
	ConfigFields []sandbox.ProviderConfigField
}

// SandboxProviderList describes provider instances and effective defaults for a project.
type SandboxProviderList struct {
	Instances      []*model.SandboxProviderInstance
	Statuses       map[string]sandbox.ProviderStatus
	Default        string
	ProjectDefault string
	SystemDefault  string
}

// RegisterProvider registers a runtime provider with the service-managed provider catalog.
func (s *SandboxService) RegisterProvider(name string, provider sandbox.Provider) {
	if s.providerManager != nil {
		s.providerManager.RegisterProvider(name, provider)
	}
}

// RegisterProviderDefinition registers provider metadata with the service-managed provider catalog.
func (s *SandboxService) RegisterProviderDefinition(name string, definition sandbox.ProviderDefinition) {
	if s.providerManager != nil {
		s.providerManager.RegisterProviderDefinition(name, definition)
	}
}

// ListProviderNames returns the registered runtime provider names.
func (s *SandboxService) ListProviderNames() []string {
	if s.providerManager == nil {
		return nil
	}
	return s.providerManager.ListProviders()
}

// RefreshProviderStatuses refreshes provider status for callers that only need
// to force status reconciliation before reading a higher-level status object.
func (s *SandboxService) RefreshProviderStatuses() {
	if s.providerManager != nil {
		_ = s.providerManager.ListProviderStatuses()
	}
}

// ListProviderStatuses returns statuses for registered runtime providers.
func (s *SandboxService) ListProviderStatuses() map[string]sandbox.ProviderStatus {
	if s.providerManager == nil {
		return nil
	}
	return s.providerManager.ListProviderStatuses()
}

// ProviderStatus returns status for a registered runtime provider.
func (s *SandboxService) ProviderStatus(providerType string) (sandbox.ProviderStatus, bool) {
	if s.providerManager == nil {
		return sandbox.ProviderStatus{}, false
	}
	return s.providerManager.GetProviderStatus(providerType)
}

// DefaultProviderName returns the process-wide default sandbox provider name.
func (s *SandboxService) DefaultProviderName() string {
	if s.providerManager == nil {
		return ""
	}
	s.providerManager.EnsureDefaultAvailable()
	return s.providerManager.DefaultProviderName()
}

// HasRuntimeProvider reports whether a provider type has a registered runtime implementation.
func (s *SandboxService) HasRuntimeProvider(providerType string) bool {
	if s.providerManager == nil {
		return false
	}
	_, ok := s.providerManager.GetProviderStatus(providerType)
	return ok
}

// HasProviderDefinition reports whether metadata exists for a provider type.
func (s *SandboxService) HasProviderDefinition(providerType string) bool {
	if s.providerManager == nil {
		return false
	}
	_, ok := s.providerManager.GetProviderDefinition(providerType)
	return ok
}

// ProviderDefinition returns metadata for a registered provider type.
func (s *SandboxService) ProviderDefinition(providerType string) sandbox.ProviderDefinition {
	if s.providerManager == nil {
		return sandbox.ProviderDefinition{}
	}
	definition, _ := s.providerManager.GetProviderDefinition(providerType)
	return definition
}

// ProviderCapabilities returns capability flags for a registered provider type.
func (s *SandboxService) ProviderCapabilities(providerType string) sandbox.ProviderStatus {
	if s.providerManager == nil {
		return sandbox.ProviderStatus{}
	}
	status, _ := s.providerManager.GetProviderStatus(providerType)
	return status
}

// ListProviderCatalog returns the registered sandbox provider type catalog.
func (s *SandboxService) ListProviderCatalog() []SandboxProviderCatalogItem {
	if s.providerManager == nil {
		return nil
	}

	statuses := s.providerManager.ListProviderStatuses()
	definitions := s.providerManager.ListProviderDefinitions()
	providerTypes := make([]SandboxProviderCatalogItem, 0, len(definitions))
	for providerType, definition := range definitions {
		status, registered := statuses[providerType]
		name := definition.Name
		if name == "" {
			name = providerType
		}
		description := definition.Description
		if description == "" {
			description = "Built-in " + name + " sandbox driver"
		}
		providerTypes = append(providerTypes, SandboxProviderCatalogItem{
			ID:           providerType,
			Name:         name,
			Icon:         definition.Icon,
			Description:  description,
			Available:    !registered || status.Available,
			BuiltIn:      registered,
			Capabilities: status,
			ConfigFields: definition.ConfigFields,
		})
	}
	sort.Slice(providerTypes, func(i, j int) bool { return providerTypes[i].Name < providerTypes[j].Name })
	return providerTypes
}

// ListProjectProviders returns configured instances plus runtime provider status
// data needed to render provider settings for a project.
func (s *SandboxService) ListProjectProviders(ctx context.Context, projectID string) (*SandboxProviderList, error) {
	project, err := s.store.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	instances, err := s.store.ListSandboxProviderInstances(ctx, projectID)
	if err != nil {
		return nil, err
	}

	statuses := map[string]sandbox.ProviderStatus{}
	if s.providerManager != nil {
		maps.Copy(statuses, s.providerManager.ListProviderStatuses())
	}

	systemDefault := s.DefaultProviderName()
	projectDefault := strings.TrimSpace(project.DefaultSandboxProviderID)
	defaultProvider := systemDefault
	if projectDefault != "" {
		defaultProvider = projectDefault
	}

	return &SandboxProviderList{
		Instances:      instances,
		Statuses:       statuses,
		Default:        defaultProvider,
		ProjectDefault: projectDefault,
		SystemDefault:  systemDefault,
	}, nil
}

// ValidateSandboxProviderID validates that a provider ID can be used by a project.
func (s *SandboxService) ValidateSandboxProviderID(ctx context.Context, projectID, providerID string) error {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil
	}
	if s.providerManager != nil {
		if _, ok := s.providerManager.GetProviderStatus(providerID); ok {
			disabled, err := s.store.IsSandboxProviderDisabled(ctx, projectID, providerID)
			if err != nil {
				return err
			}
			if disabled {
				return store.ErrNotFound
			}
			return nil
		}
	}
	instance, err := s.store.GetSandboxProviderInstance(ctx, projectID, providerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.ErrNotFound
		}
		return err
	}
	if instance.Disabled {
		return store.ErrNotFound
	}
	return nil
}

// ProviderTypeAvailable reports whether a provider type is known and available.
func (s *SandboxService) ProviderTypeAvailable(providerType string) bool {
	if s.providerManager == nil {
		return false
	}
	status, ok := s.providerManager.GetProviderStatus(providerType)
	if ok {
		return status.Available
	}
	_, ok = s.providerManager.GetProviderDefinition(providerType)
	return ok
}

// ProviderInstanceAvailable reports whether an instance has a valid, available type and config.
func (s *SandboxService) ProviderInstanceAvailable(ctx context.Context, projectID string, instance *model.SandboxProviderInstance, status sandbox.ProviderStatus) bool {
	if instance == nil || instance.Disabled || s.providerManager == nil {
		return false
	}
	definition, hasDefinition := s.providerManager.GetProviderDefinition(instance.Type)
	if !hasDefinition {
		return false
	}
	if registeredStatus, registered := s.providerManager.GetProviderStatus(instance.Type); registered {
		status = registeredStatus
		if !status.Available && !providerDefinitionUsesCredentials(definition) {
			return false
		}
	}
	return s.providerInstanceConfigAvailable(ctx, projectID, instance, definition)
}

func providerDefinitionUsesCredentials(definition sandbox.ProviderDefinition) bool {
	for _, field := range definition.ConfigFields {
		if field.Type == "credential" {
			return true
		}
	}
	return false
}

func (s *SandboxService) providerInstanceConfigAvailable(ctx context.Context, projectID string, instance *model.SandboxProviderInstance, definition sandbox.ProviderDefinition) bool {
	configMap := map[string]any{}
	if len(instance.Config) > 0 && string(instance.Config) != "null" {
		if err := json.Unmarshal(instance.Config, &configMap); err != nil {
			return false
		}
	}
	for _, field := range definition.ConfigFields {
		value := strings.TrimSpace(valueToString(configMap[field.Key]))
		if field.Required && value == "" {
			return false
		}
		if field.Type != "credential" || value == "" {
			continue
		}
		if s.credentialService == nil {
			return false
		}
		credential, err := s.credentialService.GetByID(ctx, projectID, value)
		if err != nil || !credential.IsConfigured || credential.Inactive {
			return false
		}
		if field.CredentialProvider != "" && credential.Provider != field.CredentialProvider {
			return false
		}
		if field.CredentialAuthType != "" && credential.AuthType != field.CredentialAuthType {
			return false
		}
	}
	return true
}

// ResolveSandboxRuntimeProvider resolves a project-visible provider ID to its runtime provider.
func (s *SandboxService) ResolveSandboxRuntimeProvider(ctx context.Context, projectID, providerID string) (sandbox.Provider, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, newValidationError("Sandbox provider is required")
	}
	if s.providerManager == nil {
		return nil, fmt.Errorf("sandbox provider manager is not configured")
	}

	if _, ok := s.providerManager.GetProviderStatus(providerID); ok {
		disabled, err := s.store.IsSandboxProviderDisabled(ctx, projectID, providerID)
		if err != nil {
			return nil, err
		}
		if disabled {
			return nil, store.ErrNotFound
		}
		provider, err := s.providerManager.GetProvider(providerID)
		if err != nil {
			return nil, store.ErrNotFound
		}
		return provider, nil
	}

	instance, err := s.store.GetSandboxProviderInstance(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	if instance.Disabled {
		return nil, store.ErrNotFound
	}
	provider, err := s.providerManager.GetProvider(instance.Type)
	if err != nil {
		return nil, ErrSandboxProviderTypeUnavailable
	}
	return provider, nil
}

// GetProviderResources returns provider VM resources for the default provider.
func (s *SandboxService) GetProviderResources(ctx context.Context, projectID string) (*ProviderResources, error) {
	return s.getProviderResources(ctx, projectID, s.provider)
}

// GetProviderResourcesForProvider returns provider VM resources for a project-visible provider ID.
func (s *SandboxService) GetProviderResourcesForProvider(ctx context.Context, projectID, providerID string) (*ProviderResources, error) {
	provider, err := s.ResolveSandboxRuntimeProvider(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	return s.getProviderResources(ctx, projectID, provider)
}

func (s *SandboxService) getProviderResources(ctx context.Context, projectID string, provider sandbox.Provider) (*ProviderResources, error) {
	if _, err := s.store.GetProjectByID(ctx, projectID); err != nil {
		return nil, err
	}
	resourceManager, ok := provider.(sandbox.ProviderResourceManager)
	if !ok {
		return nil, sandbox.ErrProviderResourcesUnsupported
	}
	info, err := resourceManager.GetProviderResourceInfo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &ProviderResources{Provider: info.Provider, VM: providerVMResourcesFromInfo(info)}, nil
}

// UpdateProviderResourcesForProvider updates provider VM resources for a project-visible provider ID.
func (s *SandboxService) UpdateProviderResourcesForProvider(ctx context.Context, projectID, providerID string, req UpdateProviderResourcesRequest) (*ProviderResourcesUpdateResult, error) {
	if req.MemoryMB == nil && req.DataDiskGB == nil {
		return nil, newValidationError("at least one resource must be provided")
	}
	provider, err := s.ResolveSandboxRuntimeProvider(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	project, err := s.store.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	resourceManager, ok := provider.(sandbox.ProviderResourceManager)
	if !ok {
		return nil, sandbox.ErrProviderResourcesUnsupported
	}

	currentInfo, err := resourceManager.GetProviderResourceInfo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	currentResources := providerVMResourcesFromInfo(currentInfo)
	if req.MemoryMB != nil && !currentResources.CanChangeMemory {
		return nil, newValidationError("memory updates are not supported for this provider")
	}
	if req.MemoryMB != nil {
		if *req.MemoryMB <= 0 {
			return nil, newValidationError("memoryMB must be greater than 0")
		}
		if *req.MemoryMB%1024 != 0 {
			return nil, newValidationError("memoryMB must be a whole GiB multiple")
		}
	}
	if req.DataDiskGB != nil && *req.DataDiskGB <= 0 {
		return nil, newValidationError("dataDiskGB must be greater than 0")
	}
	if req.DataDiskGB != nil && *req.DataDiskGB < currentInfo.DataDiskGB {
		return nil, newValidationError("data disk size can only increase")
	}

	previousMemory := project.VZMemoryMB
	previousDisk := project.VZDataDiskGB
	if req.MemoryMB != nil {
		memoryMB := *req.MemoryMB
		project.VZMemoryMB = &memoryMB
	}
	if req.DataDiskGB != nil {
		dataDiskGB := *req.DataDiskGB
		project.VZDataDiskGB = &dataDiskGB
	}
	if err := s.store.UpdateProject(ctx, project); err != nil {
		return nil, err
	}

	rollback := func() {
		project.VZMemoryMB = previousMemory
		project.VZDataDiskGB = previousDisk
		if updateErr := s.store.UpdateProject(context.Background(), project); updateErr != nil {
			log.Printf("Warning: failed to roll back provider resources for %s: %v", projectID, updateErr)
		}
	}
	if err := resourceManager.ApplyProviderResourceUpdate(ctx, projectID, sandbox.UpdateProviderResourcesRequest{MemoryMB: req.MemoryMB, DataDiskGB: req.DataDiskGB}); err != nil {
		rollback()
		return nil, err
	}
	updatedInfo, err := resourceManager.GetProviderResourceInfo(ctx, projectID)
	if err != nil {
		rollback()
		return nil, err
	}
	return &ProviderResourcesUpdateResult{
		Provider:        updatedInfo.Provider,
		Previous:        currentResources,
		Current:         providerVMResourcesFromInfo(updatedInfo),
		RestartRequired: true,
	}, nil
}

// GetProjectInspectionForProvider returns inspection access for a project-visible provider ID.
func (s *SandboxService) GetProjectInspectionForProvider(ctx context.Context, projectID, providerID string) (*ProjectInspection, error) {
	provider, err := s.ResolveSandboxRuntimeProvider(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetProjectByID(ctx, projectID); err != nil {
		return nil, err
	}
	inspectionManager, ok := provider.(sandbox.ProjectInspectionManager)
	if !ok {
		return nil, sandbox.ErrProjectInspectionUnsupported
	}
	info, err := inspectionManager.GetProjectInspectionInfo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &ProjectInspection{Provider: info.Provider, Available: info.Available, ContainerName: info.ContainerName, Scope: info.Scope}, nil
}

// AttachProjectInspectionForProvider attaches to inspection shell for a project-visible provider ID.
func (s *SandboxService) AttachProjectInspectionForProvider(ctx context.Context, projectID, providerID string, opts sandbox.AttachOptions) (sandbox.PTY, error) {
	provider, err := s.ResolveSandboxRuntimeProvider(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetProjectByID(ctx, projectID); err != nil {
		return nil, err
	}
	inspectionManager, ok := provider.(sandbox.ProjectInspectionManager)
	if !ok {
		return nil, sandbox.ErrProjectInspectionUnsupported
	}
	return inspectionManager.AttachProjectInspection(ctx, projectID, opts)
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
