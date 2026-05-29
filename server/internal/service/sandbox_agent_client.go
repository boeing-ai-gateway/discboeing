package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/agentexec"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
	"github.com/obot-platform/discobot/server/internal/store"
)

// Retry configuration for sandbox requests.
// Uses aggressive initial backoff to catch container startup quickly.
const (
	retryInitialDelay = 50 * time.Millisecond // Start very aggressive
	retryMaxDelay     = 2 * time.Second       // Cap delay
	retryMaxAttempts  = 15                    // Total attempts
	retryMultiplier   = 2.0                   // Double each time
)

// CredentialFetcher is a function that retrieves session credentials with effective visibility for a sandbox request.
type CredentialFetcher func(ctx context.Context, sessionID string) ([]CredentialEnvVar, error)

// MakeCredentialFetcher creates a CredentialFetcher that looks up all project credentials for a session.
// Each credential includes the effective AgentVisible value for that session.
// Returns nil if credSvc is nil (credentials will not be fetched).
func MakeCredentialFetcher(s *store.Store, credSvc *CredentialService) CredentialFetcher {
	if credSvc == nil {
		return nil
	}
	return func(ctx context.Context, sessionID string) ([]CredentialEnvVar, error) {
		sess, err := s.GetSessionByID(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
		return credSvc.GetAllForSession(ctx, sess.ProjectID, sessionID)
	}
}

// SandboxAgentClient handles communication with the agent running in a sandbox.
type SandboxAgentClient struct {
	provider          sandbox.Provider
	credentialFetcher CredentialFetcher

	// gitUserName and gitUserEmail are sent on every request via
	// X-Discobot-Git-User-Name and X-Discobot-Git-User-Email headers.
	gitUserName  string
	gitUserEmail string

	acquireHTTPClientFunc func(context.Context, string) (*sandbox.HTTPClientLease, error)
	getSecretFunc         func(context.Context, string) (string, error)
	getAuthTokenFunc      func(context.Context, string) (string, error)
}

// SandboxChatStartError preserves a structured non-2xx response from the
// sandbox POST /chat endpoint so API handlers can forward it cleanly.
type SandboxChatStartError struct {
	StatusCode   int
	ErrorCode    string
	Message      string
	CompletionID string
	QuestionID   string
}

func (e *SandboxChatStartError) Error() string {
	message := e.Message
	if message == "" {
		message = e.ErrorCode
	}
	if message == "" {
		message = "sandbox chat start failed"
	}
	return fmt.Sprintf("sandbox returned status %d: %s", e.StatusCode, message)
}

// SandboxAgentClientConfig contains optional configuration for a SandboxAgentClient.
// All fields are optional — pass nil for defaults.
type SandboxAgentClientConfig struct {
	// GitUserName is the git user.name sent on every request.
	GitUserName string
	// GitUserEmail is the git user.email sent on every request.
	GitUserEmail string
	// AcquireHTTPClient optionally overrides provider HTTP client acquisition.
	AcquireHTTPClient func(context.Context, string) (*sandbox.HTTPClientLease, error)
	// GetSecret optionally overrides provider secret lookup.
	GetSecret func(context.Context, string) (string, error)
	// GetAuthToken optionally returns a bearer token for sandbox API auth.
	GetAuthToken func(context.Context, string) (string, error)
}

// NewSandboxAgentClient creates a new sandbox agent client.
// The fetcher parameter is optional - if nil, credentials will not be automatically fetched.
// config is optional — pass nil to create a bare client without git or session config.
func NewSandboxAgentClient(provider sandbox.Provider, fetcher CredentialFetcher, config *SandboxAgentClientConfig) *SandboxAgentClient {
	c := &SandboxAgentClient{
		provider:          provider,
		credentialFetcher: fetcher,
	}
	if config != nil {
		c.gitUserName = config.GitUserName
		c.gitUserEmail = config.GitUserEmail
		if config.AcquireHTTPClient != nil {
			c.acquireHTTPClientFunc = config.AcquireHTTPClient
		}
		if config.GetSecret != nil {
			c.getSecretFunc = config.GetSecret
		}
		if config.GetAuthToken != nil {
			c.getAuthTokenFunc = config.GetAuthToken
		}
	}
	return c
}

// threadsURL returns the agent-go thread collection route.
func (c *SandboxAgentClient) threadsURL() string {
	return "http://sandbox/threads"
}

func (c *SandboxAgentClient) configureURL() string {
	return "http://sandbox/configure"
}

func (c *SandboxAgentClient) healthURL() string {
	return "http://sandbox/health"
}

// threadURL returns a URL under the agent-go thread-scoped route.
// For example, threadURL("thread-123", "/chat") returns "http://sandbox/threads/thread-123/chat".
// The HTTP client's transport routes the request to the correct sandbox container
// based on the sessionID passed to acquireHTTPClient — the URL host is always "sandbox".
func (c *SandboxAgentClient) threadURL(threadID, path string) string {
	return c.threadsURL() + "/" + url.PathEscape(threadID) + path
}

// isRetryableError checks if an error is a transient protocol error that should be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Connection refused - container not ready yet
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	// Connection reset - container restarting
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	// Broken pipe - peer accepted the connection and closed it while we were
	// writing the request body, which can happen while the agent is still
	// starting or shutting down.
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	// EOF - connection closed before response (container still starting)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	// Check for common network error patterns in the error string.
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "server closed idle connection") ||
		strings.Contains(errStr, "vsock connect") ||
		strings.Contains(errStr, "connectex:") ||
		strings.Contains(errStr, "actively refused") ||
		strings.Contains(errStr, "no connection could be made") ||
		strings.Contains(errStr, "eof")
}

// isRetryableStatus checks if an HTTP status code should trigger a retry.
func isRetryableStatus(statusCode int) bool {
	return statusCode >= 500 && statusCode < 600
}

