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

// Client calls the Meta REST API.
type Client struct {
	BaseURL     string
	HTTPClient  *http.Client
	BearerToken string
}

// New creates a Meta API client rooted at baseURL.
func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTPClient: http.DefaultClient}
}

// Response contains an HTTP response and its fully-read body.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// DecodeJSON decodes the response body into v.
func (r *Response) DecodeJSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// HTTPError reports a non-2xx response.
type HTTPError struct {
	Response *Response
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("meta api: unexpected status %d", e.Response.StatusCode)
}

type requestConfig struct {
	headers     http.Header
	body        io.Reader
	contentType string
	query       url.Values
}

// RequestOption customizes one generated client request.
type RequestOption func(*requestConfig)

// WithBearerToken sets a bearer token for one request.
func WithBearerToken(token string) RequestOption {
	return func(cfg *requestConfig) {
		if token != "" {
			cfg.headers.Set("Authorization", "Bearer "+token)
		}
	}
}

// WithHeader adds a request header.
func WithHeader(key, value string) RequestOption {
	return func(cfg *requestConfig) {
		cfg.headers.Set(key, value)
	}
}

// WithQuery adds arbitrary query parameters in addition to generated params.
func WithQuery(query url.Values) RequestOption {
	return func(cfg *requestConfig) {
		for key, values := range query {
			for _, value := range values {
				cfg.query.Add(key, value)
			}
		}
	}
}

// WithBody sets a raw request body and content type.
func WithBody(contentType string, body io.Reader) RequestOption {
	return func(cfg *requestConfig) {
		cfg.contentType = contentType
		cfg.body = body
	}
}

// WithJSONBody JSON-encodes a request body.
func WithJSONBody(value any) RequestOption {
	return func(cfg *requestConfig) {
		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(value)
		cfg.contentType = "application/json"
		cfg.body = &buf
	}
}

// Do sends a generic request to the Meta API.
//
// Generated methods call the same underlying request path, but Do is useful for
// CLI experiments and for endpoints that do not yet have typed request/response
// models.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, opts ...RequestOption) (*Response, error) {
	return c.do(ctx, method, path, query, opts...)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, opts ...RequestOption) (*Response, error) {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:3011"
	}
	u, err := url.Parse(baseURL + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for key, values := range query {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()

	cfg := &requestConfig{headers: make(http.Header), query: make(url.Values)}
	if c.BearerToken != "" {
		cfg.headers.Set("Authorization", "Bearer "+c.BearerToken)
	}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	if len(cfg.query) > 0 {
		q = u.Query()
		for key, values := range cfg.query {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), cfg.body)
	if err != nil {
		return nil, err
	}
	for key, values := range cfg.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if cfg.contentType != "" {
		req.Header.Set("Content-Type", cfg.contentType)
	}

	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := &Response{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: body}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, &HTTPError{Response: result}
	}
	return result, nil
}
