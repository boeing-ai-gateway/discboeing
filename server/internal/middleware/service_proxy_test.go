package middleware

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

// TestServiceSubdomainPattern tests the regex pattern matching for service subdomain segments.
// The pattern matches individual subdomain components (split by "."), not full hosts.
func TestServiceSubdomainPattern(t *testing.T) {
	tests := []struct {
		name        string
		segment     string
		wantMatch   bool
		wantSession string
		wantService string
	}{
		{
			name:        "valid segment with lowercase session ID",
			segment:     "abc123def456ghi7-svc-myservice",
			wantMatch:   true,
			wantSession: "abc123def456ghi7",
			wantService: "myservice",
		},
		{
			name:        "valid segment with mixed case session ID",
			segment:     "AbC123DeF456GhI7-svc-myservice",
			wantMatch:   true,
			wantSession: "AbC123DeF456GhI7",
			wantService: "myservice",
		},
		{
			name:        "valid segment with underscore in service ID",
			segment:     "session12345678901-svc-my_service",
			wantMatch:   true,
			wantSession: "session12345678901",
			wantService: "my_service",
		},
		{
			name:        "valid segment with hyphen in service ID",
			segment:     "session12345678901-svc-my-service",
			wantMatch:   true,
			wantSession: "session12345678901",
			wantService: "my-service",
		},
		{
			name:        "valid segment with numbers in service ID",
			segment:     "session12345678901-svc-service123",
			wantMatch:   true,
			wantSession: "session12345678901",
			wantService: "service123",
		},
		{
			name:        "minimum session ID length (10 chars)",
			segment:     "abcdefghij-svc-svc",
			wantMatch:   true,
			wantSession: "abcdefghij",
			wantService: "svc",
		},
		{
			name:        "maximum session ID length (26 chars)",
			segment:     "abcdefghijklmnopqrstuvwxyz-svc-svc",
			wantMatch:   true,
			wantSession: "abcdefghijklmnopqrstuvwxyz",
			wantService: "svc",
		},
		{
			name:      "session ID too short (9 chars)",
			segment:   "abcdefghi-svc-myservice",
			wantMatch: false,
		},
		{
			name:      "session ID too long (27 chars)",
			segment:   "abcdefghijklmnopqrstuvwxyza-svc-myservice",
			wantMatch: false,
		},
		{
			name:      "plain domain segment",
			segment:   "localhost:3000",
			wantMatch: false,
		},
		{
			name:      "api subdomain segment",
			segment:   "api",
			wantMatch: false,
		},
		{
			name:      "missing -svc- separator",
			segment:   "session12345678901-myservice",
			wantMatch: false,
		},
		{
			name:      "uppercase in service ID (invalid)",
			segment:   "session12345678901-svc-MyService",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := serviceSubdomainPattern.FindStringSubmatch(tt.segment)

			if tt.wantMatch {
				if matches == nil {
					t.Errorf("expected segment %q to match pattern, but it didn't", tt.segment)
					return
				}
				if matches[1] != tt.wantSession {
					t.Errorf("session ID = %q, want %q", matches[1], tt.wantSession)
				}
				if matches[2] != tt.wantService {
					t.Errorf("service ID = %q, want %q", matches[2], tt.wantService)
				}
			} else {
				if matches != nil {
					t.Errorf("expected segment %q NOT to match pattern, but got matches: %v", tt.segment, matches)
				}
			}
		})
	}
}

// mockSandboxService implements the narrow service-proxy sandbox interface for testing.
type mockSandboxProvider struct {
	sandboxes    map[string]*sandbox.Sandbox
	client       *http.Client
	getCalls     int
	listCalls    int
	acquireCalls int
}

func (m *mockSandboxProvider) ImageExists(_ context.Context) bool {
	return true
}

func (m *mockSandboxProvider) Image() string {
	return "test-image"
}

func (m *mockSandboxProvider) PrepareState(_ context.Context, _ string, _ sandbox.CreateOptions) ([]byte, error) {
	return nil, nil
}

func (m *mockSandboxProvider) Create(_ context.Context, state []byte, _ string, _ sandbox.CreateOptions) (*sandbox.Sandbox, []byte, error) {
	return nil, state, nil
}

func (m *mockSandboxProvider) Start(_ context.Context, state []byte, _ string) ([]byte, error) {
	return state, nil
}

func (m *mockSandboxProvider) Stop(_ context.Context, state []byte, _ string, _ time.Duration) ([]byte, error) {
	return state, nil
}

func (m *mockSandboxProvider) Remove(_ context.Context, state []byte, _ string, _ ...sandbox.RemoveOption) ([]byte, error) {
	return state, nil
}

func (m *mockSandboxProvider) GetSandbox(_ context.Context, sessionID string) (*sandbox.Sandbox, error) {
	m.getCalls++
	if sb, ok := m.sandboxes[sessionID]; ok {
		return sb, nil
	}
	return nil, nil
}

func (m *mockSandboxProvider) GetSecret(_ context.Context, _ []byte, _ string) (string, error) {
	return "", nil
}

