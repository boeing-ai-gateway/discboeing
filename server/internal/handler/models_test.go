package handler

import (
	"reflect"
	"testing"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/server/internal/service"
)

// TestModelConversion ensures that service models are mapped to generated API models.
func TestModelConversion(t *testing.T) {
	testCases := []struct {
		name         string
		serviceModel service.Model
		expectedInfo api.ModelInfo
	}{
		{
			name: "model with all fields",
			serviceModel: service.Model{
				ID:               "anthropic/claude-opus-4",
				Name:             "Claude Opus 4",
				Provider:         "Anthropic",
				Description:      "Most capable model",
				Reasoning:        true,
				ReasoningLevels:  []string{"low", "medium", "high"},
				DefaultReasoning: "medium",
				ServiceTiers:     []string{"priority"},
			},
			expectedInfo: api.ModelInfo{
				Id:               "anthropic/claude-opus-4",
				Name:             "Claude Opus 4",
				Provider:         "Anthropic",
				Description:      new("Most capable model"),
				Reasoning:        new(true),
				ReasoningLevels:  new([]string{"low", "medium", "high"}),
				DefaultReasoning: new("medium"),
				ServiceTiers:     new([]string{"priority"}),
			},
		},
		{
			name: "model without optional fields",
			serviceModel: service.Model{
				ID:       "provider/model-id",
				Name:     "Model Name",
				Provider: "Provider",
			},
			expectedInfo: api.ModelInfo{
				Id:       "provider/model-id",
				Name:     "Model Name",
				Provider: "Provider",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := toModelInfos([]service.Model{tc.serviceModel})
			if len(result) != 1 {
				t.Fatalf("expected one model info, got %d", len(result))
			}
			if !reflect.DeepEqual(result[0], tc.expectedInfo) {
				t.Errorf("Conversion failed:\nGot:      %+v\nExpected: %+v", result[0], tc.expectedInfo)
			}
		})
	}
}
