package exedev

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
)

type listCacheEntry struct {
	sandboxes []*sandbox.Sandbox
	expires   time.Time
}

type listCall struct {
	done      chan struct{}
	sandboxes []*sandbox.Sandbox
	err       error
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

	out, err := p.client.Exec(ctx, newCommand("ls", "--json", "--l").render())
	if err != nil {
		call.err = fmt.Errorf("list exe.dev VMs: %w", err)
		return nil, call.err
	}
	vms := parseVMs(out)

	result := make([]*sandbox.Sandbox, 0, len(vms))
	for _, vm := range vms {
		sessionID := sessionIDFromTags(vm.Tags)
		if sessionID == "" {
			if id, ok := strings.CutPrefix(vm.Name, p.cfg.VMNamePrefix+"-"); ok {
				sessionID = id
			}
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
		expires:   time.Now().Add(p.timings.listCacheTTL),
	}
}

func (p *Provider) invalidateListCache() {
	p.listMu.Lock()
	defer p.listMu.Unlock()
	p.listCache = listCacheEntry{}
}
