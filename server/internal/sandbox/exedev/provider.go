package exedev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

const containerPort = 3002
const defaultEndpoint = "https://exe.dev/exec"
const defaultVMHostSuffix = "exe.xyz"
const defaultVMNamePrefix = "discobot"
const defaultStopCommand = "ssh ${name} sudo shutdown -h now"
const defaultSandboxImage = "ghcr.io/obot-platform/discobot:main"
const stopCommandNamePlaceholder = "${name}"

var (
	createVisibilityPollInterval       = 2 * time.Second
	createVisibilityPollRequestTimeout = 15 * time.Second
	createVisibilityMaxWait            = 2 * time.Minute
	vmRunningMaxWait                   = 10 * time.Minute
	rateLimitRetryDelay                = 5 * time.Second
	rateLimitRetryTimeout              = 2 * time.Minute
	listCacheTTL                       = 2 * time.Second
)

var nonDNSName = regexp.MustCompile(`[^a-z0-9-]+`)

type commandClient interface {
	Exec(ctx context.Context, command string) ([]byte, error)
}

type Config struct {
	Endpoint     string `json:"endpoint,omitempty"`
	Token        string `json:"token,omitempty"`
	VMHostSuffix string `json:"vmHostSuffix,omitempty"`
	VMNamePrefix string `json:"vmNamePrefix,omitempty"`
	StopCommand  string `json:"stopCommand,omitempty"`
	SandboxImage string `json:"sandboxImage,omitempty"`
}

// Provider implements sandbox.Provider using exe.dev's HTTPS command API.
type Provider struct {
	cfg Config

	client    commandClient
	httpCache *sandbox.HTTPClientCache

	listMu       sync.Mutex
	listCache    listCacheEntry
	listInFlight *listCall
}

type listCacheEntry struct {
	sandboxes []*sandbox.Sandbox
	expires   time.Time
}

type listCall struct {
	done      chan struct{}
	sandboxes []*sandbox.Sandbox
	err       error
}

type providerState struct {
	VMName       string         `json:"vmName,omitempty"`
	VMURL        string         `json:"vmUrl,omitempty"`
	VMAPIKey     string         `json:"vmApiKey,omitempty"`
	SharedSecret string         `json:"sharedSecret,omitempty"`
	Status       sandbox.Status `json:"status,omitempty"`
	CreatedAt    time.Time      `json:"createdAt,omitzero"`
}

// NewProvider creates an exe.dev provider backed by POST /exec.
func NewProvider(cfg Config) (*Provider, error) {
	cfg = cfg.withDefaults()
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("exe.dev endpoint is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("exe.dev token is required")
	}
	return NewProviderWithClient(cfg, &httpCommandClient{
		endpoint: cfg.Endpoint,
		client:   &http.Client{Timeout: 2 * time.Minute},
		token:    cfg.Token,
	})
}

func NewProviderWithClient(cfg Config, client commandClient) (*Provider, error) {
	if client == nil {
		return nil, fmt.Errorf("exe.dev command client is required")
	}
	return &Provider{
		cfg:       cfg.withDefaults(),
		client:    client,
		httpCache: sandbox.NewHTTPClientCache(),
	}, nil
}

func (c Config) withDefaults() Config {
	c.Token = strings.TrimSpace(c.Token)
	if c.Endpoint == "" {
		c.Endpoint = defaultEndpoint
	}
	if c.VMHostSuffix == "" {
		c.VMHostSuffix = defaultVMHostSuffix
	}
	if c.VMNamePrefix == "" {
		c.VMNamePrefix = defaultVMNamePrefix
	}
	if c.StopCommand == "" {
		c.StopCommand = defaultStopCommand
	}
	if c.SandboxImage == "" {
		c.SandboxImage = defaultSandboxImage
	}
	return c
}

func (p *Provider) ImageExists(context.Context) bool { return true }

func (p *Provider) Image() string { return p.cfg.SandboxImage }

func (p *Provider) IsLocal() bool { return false }

func (p *Provider) Definition() sandbox.ProviderDefinition {
	return Definition()
}

