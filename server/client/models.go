package client

import (
	"context"
	"net/http"
)

// ModelsService covers model and provider discovery endpoints.
type ModelsService struct{ client *Client }

type modelsResponse struct {
	Models []ModelInfo `json:"models"`
}

func (s *ModelsService) List(ctx context.Context, projectID string) ([]ModelInfo, error) {
	var out modelsResponse
	if err := s.client.do(ctx, http.MethodGet, projectPath(projectID, "/models"), nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}