// retryWithBackoff executes fn with exponential backoff on retryable errors.
// Returns the result of fn or the last error after max attempts.
func retryWithBackoff[T any](ctx context.Context, fn func() (T, int, error)) (T, error) {
	var zero T
	delay := retryInitialDelay

	for attempt := 1; attempt <= retryMaxAttempts; attempt++ {
		result, statusCode, err := fn()

		// Success
		if err == nil && !isRetryableStatus(statusCode) {
			return result, nil
		}

		// Check if we should retry
		shouldRetry := isRetryableError(err) || isRetryableStatus(statusCode)
		if !shouldRetry || attempt == retryMaxAttempts {
			if err != nil {
				return zero, err
			}
			if isRetryableStatus(statusCode) {
				return zero, fmt.Errorf("sandbox returned retryable status %d after %d attempts", statusCode, attempt)
			}
			return result, nil
		}

		// Wait before retry, respecting context cancellation
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}

		// Increase delay for next iteration
		delay = min(time.Duration(float64(delay)*retryMultiplier), retryMaxDelay)
	}

	return zero, fmt.Errorf("max retry attempts exceeded")
}

// SSELine represents a raw SSE event from the sandbox.
// The content is passed through without parsing - the sandbox
// is expected to send data in AI SDK UIMessage Stream format.
type SSELine struct {
	// ID is the optional SSE event id.
	ID string
	// Event is the optional SSE event name.
	Event string
	// Data is the raw JSON payload (without "data: " prefix).
	Data string
	// Done indicates the upstream emitted a terminal done marker.
	// Agent-go no longer sends this, but the field is kept so the client can
	// tolerate older stream sources during the transition.
	Done bool
}