func Definition() sandbox.ProviderDefinition {
	return sandbox.ProviderDefinition{
		Name:        "exe.dev",
		Icon:        "https://exe.dev/static/exy.png",
		Description: "exe.dev VM sandbox driver",
		ConfigFields: []sandbox.ProviderConfigField{
			{Key: "endpoint", Label: "Command endpoint", Type: "text", Placeholder: defaultEndpoint, Description: "HTTPS endpoint used by Discobot to issue exe.dev commands.", Advanced: true},
			{Key: "credentialId", Label: "API credential", Type: "credential", Description: "Credential containing the exe.dev API token.", Required: true, CredentialProvider: "exedev", CredentialAuthType: "api_key"},
			{Key: "vmHostSuffix", Label: "VM host suffix", Type: "text", Placeholder: defaultVMHostSuffix, Description: "DNS suffix used to reach created VMs.", Advanced: true},
			{Key: "vmNamePrefix", Label: "VM name prefix", Type: "text", Placeholder: defaultVMNamePrefix, Description: "Prefix for VMs created by Discobot.", Advanced: true},
			{Key: "stopCommand", Label: "Stop command", Type: "textarea", Placeholder: defaultStopCommand, Description: "Optional command template used when stopping a VM. ${name} is replaced with the quoted VM name.", Advanced: true},
			{Key: "sandboxImage", Label: "Sandbox image", Type: "text", Placeholder: defaultSandboxImage, Description: "Optional sandbox image override for this provider instance. Leave blank to use the remote sandbox image setting.", Advanced: true},
		},
	}
}

func (p *Provider) Status() sandbox.ProviderStatus {
	status := sandbox.ProviderStatus{Available: true, State: "ready"}
	if p.cfg.Token == "" {
		status.Available = false
		status.State = "not_available"
		status.Message = "exe.dev token is not configured"
	}
	return status
}

func (p *Provider) PrepareState(_ context.Context, sessionID string, opts sandbox.CreateOptions) ([]byte, error) {
	name := vmName(p.cfg.VMNamePrefix, sessionID)
	return marshalState(providerState{
		VMName:       name,
		VMURL:        p.vmURL(name),
		SharedSecret: opts.SharedSecret,
	})
}

func (p *Provider) Create(ctx context.Context, state []byte, sessionID string, opts sandbox.CreateOptions) (*sandbox.Sandbox, []byte, error) {
	ps := parseState(state)
	name := firstNonEmpty(ps.VMName, vmName(p.cfg.VMNamePrefix, sessionID))
	env := maps.Clone(opts.Env)
	cmd := buildNewCommand(name, p.cfg.SandboxImage, env, opts)
	log.Printf("Creating exe.dev VM %q for session %s with image %q", name, sessionID, p.cfg.SandboxImage)
	out, err := p.client.Exec(ctx, cmd)
	var vm vmInfo
	if err != nil {
		log.Printf("exe.dev VM creation command failed for %q session %s; checking VM state with ls: %v", name, sessionID, err)
		var inspectErr error
		vm, inspectErr = p.inspectVMWithTimeout(ctx, name)
		if inspectErr != nil {
			return nil, state, fmt.Errorf("create exe.dev VM: create command failed and VM %q could not be inspected: %w", name, errors.Join(err, inspectErr))
		}
		if vm.Name == "" {
			return nil, state, fmt.Errorf("create exe.dev VM: create command failed and VM %q was not found: %w", name, err)
		}
		logVMWaitStatus("visible after create error", name, vm, new(sandbox.Status), new(string))
	} else {
		vm = parseVM(out)
	}

	created := time.Now()
	if vm.Name == "" {
		var waitErr error
		vm, waitErr = p.waitForVMVisible(ctx, name)
		if waitErr != nil {
			return nil, state, fmt.Errorf("create exe.dev VM: VM %q did not become visible: %w", name, waitErr)
		}
	}
	if vm.Name != "" {
		name = vm.Name
	}
	if !vm.CreatedAt.IsZero() {
		created = vm.CreatedAt
	}
	apiKey := ps.VMAPIKey
	if apiKey == "" {
		var keyErr error
		apiKey, keyErr = p.generateVMAPIKey(ctx, name, sessionID)
		if keyErr != nil {
			return nil, state, fmt.Errorf("generate exe.dev VM API key: %w", keyErr)
		}
	}
	status := vm.Status
	if status == "" {
		status = sandbox.StatusRunning
	}
	ps.VMName = name
	ps.VMURL = p.vmURL(name)
	ps.VMAPIKey = apiKey
	ps.SharedSecret = firstNonEmpty(ps.SharedSecret, opts.SharedSecret)
	ps.Status = status
	ps.CreatedAt = created
	newState, err := marshalState(ps)
	if err != nil {
		return nil, state, err
	}

	metadata := map[string]string{
		"provider":   "exedev",
		"managed":    "true",
		"session_id": sessionID,
		"vm_name":    name,
		"vm_url":     p.vmURL(name),
	}
	maps.Copy(metadata, opts.Labels)

	started := time.Now()
	sb := &sandbox.Sandbox{
		ID:        name,
		SessionID: sessionID,
		Status:    status,
		Image:     p.cfg.SandboxImage,
		CreatedAt: created,
		StartedAt: &started,
		Metadata:  metadata,
		Ports: []sandbox.AssignedPort{{
			ContainerPort: containerPort,
			HostPort:      443,
			HostIP:        name + "." + strings.TrimPrefix(p.cfg.VMHostSuffix, "."),
			Protocol:      "tcp",
		}},
		Env: env,
	}

	p.invalidateListCache()
	return cloneSandbox(sb), newState, nil
}

