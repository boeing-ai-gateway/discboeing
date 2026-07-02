package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/boeing-ai-gateway/discboeing/server/internal/providers"
)

// Model represents a model available for selection (for API responses)
type Model struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Provider         string   `json:"provider"`
	Description      string   `json:"description,omitempty"`
	Reasoning        bool     `json:"reasoning,omitempty"` // Whether model supports extended thinking
	ReasoningLevels  []string `json:"reasoningLevels,omitempty"`
	DefaultReasoning string   `json:"defaultReasoning,omitempty"`
	ServiceTiers     []string `json:"serviceTiers,omitempty"`
}

// ModelsService handles model listing operations
type ModelsService struct {
	credentialService *CredentialService
}

// NewModelsService creates a new models service
func NewModelsService(credSvc *CredentialService) *ModelsService {
	return &ModelsService{
		credentialService: credSvc,
	}
}

// GetModelsForProject returns available models based only on configured project credentials.
func (s *ModelsService) GetModelsForProject(ctx context.Context, projectID string) ([]Model, error) {
	credentials, err := s.credentialService.List(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	providerIDs := make([]string, 0)
	providerSet := make(map[string]bool)
	for _, cred := range credentials {
		if !cred.IsConfigured {
			continue
		}
		if cred.Inactive {
			continue
		}
		if strings.HasPrefix(cred.Provider, customProviderPrefix) || strings.HasPrefix(cred.Provider, mcpProviderPrefix) {
			continue
		}
		if providerSet[cred.Provider] {
			continue
		}
		providerSet[cred.Provider] = true
		providerIDs = append(providerIDs, cred.Provider)
	}

	if len(providerIDs) == 0 {
		return []Model{}, nil
	}

	providerModels, err := providers.GetModelsForProviders(providerIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}

	models := make([]Model, len(providerModels))
	for i, pm := range providerModels {
		models[i] = Model{
			ID:               pm.ID,
			Name:             pm.Name,
			Provider:         pm.Provider,
			Reasoning:        pm.Reasoning,
			ReasoningLevels:  pm.ReasoningLevels,
			DefaultReasoning: pm.DefaultReasonLevel,
			ServiceTiers:     append([]string(nil), pm.ServiceTiers...),
		}
	}

	return models, nil
}
