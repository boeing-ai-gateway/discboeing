package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

// serviceSubdomainPattern matches a single {session-id}-svc-{service-id} subdomain component.
// Session IDs are 10-26 alphanumeric chars (case-insensitive in URLs).
// Service IDs are normalized lowercase (a-z0-9_- only).
var serviceSubdomainPattern = regexp.MustCompile(`^([0-9A-Za-z]{10,26})-svc-([a-z0-9_-]+)$`)

const (
	discobotForwardedForHeader   = "X-Discobot-Forwarded-For"
	discobotForwardedHostHeader  = "X-Discobot-Forwarded-Host"
	discobotForwardedPathHeader  = "X-Discobot-Forwarded-Path"
	discobotForwardedProtoHeader = "X-Discobot-Forwarded-Proto"

	serviceProxyRouteCacheTTL     = 10 * time.Second
	serviceProxyRouteCacheMaxSize = 1024
	serviceProxyRouteCacheMaxKey  = 512
	serviceProxySlowLogAfter      = 100 * time.Millisecond
)

// ConnectionTracker tracks active connections per session.
// Implementations must be safe for concurrent use.
type ConnectionTracker interface {
	// Track registers an active connection for sessionID and returns a release
	// function that must be called when the connection ends.
	Track(sessionID string) func()
}

// SandboxService obtains session sandbox information and clients for the service
// proxy. The concrete implementation is service.SandboxService; keeping this
// narrow avoids exposing raw sandbox providers to middleware.
type SandboxService interface {
	GetSandbox(ctx context.Context, sessionID string) (*sandbox.Sandbox, error)
	ListSandboxes(ctx context.Context) ([]*sandbox.Sandbox, error)
	AcquireHTTPClient(ctx context.Context, sessionID string) (*sandbox.HTTPClientLease, error)
}

type serviceProxyRoute struct {
	SessionID string
	ServiceID string
	ExpiresAt time.Time
}

type serviceProxyRouteCache struct {
	mu      sync.Mutex
	entries map[string]serviceProxyRoute
}

func newServiceProxyRouteCache() *serviceProxyRouteCache {
	return &serviceProxyRouteCache{entries: make(map[string]serviceProxyRoute)}
}

func (c *serviceProxyRouteCache) Get(key string, now time.Time) (serviceProxyRoute, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	route, ok := c.entries[key]
	if !ok {
		return serviceProxyRoute{}, false
	}
	if now.After(route.ExpiresAt) {
		delete(c.entries, key)
		return serviceProxyRoute{}, false
	}
	return route, true
}

func (c *serviceProxyRouteCache) Set(key string, route serviceProxyRoute) {
	c.mu.Lock()
	c.pruneExpiredLocked(time.Now())
	if len(c.entries) >= serviceProxyRouteCacheMaxSize {
		c.pruneOldestLocked()
	}
	c.entries[key] = route
	c.mu.Unlock()
}

func (c *serviceProxyRouteCache) Delete(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *serviceProxyRouteCache) pruneExpiredLocked(now time.Time) {
	for key, route := range c.entries {
		if now.After(route.ExpiresAt) {
			delete(c.entries, key)
		}
	}
}