func (p *Provider) Start(ctx context.Context, state []byte, sessionID string) ([]byte, error) {
	ps := parseState(state)
	name := firstNonEmpty(ps.VMName, vmName(p.cfg.VMNamePrefix, sessionID))
	vm, err := p.inspectVM(ctx, name)
	if err != nil {
		return state, err
	}
	if vm.Name == "" {
		return state, sandbox.ErrNotFound
	}
	if vm.Status == sandbox.StatusRunning {
		ps.Status = sandbox.StatusRunning
		return marshalState(ps)
	}
	if vm.Status == sandbox.StatusCreated {
		log.Printf("Waiting for exe.dev VM %q for session %s to finish starting", vm.Name, sessionID)
		if _, err := p.waitForVMRunning(ctx, vm.Name); err != nil {
			return state, err
		}
		ps.Status = sandbox.StatusRunning
		p.invalidateListCache()
		return marshalState(ps)
	}
	log.Printf("Restarting exe.dev VM %q for session %s", vm.Name, sessionID)
	if _, err := p.client.Exec(ctx, "restart --json "+quoteArg(vm.Name)); err != nil {
		return state, fmt.Errorf("restart exe.dev VM: %w", err)
	}
	if _, err := p.waitForVMRunning(ctx, vm.Name); err != nil {
		return state, err
	}
	ps.Status = sandbox.StatusRunning
	p.invalidateListCache()
	return marshalState(ps)
}

func (p *Provider) Stop(ctx context.Context, state []byte, sessionID string, _ time.Duration) ([]byte, error) {
	ps := parseState(state)
	name := firstNonEmpty(ps.VMName, vmName(p.cfg.VMNamePrefix, sessionID))
	vm, err := p.inspectVM(ctx, name)
	if err != nil {
		return state, err
	}
	if vm.Name == "" {
		return state, nil
	}
	if vm.Status != sandbox.StatusRunning {
		ps.Status = vm.Status
		return marshalState(ps)
	}
	if p.cfg.StopCommand != "" {
		log.Printf("Stopping exe.dev VM %q for session %s", vm.Name, sessionID)
		cmd := renderStopCommand(p.cfg.StopCommand, vm.Name)
		if _, err := p.client.Exec(ctx, cmd); err != nil {
			return state, fmt.Errorf("stop exe.dev VM: %w", err)
		}
	}
	ps.Status = sandbox.StatusStopped
	p.invalidateListCache()
	return marshalState(ps)
}

func renderStopCommand(command, name string) string {
	return strings.ReplaceAll(command, stopCommandNamePlaceholder, quoteArg(name))
}

func (p *Provider) Remove(ctx context.Context, state []byte, sessionID string, _ ...sandbox.RemoveOption) ([]byte, error) {
	name := firstNonEmpty(parseState(state).VMName, vmName(p.cfg.VMNamePrefix, sessionID))
	log.Printf("Removing exe.dev VM %q for session %s", name, sessionID)
	if _, err := p.client.Exec(ctx, "rm --json "+quoteArg(name)); err != nil {
		if isVMNotFoundError(err) {
			return nil, nil
		}
		return state, fmt.Errorf("remove exe.dev VM: %w", err)
	}

	p.invalidateListCache()
	return nil, nil
}

