package exedev

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
)

func (p *Provider) inspectVM(ctx context.Context, name string) (vmInfo, error) {
	out, err := p.client.Exec(ctx, newCommand("ls", "--json", "--l", name).render())
	if err != nil {
		return vmInfo{}, err
	}
	return parseVM(out), nil
}

func (p *Provider) generateVMAPIKey(ctx context.Context, name, sessionID string) (string, error) {
	label := "discboeing-session-" + sessionID
	out, err := p.client.Exec(ctx, newCommand("ssh-key", "generate-api-key", "--vm="+name, "--label="+label).render())
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
	inspectCtx, inspectCancel := context.WithTimeout(ctx, p.timings.createVisibilityPollRequestTimeout)
	defer inspectCancel()
	return p.inspectVM(inspectCtx, name)
}

func (p *Provider) waitForVMVisible(ctx context.Context, name string) (vmInfo, error) {
	ctx, cancel := contextWithDefaultTimeout(ctx, p.timings.createVisibilityMaxWait)
	defer cancel()

	var lastErr error
	var lastStatus sandbox.Status
	var lastImage string
	for {
		inspectCtx, inspectCancel := context.WithTimeout(ctx, p.timings.createVisibilityPollRequestTimeout)
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
		case <-time.After(p.timings.createVisibilityPollInterval):
		}
	}
}

func (p *Provider) waitForVMRunning(ctx context.Context, name string) (vmInfo, error) {
	ctx, cancel := contextWithDefaultTimeout(ctx, p.timings.vmRunningMaxWait)
	defer cancel()

	var lastErr error
	var lastStatus sandbox.Status
	var lastImage string
	for {
		inspectCtx, inspectCancel := context.WithTimeout(ctx, p.timings.createVisibilityPollRequestTimeout)
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
		case <-time.After(p.timings.createVisibilityPollInterval):
		}
	}
}

func (p *Provider) waitForVMStopped(ctx context.Context, name string) (vmInfo, error) {
	ctx, cancel := contextWithDefaultTimeout(ctx, p.timings.vmStoppedMaxWait)
	defer cancel()

	var lastErr error
	var lastStatus sandbox.Status
	var lastImage string
	for {
		inspectCtx, inspectCancel := context.WithTimeout(ctx, p.timings.createVisibilityPollRequestTimeout)
		vm, err := p.inspectVM(inspectCtx, name)
		inspectCancel()
		if err == nil {
			logVMWaitStatus("stopped", name, vm, &lastStatus, &lastImage)
			if vm.Name == "" || vm.Status != sandbox.StatusRunning {
				return vm, nil
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
		case <-time.After(p.timings.createVisibilityPollInterval):
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