func (m *mockSandboxProvider) ListSandboxes(_ context.Context) ([]*sandbox.Sandbox, error) {
	m.listCalls++
	var result []*sandbox.Sandbox
	for _, sb := range m.sandboxes {
		result = append(result, sb)
	}
	return result, nil
}

func (m *mockSandboxProvider) AcquireHTTPClient(_ context.Context, _ string) (*sandbox.HTTPClientLease, error) {
	m.acquireCalls++
	return &sandbox.HTTPClientLease{Client: m.client}, nil
}

func (m *mockSandboxProvider) Watch(_ context.Context) (<-chan sandbox.StateEvent, error) {
	return nil, nil
}

func (m *mockSandboxProvider) Reconcile(_ context.Context) error {
	return nil
}

func (m *mockSandboxProvider) RemoveProject(_ context.Context, _ string) error {
	return nil
}

func (m *mockSandboxProvider) ClearCache(_ context.Context, _ string) error {
	return nil
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

	middleware := ServiceProxy(provider, nil)(next)

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

// TestServiceProxySessionNotFound verifies that when no valid session is found,
// the request passes through to the next handler (e.g. may be a nested discobot
// subdomain where none of the session IDs belong to this instance).
func TestServiceProxySessionNotFound(t *testing.T) {
	provider := &mockSandboxProvider{
		sandboxes: map[string]*sandbox.Sandbox{},
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := ServiceProxy(provider, nil)(next)

	req := httptest.NewRequest("GET", "http://nonexistent1234-svc-myservice.localhost:3000/", nil)
	req.Host = "nonexistent1234-svc-myservice.localhost:3000"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("expected next handler to be called when no valid session found")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// TestServiceProxyNestedSubdomains verifies that nested discobot subdomains
// correctly resolve to the first valid session ID.
func TestServiceProxyNestedSubdomains(t *testing.T) {
	outerSessionID := "zivnuflwywnlfxkr"

	// Track what path the proxy sent
	var proxiedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxiedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create a transport that redirects "sandbox" to the test backend
	backendURL, _ := url.Parse(backend.URL)
	transport := &http.Transport{
		DialContext: (&net.Dialer{}).DialContext,
	}
	// Use a custom RoundTripper that rewrites the host
	provider := &mockSandboxProvider{
		sandboxes: map[string]*sandbox.Sandbox{
			outerSessionID: {SessionID: outerSessionID},
		},
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = backendURL.Scheme
				req.URL.Host = backendURL.Host
				return transport.RoundTrip(req)
			}),
		},
	}

	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called for valid nested subdomain")
	})

	middleware := ServiceProxy(provider, nil)(next)

	// Inner session doesn't exist on this instance, outer does
	host := "UMHkK8J0U98kA85p-svc-ui." + outerSessionID + "-svc-api.localhost:3001"
	req := httptest.NewRequest("GET", "http://"+host+"/some/path", nil)
	req.Host = host
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	// Should proxy to the outer session's service
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	// The proxied path should target the outer session's service ID "api"
	wantPath := "/services/api/http/some/path"
	if proxiedPath != wantPath {
		t.Errorf("proxied path = %q, want %q", proxiedPath, wantPath)
	}
}

// TestServiceProxyXForwardedHost verifies that X-Forwarded-Host is checked
// when the Host header doesn't contain a valid service subdomain.
func TestServiceProxyXForwardedHost(t *testing.T) {
	sessionID := "zivnuflwywnlfxkr"

	var proxiedPath string
	var proxiedXFwdHost string
	var proxiedDiscobotFwdHost string
	var proxiedDiscobotFwdPath string
	var proxiedDiscobotFwdProto string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxiedPath = r.URL.Path
		proxiedXFwdHost = r.Header.Get("X-Forwarded-Host")
		proxiedDiscobotFwdHost = r.Header.Get(discobotForwardedHostHeader)
		proxiedDiscobotFwdPath = r.Header.Get(discobotForwardedPathHeader)
		proxiedDiscobotFwdProto = r.Header.Get(discobotForwardedProtoHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	transport := &http.Transport{
		DialContext: (&net.Dialer{}).DialContext,
	}
	provider := &mockSandboxProvider{
		sandboxes: map[string]*sandbox.Sandbox{
			sessionID: {SessionID: sessionID},
		},
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = backendURL.Scheme
				req.URL.Host = backendURL.Host
				return transport.RoundTrip(req)
			}),
		},
	}

	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called when X-Forwarded-Host has valid service subdomain")
	})

	middleware := ServiceProxy(provider, nil)(next)

	// Simulate a nested discobot: Host is internal, but X-Forwarded-Host
	// carries the full multi-level subdomain chain from the outer proxy.
	originalChain := "bCfyeG08yfDammp5-svc-ui." + sessionID + "-svc-api.localhost:3001"
	req := httptest.NewRequest("GET", "http://localhost:3001/some/path", nil)
	req.Host = "localhost:3001"
	req.Header.Set("X-Forwarded-Host", originalChain)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	wantPath := "/services/api/http/some/path"
	if proxiedPath != wantPath {
		t.Errorf("proxied path = %q, want %q", proxiedPath, wantPath)
	}
	// The outgoing X-Forwarded-Host must preserve the full chain so the
	// next nested discobot level can find its own service subdomain.
	if proxiedXFwdHost != originalChain {
		t.Errorf("X-Forwarded-Host = %q, want full chain %q", proxiedXFwdHost, originalChain)
	}
	if proxiedDiscobotFwdHost != originalChain {
		t.Errorf("%s = %q, want %q", discobotForwardedHostHeader, proxiedDiscobotFwdHost, originalChain)
	}
	if proxiedDiscobotFwdPath != "/some/path" {
		t.Errorf("%s = %q, want /some/path", discobotForwardedPathHeader, proxiedDiscobotFwdPath)
	}
	if proxiedDiscobotFwdProto != "http" {
		t.Errorf("%s = %q, want http", discobotForwardedProtoHeader, proxiedDiscobotFwdProto)
	}
}