func (p *Provider) Get(ctx context.Context, state []byte, sessionID string) (*sandbox.Sandbox, error) {
	ps := parseState(state)
	name := firstNonEmpty(ps.VMName, vmName(p.cfg.VMNamePrefix, sessionID))
	if ps.CreatedAt.IsZero() {
		vm, err := p.inspectVM(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("inspect exe.dev VM: %w", err)
		}
		if vm.Name == "" {
			return nil, sandbox.ErrNotFound
		}
		return p.sandboxFromVM(sessionID, ps, vm), nil
	}

	return p.sandboxFromState(sessionID, ps), nil
}

func (p *Provider) sandboxFromState(sessionID string, ps providerState) *sandbox.Sandbox {
	name := firstNonEmpty(ps.VMName, vmName(p.cfg.VMNamePrefix, sessionID))
	created := ps.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	return p.sandboxFromVM(sessionID, ps, vmInfo{
		Name:      name,
		Image:     p.cfg.SandboxImage,
		Status:    statusOr(ps.Status, sandbox.StatusRunning),
		CreatedAt: created,
	})
}

func (p *Provider) sandboxFromVM(sessionID string, ps providerState, vm vmInfo) *sandbox.Sandbox {
	name := firstNonEmpty(vm.Name, ps.VMName, vmName(p.cfg.VMNamePrefix, sessionID))
	if name == "" {
		return nil
	}
	created := vm.CreatedAt
	if created.IsZero() {
		created = ps.CreatedAt
	}
	if created.IsZero() {
		created = time.Now()
	}
	status := vm.Status
	if status == "" {
		status = sandbox.StatusRunning
	}
	vmURL := firstNonEmpty(ps.VMURL, p.vmURL(name))
	vmHost := strings.TrimSuffix(strings.TrimPrefix(vmURL, "https://"), "/")
	if vmHost == "" {
		vmHost = name + "." + strings.TrimPrefix(p.cfg.VMHostSuffix, ".")
	}
	return &sandbox.Sandbox{
		ID:        name,
		SessionID: sessionID,
		Status:    status,
		Image:     firstNonEmpty(vm.Image, p.cfg.SandboxImage),
		CreatedAt: created,
		Metadata: map[string]string{
			"provider":   "exedev",
			"managed":    "true",
			"session_id": sessionID,
			"vm_name":    name,
			"vm_url":     vmURL,
		},
		Ports: []sandbox.AssignedPort{{ContainerPort: containerPort, HostPort: 443, HostIP: vmHost, Protocol: "tcp"}},
	}
}

func (p *Provider) GetSecret(_ context.Context, state []byte, _ string) (string, error) {
	secret := parseState(state).SharedSecret
	if secret == "" {
		return "", sandbox.ErrNotFound
	}
	return secret, nil
}

