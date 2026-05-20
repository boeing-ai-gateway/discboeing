package exedev

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

type fakeCommandClient struct {
	commands []string
	outputs  map[string]string
}

func (c *fakeCommandClient) Exec(_ context.Context, command string) ([]byte, error) {
	c.commands = append(c.commands, command)
	for prefix, output := range c.outputs {
		if strings.HasPrefix(command, prefix) {
			return []byte(output), nil
		}
	}
	if strings.HasPrefix(command, "ssh-key generate-api-key") {
		return []byte(`{"api_key":"vm-api-key"}`), nil
	}
	return []byte(`{"name":"discobot-session-1","status":"running"}`), nil
}

type timeoutThenSuccessClient struct {
	commands []string
}

func (c *timeoutThenSuccessClient) Exec(ctx context.Context, command string) ([]byte, error) {
	c.commands = append(c.commands, command)
	if strings.HasPrefix(command, "ssh-key generate-api-key") {
		return []byte(`{"api_key":"vm-api-key"}`), nil
	}
	if len(c.commands) == 1 {
		return []byte(`{}`), nil
	}
	if len(c.commands) == 2 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return []byte(`{"name":"discobot-session-1","status":"creating","image":"ubuntu:22.04"}`), nil
}

func TestCreateRetriesVisibilityPollAfterRequestTimeout(t *testing.T) {
	oldInterval := createVisibilityPollInterval
	oldRequestTimeout := createVisibilityPollRequestTimeout
	createVisibilityPollInterval = time.Millisecond
	createVisibilityPollRequestTimeout = time.Millisecond
	t.Cleanup(func() {
		createVisibilityPollInterval = oldInterval
		createVisibilityPollRequestTimeout = oldRequestTimeout
	})

	client := &timeoutThenSuccessClient{}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := provider.PrepareState(context.Background(), "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	sb, _, err := provider.Create(context.Background(), state, "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if sb.Status != sandbox.StatusCreated {
		t.Fatalf("sandbox status = %q, want %q", sb.Status, sandbox.StatusCreated)
	}
	if len(client.commands) != 4 {
		t.Fatalf("commands = %v", client.commands)
	}
}

func TestCreateChecksForVisibleVMAfterCommandTimeout(t *testing.T) {
	client := &sequenceCommandClient{responses: []commandResponse{
		{err: commandError{statusCode: http.StatusGatewayTimeout, body: `{"error":"command timed out"}`}},
		{output: `{"name":"discobot-session-1","status":"creating","image":"ubuntu:22.04"}`},
		{output: `{"api_key":"vm-api-key"}`},
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := provider.PrepareState(context.Background(), "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	sb, _, err := provider.Create(context.Background(), state, "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if sb.Status != sandbox.StatusCreated {
		t.Fatalf("sandbox status = %q, want %q", sb.Status, sandbox.StatusCreated)
	}
	if len(client.commands) != 3 {
		t.Fatalf("commands = %v", client.commands)
	}
	if got, want := client.commands[1], "ls --json --l discobot-session-1"; got != want {
		t.Fatalf("second command = %q, want %q", got, want)
	}
}

func TestCreateUsesLSAsAuthoritativeAfterGenericCreateError(t *testing.T) {
	client := &sequenceCommandClient{responses: []commandResponse{
		{err: commandError{statusCode: http.StatusInternalServerError, body: `{"error":"temporary failure"}`}},
		{output: `{"name":"discobot-session-1","status":"creating","image":"ubuntu:22.04"}`},
		{output: `{"api_key":"vm-api-key"}`},
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := provider.PrepareState(context.Background(), "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	sb, newState, err := provider.Create(context.Background(), state, "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if sb.Status != sandbox.StatusCreated {
		t.Fatalf("sandbox status = %q, want %q", sb.Status, sandbox.StatusCreated)
	}
	if got := parseState(newState).VMAPIKey; got != "vm-api-key" {
		t.Fatalf("VM API key = %q", got)
	}
	if len(client.commands) != 3 {
		t.Fatalf("commands = %v", client.commands)
	}
	if got, want := client.commands[1], "ls --json --l discobot-session-1"; got != want {
		t.Fatalf("second command = %q, want %q", got, want)
	}
}

func TestCreateReturnsErrorWhenCommandTimeoutDoesNotCreateVisibleVM(t *testing.T) {
	client := &sequenceCommandClient{responses: []commandResponse{
		{err: commandError{statusCode: http.StatusGatewayTimeout, body: `{"error":"command timed out"}`}},
		{output: `{}`},
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := provider.PrepareState(context.Background(), "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = provider.Create(context.Background(), state, "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err == nil {
		t.Fatal("expected create error")
	}
	if !strings.Contains(err.Error(), "command timed out") {
		t.Fatalf("error = %q, want command timeout", err.Error())
	}
	if !strings.Contains(err.Error(), `VM "discobot-session-1" was not found`) {
		t.Fatalf("error = %q, want VM not found", err.Error())
	}
	if len(client.commands) != 2 {
		t.Fatalf("commands = %v", client.commands)
	}
	if got, want := client.commands[1], "ls --json --l discobot-session-1"; got != want {
		t.Fatalf("second command = %q, want %q", got, want)
	}
}

type sequenceCommandClient struct {
	commands  []string
	responses []commandResponse
}

type commandResponse struct {
	output string
	err    error
}

func (c *sequenceCommandClient) Exec(_ context.Context, command string) ([]byte, error) {
	c.commands = append(c.commands, command)
	if len(c.responses) == 0 {
		return nil, fmt.Errorf("unexpected command %q", command)
	}
	resp := c.responses[0]
	c.responses = c.responses[1:]
	return []byte(resp.output), resp.err
}

func TestSanitizeCommandForLogRedactsSecretEnvValues(t *testing.T) {
	command := joinArgs([]string{
		"new",
		"--json",
		"--env",
		"DISCOBOT_SECRET=sensitive secret",
		"--env",
		"WORKSPACE_SOURCE=https://example.com/repo.git",
		"--env=API_TOKEN=token-value",
		"--env",
		"NORMAL=value with spaces",
	})

	got := sanitizeCommandForLog(command)
	for _, secret := range []string{"sensitive secret", "token-value"} {
		if strings.Contains(got, secret) {
			t.Fatalf("sanitized command %q contains secret %q", got, secret)
		}
	}
	for _, want := range []string{
		"DISCOBOT_SECRET=<redacted>",
		"WORKSPACE_SOURCE=https://example.com/repo.git",
		"API_TOKEN=<redacted>",
		"NORMAL=value with spaces",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sanitized command %q does not contain %q", got, want)
		}
	}
}

func TestHTTPCommandClientRetriesRateLimitResponse(t *testing.T) {
	oldDelay := rateLimitRetryDelay
	rateLimitRetryDelay = 10 * time.Millisecond
	t.Cleanup(func() { rateLimitRetryDelay = oldDelay })

	var mu sync.Mutex
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "ls --json --l discobot-session-1" {
			t.Fatalf("command body = %q", string(body))
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q", got)
		}

		mu.Lock()
		attempts++
		attempt := attempts
		mu.Unlock()

		if attempt == 1 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"name":"discobot-session-1","status":"running"}`))
	}))
	defer server.Close()

	client := &httpCommandClient{
		endpoint: server.URL,
		token:    "token",
		client:   server.Client(),
	}

	start := time.Now()
	out, err := client.Exec(context.Background(), "ls --json --l discobot-session-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"status":"running"`) {
		t.Fatalf("output = %q", string(out))
	}
	if elapsed := time.Since(start); elapsed < rateLimitRetryDelay {
		t.Fatalf("elapsed = %s, want at least %s", elapsed, rateLimitRetryDelay)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestListCoalescesConcurrentCalls(t *testing.T) {
	client := newBlockingListClient(`{"vms":[{"vm_name":"discobot-session-1","status":"running","image":"ubuntu:22.04"}]}`)
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}

	const callers = 5
	errCh := make(chan error, callers)
	for range callers {
		go func() {
			sandboxes, err := provider.List(context.Background())
			if err != nil {
				errCh <- err
				return
			}
			if len(sandboxes) != 1 || sandboxes[0].SessionID != "session-1" {
				errCh <- fmt.Errorf("sandboxes = %#v", sandboxes)
				return
			}
			errCh <- nil
		}()
	}

	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first list call")
	}
	close(client.release)

	for range callers {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
	if calls := client.calls(); calls != 1 {
		t.Fatalf("list calls = %d, want 1", calls)
	}
}

func TestListUsesShortLivedCache(t *testing.T) {
	oldTTL := listCacheTTL
	listCacheTTL = time.Hour
	t.Cleanup(func() { listCacheTTL = oldTTL })

	client := &countingListClient{output: `{"vms":[{"vm_name":"discobot-session-1","status":"running","image":"ubuntu:22.04"}]}`}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}

	first, err := provider.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 {
		t.Fatalf("first list length = %d", len(first))
	}
	first[0].SessionID = "mutated"

	second, err := provider.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].SessionID != "session-1" {
		t.Fatalf("second list = %#v", second)
	}
	if calls := client.calls(); calls != 1 {
		t.Fatalf("list calls = %d, want 1", calls)
	}
}

type blockingListClient struct {
	output  string
	started chan struct{}
	release chan struct{}

	mu        sync.Mutex
	callCount int
}

func newBlockingListClient(output string) *blockingListClient {
	return &blockingListClient{
		output:  output,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (c *blockingListClient) Exec(_ context.Context, command string) ([]byte, error) {
	if command != "ls --json --l" {
		return nil, fmt.Errorf("unexpected command %q", command)
	}
	c.mu.Lock()
	c.callCount++
	if c.callCount == 1 {
		close(c.started)
	}
	c.mu.Unlock()
	<-c.release
	return []byte(c.output), nil
}

func (c *blockingListClient) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callCount
}

type countingListClient struct {
	output string

	mu        sync.Mutex
	callCount int
}

func (c *countingListClient) Exec(_ context.Context, command string) ([]byte, error) {
	if command != "ls --json --l" {
		return nil, fmt.Errorf("unexpected command %q", command)
	}
	c.mu.Lock()
	c.callCount++
	c.mu.Unlock()
	return []byte(c.output), nil
}

func (c *countingListClient) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callCount
}

func TestCreateBuildsNewCommand(t *testing.T) {
	client := &fakeCommandClient{outputs: map[string]string{
		"new": `{"name":"discobot-session-1","status":"running","image":"ubuntu:22.04","created_at":"2026-05-07T04:09:04Z"}`,
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}

	opts := sandbox.CreateOptions{
		SharedSecret: "secret",
		Env: map[string]string{
			"DISCOBOT_PROJECT_ID":  "project-1",
			"DISCOBOT_SECRET":      "hashed-secret",
			"WORKSPACE_TARGET_REF": "main",
		},
		WorkspaceSource:    "https://github.com/obot-platform/discobot.git",
		WorkspaceTargetRef: "main",
		ProjectID:          "project-1",
		Resources: sandbox.ResourceConfig{
			CPUCores: 2,
			MemoryMB: 4096,
			DiskMB:   51200,
		},
	}
	state, err := provider.PrepareState(context.Background(), "session-1", opts)
	if err != nil {
		t.Fatal(err)
	}
	sb, newState, err := provider.Create(context.Background(), state, "session-1", opts)
	if err != nil {
		t.Fatal(err)
	}
	if sb.ID != "discobot-session-1" || sb.Status != sandbox.StatusRunning {
		t.Fatalf("sandbox = %#v", sb)
	}
	if len(client.commands) != 2 {
		t.Fatalf("commands = %v", client.commands)
	}
	command := client.commands[0]
	for _, want := range []string{
		"new",
		"--name=discobot-session-1",
		"--image=ubuntu:22.04",
		"--cpu=2",
		"--memory=4096MB",
		"--disk=51200MB",
		"--env DISCOBOT_PROJECT_ID=project-1",
		"--env WORKSPACE_TARGET_REF=main",
		"--tag=discobot,discobot-session-discobot-session-1",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("command %q does not contain %q", command, want)
		}
	}
	if !strings.Contains(command, "--env DISCOBOT_SECRET=hashed-secret") {
		t.Fatalf("command %q does not include provided secret env", command)
	}
	if got := parseState(newState).VMAPIKey; got != "vm-api-key" {
		t.Fatalf("state VM API key = %q", got)
	}
	if got, want := client.commands[1], "ssh-key generate-api-key --vm=discobot-session-1 --label=discobot-session-session-1"; got != want {
		t.Fatalf("API key command = %q, want %q", got, want)
	}
}

func TestCreateWaitsForReservedVMNameToBecomeVisible(t *testing.T) {
	client := &sequenceCommandClient{responses: []commandResponse{
		{err: fmt.Errorf(`exe.dev command failed with status 422: {"error":"VM name \"discobot-session-1\" is not available"}`)},
		{output: `{"name":"discobot-session-1","status":"creating","image":"ubuntu:22.04"}`},
		{output: `{"api_key":"vm-api-key"}`},
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}

	state, err := provider.PrepareState(context.Background(), "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	sb, newState, err := provider.Create(context.Background(), state, "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if sb.ID != "discobot-session-1" {
		t.Fatalf("sandbox ID = %q", sb.ID)
	}
	if sb.Status != sandbox.StatusCreated {
		t.Fatalf("sandbox status = %q, want %q", sb.Status, sandbox.StatusCreated)
	}
	if got := parseState(newState).VMName; got != "discobot-session-1" {
		t.Fatalf("state VM name = %q", got)
	}
	if len(client.commands) != 3 {
		t.Fatalf("commands = %v", client.commands)
	}
	if !strings.HasPrefix(client.commands[0], "new ") {
		t.Fatalf("first command = %q", client.commands[0])
	}
	if got, want := client.commands[1], "ls --json --l discobot-session-1"; got != want {
		t.Fatalf("second command = %q, want %q", got, want)
	}
	if got, want := client.commands[2], "ssh-key generate-api-key --vm=discobot-session-1 --label=discobot-session-session-1"; got != want {
		t.Fatalf("third command = %q, want %q", got, want)
	}
}

func TestRemoveUsesKnownVMNameWithoutInspecting(t *testing.T) {
	client := &sequenceCommandClient{responses: []commandResponse{
		{output: `{}`},
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := provider.PrepareState(context.Background(), "session-1", sandbox.CreateOptions{SharedSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	newState, err := provider.Remove(context.Background(), state, "session-1", sandbox.RemoveVolumes())
	if err != nil {
		t.Fatal(err)
	}
	if len(newState) != 0 {
		t.Fatalf("new state = %q, want empty", string(newState))
	}
	if len(client.commands) != 1 {
		t.Fatalf("commands = %v", client.commands)
	}
	if got, want := client.commands[0], "rm --json discobot-session-1"; got != want {
		t.Fatalf("remove command = %q, want %q", got, want)
	}
}

func TestNewProviderRequiresToken(t *testing.T) {
	_, err := NewProvider(Config{
		Endpoint: "https://exe.dev/exec",
	})
	if err == nil {
		t.Fatal("expected missing token error")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewProviderTrimsToken(t *testing.T) {
	provider, err := NewProvider(Config{
		Endpoint: "https://exe.dev/exec",
		Token:    " exe1.test-token\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	client, ok := provider.client.(*httpCommandClient)
	if !ok {
		t.Fatalf("client = %T", provider.client)
	}
	if client.token != "exe1.test-token" {
		t.Fatalf("token = %q", client.token)
	}
}

func TestParseVMsFlexibleShapes(t *testing.T) {
	vms := parseVMs([]byte(`{"vms":[{"name":"a","state":"stopped","image_name":"ubuntu"},{"hostname":"b","status":"ready"}]}`))
	if len(vms) != 2 {
		t.Fatalf("len(vms) = %d", len(vms))
	}
	if vms[0].Name != "a" || vms[0].Status != sandbox.StatusStopped || vms[0].Image != "ubuntu" {
		t.Fatalf("vms[0] = %#v", vms[0])
	}
	if vms[1].Name != "b" || vms[1].Status != sandbox.StatusRunning {
		t.Fatalf("vms[1] = %#v", vms[1])
	}
}

func TestParseVMsExeDevListShape(t *testing.T) {
	vms := parseVMs([]byte(`{
		"vms": [
			{
				"created_at": "2026-05-09T22:14:33Z",
				"https_url": "https://discobot-ztivz7cunc9mvc7a.exe.xyz",
				"image": "obot-platform/discobot:main",
				"status": "running",
				"vm_name": "discobot-ztivz7cunc9mvc7a"
			}
		]
	}`))
	if len(vms) != 1 {
		t.Fatalf("len(vms) = %d", len(vms))
	}
	if vms[0].Name != "discobot-ztivz7cunc9mvc7a" {
		t.Fatalf("VM name = %q", vms[0].Name)
	}
	if vms[0].Status != sandbox.StatusRunning {
		t.Fatalf("VM status = %q", vms[0].Status)
	}
	if vms[0].Image != "obot-platform/discobot:main" {
		t.Fatalf("VM image = %q", vms[0].Image)
	}
	if vms[0].CreatedAt.IsZero() {
		t.Fatal("expected created_at to parse")
	}
}

func TestAcquireHTTPClientRewritesRequests(t *testing.T) {
	client := &fakeCommandClient{outputs: map[string]string{
		"ls --json --l discobot-session-1": `{"name":"discobot-session-1","status":"running"}`,
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := marshalState(providerState{
		VMName:   "discobot-session-1",
		VMURL:    "https://discobot-session-1.exe.xyz/",
		VMAPIKey: "vm-api-key",
	})
	if err != nil {
		t.Fatal(err)
	}

	lease, err := provider.AcquireHTTPClient(context.Background(), state, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()

	transport := lease.Client.Transport.(*vmHTTPTransport)
	transport.base = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "discobot-session-1.exe.xyz" {
			t.Fatalf("url = %s", req.URL.String())
		}
		if req.Host != "discobot-session-1.exe.xyz" {
			t.Fatalf("host = %q", req.Host)
		}
		if got := req.Header.Get("X-Exedev-Authorization"); got != "Bearer vm-api-key" {
			t.Fatalf("auth = %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})

	resp, err := lease.Client.Get("http://sandbox/threads")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if got, want := transport.WebSocketURL("ws://sandbox/exec/abc/attach"), "wss://discobot-session-1.exe.xyz/exec/abc/attach"; got != want {
		t.Fatalf("websocket URL = %q, want %q", got, want)
	}
	if got := transport.Headers().Get("X-Exedev-Authorization"); got != "Bearer vm-api-key" {
		t.Fatalf("websocket auth header = %q", got)
	}
}

func TestAcquireHTTPClientUsesPersistedVMTargetWithoutInspecting(t *testing.T) {
	client := &fakeCommandClient{}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := marshalState(providerState{
		VMName:       "discobot-session-1",
		VMURL:        "https://discobot-session-1.exe.xyz/",
		VMAPIKey:     "vm-api-key",
		SharedSecret: "secret",
		CreatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	lease, err := provider.AcquireHTTPClient(context.Background(), state, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()

	if len(client.commands) != 0 {
		t.Fatalf("commands = %v, want none", client.commands)
	}
}

func TestStopRendersJavaScriptStyleNamePlaceholder(t *testing.T) {
	client := &fakeCommandClient{outputs: map[string]string{
		"ls --json --l discobot-session-1": `{"name":"discobot-session-1","status":"running"}`,
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := provider.PrepareState(context.Background(), "session-1", sandbox.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := provider.Stop(context.Background(), state, "session-1", 0); err != nil {
		t.Fatal(err)
	}

	if len(client.commands) == 0 {
		t.Fatalf("commands = %v", client.commands)
	}
	if got, want := client.commands[len(client.commands)-1], "ssh discobot-session-1 sudo shutdown -h now"; got != want {
		t.Fatalf("stop command = %q, want %q", got, want)
	}
}

func TestStopPersistsStoppedStatusForGet(t *testing.T) {
	client := &fakeCommandClient{outputs: map[string]string{
		"ls --json --l discobot-session-1": `{"name":"discobot-session-1","status":"running"}`,
	}}
	provider, err := NewProviderWithClient(testConfig(), client)
	if err != nil {
		t.Fatal(err)
	}
	state, err := marshalState(providerState{
		VMName:       "discobot-session-1",
		VMURL:        "https://discobot-session-1.exe.xyz",
		VMAPIKey:     "vm-api-key",
		SharedSecret: "secret",
		Status:       sandbox.StatusRunning,
		CreatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	state, err = provider.Stop(context.Background(), state, "session-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := parseState(state).Status; got != sandbox.StatusStopped {
		t.Fatalf("persisted status = %q, want %q", got, sandbox.StatusStopped)
	}

	sb, err := provider.Get(context.Background(), state, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if sb.Status != sandbox.StatusStopped {
		t.Fatalf("sandbox status = %q, want %q", sb.Status, sandbox.StatusStopped)
	}
	if len(client.commands) != 2 {
		t.Fatalf("commands = %v, want inspect and stop only", client.commands)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func testConfig() Config {
	return Config{
		SandboxImage: "ubuntu:22.04",
		Token:        "token",
		Endpoint:     "https://exe.dev/exec",
		VMHostSuffix: "exe.xyz",
		VMNamePrefix: "discobot",
		StopCommand:  "ssh ${name} sudo shutdown -h now",
	}
}
