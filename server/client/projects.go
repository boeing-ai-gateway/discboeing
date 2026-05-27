package client

import (
	"context"
	"net/http"
)

// ProjectsService covers project-level CRUD and membership endpoints.
type ProjectsService struct{ client *Client }

type CreateProjectRequest struct {
	Name string `json:"name"`
}

type UpdateProjectRequest struct {
	Name string `json:"name"`
}

func (s *ProjectsService) List(ctx context.Context) ([]Project, error) {
	var out []Project
	if err := s.client.do(ctx, http.MethodGet, "/api/projects", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ProjectsService) Get(ctx context.Context, projectID string) (*Project, error) {
	var out Project
	if err := s.client.do(ctx, http.MethodGet, projectPath(projectID, "/"), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *ProjectsService) Create(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	var out Project
	if err := s.client.do(ctx, http.MethodPost, "/api/projects", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *ProjectsService) Update(ctx context.Context, projectID string, req UpdateProjectRequest) (*Project, error) {
	var out Project
	if err := s.client.do(ctx, http.MethodPut, projectPath(projectID, "/"), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *ProjectsService) Delete(ctx context.Context, projectID string) error {
	return s.client.do(ctx, http.MethodDelete, projectPath(projectID, "/"), nil, nil, nil)
}
