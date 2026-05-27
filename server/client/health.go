package client

import (
	"context"
	"net/http"
)

// HealthService covers health, status, config, and API metadata endpoints.
type HealthService struct{ client *Client }

func (s *HealthService) Health(ctx context.Context) (*HealthResponse, error) {
	var out HealthResponse
	if err := s.client.do(ctx, http.MethodGet, "/health", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *HealthService) ServerConfig(ctx context.Context) (*ServerConfig, error) {
	var out ServerConfig
	if err := s.client.do(ctx, http.MethodGet, "/api/server-config", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *HealthService) Routes(ctx context.Context) ([]RouteInfo, error) {
	var out []RouteInfo
	if err := s.client.do(ctx, http.MethodGet, "/api/routes", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
