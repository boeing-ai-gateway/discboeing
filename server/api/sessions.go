package api

import (
	"context"
	"net/http"
	"net/url"
)

// SessionsService covers sessions, threads, chat, credentials, files, and lifecycle endpoints.
type SessionsService struct{ client *Client }

// CreateSessionRequest represents the request body for creating a session.
type CreateSessionRequest struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	ProviderID  string `json:"providerId,omitempty"`
}

type UpdateSessionRequest struct {
	Name        *string `json:"name,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
	Description *string `json:"description,omitempty"`
}

type sessionsResponse struct {
	Sessions []Session `json:"sessions"`
}

func (s *SessionsService) List(ctx context.Context, projectID string) ([]Session, error) {
	var out sessionsResponse
	if err := s.client.do(ctx, http.MethodGet, projectPath(projectID, "/sessions/"), nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Sessions, nil
}

func (s *SessionsService) Create(ctx context.Context, projectID string, req CreateSessionRequest) (*Session, error) {
	var out Session
	if err := s.client.do(ctx, http.MethodPost, projectPath(projectID, "/sessions/"), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *SessionsService) Get(ctx context.Context, projectID, sessionID string) (*Session, error) {
	var out Session
	if err := s.client.do(ctx, http.MethodGet, sessionPath(projectID, sessionID, "/"), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *SessionsService) Stop(ctx context.Context, projectID, sessionID string) error {
	return s.client.do(ctx, http.MethodPost, sessionPath(projectID, sessionID, "/stop"), nil, nil, nil)
}

func (s *SessionsService) Update(ctx context.Context, projectID string, sessionID string, req UpdateSessionRequest) (*Session, error) {
	var out Session
	if err := s.client.do(ctx, http.MethodPatch, sessionPath(projectID, sessionID, "/"), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *SessionsService) Delete(ctx context.Context, projectID string, sessionID string) error {
	return s.client.do(ctx, http.MethodDelete, sessionPath(projectID, sessionID, "/"), nil, nil, nil)
}

type CreateThreadRequest struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type UpdateThreadRequest struct {
	Name string `json:"name,omitempty"`
}

func (s *SessionsService) ListThreads(ctx context.Context, projectID string, sessionID string) ([]Thread, error) {
	var out ThreadsResponse
	if err := s.client.do(ctx, http.MethodGet, sessionPath(projectID, sessionID, "/threads"), nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Threads, nil
}

func (s *SessionsService) CreateThread(ctx context.Context, projectID string, sessionID string, req CreateThreadRequest) (*Thread, error) {
	var out Thread
	if err := s.client.do(ctx, http.MethodPost, sessionPath(projectID, sessionID, "/threads"), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *SessionsService) GetThread(ctx context.Context, projectID string, sessionID string, threadID string) (*Thread, error) {
	var out Thread
	path := sessionPath(projectID, sessionID, "/threads/"+url.PathEscape(threadID))
	if err := s.client.do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *SessionsService) UpdateThread(ctx context.Context, projectID string, sessionID string, threadID string, req UpdateThreadRequest) (*Thread, error) {
	var out Thread
	path := sessionPath(projectID, sessionID, "/threads/"+url.PathEscape(threadID))
	if err := s.client.do(ctx, http.MethodPatch, path, nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *SessionsService) DeleteThread(ctx context.Context, projectID string, sessionID string, threadID string) error {
	path := sessionPath(projectID, sessionID, "/threads/"+url.PathEscape(threadID))
	return s.client.do(ctx, http.MethodDelete, path, nil, nil, nil)
}