func (c *serviceProxyRouteCache) pruneOldestLocked() {
	var oldestKey string
	var oldestExpiry time.Time
	for key, route := range c.entries {
		if oldestKey == "" || route.ExpiresAt.Before(oldestExpiry) {
			oldestKey = key
			oldestExpiry = route.ExpiresAt
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// findSessionID finds the actual session ID with correct casing.
// DNS/URLs are case-insensitive, so we need to do a case-insensitive lookup.
func findSessionID(ctx context.Context, sandboxSvc SandboxService, urlSessionID string) (string, error) {
	// First try exact match (fast path)
	sb, err := sandboxSvc.GetSandbox(ctx, urlSessionID)
	if err == nil && sb != nil {
		return sb.SessionID, nil
	}

	// Fall back to case-insensitive search via List
	sandboxes, err := sandboxSvc.ListSandboxes(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list sandboxes: %w", err)
	}

	lowerURLSessionID := strings.ToLower(urlSessionID)
	for _, sb := range sandboxes {
		if strings.ToLower(sb.SessionID) == lowerURLSessionID {
			return sb.SessionID, nil
		}
	}

	return "", fmt.Errorf("session not found: %s", urlSessionID)
}

// ServiceProxy creates middleware that intercepts requests to service subdomains
// and proxies them to the agent-api's HTTP proxy endpoint using httputil.ReverseProxy.
//
// Subdomain format: {session-id}-svc-{service-id}.{base-domain}
// Example: 01HXYZ123456789ABCDEFGHIJ-svc-myservice.localhost:3000
//
// The proxy does NOT pass credentials to the agent-api, as service HTTP
// endpoints are considered public within the sandbox.
//
// This properly handles:
// - HTTP/1.1 and HTTP/2
// - WebSocket upgrades
// - Server-Sent Events (SSE)
// - Chunked transfer encoding
// - Request/response streaming
//
// tracker, if non-nil, is notified for every proxied request (including long-lived
// connections such as SSE and WebSocket) so that the idle monitor can avoid
// shutting down sandboxes with live service-proxy connections.
func ServiceProxy(sandboxSvc SandboxService, tracker ConnectionTracker) func(http.Handler) http.Handler {
	routeCache := newServiceProxyRouteCache()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()

			// Check both Host and X-Forwarded-Host for service subdomains.
			// In nested discobot, the outer proxy sets X-Forwarded-Host to
			// the original host before rewriting, so the inner instance's
			// service subdomain may only appear there.
			hosts := []string{r.Host}
			if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" && fwdHost != r.Host {
				hosts = append(hosts, fwdHost)
			}

			ctx := r.Context()
			var sessionID, serviceID string
			routeCacheKey, cacheableRoute := serviceProxyRouteCacheKey(hosts)
			resolveStarted := time.Now()
			cacheHit := false
			if cacheableRoute {
				if route, ok := routeCache.Get(routeCacheKey, resolveStarted); ok {
					sessionID = route.SessionID
					serviceID = route.ServiceID
					cacheHit = true
				}
			}
			if !cacheHit {
				// Split each host into subdomain components and find the first one
				// with a valid session ID. This handles nested discobot where
				// multiple {id}-svc-{name} components may be chained, e.g.:
				//   inner-svc-ui.outer-svc-api.localhost:3001
				// We need to find the component whose session ID exists on THIS instance.
				for _, host := range hosts {
					for part := range strings.SplitSeq(host, ".") {
						matches := serviceSubdomainPattern.FindStringSubmatch(part)
						if matches == nil {
							continue
						}
						sid, err := findSessionID(ctx, sandboxSvc, matches[1])
						if err != nil {
							continue
						}
						sessionID = sid
						serviceID = matches[2]
						break
					}
					if sessionID != "" {
						break
					}
				}
				if sessionID != "" && cacheableRoute {
					routeCache.Set(routeCacheKey, serviceProxyRoute{
						SessionID: sessionID,
						ServiceID: serviceID,
						ExpiresAt: resolveStarted.Add(serviceProxyRouteCacheTTL),
					})
				}
			}
			resolveDuration := time.Since(resolveStarted)

			if sessionID == "" {
				// No valid service subdomain found, continue to next handler
				next.ServeHTTP(w, r)
				return
			}

			// Get HTTP client for the sandbox (handles transport-level routing)
			acquireStarted := time.Now()
			clientLease, err := sandboxSvc.AcquireHTTPClient(ctx, sessionID)
			acquireDuration := time.Since(acquireStarted)
			if err != nil {
				if cacheableRoute {
					routeCache.Delete(routeCacheKey)
				}
				writeJSONError(w, http.StatusBadGateway, "Failed to connect to sandbox", map[string]string{
					"sessionId": sessionID,
					"serviceId": serviceID,
					"message":   err.Error(),
				})
				return
			}
			defer clientLease.Release()

			// Target URL for the agent-api
			// The agent-api expects: /services/:id/http/*
			target, _ := url.Parse("http://sandbox")

			// Create reverse proxy
			proxy := &httputil.ReverseProxy{
				Director: func(req *http.Request) {
					req.URL.Scheme = target.Scheme
					req.URL.Host = target.Host
					req.URL.Path = "/services/" + serviceID + "/http" + r.URL.Path
					req.URL.RawQuery = r.URL.RawQuery

					// Set the Host header to the target
					req.Host = target.Host

					forwardedPath := r.URL.Path
					forwardedProto := getScheme(r)
					forwardedHost := r.Header.Get("X-Forwarded-Host")
					if forwardedHost == "" {
						forwardedHost = r.Host
					}

					// Set x-forwarded-* headers.
					req.Header.Set("X-Forwarded-Path", forwardedPath)
					req.Header.Set("X-Forwarded-Proto", forwardedProto)
					req.Header.Set("X-Forwarded-Host", forwardedHost)

					// Preserve or append X-Forwarded-For
					clientIP := r.RemoteAddr
					if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
						clientIP = clientIP[:idx]
					}
					forwardedFor := clientIP
					if prior := r.Header.Get("X-Forwarded-For"); prior != "" {
						forwardedFor = prior + ", " + clientIP
					}
					req.Header.Set("X-Forwarded-For", forwardedFor)

					// exe.dev's public VM proxy is expected to set/overwrite the
					// standard X-Forwarded-* headers before the request reaches
					// agent-go. Carry Discobot's intended values in private headers
					// so agent-go can restore them before forwarding to the final
					// workspace service.
					req.Header.Set(discobotForwardedForHeader, forwardedFor)
					req.Header.Set(discobotForwardedHostHeader, forwardedHost)
					req.Header.Set(discobotForwardedPathHeader, forwardedPath)
					req.Header.Set(discobotForwardedProtoHeader, forwardedProto)
				},
				Transport: clientLease.Client.Transport,
				ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
					log.Printf("[ServiceProxy] Error proxying request to %s: %v", r.URL.String(), err)
					writeJSONError(w, http.StatusBadGateway, "Service unavailable", map[string]string{
						"sessionId": sessionID,
						"serviceId": serviceID,
						"message":   err.Error(),
					})
				},
				// Streaming support - don't buffer responses
				FlushInterval: -1, // Flush immediately
			}

			// Track this connection so the idle monitor won't stop the sandbox
			// while the request (including long-lived SSE/WebSocket) is in flight.
			if tracker != nil {
				release := tracker.Track(sessionID)
				defer release()
			}

			w.Header().Add("Server-Timing", formatServiceProxyTiming(resolveDuration, acquireDuration, started, cacheHit))
			defer func() {
				totalDuration := time.Since(started)
				if totalDuration >= serviceProxySlowLogAfter {
					log.Printf("[ServiceProxy] slow request host=%q path=%q session=%q service=%q cacheHit=%t resolve=%s acquire=%s total=%s",
						r.Host, r.URL.Path, sessionID, serviceID, cacheHit, resolveDuration, acquireDuration, totalDuration)
				}
			}()

			proxy.ServeHTTP(w, r)
		})
	}
}

func serviceProxyRouteCacheKey(hosts []string) (string, bool) {
	normalized := make([]string, 0, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		normalized = append(normalized, host)
	}
	key := strings.Join(normalized, "\x00")
	if key == "" || len(key) > serviceProxyRouteCacheMaxKey {
		return "", false
	}
	return key, true
}

func formatServiceProxyTiming(resolveDuration, acquireDuration time.Duration, started time.Time, cacheHit bool) string {
	cacheDescription := "miss"
	if cacheHit {
		cacheDescription = "hit"
	}
	return fmt.Sprintf("discobot-resolve;dur=%.3f;desc=%q, discobot-acquire;dur=%.3f, discobot-total-start;dur=%.3f",
		durationMillis(resolveDuration), cacheDescription, durationMillis(acquireDuration), durationMillis(time.Since(started)))
}

func durationMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, errorType string, fields map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Build JSON manually to avoid import cycles
	parts := []string{fmt.Sprintf(`"error":%q`, errorType)}
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf(`%q:%q`, k, v))
	}
	fmt.Fprintf(w, "{%s}", strings.Join(parts, ","))
}

// getScheme returns the request scheme (http or https).
func getScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}
