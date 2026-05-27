package client

import (
	"context"
	"net/http"
	"net/url"
)

// WorkspacesService covers workspace CRUD, validation, providers, and git-backed workspace operations.
type WorkspacesService struct{ client *Client }

type CreateWorkspaceRequest struct {
	Name       string `json:"name,omitempty"`
	Path       string `json:"path"`
	SourceType string `json:"sourceType"`
}

type ValidateWorkspaceRequest struct {
	Path       string `json:"path"`
	SourceType string `json:"sourceType"`
}

type ValidateWorkspaceResponse struct {
	Path           string       `json:"path"`
	SourceType     string       `json:"sourceType"`
	Valid          bool         `json:"valid"`
	Classification string       `json:"classification"`
	Error          string       `json:"error,omitempty"`
	Suggestions    []Suggestion `json:"suggestions"`
	AuthProvider   string       `json:"authProvider,omitempty"`
	AuthRequired   bool         `json:"authRequired,omitempty"`
	AuthMessage    string       `json:"authMessage,omitempty"`
}

type UpdateWorkspaceRequest struct {
	Path        string  `json:"path,omitempty"`
	DisplayName *string `json:"displayName"`
}

type workspacesResponse struct {
	Workspaces []Workspace `json:"workspaces"`
}

func (s *WorkspacesService) List(ctx context.Context, projectID string) ([]Workspace, error) {
	var out workspacesResponse
	if err := s.client.do(ctx, http.MethodGet, projectPath(projectID, "/workspaces/"), nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Workspaces, nil
}

func (s *WorkspacesService) Create(ctx context.Context, projectID string, req CreateWorkspaceRequest) (*Workspace, error) {
	var out Workspace
	if err := s.client.do(ctx, http.MethodPost, projectPath(projectID, "/workspaces/"), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *WorkspacesService) Validate(ctx context.Context, projectID string, req ValidateWorkspaceRequest) (*ValidateWorkspaceResponse, error) {
	var out ValidateWorkspaceResponse
	if err := s.client.do(ctx, http.MethodPost, projectPath(projectID, "/workspaces/validate"), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *WorkspacesService) Update(ctx context.Context, projectID string, workspaceID string, req UpdateWorkspaceRequest) (*Workspace, error) {
	var out Workspace
	if err := s.client.do(ctx, http.MethodPatch, projectPath(projectID, "/workspaces/"+url.PathEscape(workspaceID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *WorkspacesService) Delete(ctx context.Context, projectID string, workspaceID string) error {
	return s.client.do(ctx, http.MethodDelete, projectPath(projectID, "/workspaces/"+url.PathEscape(workspaceID)), nil, nil, nil)
}