func (p *Provider) List(ctx context.Context) ([]*sandbox.Sandbox, error) {
	if sandboxes, ok := p.cachedList(); ok {
		return sandboxes, nil
	}
	call, owner := p.startList()
	if !owner {
		select {
		case <-call.done:
			return cloneSandboxes(call.sandboxes), call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	defer p.finishList(call)

	out, err := p.client.Exec(ctx, "ls --json --l")
	if err != nil {
		call.err = fmt.Errorf("list exe.dev VMs: %w", err)
		return nil, call.err
	}
	vms := parseVMs(out)

	result := make([]*sandbox.Sandbox, 0, len(vms))
	for _, vm := range vms {
		sessionID := ""
		if id, ok := strings.CutPrefix(vm.Name, p.cfg.VMNamePrefix+"-"); ok {
			sessionID = id
		}
		if sessionID == "" {
			continue
		}
		created := vm.CreatedAt
		if created.IsZero() {
			created = time.Now()
		}
		result = append(result, &sandbox.Sandbox{
			ID:        vm.Name,
			SessionID: sessionID,
			Status:    vm.Status,
			Image:     firstNonEmpty(vm.Image, p.cfg.SandboxImage),
			CreatedAt: created,
			Metadata: map[string]string{
				"provider":   "exedev",
				"managed":    "true",
				"session_id": sessionID,
				"vm_name":    vm.Name,
				"vm_url":     p.vmURL(vm.Name),
			},
			Ports: []sandbox.AssignedPort{{ContainerPort: containerPort, HostPort: 443, HostIP: vm.Name + "." + strings.TrimPrefix(p.cfg.VMHostSuffix, "."), Protocol: "tcp"}},
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SessionID < result[j].SessionID })
	call.sandboxes = cloneSandboxes(result)
	p.storeListCache(call.sandboxes)
	return cloneSandboxes(result), nil
}

func (p *Provider) cachedList() ([]*sandbox.Sandbox, bool) {
	p.listMu.Lock()
	defer p.listMu.Unlock()
	if time.Now().After(p.listCache.expires) {
		return nil, false
	}
	return cloneSandboxes(p.listCache.sandboxes), true
}

func (p *Provider) startList() (*listCall, bool) {
	p.listMu.Lock()
	defer p.listMu.Unlock()
	if time.Now().Before(p.listCache.expires) {
		call := &listCall{
			done:      make(chan struct{}),
			sandboxes: cloneSandboxes(p.listCache.sandboxes),
		}
		close(call.done)
		return call, false
	}
	if p.listInFlight != nil {
		return p.listInFlight, false
	}
	p.listInFlight = &listCall{done: make(chan struct{})}
	return p.listInFlight, true
}

func (p *Provider) finishList(call *listCall) {
	p.listMu.Lock()
	if p.listInFlight == call {
		p.listInFlight = nil
	}
	p.listMu.Unlock()
	close(call.done)
}

func (p *Provider) storeListCache(sandboxes []*sandbox.Sandbox) {
	p.listMu.Lock()
	defer p.listMu.Unlock()
	p.listCache = listCacheEntry{
		sandboxes: cloneSandboxes(sandboxes),
		expires:   time.Now().Add(listCacheTTL),
	}
}

func (p *Provider) invalidateListCache() {
	p.listMu.Lock()
	defer p.listMu.Unlock()
	p.listCache = listCacheEntry{}
}

func (p *Provider) AcquireHTTPClient(ctx context.Context, state []byte, sessionID string) (*sandbox.HTTPClientLease, error) {
	ps := parseState(state)
	if ps.VMAPIKey == "" {
		return nil, fmt.Errorf("exe.dev VM API key is missing from sandbox state")
	}
	sb, err := p.Get(ctx, state, sessionID)
	if err != nil {
		return nil, err
	}
	if sb.Status != sandbox.StatusRunning {
		return nil, fmt.Errorf("sandbox is not running: %s", sb.Status)
	}
	vmHost := sb.Ports[0].HostIP
	target := "https://" + vmHost
	return p.httpCache.Acquire(sessionID, target, func() (*http.Client, error) {
		return &http.Client{
			Transport: &vmHTTPTransport{
				base:   http.DefaultTransport,
				host:   vmHost,
				token:  ps.VMAPIKey,
				scheme: "https",
			},
			Timeout: 60 * time.Second,
		}, nil
	})
}

func (p *Provider) Watch(ctx context.Context) (<-chan sandbox.StateEvent, error) {
	ch := make(chan sandbox.StateEvent, 32)
	go func() {
		defer close(ch)
		sendSnapshot := func() bool {
			sandboxes, err := p.List(ctx)
			if err != nil {
				return true
			}
			for _, sb := range sandboxes {
				select {
				case ch <- sandbox.StateEvent{SessionID: sb.SessionID, Status: sb.Status, Timestamp: time.Now(), Error: sb.Error}:
				case <-ctx.Done():
					return false
				}
			}
			return true
		}
		if !sendSnapshot() {
			return
		}
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !sendSnapshot() {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (p *Provider) Reconcile(context.Context) error             { return nil }
func (p *Provider) RemoveProject(context.Context, string) error { return nil }

func (p *Provider) inspectVM(ctx context.Context, name string) (vmInfo, error) {
	out, err := p.client.Exec(ctx, "ls --json --l "+quoteArg(name))
	if err != nil {
		return vmInfo{}, err
	}
	return parseVM(out), nil
}

func (p *Provider) generateVMAPIKey(ctx context.Context, name, sessionID string) (string, error) {
	label := "discobot-session-" + sessionID
	out, err := p.client.Exec(ctx, joinArgs([]string{"ssh-key", "generate-api-key", "--vm=" + name, "--label=" + label}))
	if err != nil {
		return "", err
	}
	apiKey := parseAPIKey(out)
	if apiKey == "" {
		return "", fmt.Errorf("exe.dev API key response did not include a key")
	}
	return apiKey, nil
}

func (p *Provider) inspectVMWithTimeout(ctx context.Context, name string) (vmInfo, error) {
	inspectCtx, inspectCancel := context.WithTimeout(ctx, createVisibilityPollRequestTimeout)
	defer inspectCancel()
	return p.inspectVM(inspectCtx, name)
}

func (p *Provider) waitForVMVisible(ctx context.Context, name string) (vmInfo, error) {
	ctx, cancel := contextWithDefaultTimeout(ctx, createVisibilityMaxWait)
	defer cancel()

	var lastErr error
	var lastStatus sandbox.Status
	var lastImage string
	for {
		inspectCtx, inspectCancel := context.WithTimeout(ctx, createVisibilityPollRequestTimeout)
		vm, err := p.inspectVM(inspectCtx, name)
		inspectCancel()
		if err == nil && vm.Name != "" {
			logVMWaitStatus("visible", name, vm, &lastStatus, &lastImage)
			return vm, nil
		}
		if err != nil {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return vmInfo{}, lastErr
			}
			return vmInfo{}, ctx.Err()
		case <-time.After(createVisibilityPollInterval):
		}
	}
}

func (p *Provider) waitForVMRunning(ctx context.Context, name string) (vmInfo, error) {
	ctx, cancel := contextWithDefaultTimeout(ctx, vmRunningMaxWait)
	defer cancel()

	var lastErr error
	var lastStatus sandbox.Status
	var lastImage string
	for {
		inspectCtx, inspectCancel := context.WithTimeout(ctx, createVisibilityPollRequestTimeout)
		vm, err := p.inspectVM(inspectCtx, name)
		inspectCancel()
		if err == nil {
			logVMWaitStatus("running", name, vm, &lastStatus, &lastImage)
			switch vm.Status {
			case sandbox.StatusRunning:
				return vm, nil
			case sandbox.StatusFailed:
				return vmInfo{}, fmt.Errorf("exe.dev VM %q failed to start", name)
			}
		}
		if err != nil {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return vmInfo{}, lastErr
			}
			return vmInfo{}, ctx.Err()
		case <-time.After(createVisibilityPollInterval):
		}
	}
}

func logVMWaitStatus(waitingFor, name string, vm vmInfo, lastStatus *sandbox.Status, lastImage *string) {
	if vm.Name == "" {
		return
	}
	if vm.Status == *lastStatus && vm.Image == *lastImage {
		return
	}
	*lastStatus = vm.Status
	*lastImage = vm.Image
	log.Printf("exe.dev VM %q status while waiting for %s: status=%q image=%q", name, waitingFor, vm.Status, vm.Image)
}

func contextWithDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func isVMNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist")
}

func parseState(data []byte) providerState {
	var state providerState
	if len(data) == 0 {
		return state
	}
	_ = json.Unmarshal(data, &state)
	return state
}

func marshalState(state providerState) ([]byte, error) {
	if state.VMName == "" && state.VMURL == "" && state.VMAPIKey == "" && state.SharedSecret == "" && state.CreatedAt.IsZero() {
		return nil, nil
	}
	return json.Marshal(state)
}

func (p *Provider) vmURL(name string) string {
	return "https://" + name + "." + strings.TrimPrefix(p.cfg.VMHostSuffix, ".") + "/"
}

type httpCommandClient struct {
	endpoint string
	token    string
	client   *http.Client
}

func (c *httpCommandClient) Exec(ctx context.Context, command string) ([]byte, error) {
	ctx, cancel := contextWithDefaultTimeout(ctx, rateLimitRetryTimeout)
	defer cancel()

	sanitizedCommand := sanitizeCommandForLog(command)
	for {
		log.Printf("Running exe.dev command: %s", sanitizedCommand)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(command))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Authorization", "Bearer "+c.token)
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			delay := retryAfterDelay(resp.Header.Get("Retry-After"), rateLimitRetryDelay)
			log.Printf("exe.dev command rate limited; retrying in %s: %s", delay, sanitizedCommand)
			if err := sleepContext(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, commandError{
				statusCode: resp.StatusCode,
				body:       strings.TrimSpace(string(body)),
			}
		}
		return body, nil
	}
}

type commandError struct {
	statusCode int
	body       string
}

func (e commandError) Error() string {
	return fmt.Sprintf("exe.dev command failed with status %d: %s", e.statusCode, e.body)
}

func retryAfterDelay(value string, fallback time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		if delay := time.Until(retryAt); delay > 0 {
			return delay
		}
		return 0
	}
	return fallback
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type vmHTTPTransport struct {
	base   http.RoundTripper
	host   string
	token  string
	scheme string
}

func (t *vmHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	urlCopy := *req.URL
	urlCopy.Scheme = t.scheme
	urlCopy.Host = t.host
	clone.URL = &urlCopy
	clone.Host = t.host
	clone.Header.Set("X-Exedev-Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

func (t *vmHTTPTransport) Headers() http.Header {
	headers := make(http.Header)
	headers.Set("X-Exedev-Authorization", "Bearer "+t.token)
	return headers
}

func (t *vmHTTPTransport) WebSocketURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Scheme = "wss"
	u.Host = t.host
	return u.String()
}

type vmInfo struct {
	Name      string
	Image     string
	Status    sandbox.Status
	CreatedAt time.Time
}

func buildNewCommand(name, image string, env map[string]string, opts sandbox.CreateOptions) string {
	args := []string{"new", "--json", "--name=" + name, "--no-email"}
	if image != "" {
		args = append(args, "--image="+image)
	}
	if opts.Resources.CPUCores > 0 {
		args = append(args, "--cpu="+strconv.FormatFloat(opts.Resources.CPUCores, 'f', -1, 64))
	}
	if opts.Resources.MemoryMB > 0 {
		args = append(args, "--memory="+strconv.Itoa(opts.Resources.MemoryMB)+"MB")
	}
	if opts.Resources.DiskMB > 0 {
		args = append(args, "--disk="+strconv.Itoa(opts.Resources.DiskMB)+"MB")
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--env", key+"="+env[key])
	}
	args = append(args, "--tag=discobot,discobot-session-"+name)
	return joinArgs(args)
}

func vmName(prefix, sessionID string) string {
	prefix = strings.Trim(nonDNSName.ReplaceAllString(strings.ToLower(prefix), "-"), "-")
	if prefix == "" {
		prefix = "discobot"
	}
	name := strings.Trim(nonDNSName.ReplaceAllString(strings.ToLower(sessionID), "-"), "-")
	if name == "" {
		name = "session"
	}
	full := prefix + "-" + name
	if len(full) > 63 {
		full = strings.TrimRight(full[:63], "-")
	}
	return full
}

func joinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = quoteArg(arg)
	}
	return strings.Join(quoted, " ")
}

func sanitizeCommandForLog(command string) string {
	args, ok := splitCommandArgs(command)
	if !ok {
		return command
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--env" && i+1 < len(args) {
			args[i+1] = sanitizeEnvArg(args[i+1])
			i++
			continue
		}
		if envArg, ok := strings.CutPrefix(arg, "--env="); ok {
			args[i] = "--env=" + sanitizeEnvArg(envArg)
		}
	}
	return joinArgs(args)
}

