package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client is a typed HTTP client for the Discobot server API.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client

	Health      *HealthService
	Projects    *ProjectsService
	Workspaces  *WorkspacesService
	Sessions    *SessionsService
	Models      *ModelsService
	Credentials *CredentialsService
	Events      *EventsService
	Files       *FilesService
	Git         *GitService
	Hooks       *HooksService
	Services    *ServicesService
	Terminal    *TerminalService
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client used for API requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// NewClient creates a Discobot API client rooted at baseURL, for example
// http://127.0.0.1:3001.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base URL must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	c := &Client{
		baseURL:    parsed,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}

	c.Health = &HealthService{client: c}
	c.Projects = &ProjectsService{client: c}
	c.Workspaces = &WorkspacesService{client: c}
	c.Sessions = &SessionsService{client: c}
	c.Models = &ModelsService{client: c}
	c.Credentials = &CredentialsService{client: c}
	c.Events = &EventsService{client: c}
	c.Files = &FilesService{client: c}
	c.Git = &GitService{client: c}
	c.Hooks = &HooksService{client: c}
	c.Services = &ServicesService{client: c}
	c.Terminal = &TerminalService{client: c}

	return c, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reqBody = buf
	}

	reqURL := c.resolve(path, query)
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) resolve(path string, query url.Values) string {
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = query.Encode()
	return u.String()
}

// WebSocketURL resolves a server websocket path against the client's base URL.
func (c *Client) WebSocketURL(path string) string {
	u := *c.baseURL
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = ""
	return u.String()
}

func decodeError(resp *http.Response) error {
	var apiErr ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
		return fmt.Errorf("discobot API %s: %s", resp.Status, apiErr.Error)
	}
	return fmt.Errorf("discobot API %s", resp.Status)
}

func projectPath(projectID, suffix string) string {
	return "/api/projects/" + url.PathEscape(projectID) + suffix
}

func sessionPath(projectID, sessionID, suffix string) string {
	return projectPath(projectID, "/sessions/"+url.PathEscape(sessionID)+suffix)
}