func TestServiceProxyUsesHTTPClientAcquirer(t *testing.T) {
	sessionID := "zivnuflwywnlfxkr"

	var proxiedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxiedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	transport := &http.Transport{
		DialContext: (&net.Dialer{}).DialContext,
	}
	provider := &mockSandboxProvider{
		sandboxes: map[string]*sandbox.Sandbox{
			sessionID: {SessionID: sessionID},
		},
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = backendURL.Scheme
				req.URL.Host = backendURL.Host
				return transport.RoundTrip(req)
			}),
		},
	}

	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called for valid service subdomain")
	})

	middleware := ServiceProxy(provider, nil)(next)
	host := sessionID + "-svc-ui.localhost:3001"
	req := httptest.NewRequest("GET", "http://"+host+"/hello", nil)
	req.Host = host
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if proxiedPath != "/services/ui/http/hello" {
		t.Fatalf("proxied path = %q, want /services/ui/http/hello", proxiedPath)
	}
	if provider.acquireCalls != 1 {
		t.Fatalf("AcquireHTTPClient called %d times, want 1", provider.acquireCalls)
	}
}

func TestServiceProxyCachesRouteResolution(t *testing.T) {
	sessionID := "zivnuflwywnlfxkr"

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	transport := &http.Transport{
		DialContext: (&net.Dialer{}).DialContext,
	}
	provider := &mockSandboxProvider{
		sandboxes: map[string]*sandbox.Sandbox{
			sessionID: {SessionID: sessionID},
		},
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = backendURL.Scheme
				req.URL.Host = backendURL.Host
				return transport.RoundTrip(req)
			}),
		},
	}

	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called for valid service subdomain")
	})

	middleware := ServiceProxy(provider, nil)(next)
	host := sessionID + "-svc-ui.localhost:3001"
	for i := range 2 {
		req := httptest.NewRequest("GET", "http://"+host+"/asset-"+string(rune('a'+i))+".js", nil)
		req.Host = host
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d", i, rr.Code, http.StatusOK)
		}
		if rr.Header().Get("Server-Timing") == "" {
			t.Fatalf("request %d missing Server-Timing header", i)
		}
	}

	if provider.getCalls != 1 {
		t.Fatalf("GetSandbox called %d times, want 1", provider.getCalls)
	}
	if provider.listCalls != 0 {
		t.Fatalf("ListSandboxes called %d times, want 0", provider.listCalls)
	}
	if provider.acquireCalls != 2 {
		t.Fatalf("AcquireHTTPClient called %d times, want 2", provider.acquireCalls)
	}
}

func TestServiceProxyRouteCacheBoundsEntries(t *testing.T) {
	cache := newServiceProxyRouteCache()
	now := time.Now()

	cache.Set("expired", serviceProxyRoute{
		SessionID: "expiredsession",
		ServiceID: "ui",
		ExpiresAt: now.Add(-time.Second),
	})

	for i := range serviceProxyRouteCacheMaxSize + 10 {
		cache.Set("key-"+strconv.Itoa(i), serviceProxyRoute{
			SessionID: "zivnuflwywnlfxkr",
			ServiceID: "ui",
			ExpiresAt: now.Add(serviceProxyRouteCacheTTL),
		})
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) > serviceProxyRouteCacheMaxSize {
		t.Fatalf("cache size = %d, want at most %d", len(cache.entries), serviceProxyRouteCacheMaxSize)
	}
	if _, ok := cache.entries["expired"]; ok {
		t.Fatal("expired entry was not pruned on insertion")
	}
}

func TestServiceProxyRouteCacheKeyCapsInput(t *testing.T) {
	key, ok := serviceProxyRouteCacheKey([]string{" Session12345678901-SVC-UI.Localhost:3001 "})
	if !ok {
		t.Fatal("expected normal host to be cacheable")
	}
	if key != "session12345678901-svc-ui.localhost:3001" {
		t.Fatalf("cache key = %q, want normalized host", key)
	}

	_, ok = serviceProxyRouteCacheKey([]string{strings.Repeat("a", serviceProxyRouteCacheMaxKey+1)})
	if ok {
		t.Fatal("expected oversized host chain not to be cacheable")
	}
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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
