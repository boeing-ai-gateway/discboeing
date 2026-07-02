package exedev

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
)

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
	cmd := buildNewCommand(name, p.cfg.SandboxImage, sessionID, env, opts)
	log.Printf("Creating exe.dev VM %q for session %s with image %q", name, sessionID, p.cfg.SandboxImage)
	vm, err := p.createVM(ctx, name, sessionID, cmd)
	if err != nil {
		return nil, state, err
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
	apiKey, keyErr := p.ensureVMAPIKey(ctx, ps, name, sessionID)
	if keyErr != nil {
		return nil, state, keyErr
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

func (p *Provider) createVM(ctx context.Context, name, sessionID, cmd string) (vmInfo, error) {
	out, err := p.client.Exec(ctx, cmd)
	if err == nil {
		return parseVM(out), nil
	}
	if !shouldInspectAfterCreateError(err) {
		return vmInfo{}, fmt.Errorf("create exe.dev VM: %w", err)
	}
	return p.recoverCreatedVM(ctx, name, sessionID, err)
}

func (p *Provider) recoverCreatedVM(ctx context.Context, name, sessionID string, createErr error) (vmInfo, error) {
	log.Printf("exe.dev VM creation command failed for %q session %s; checking VM state with ls: %v", name, sessionID, createErr)
	vm, inspectErr := p.inspectVMWithTimeout(ctx, name)
	if inspectErr != nil {
		return vmInfo{}, fmt.Errorf("create exe.dev VM: create command failed and VM %q could not be inspected: %w", name, errors.Join(createErr, inspectErr))
	}
	if vm.Name == "" {
		return vmInfo{}, fmt.Errorf("create exe.dev VM: create command failed and VM %q was not found: %w", name, createErr)
	}
	logVMWaitStatus("visible after create error", name, vm, new(sandbox.Status), new(string))
	return vm, nil
}

func (p *Provider) ensureVMAPIKey(ctx context.Context, state providerState, name, sessionID string) (string, error) {
	if state.VMAPIKey != "" {
		return state.VMAPIKey, nil
	}
	apiKey, err := p.generateVMAPIKey(ctx, name, sessionID)
	if err != nil {
		return "", fmt.Errorf("generate exe.dev VM API key: %w", err)
	}
	return apiKey, nil
}

func shouldInspectAfterCreateError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	var cmdErr commandError
	if errors.As(err, &cmdErr) {
		switch cmdErr.statusCode {
		case http.StatusRequestTimeout, http.StatusConflict, http.StatusGatewayTimeout:
			return true
		case http.StatusUnprocessableEntity:
			return isNameConflictMessage(msg)
		default:
			return false
		}
	}
	return isNameConflictMessage(msg) || strings.Contains(msg, "timeout") || strings.Contains(msg, "timed out") || strings.Contains(msg, "deadline")
}

func isNameConflictMessage(msg string) bool {
	for _, marker := range []string{"already exists", "name conflict", "name is not available", "vm name", "reserved"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
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
	if _, err := p.client.Exec(ctx, newCommand("restart", "--json", vm.Name).render()); err != nil {
		return state, fmt.Errorf("restart exe.dev VM: %w", err)
	}
	if _, err := p.waitForVMRunning(ctx, vm.Name); err != nil {
		return state, err
	}
	ps.Status = sandbox.StatusRunning
	p.invalidateListCache()
	return marshalState(ps)
}

func (p *Provider) Stop(ctx context.Context, state []byte, sessionID string, stopTimeout time.Duration) ([]byte, error) {
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
		if stopTimeout > 0 {
			waitCtx, cancel := context.WithTimeout(ctx, stopTimeout)
			stoppedVM, err := p.waitForVMStopped(waitCtx, vm.Name)
			cancel()
			if err != nil {
				log.Printf("exe.dev VM %q did not report stopped after stop command; preserving requested stopped state: %v", vm.Name, err)
			} else if stoppedVM.Status != "" {
				ps.Status = stoppedVM.Status
			}
		}
	}
	if ps.Status == "" || ps.Status == sandbox.StatusRunning {
		ps.Status = sandbox.StatusStopped
	}
	p.invalidateListCache()
	return marshalState(ps)
}

func (p *Provider) Remove(ctx context.Context, state []byte, sessionID string, _ ...sandbox.RemoveOption) ([]byte, error) {
	name := firstNonEmpty(parseState(state).VMName, vmName(p.cfg.VMNamePrefix, sessionID))
	log.Printf("Removing exe.dev VM %q for session %s", name, sessionID)
	if _, err := p.client.Exec(ctx, newCommand("rm", "--json", name).render()); err != nil {
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

func renderStopCommand(command, name string) string {
	return strings.ReplaceAll(command, stopCommandNamePlaceholder, quoteArg(name))
}