// acquireHTTPClient returns a leased HTTP client configured for the sandbox.
// Callers must release the lease after the request or stream completes.
func (c *SandboxAgentClient) acquireHTTPClient(ctx context.Context, sessionID string) (*sandbox.HTTPClientLease, error) {
	if c.acquireHTTPClientFunc != nil {
		lease, err := c.acquireHTTPClientFunc(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		return c.withSandboxAuth(ctx, lease, sessionID), nil
	}
	lease, err := sandbox.AcquireHTTPClient(ctx, c.provider, nil, sessionID)
	if err != nil {
		return nil, err
	}
	return c.withSandboxAuth(ctx, lease, sessionID), nil
}

func (c *SandboxAgentClient) withSandboxAuth(ctx context.Context, lease *sandbox.HTTPClientLease, sessionID string) *sandbox.HTTPClientLease {
	lease.Client = &http.Client{
		Transport: &sandboxAuthTransport{
			base: lease.Client.Transport,
			token: func(tokenCtx context.Context) string {
				if tokenCtx == nil {
					tokenCtx = ctx
				}
				return c.sandboxAuthToken(tokenCtx, sessionID)
			},
		},
		Timeout: lease.Client.Timeout,
	}
	return lease
}

func (c *SandboxAgentClient) sandboxAuthToken(ctx context.Context, sessionID string) string {
	tokens := c.sandboxAuthTokens(ctx, sessionID)
	if len(tokens) > 0 {
		return tokens[0]
	}
	return ""
}

func (c *SandboxAgentClient) sandboxAuthTokens(ctx context.Context, sessionID string) []string {
	var tokens []string
	addToken := func(token string) {
		if token == "" {
			return
		}
		if slices.Contains(tokens, token) {
			return
		}
		tokens = append(tokens, token)
	}

	if c.getAuthTokenFunc != nil {
		token, err := c.getAuthTokenFunc(ctx, sessionID)
		if err == nil {
			addToken(token)
		}
	}
	if c.getSecretFunc != nil {
		secret, err := c.getSecretFunc(ctx, sessionID)
		if err == nil {
			addToken(secret)
		}
		return tokens
	}
	secret, err := c.provider.GetSecret(ctx, nil, sessionID)
	if err == nil {
		addToken(secret)
	}
	return tokens
}

type sandboxAuthTransport struct {
	base  http.RoundTripper
	token func(context.Context) string
}

func (t *sandboxAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if token := t.authToken(req.Context()); token != "" && clone.Header.Get("Authorization") == "" {
		clone.Header.Set("Authorization", "Bearer "+token)
	}
	return t.baseTransport().RoundTrip(clone)
}

func (t *sandboxAuthTransport) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if transport, ok := t.base.(*http.Transport); ok && transport.DialContext != nil {
		return transport.DialContext(ctx, network, addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func (t *sandboxAuthTransport) Headers() http.Header {
	headers := cloneHeaders(t.baseHeaders())
	if token := t.authToken(context.Background()); token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}
	return headers
}

func (t *sandboxAuthTransport) WebSocketURL(rawURL string) string {
	if transport, ok := t.base.(interface{ WebSocketURL(string) string }); ok {
		return transport.WebSocketURL(rawURL)
	}
	return rawURL
}

func (t *sandboxAuthTransport) baseTransport() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}

func (t *sandboxAuthTransport) baseHeaders() http.Header {
	if transport, ok := t.base.(interface{ Headers() http.Header }); ok {
		return transport.Headers()
	}
	return nil
}

func (t *sandboxAuthTransport) authToken(ctx context.Context) string {
	if t.token == nil {
		return ""
	}
	return t.token(ctx)
}

func cloneHeaders(headers http.Header) http.Header {
	clone := make(http.Header)
	for key, values := range headers {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}

// Attach creates an interactive PTY session to the sandbox.
// If user is empty, the container's default user is used.
// env contains additional environment variables to set in the session.
func (c *SandboxAgentClient) Attach(ctx context.Context, sessionID string, rows, cols int, user, workDir string, env map[string]string) (sandbox.PTY, error) {
	lease, err := c.acquireHTTPClient(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	session, err := agentexec.Create(ctx, lease.Client, agentexec.CreateRequest{
		Kind:    "user",
		WorkDir: workDir,
		Env:     env,
		User:    user,
		TTY:     true,
		Rows:    rows,
		Cols:    cols,
	})
	if err != nil {
		lease.Release()
		return nil, fmt.Errorf("%w: %v", sandbox.ErrAttachFailed, err)
	}
	stream, err := agentexec.Attach(ctx, lease, session.ID)
	if err != nil {
		_ = agentexec.Kill(context.Background(), lease.Client, session.ID)
		lease.Release()
		return nil, fmt.Errorf("%w: %v", sandbox.ErrAttachFailed, err)
	}
	return stream, nil
}

// AttachTerminal creates or reuses a persistent terminal PTY for a session.
func (c *SandboxAgentClient) AttachTerminal(ctx context.Context, sessionID string, rows, cols int, user, workDir, reuseKey string, env map[string]string) (sandbox.PTY, error) {
	lease, err := c.acquireHTTPClient(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return agentexec.CreateAndAttach(ctx, lease, agentexec.CreateRequest{
		Kind:     "user",
		Name:     "terminal",
		ReuseKey: "terminal:" + reuseKey,
		HomeDir:  workDir == "",
		WorkDir:  workDir,
		Env:      env,
		User:     user,
		TTY:      true,
		Rows:     rows,
		Cols:     cols,
	})
}

// ExecStream runs a command with bidirectional streaming I/O in the session's sandbox.
func (c *SandboxAgentClient) ExecStream(ctx context.Context, sessionID string, cmd []string, opts sandbox.ExecStreamOptions) (sandbox.Stream, error) {
	lease, err := c.acquireHTTPClient(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	session, err := agentexec.Create(ctx, lease.Client, agentexec.CreateRequest{
		Kind:    "user",
		Cmd:     cmd,
		WorkDir: opts.WorkDir,
		Env:     opts.Env,
		User:    opts.User,
		TTY:     opts.TTY,
	})
	if err != nil {
		lease.Release()
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}
	stream, err := agentexec.Attach(ctx, lease, session.ID)
	if err != nil {
		_ = agentexec.Kill(context.Background(), lease.Client, session.ID)
		lease.Release()
		return nil, fmt.Errorf("failed to attach exec: %w", err)
	}
	return stream, nil
}

// RequestOptions contains optional parameters for sandbox requests.
type RequestOptions struct {
	// SkipCredentials opts out of automatic credential fetching.
	// By default, credentials are fetched and sent with requests.
	SkipCredentials bool
	authToken       string

	// Reasoning controls extended thinking using a string reasoning level such as
	// "auto", "low", "medium", "high", "xhigh", "none", "default", or
	// "" for model/provider default behavior.
	Reasoning string

	// ServiceTier optionally selects a provider latency tier, such as "fast".
	ServiceTier string

	// RunAfter queues the prompt until the given RFC3339 timestamp.
	RunAfter string

	// LastEventID forwards the client's SSE resume cursor when reconnecting.
	LastEventID string
}

func optsRunAfter(opts *RequestOptions) string {
	if opts == nil {
		return ""
	}
	return opts.RunAfter
}

func (c *SandboxAgentClient) requestAuthTokens(ctx context.Context, sessionID string, opts *RequestOptions) []string {
	if opts != nil && opts.authToken != "" {
		return []string{opts.authToken}
	}
	tokens := c.sandboxAuthTokens(ctx, sessionID)
	if len(tokens) == 0 {
		return []string{""}
	}
	return tokens
}

func requestOptionsWithAuthToken(opts *RequestOptions, authToken string) *RequestOptions {
	if opts == nil {
		return &RequestOptions{authToken: authToken}
	}
	copied := *opts
	copied.authToken = authToken
	return &copied
}

type GetCommitsRequest struct {
	TargetCommit string
	HeadCommit   string
	Directory    string
}

// applyRequestAuth sets Authorization and credentials headers on a request.
// Credentials are automatically fetched unless SkipCredentials is set.
func (c *SandboxAgentClient) applyRequestAuth(ctx context.Context, req *http.Request, sessionID string, opts *RequestOptions) error {
	// Add Authorization header with Bearer token
	token := ""
	if opts != nil && opts.authToken != "" {
		token = opts.authToken
	} else {
		token = c.sandboxAuthToken(ctx, sessionID)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Auto-fetch credentials if fetcher is set and not skipped
	skipCreds := opts != nil && opts.SkipCredentials
	if c.credentialFetcher != nil && !skipCreds {
		creds, err := c.credentialFetcher(ctx, sessionID)
		if err != nil {
			// Log warning but don't fail - credentials are optional
			fmt.Printf("Warning: failed to fetch credentials for session %s: %v\n", sessionID, err)
		} else if len(creds) > 0 {
			credJSON, err := json.Marshal(creds)
			if err != nil {
				return fmt.Errorf("failed to marshal credentials: %w", err)
			}
			req.Header.Set("X-Discobot-Credentials", string(credJSON))
		}
	}

	// Add git user config headers from client configuration
	if c.gitUserName != "" {
		req.Header.Set("X-Discobot-Git-User-Name", c.gitUserName)
	}
	if c.gitUserEmail != "" {
		req.Header.Set("X-Discobot-Git-User-Email", c.gitUserEmail)
	}

	return nil
}

type sandboxConfigureRequest struct {
	Model                  string             `json:"model,omitempty"`
	WorkspaceOrigin        string             `json:"workspaceOrigin,omitempty"`
	WorkspaceSource        string             `json:"workspaceSource,omitempty"`
	WorkspaceSourceType    string             `json:"workspaceSourceType,omitempty"`
	WorkspaceCommit        string             `json:"workspaceCommit,omitempty"`
	WorkspaceTargetRef     string             `json:"workspaceTargetRef,omitempty"`
	SessionID              string             `json:"sessionId,omitempty"`
	MCPOAuthRedirectBase   string             `json:"mcpOAuthRedirectBase,omitempty"`
	DiscobotServerURL      string             `json:"discobotServerUrl,omitempty"`
	DiscobotProjectID      string             `json:"discobotProjectId,omitempty"`
	EnableGitControlSocket bool               `json:"enableGitControlSocket,omitempty"`
	Credentials            []CredentialEnvVar `json:"credentials,omitempty"`
	GitUserName            string             `json:"gitUserName,omitempty"`
	GitUserEmail           string             `json:"gitUserEmail,omitempty"`
}

type sandboxConfigureEvent struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (c *SandboxAgentClient) NeedsConfigure(ctx context.Context, sessionID string) (bool, error) {
	return retryWithBackoff(ctx, func() (bool, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return true, 0, err
		}
		defer lease.Release()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.healthURL(), nil)
		if err != nil {
			return false, 0, err
		}
		if err := c.applyRequestAuth(ctx, req, sessionID, &RequestOptions{SkipCredentials: true}); err != nil {
			return false, 0, err
		}
		resp, err := lease.Client.Do(req)
		if err != nil {
			return true, 0, err
		}
		defer resp.Body.Close()

		var body struct {
			Code       string `json:"code"`
			Configured *bool  `json:"configured"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body)
		if body.Code == "AGENT_NOT_CONFIGURED" {
			return true, 0, nil
		}
		if body.Configured != nil {
			return !*body.Configured, 0, nil
		}
		return false, resp.StatusCode, nil
	})
}

// Configure posts dynamic runtime configuration to an agent-go instance that is
// waiting in bootstrap mode and consumes its SSE progress stream until it
// reaches a terminal ready/error state.
func (c *SandboxAgentClient) Configure(ctx context.Context, sessionID string, cfg sandboxConfigureRequest, progress func(sandboxConfigureEvent)) error {
	bodyBytes, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal configure request: %w", err)
	}

	var configureLease *sandbox.HTTPClientLease
	authTokens := c.sandboxAuthTokens(ctx, sessionID)
	if len(authTokens) == 0 {
		authTokens = []string{""}
	}
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		for i, authToken := range authTokens {
			lease, err := c.acquireHTTPClient(ctx, sessionID)
			if err != nil {
				return nil, 0, err
			}
			client := *lease.Client
			client.Timeout = 0

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.configureURL(), bytes.NewReader(bodyBytes))
			if err != nil {
				lease.Release()
				return nil, 0, err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")
			if err := c.applyRequestAuth(ctx, req, sessionID, &RequestOptions{SkipCredentials: true, authToken: authToken}); err != nil {
				lease.Release()
				return nil, 0, err
			}

			resp, err := client.Do(req)
			if err != nil {
				lease.Release()
				return nil, 0, err
			}
			if resp.StatusCode == http.StatusUnauthorized && i < len(authTokens)-1 {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				lease.Release()
				continue
			}
			if isRetryableStatus(resp.StatusCode) {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				lease.Release()
				return nil, resp.StatusCode, fmt.Errorf("configure returned status %d", resp.StatusCode)
			}
			configureLease = lease
			return resp, resp.StatusCode, nil
		}
		return nil, 0, fmt.Errorf("configure failed before request")
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer func() {
		if configureLease != nil {
			configureLease.Release()
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("configure returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var last sandboxConfigureEvent
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var event sandboxConfigureEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return fmt.Errorf("failed to decode configure event: %w", err)
		}
		last = event
		if progress != nil {
			progress(event)
		}
		switch event.Status {
		case "ready":
			return nil
		case "error":
			if event.Error == "" {
				event.Error = "agent configuration failed"
			}
			return fmt.Errorf("%s", event.Error)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed reading configure stream: %w", err)
	}
	if last.Status == "ready" {
		return nil
	}
	return fmt.Errorf("configure stream ended before ready")
}

// StartChat sends messages to the sandbox and returns the initial completion metadata.
func (c *SandboxAgentClient) StartChat(ctx context.Context, sessionID, threadID string, messages json.RawMessage, model string, opts *RequestOptions) (*sandboxapi.ChatStartedResponse, error) {
	// Build the request body once - pass messages through as-is
	reasoning := ""
	serviceTier := ""
	if opts != nil {
		reasoning = opts.Reasoning
		serviceTier = opts.ServiceTier
	}
	reqBody := sandboxapi.ChatRequest{
		Messages:    messages,
		Model:       model,
		Reasoning:   reasoning,
		ServiceTier: serviceTier,
		RunAfter:    optsRunAfter(opts),
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	authTokens := c.requestAuthTokens(ctx, sessionID, opts)
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		for i, authToken := range authTokens {
			lease, err := c.acquireHTTPClient(ctx, sessionID)
			if err != nil {
				return nil, 0, err
			}
			client := lease.Client

			req, err := http.NewRequestWithContext(ctx, "POST", c.threadURL(threadID, "/chat"), bytes.NewReader(bodyBytes))
			if err != nil {
				lease.Release()
				return nil, 0, fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			if err := c.applyRequestAuth(ctx, req, sessionID, requestOptionsWithAuthToken(opts, authToken)); err != nil {
				lease.Release()
				return nil, 0, err
			}

			resp, err := client.Do(req)
			if err != nil {
				lease.Release()
				return nil, 0, err
			}
			if resp.StatusCode == http.StatusUnauthorized && i < len(authTokens)-1 {
				log.Printf("[SandboxAgentClient] Sandbox returned 401 for session %s start chat using auth candidate %d/%d; retrying next candidate", sessionID, i+1, len(authTokens))
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				lease.Release()
				continue
			}
			lease.Release()
			return resp, resp.StatusCode, nil
		}
		return nil, 0, fmt.Errorf("chat start failed before request")
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		var conflict sandboxapi.ChatConflictResponse
		if err := json.Unmarshal(body, &conflict); err == nil && conflict.Error == "completion_in_progress" {
			return nil, &SandboxChatStartError{
				StatusCode:   resp.StatusCode,
				ErrorCode:    conflict.Error,
				CompletionID: conflict.CompletionID,
			}
		}
		var turnConflict sandboxapi.ChatTurnStateConflictResponse
		if err := json.Unmarshal(body, &turnConflict); err == nil && turnConflict.Error != "" {
			return nil, &SandboxChatStartError{
				StatusCode: resp.StatusCode,
				ErrorCode:  turnConflict.Error,
				Message:    turnConflict.Message,
				QuestionID: turnConflict.QuestionID,
			}
		}
		var apiErr sandboxapi.ErrorResponse
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != "" {
			return nil, &SandboxChatStartError{
				StatusCode: resp.StatusCode,
				ErrorCode:  apiErr.Error,
			}
		}
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read sandbox response: %w", err)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return &sandboxapi.ChatStartedResponse{Status: "started"}, nil
	}

	var started sandboxapi.ChatStartedResponse
	if err := json.Unmarshal(body, &started); err != nil {
		return nil, fmt.Errorf("failed to decode sandbox response: %w", err)
	}
	if started.Status == "" {
		started.Status = "started"
	}
	return &started, nil
}

// SendMessages sends messages to the sandbox and returns a channel of raw SSE lines.
// The sandbox is expected to respond with SSE events in AI SDK UIMessage Stream format.
// Messages and responses are passed through without parsing.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) SendMessages(ctx context.Context, sessionID, threadID string, messages json.RawMessage, model string, opts *RequestOptions) (<-chan SSELine, error) {
	if _, err := c.StartChat(ctx, sessionID, threadID, messages, model, opts); err != nil {
		return nil, err
	}

	// POST returns 202 Accepted - now GET the SSE stream
	return c.GetStream(ctx, sessionID, threadID, opts)
}

// GetStream connects to the sandbox's long-lived SSE stream for a thread.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) GetStream(ctx context.Context, sessionID, threadID string, opts *RequestOptions) (<-chan SSELine, error) {
	// Use retry logic to handle transient connection errors during container startup
	var streamLease *sandbox.HTTPClientLease
	authTokens := c.requestAuthTokens(ctx, sessionID, opts)
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		for i, authToken := range authTokens {
			lease, err := c.acquireHTTPClient(ctx, sessionID)
			if err != nil {
				// Don't retry on sandbox not running - let caller handle reconciliation
				return nil, 0, err
			}
			client := *lease.Client
			client.Timeout = 0 // SSE stream - no timeout

			// URL host is ignored - the client's transport handles routing to the sandbox
			req, err := http.NewRequestWithContext(ctx, "GET", c.threadURL(threadID, "/chat/stream"), nil)
			if err != nil {
				lease.Release()
				return nil, 0, fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Accept", "text/event-stream")
			if opts != nil && opts.LastEventID != "" {
				req.Header.Set("Last-Event-ID", opts.LastEventID)
			}

			if err := c.applyRequestAuth(ctx, req, sessionID, requestOptionsWithAuthToken(opts, authToken)); err != nil {
				lease.Release()
				return nil, 0, err
			}

			resp, err := client.Do(req)
			if err != nil {
				lease.Release()
				return nil, 0, err
			}
			if resp.StatusCode == http.StatusUnauthorized && i < len(authTokens)-1 {
				log.Printf("[SandboxAgentClient] Sandbox returned 401 for session %s chat stream using auth candidate %d/%d; retrying next candidate", sessionID, i+1, len(authTokens))
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				lease.Release()
				continue
			}

			streamLease = lease

			return resp, resp.StatusCode, nil
		}
		return nil, 0, fmt.Errorf("chat stream failed before request")
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if streamLease != nil {
			streamLease.Release()
		}
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	// Create channel for raw SSE lines
	lineCh := make(chan SSELine, 100)

	// Start goroutine to read SSE lines and pass through
	go func() {
		defer close(lineCh)
		defer func() { _ = resp.Body.Close() }()
		defer func() {
			if streamLease != nil {
				streamLease.Release()
			}
		}()

		if err := streamSSELines(ctx, resp.Body, lineCh); err != nil && ctx.Err() == nil {
			log.Printf("[SandboxAgentClient] Error reading chat stream for session %s: %v", sessionID, err)
			errorData, marshalErr := json.Marshal(struct {
				Type      string `json:"type"`
				ErrorText string `json:"errorText"`
			}{
				Type:      "error",
				ErrorText: fmt.Sprintf("failed to read chat stream: %v", err),
			})
			if marshalErr == nil {
				select {
				case lineCh <- SSELine{Data: string(errorData)}:
				case <-ctx.Done():
				}
			}
			return
		}
	}()

	return lineCh, nil
}

func streamSSELines(ctx context.Context, body io.Reader, lineCh chan<- SSELine) error {
	reader := newChunkedLineReader(body)
	current := SSELine{}
	hasCurrent := false

	emitCurrent := func() bool {
		if !hasCurrent {
			return true
		}
		if current.Event == "done" {
			current.Done = true
		}
		if current.Data == "[DONE]" {
			current.Event = "done"
			current.Data = `{}`
			current.Done = true
		}
		select {
		case lineCh <- current:
			current = SSELine{}
			hasCurrent = false
			return true
		case <-ctx.Done():
			return false
		}
	}

	for {
		line, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		if strings.HasPrefix(line, ":") {
			continue
		}
		if line == "" {
			if !emitCurrent() {
				return ctx.Err()
			}
			continue
		}

		hasCurrent = true
		field, value, hasColon := strings.Cut(line, ":")
		if hasColon && strings.HasPrefix(value, " ") {
			value = value[1:]
		}

		switch field {
		case "id":
			current.ID = value
		case "event":
			current.Event = value
		case "data":
			if current.Data == "" {
				current.Data = value
			} else {
				current.Data += "\n" + value
			}
		}
	}

	if hasCurrent {
		if !emitCurrent() {
			return ctx.Err()
		}
	}

	return nil
}

type chunkedLineReader struct {
	reader *bytes.Buffer
	source io.Reader
}

func newChunkedLineReader(source io.Reader) *chunkedLineReader {
	return &chunkedLineReader{
		reader: &bytes.Buffer{},
		source: source,
	}
}

func (r *chunkedLineReader) ReadLine() (string, error) {
	for {
		if idx := bytes.IndexByte(r.reader.Bytes(), '\n'); idx >= 0 {
			line := r.reader.Next(idx + 1)
			return strings.TrimRight(string(line), "\r\n"), nil
		}

		buf := make([]byte, 32*1024)
		n, err := r.source.Read(buf)
		if n > 0 {
			_, _ = r.reader.Write(buf[:n])
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) && r.reader.Len() > 0 {
				line := r.reader.Next(r.reader.Len())
				return strings.TrimRight(string(line), "\r\n"), nil
			}
			return "", err
		}
	}
}

// ListThreads retrieves all threads from the sandbox agent.
func (c *SandboxAgentClient) ListThreads(ctx context.Context, sessionID string) (*sandboxapi.ListThreadsResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "GET", c.threadsURL(), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list threads: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.ListThreadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetSessionActivity retrieves the aggregate thread activity snapshot from the
// sandbox agent in a single session-level request.
func (c *SandboxAgentClient) GetSessionActivity(ctx context.Context, sessionID string) (*sandboxapi.SessionActivityResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.threadsURL()+"/activity", nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		if err := c.applyRequestAuth(ctx, req, sessionID, &RequestOptions{SkipCredentials: true}); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session activity: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.SessionActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// StreamSessionActivity connects to the sandbox agent's session-level activity
// SSE stream. The agent sends an initial snapshot followed by snapshots whenever
// activity changes.
func (c *SandboxAgentClient) StreamSessionActivity(ctx context.Context, sessionID string) (<-chan *sandboxapi.SessionActivityResponse, error) {
	var streamLease *sandbox.HTTPClientLease
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		client := *lease.Client
		client.Timeout = 0

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.threadsURL()+"/activity/stream", nil)
		if err != nil {
			lease.Release()
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Accept", "text/event-stream")
		if err := c.applyRequestAuth(ctx, req, sessionID, &RequestOptions{SkipCredentials: true}); err != nil {
			lease.Release()
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			lease.Release()
			return nil, 0, err
		}
		if isRetryableStatus(resp.StatusCode) {
			_ = resp.Body.Close()
			lease.Release()
			return nil, resp.StatusCode, nil
		}
		streamLease = lease
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to stream session activity: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if streamLease != nil {
			streamLease.Release()
		}
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	activityCh := make(chan *sandboxapi.SessionActivityResponse, 16)
	go func() {
		defer close(activityCh)
		defer func() { _ = resp.Body.Close() }()
		defer func() {
			if streamLease != nil {
				streamLease.Release()
			}
		}()

		lineCh := make(chan SSELine, 16)
		errCh := make(chan error, 1)
		go func() {
			errCh <- streamSSELines(ctx, resp.Body, lineCh)
			close(lineCh)
		}()

		for line := range lineCh {
			if line.Event == "ping" || line.Data == "" {
				continue
			}
			if line.Event != "" && line.Event != "activity" {
				continue
			}
			var snapshot sandboxapi.SessionActivityResponse
			if err := json.Unmarshal([]byte(line.Data), &snapshot); err != nil {
				log.Printf("[SandboxAgentClient] Failed to decode activity event for session %s: %v", sessionID, err)
				continue
			}
			select {
			case activityCh <- &snapshot:
			case <-ctx.Done():
				return
			}
		}

		if err := <-errCh; err != nil && ctx.Err() == nil {
			log.Printf("[SandboxAgentClient] Error reading activity stream for session %s: %v", sessionID, err)
		}
	}()

	return activityCh, nil
}

// GetThread retrieves a specific thread from the sandbox agent.
func (c *SandboxAgentClient) GetThread(ctx context.Context, sessionID, threadID string) (*sandboxapi.Thread, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "GET", c.threadURL(threadID, ""), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.Thread
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateThread creates a new thread in the sandbox agent.
func (c *SandboxAgentClient) CreateThread(ctx context.Context, sessionID string, reqBody *sandboxapi.CreateThreadRequest) (*sandboxapi.Thread, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "POST", c.threadsURL(), bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.Thread
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateThread updates a thread in the sandbox agent.
func (c *SandboxAgentClient) UpdateThread(ctx context.Context, sessionID, threadID string, reqBody *sandboxapi.UpdateThreadRequest) (*sandboxapi.Thread, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "PUT", c.threadURL(threadID, ""), bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update thread: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.Thread
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteQueuedPrompt removes a queued prompt from a thread in the sandbox agent.
func (c *SandboxAgentClient) DeleteQueuedPrompt(ctx context.Context, sessionID, threadID, queuedPromptID string) (*sandboxapi.DeleteQueuedPromptResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "DELETE", c.threadURL(threadID, "/queue/"+url.PathEscape(queuedPromptID)), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete queued prompt: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.DeleteQueuedPromptResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateQueuedPrompt updates a queued prompt in the sandbox agent.
func (c *SandboxAgentClient) UpdateQueuedPrompt(ctx context.Context, sessionID, threadID, queuedPromptID string, reqBody *sandboxapi.UpdateQueuedPromptRequest) (*sandboxapi.UpdateQueuedPromptResponse, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.threadURL(threadID, "/queue/"+url.PathEscape(queuedPromptID)), bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update queued prompt: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.UpdateQueuedPromptResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteThread removes a thread from the sandbox agent.
func (c *SandboxAgentClient) DeleteThread(ctx context.Context, sessionID, threadID string) (*sandboxapi.DeleteThreadResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "DELETE", c.threadURL(threadID, ""), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete thread: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.DeleteThreadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetChatStatus retrieves the completion status for a thread from the sandbox.
// Calls GET /threads/{id}/chat/status which returns {"isRunning": bool}.
func (c *SandboxAgentClient) GetChatStatus(ctx context.Context, sessionID, threadID string) (*sandboxapi.ChatStatusResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "GET", c.threadURL(threadID, "/chat/status"), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, &RequestOptions{SkipCredentials: true}); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get chat status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var status sandboxapi.ChatStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode chat status: %w", err)
	}
	return &status, nil
}

// CancelCompletion cancels an in-progress completion in the sandbox.
// Returns ErrNoActiveCompletion if no completion is active (409 status).
// Retries with exponential backoff on connection errors.
func (c *SandboxAgentClient) CancelCompletion(ctx context.Context, sessionID, threadID string) (*CancelCompletionResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "POST", c.threadURL(threadID, "/chat/cancel"), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to cancel completion: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusConflict {
		return nil, ErrNoActiveCompletion
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result CancelCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// AskUserQuestion Methods
// ============================================================================

// GetQuestion returns the pending AskUserQuestion for a specific question ID.
func (c *SandboxAgentClient) GetQuestion(ctx context.Context, sessionID, threadID string, toolUseID string) (*sandboxapi.PendingQuestionResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		url := c.threadURL(threadID, "/chat/question")
		if toolUseID != "" {
			url = c.threadURL(threadID, "/chat/question/"+toolUseID)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, &RequestOptions{SkipCredentials: true}); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get question: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.PendingQuestionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// AnswerQuestion submits the user's answer to a pending AskUserQuestion.
func (c *SandboxAgentClient) AnswerQuestion(ctx context.Context, sessionID, threadID string, req *sandboxapi.AnswerQuestionRequest) (*sandboxapi.AnswerQuestionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.threadURL(threadID, "/chat/answer/"+req.ToolUseID), bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		if err := c.applyRequestAuth(ctx, httpReq, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to answer question: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoActiveCompletion
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.AnswerQuestionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// File System Methods
// ============================================================================

// ListFiles lists directory contents in the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) ListFiles(ctx context.Context, sessionID string, path string, includeHidden bool) (*sandboxapi.ListFilesResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		params := url.Values{}
		params.Set("path", path)
		if includeHidden {
			params.Set("hidden", "true")
		}

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/files?"+params.Encode(), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.ListFilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// SearchFiles performs a fuzzy search over workspace files in the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) SearchFiles(ctx context.Context, sessionID string, query string, limit int) (*sandboxapi.SearchFilesResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		params := url.Values{}
		params.Set("q", query)
		params.Set("limit", fmt.Sprintf("%d", limit))

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/files/search?"+params.Encode(), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.SearchFilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ReadFile reads file content from the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) ReadFile(ctx context.Context, sessionID string, path string) (*sandboxapi.ReadFileResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		params := url.Values{}
		params.Set("path", path)

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/files/read?"+params.Encode(), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.ReadFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ReadThreadArtifact reads a thread-local artifact from the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) ReadThreadArtifact(ctx context.Context, sessionID, threadID, uri string) (*sandboxapi.ReadFileResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		params := url.Values{}
		params.Set("uri", uri)

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/threads/"+url.PathEscape(threadID)+"/artifacts/read?"+params.Encode(), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read thread artifact: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.ReadFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// WriteFile writes file content to the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) WriteFile(ctx context.Context, sessionID string, req *sandboxapi.WriteFileRequest) (*sandboxapi.WriteFileResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		httpReq, err := http.NewRequestWithContext(ctx, "POST", "http://sandbox/files/write", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		if err := c.applyRequestAuth(ctx, httpReq, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.WriteFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteFile deletes a file or directory in the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) DeleteFile(ctx context.Context, sessionID string, req *sandboxapi.DeleteFileRequest) (*sandboxapi.DeleteFileResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		httpReq, err := http.NewRequestWithContext(ctx, "POST", "http://sandbox/files/delete", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		if err := c.applyRequestAuth(ctx, httpReq, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.DeleteFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// RenameFile renames/moves a file or directory in the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) RenameFile(ctx context.Context, sessionID string, req *sandboxapi.RenameFileRequest) (*sandboxapi.RenameFileResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		httpReq, err := http.NewRequestWithContext(ctx, "POST", "http://sandbox/files/rename", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		if err := c.applyRequestAuth(ctx, httpReq, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to rename file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.RenameFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetUserInfo retrieves the default user info from the sandbox.
// This is used to determine which user to run terminal sessions as.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) GetUserInfo(ctx context.Context, sessionID string) (*sandboxapi.UserResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/user", nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListCommands retrieves available slash commands from the sandbox.
func (c *SandboxAgentClient) ListCommands(ctx context.Context, sessionID string) (*sandboxapi.ListCommandsResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/commands", nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list commands: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.ListCommandsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetDiff retrieves diff information from the sandbox.
// If path is non-empty, returns a single file diff.
// If format is "files", returns just file paths.
// Otherwise returns full diff with patches.
// When targetCommit is non-empty, the sandbox diffs against that commit/ref.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) GetDiff(ctx context.Context, sessionID, path, format, targetCommit string) (any, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		// Build URL with query parameters
		params := url.Values{}
		if path != "" {
			params.Set("path", path)
		}
		if format != "" {
			params.Set("format", format)
		}
		if strings.TrimSpace(targetCommit) != "" {
			params.Set("target", strings.TrimSpace(targetCommit))
		}

		requestURL := "http://sandbox/diff"
		if encoded := params.Encode(); encoded != "" {
			requestURL += "?" + encoded
		}

		req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	// Decode based on request parameters
	if path != "" {
		var result sandboxapi.SingleFileDiffResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &result, nil
	}

	if format == "files" {
		var result sandboxapi.DiffFilesResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &result, nil
	}

	var result sandboxapi.DiffResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GetCommits retrieves git format-patch output from the sandbox for changes
// relative to a target commit and optional explicit tip commit in an optional
// git working directory.
// Returns the patch string and commit count on success, or an error on failure.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) GetCommits(ctx context.Context, sessionID string, commitsReq GetCommitsRequest) (*sandboxapi.CommitsResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		values := url.Values{}
		if strings.TrimSpace(commitsReq.TargetCommit) != "" {
			values.Set("target", strings.TrimSpace(commitsReq.TargetCommit))
		}
		if strings.TrimSpace(commitsReq.HeadCommit) != "" {
			values.Set("head", strings.TrimSpace(commitsReq.HeadCommit))
		}
		if strings.TrimSpace(commitsReq.Directory) != "" {
			values.Set("cwd", strings.TrimSpace(commitsReq.Directory))
		}

		url := "http://sandbox/commits"
		if encoded := values.Encode(); encoded != "" {
			url += "?" + encoded
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for error responses (400, 404, 409)
	if resp.StatusCode != http.StatusOK {
		var errResp sandboxapi.CommitsErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
		}
		if errResp.Error == "no_commits" {
			return nil, &CommitsNoOpError{IsClean: errResp.IsClean, HeadCommit: errResp.HeadCommit}
		}
		return nil, fmt.Errorf("commits error (%s): %s", errResp.Error, errResp.Message)
	}

	var result sandboxapi.CommitsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Hook Methods
// ============================================================================

// GetHooksStatus retrieves hook evaluation status from the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) GetHooksStatus(ctx context.Context, sessionID string) (*sandboxapi.HooksStatusResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/hooks/status", nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get hooks status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.HooksStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetHooksState retrieves hook status and inline outputs from the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) GetHooksState(ctx context.Context, sessionID string) (*sandboxapi.HooksStateResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/hooks/state", nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get hooks state: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.HooksStateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetHookOutput retrieves the output log for a specific hook from the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) GetHookOutput(ctx context.Context, sessionID, hookID string) (*sandboxapi.HookOutputResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		url := fmt.Sprintf("http://sandbox/hooks/%s/output", hookID)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get hook output: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.HookOutputResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DownloadHookOutput retrieves the full hook output log from the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) DownloadHookOutput(ctx context.Context, sessionID, hookID string) ([]byte, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		url := fmt.Sprintf("http://sandbox/hooks/%s/output/download", hookID)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download hook output: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// RerunHook manually reruns a specific hook in the sandbox.
func (c *SandboxAgentClient) RerunHook(ctx context.Context, sessionID, hookID string) (*sandboxapi.HookRerunResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		url := fmt.Sprintf("http://sandbox/hooks/%s/rerun", hookID)
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to rerun hook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.HookRerunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateHooksExecution toggles whether hook failures report back to the LLM.
func (c *SandboxAgentClient) UpdateHooksExecution(ctx context.Context, sessionID string, paused bool) (*sandboxapi.HooksStatusResponse, error) {
	body, err := json.Marshal(sandboxapi.UpdateHooksExecutionRequest{Paused: paused})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "PATCH", "http://sandbox/hooks/execution", bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update hook execution: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.HooksStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateHookExecution toggles whether one hook reports failures back to the LLM.
func (c *SandboxAgentClient) UpdateHookExecution(ctx context.Context, sessionID, hookID string, paused bool) (*sandboxapi.HooksStatusResponse, error) {
	body, err := json.Marshal(sandboxapi.UpdateHooksExecutionRequest{Paused: paused})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		url := fmt.Sprintf("http://sandbox/hooks/%s/execution", hookID)
		req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update hook execution: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.HooksStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Service Methods
// ============================================================================

// ListServices retrieves all services from the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) ListServices(ctx context.Context, sessionID string) (*sandboxapi.ListServicesResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(ctx, "GET", "http://sandbox/services", nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.ListServicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// StartService starts a service in the sandbox.
// Returns immediately with status "starting" (202 Accepted).
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) StartService(ctx context.Context, sessionID string, serviceID string) (*sandboxapi.StartServiceResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		url := "http://sandbox/services/" + serviceID + "/start"
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 202 Accepted is success, also handle 200 OK
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.StartServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// StopService stops a service in the sandbox.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) StopService(ctx context.Context, sessionID string, serviceID string) (*sandboxapi.StopServiceResponse, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		url := "http://sandbox/services/" + serviceID + "/stop"
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to stop service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.StopServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetServiceOutput connects to the sandbox's SSE stream for service output.
// Returns a channel of raw SSE lines. The channel is closed when the service
// stops or the context is cancelled.
// Retries with exponential backoff on connection errors and 5xx responses.
func (c *SandboxAgentClient) GetServiceOutput(ctx context.Context, sessionID string, serviceID string) (<-chan SSELine, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client
		client.Timeout = 0 // SSE stream - no timeout

		url := "http://sandbox/services/" + serviceID + "/output"
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Accept", "text/event-stream")

		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}

		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get service output: %w", err)
	}

	// 404 means service not found
	if resp.StatusCode == http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("service not found: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	// Create channel for raw SSE lines
	lineCh := make(chan SSELine, 100)

	// Start goroutine to read SSE lines and pass through
	go func() {
		defer close(lineCh)
		defer func() { _ = resp.Body.Close() }()

		serviceCh := make(chan SSELine, 100)
		doneCh := make(chan error, 1)
		go func() {
			doneCh <- streamSSELines(ctx, resp.Body, serviceCh)
			close(serviceCh)
		}()

		for line := range serviceCh {
			if line.Data == "" && !line.Done {
				continue
			}
			select {
			case lineCh <- line:
			case <-ctx.Done():
				return
			}
		}

		if err := <-doneCh; err != nil && ctx.Err() == nil {
			log.Printf("[SandboxAgentClient] Error reading service output stream for session %s: %v", sessionID, err)
		}
	}()

	return lineCh, nil
}
