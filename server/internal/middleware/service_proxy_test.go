package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

// TestParseServiceSubdomain tests the hostname parsing for service subdomains
func TestParseServiceSubdomain(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		wantMatch   bool
		wantSession string
		wantService string
		wantInner   string
	}{
		{
			name:        "valid subdomain with lowercase session ID",
			host:        "abc123def456ghi7-svc-myservice.localhost:3000",
			wantMatch:   true,
			wantSession: "abc123def456ghi7",
			wantService: "myservice",
			wantInner:   "localhost:3000",
		},
		{
			name:        "valid subdomain with mixed case session ID",
			host:        "AbC123DeF456GhI7-svc-myservice.example.com",
			wantMatch:   true,
			wantSession: "AbC123DeF456GhI7",
			wantService: "myservice",
			wantInner:   "example.com",
		},
		{
			name:        "valid subdomain with underscore in service ID",
			host:        "session12345678901-svc-my_service.localhost:3000",
			wantMatch:   true,
			wantSession: "session12345678901",
			wantService: "my_service",
			wantInner:   "localhost:3000",
		},
		{
			name:        "valid subdomain with hyphen in service ID",
			host:        "session12345678901-svc-my-service.localhost:3000",
			wantMatch:   true,
			wantSession: "session12345678901",
			wantService: "my-service",
			wantInner:   "localhost:3000",
		},
		{
			name:        "valid subdomain with numbers in service ID",
			host:        "session12345678901-svc-service123.localhost:3000",
			wantMatch:   true,
			wantSession: "session12345678901",
			wantService: "service123",
			wantInner:   "localhost:3000",
		},
		{
			name:        "minimum session ID length (10 chars)",
			host:        "abcdefghij-svc-svc.localhost:3000",
			wantMatch:   true,
			wantSession: "abcdefghij",
			wantService: "svc",
			wantInner:   "localhost:3000",
		},
		{
			name:        "maximum session ID length (26 chars)",
			host:        "abcdefghijklmnopqrstuvwxyz-svc-svc.localhost:3000",
			wantMatch:   true,
			wantSession: "abcdefghijklmnopqrstuvwxyz",
			wantService: "svc",
			wantInner:   "localhost:3000",
		},
		{
			name:      "session ID too short (9 chars)",
			host:      "abcdefghi-svc-myservice.localhost:3000",
			wantMatch: false,
		},
		{
			name:      "session ID too long (27 chars)",
			host:      "abcdefghijklmnopqrstuvwxyza-svc-myservice.localhost:3000",
			wantMatch: false,
		},
		{
			name:      "no service subdomain - regular domain",
			host:      "localhost:3000",
			wantMatch: false,
		},
		{
			name:      "no service subdomain - api subdomain",
			host:      "api.localhost:3000",
			wantMatch: false,
		},
		{
			name:      "missing -svc- separator",
			host:      "session12345678901-myservice.localhost:3000",
			wantMatch: false,
		},
		{
			name:      "uppercase in service ID (invalid)",
			host:      "session12345678901-svc-MyService.localhost:3000",
			wantMatch: false,
		},
		// Nested Discobot tests
		{
			name:        "nested: two levels of service subdomains",
			host:        "D9szBjRw19MEtS8K-svc-ui.3vuqc0jqv5qmt3me-svc-api.localhost:3001",
			wantMatch:   true,
			wantSession: "3vuqc0jqv5qmt3me",
			wantService: "api",
			wantInner:   "D9szBjRw19MEtS8K-svc-ui.localhost:3001",
		},
		{
			name:        "nested: three levels of service subdomains",
			host:        "innermost1234567-svc-web.middle1234567890-svc-ui.outer12345678901-svc-api.localhost:3001",
			wantMatch:   true,
			wantSession: "outer12345678901",
			wantService: "api",
			wantInner:   "innermost1234567-svc-web.middle1234567890-svc-ui.localhost:3001",
		},
		{
			name:        "nested: service subdomain with multi-part base domain",
			host:        "inner12345678901-svc-ui.outer12345678901-svc-api.app.example.com",
			wantMatch:   true,
			wantSession: "outer12345678901",
			wantService: "api",
			wantInner:   "inner12345678901-svc-ui.app.example.com",
		},
		{
			name:        "no port",
			host:        "session12345678901-svc-myservice.localhost",
			wantMatch:   true,
			wantSession: "session12345678901",
			wantService: "myservice",
			wantInner:   "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID, serviceID, innerHost, ok := parseServiceSubdomain(tt.host)

			if tt.wantMatch {
				if !ok {
					t.Errorf("expected host %q to match pattern, but it didn't", tt.host)
					return
				}
				if sessionID != tt.wantSession {
					t.Errorf("session ID = %q, want %q", sessionID, tt.wantSession)
				}
				if serviceID != tt.wantService {
					t.Errorf("service ID = %q, want %q", serviceID, tt.wantService)
				}
				if innerHost != tt.wantInner {
					t.Errorf("inner host = %q, want %q", innerHost, tt.wantInner)
				}
			} else {
				if ok {
					t.Errorf("expected host %q NOT to match pattern, but got session=%q service=%q inner=%q",
						tt.host, sessionID, serviceID, innerHost)
				}
			}
		})
	}
}

// mockSandboxProvider implements sandbox.Provider for testing
type mockSandboxProvider struct {
	sandboxes map[string]*sandbox.Sandbox
	client    *http.Client
}