func sanitizeEnvArg(arg string) string {
	key, _, ok := strings.Cut(arg, "=")
	if !ok || !shouldRedactEnvValue(key) {
		return arg
	}
	return key + "=<redacted>"
}

func shouldRedactEnvValue(key string) bool {
	key = strings.ToUpper(key)
	for _, marker := range []string{"SECRET", "TOKEN", "PASSWORD", "PASS", "KEY", "CREDENTIAL", "AUTH"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func splitCommandArgs(command string) ([]string, bool) {
	var args []string
	var current strings.Builder
	inSingleQuote := false
	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch {
		case inSingleQuote:
			if ch == '\'' {
				if i+3 < len(command) && command[i+1] == '\\' && command[i+2] == '\'' && command[i+3] == '\'' {
					current.WriteByte('\'')
					i += 3
					continue
				}
				inSingleQuote = false
				continue
			}
			current.WriteByte(ch)
		case ch == '\'':
			inSingleQuote = true
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if inSingleQuote {
		return nil, false
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, true
}

func quoteArg(arg string) string {
	if arg == "" {
		return "''"
	}
	if strings.ContainsAny(arg, " \t\n\r'\"\\$`!&|;<>*?()[]{}") {
		return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return arg
}

func parseVM(out []byte) vmInfo {
	vms := parseVMs(out)
	if len(vms) == 0 {
		return vmInfo{}
	}
	return vms[0]
}

func parseAPIKey(out []byte) string {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return ""
	}

	var raw any
	if err := json.Unmarshal(out, &raw); err == nil {
		if key := apiKeyFromRaw(raw); key != "" {
			return key
		}
	}

	return string(out)
}

func apiKeyFromRaw(raw any) string {
	switch value := raw.(type) {
	case map[string]any:
		if key := firstString(value, "api_key", "apiKey", "key", "token", "secret", "value"); key != "" {
			return key
		}
		for _, nestedKey := range []string{"data", "result", "apiKey", "api_key"} {
			if nested, ok := value[nestedKey]; ok {
				if key := apiKeyFromRaw(nested); key != "" {
					return key
				}
			}
		}
	case []any:
		for _, item := range value {
			if key := apiKeyFromRaw(item); key != "" {
				return key
			}
		}
	case string:
		return strings.TrimSpace(value)
	}
	return ""
}

func parseVMs(out []byte) []vmInfo {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil
	}

	var raw any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}
	items := normalizeItems(raw)
	vms := make([]vmInfo, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			vm := vmFromMap(m)
			if vm.Name != "" {
				vms = append(vms, vm)
			}
		}
	}
	return vms
}

func normalizeItems(raw any) []any {
	switch value := raw.(type) {
	case []any:
		return value
	case map[string]any:
		for _, key := range []string{"vms", "machines", "instances", "items", "data", "result"} {
			if arr, ok := value[key].([]any); ok {
				return arr
			}
		}
		return []any{value}
	default:
		return nil
	}
}

func vmFromMap(m map[string]any) vmInfo {
	status := sandbox.Status(strings.ToLower(firstString(m, "status", "state", "phase")))
	switch status {
	case "", "ready", "started", "active", "up":
		status = sandbox.StatusRunning
	case "stopping", "stopped", "down", "off", "exited":
		status = sandbox.StatusStopped
	case "failed", "error", "crashed":
		status = sandbox.StatusFailed
	case "creating", "created", "pending", "starting":
		status = sandbox.StatusCreated
	}
	return vmInfo{
		Name:      firstString(m, "name", "vm_name", "vm", "hostname", "id"),
		Image:     firstString(m, "image", "image_name"),
		Status:    status,
		CreatedAt: firstTime(m, "created_at", "createdAt", "created", "ctime"),
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := m[key].(type) {
		case string:
			return value
		case json.Number:
			return value.String()
		case float64:
			if value == float64(int64(value)) {
				return strconv.FormatInt(int64(value), 10)
			}
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return ""
}

func firstTime(m map[string]any, keys ...string) time.Time {
	for _, key := range keys {
		if s, ok := m[key].(string); ok && s != "" {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, s); err == nil {
					return t
				}
			}
		}
	}
	return time.Time{}
}

func cloneSandbox(sb *sandbox.Sandbox) *sandbox.Sandbox {
	if sb == nil {
		return nil
	}
	clone := *sb
	clone.Metadata = maps.Clone(sb.Metadata)
	clone.Env = maps.Clone(sb.Env)
	clone.Ports = append([]sandbox.AssignedPort(nil), sb.Ports...)
	return &clone
}

func cloneSandboxes(sandboxes []*sandbox.Sandbox) []*sandbox.Sandbox {
	if sandboxes == nil {
		return nil
	}
	clones := make([]*sandbox.Sandbox, len(sandboxes))
	for i, sb := range sandboxes {
		clones[i] = cloneSandbox(sb)
	}
	return clones
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func statusOr(value, fallback sandbox.Status) sandbox.Status {
	if value != "" {
		return value
	}
	return fallback
}

var _ sandbox.Provider = (*Provider)(nil)
var _ sandbox.StatusProvider = (*Provider)(nil)