func (m *mockSandboxProvider) ImageExists(_ context.Context) bool {
	return true
}

func (m *mockSandboxProvider) Image() string {
	return "test-image"
}

func (m *mockSandboxProvider) Create(_ context.Context, _ string, _ sandbox.CreateOptions) (*sandbox.Sandbox, error) {
	return nil, nil
}

func (m *mockSandboxProvider) Start(_ context.Context, _ string) error {
	return nil
}

func (m *mockSandboxProvider) Stop(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockSandboxProvider) Remove(_ context.Context, _ string, _ ...sandbox.RemoveOption) error {
	return nil
}

func (m *mockSandboxProvider) Get(_ context.Context, sessionID string) (*sandbox.Sandbox, error) {
	if sb, ok := m.sandboxes[sessionID]; ok {
		return sb, nil
	}
	return nil, nil
}

func (m *mockSandboxProvider) GetSecret(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockSandboxProvider) List(_ context.Context) ([]*sandbox.Sandbox, error) {
	var result []*sandbox.Sandbox
	for _, sb := range m.sandboxes {
		result = append(result, sb)
	}
	return result, nil
}

func (m *mockSandboxProvider) Exec(_ context.Context, _ string, _ []string, _ sandbox.ExecOptions) (*sandbox.ExecResult, error) {
	return nil, nil
}

func (m *mockSandboxProvider) Attach(_ context.Context, _ string, _ sandbox.AttachOptions) (sandbox.PTY, error) {
	return nil, nil
}

func (m *mockSandboxProvider) ExecStream(_ context.Context, _ string, _ []string, _ sandbox.ExecStreamOptions) (sandbox.Stream, error) {
	return nil, nil
}

func (m *mockSandboxProvider) HTTPClient(_ context.Context, _ string) (*http.Client, error) {
	return m.client, nil
}

func (m *mockSandboxProvider) Watch(_ context.Context) (<-chan sandbox.StateEvent, error) {
	return nil, nil
}

// TestServiceProxyNonServiceSubdomain verifies that non-service requests pass through
func TestServiceProxyNonServiceSubdomain(t *testing.T) {
	provider := &mockSandboxProvider{
		sandboxes: map[string]*sandbox.Sandbox{},
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("next handler"))
	})

	middleware := ServiceProxy(provider)(next)

	tests := []struct {
		name string
		host string
	}{
		{"regular localhost", "localhost:3000"},
		{"api subdomain", "api.localhost:3000"},
		{"production domain", "app.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false
			req := httptest.NewRequest("GET", "http://"+tt.host+"/some/path", nil)
			req.Host = tt.host
			rr := httptest.NewRecorder()

			middleware.ServeHTTP(rr, req)

			if !nextCalled {
				t.Errorf("expected next handler to be called for host %q", tt.host)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
			}
		})
	}
}

// TestServiceProxySessionNotFound verifies error handling when session doesn't exist
func TestServiceProxySessionNotFound(t *testing.T) {
	provider := &mockSandboxProvider{
		sandboxes: map[string]*sandbox.Sandbox{},
	}

	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called")
	})

	middleware := ServiceProxy(provider)(next)

	req := httptest.NewRequest("GET", "http://nonexistent1234-svc-myservice.localhost:3000/", nil)
	req.Host = "nonexistent1234-svc-myservice.localhost:3000"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}

	// Check that response contains error information
	body := rr.Body.String()
	if body == "" {
		t.Error("expected error response body")
	}
}

// TestFindSessionIDCaseInsensitive verifies case-insensitive session ID lookup
func TestFindSessionIDCaseInsensitive(t *testing.T) {
	provider := &mockSandboxProvider{
		sandboxes: map[string]*sandbox.Sandbox{
			"AbCdEfGhIjKlMnOp": {SessionID: "AbCdEfGhIjKlMnOp"},
		},
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		urlID     string
		wantID    string
		wantError bool
	}{
		{
			name:   "exact match",
			urlID:  "AbCdEfGhIjKlMnOp",
			wantID: "AbCdEfGhIjKlMnOp",
		},
		{
			name:   "lowercase match",
			urlID:  "abcdefghijklmnop",
			wantID: "AbCdEfGhIjKlMnOp",
		},
		{
			name:   "uppercase match",
			urlID:  "ABCDEFGHIJKLMNOP",
			wantID: "AbCdEfGhIjKlMnOp",
		},
		{
			name:      "no match",
			urlID:     "notexisting1234",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findSessionID(ctx, provider, tt.urlID)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.wantID {
				t.Errorf("findSessionID() = %q, want %q", got, tt.wantID)
			}
		})
	}
}

// TestGetScheme tests scheme detection
func TestGetScheme(t *testing.T) {
	tests := []struct {
		name       string
		setupReq   func(*http.Request)
		wantScheme string
	}{
		{
			name:       "plain HTTP",
			setupReq:   func(_ *http.Request) {},
			wantScheme: "http",
		},
		{
			name: "X-Forwarded-Proto https",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "https")
			},
			wantScheme: "https",
		},
		{
			name: "X-Forwarded-Proto http (explicit)",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "http")
			},
			wantScheme: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/", nil)
			tt.setupReq(req)

			got := getScheme(req)
			if got != tt.wantScheme {
				t.Errorf("getScheme() = %q, want %q", got, tt.wantScheme)
			}
		})
	}
}
