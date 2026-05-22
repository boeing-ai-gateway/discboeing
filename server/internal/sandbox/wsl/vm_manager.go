//go:build windows

package wsl

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	winio "github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
	"github.com/obot-platform/discobot/server/internal/startup"
)

const dockerVsockPort = 2375

// VMManager adapts the managed WSL distro lifecycle to vm.ProjectVMManager.
type VMManager struct {
	cfg           *config.Config
	manager       *Manager
	systemManager *startup.SystemManager

	ready chan struct{}

	mu         sync.RWMutex
	projectVMs map[string]*projectVM
	vmID       guid.GUID
	vmIDErr    error
	vmIDOnce   sync.Once
}

func NewVMManager(cfg *config.Config, systemManager *startup.SystemManager) *VMManager {
	ready := make(chan struct{})
	close(ready)
	return &VMManager{
		cfg:           cfg,
		manager:       NewManager(cfg),
		systemManager: systemManager,
		ready:         ready,
		projectVMs:    make(map[string]*projectVM),
	}
}

func (m *VMManager) Ready() <-chan struct{} {
	return m.ready
}

func (m *VMManager) Err() error {
	return nil
}

func (m *VMManager) GetOrCreateVM(ctx context.Context, projectID string) (vm.ProjectVM, error) {
	if projectID == "" {
		projectID = "local"
	}

	m.mu.RLock()
	if projectVM, ok := m.projectVMs[projectID]; ok {
		m.mu.RUnlock()
		return projectVM, nil
	}
	m.mu.RUnlock()

	if err := m.ensureManagedDistroRunning(ctx); err != nil {
		return nil, err
	}

	vmID, err := m.resolveVMID()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if projectVM, ok := m.projectVMs[projectID]; ok {
		return projectVM, nil
	}

	projectVM := &projectVM{
		projectID: projectID,
		distro:    strings.TrimSpace(m.cfg.WSLDistroName),
		vmID:      vmID,
	}
	m.projectVMs[projectID] = projectVM
	return projectVM, nil
}

func (m *VMManager) GetVM(projectID string) (vm.ProjectVM, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	projectVM, ok := m.projectVMs[projectID]
	return projectVM, ok
}

func (m *VMManager) ListProjectIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.projectVMs))
	for projectID := range m.projectVMs {
		ids = append(ids, projectID)
	}
	return ids
}

func (m *VMManager) RemoveVM(projectID string) error {
	m.mu.Lock()
	delete(m.projectVMs, projectID)
	remaining := len(m.projectVMs)
	m.mu.Unlock()

	if remaining == 0 {
		return m.manager.Stop(context.Background())
	}
	return nil
}

func (m *VMManager) Shutdown() {
	m.mu.Lock()
	m.projectVMs = make(map[string]*projectVM)
	m.mu.Unlock()
	_ = m.manager.Stop(context.Background())
}

func (m *VMManager) Status() sandbox.ProviderStatus {
	details := StatusDetails{
		DistroName:        m.cfg.WSLDistroName,
		InstallDir:        m.cfg.WSLInstallDir,
		StateDir:          m.cfg.WSLStateDir,
		StatePath:         m.manager.state.Path(),
		VarDiskPath:       m.manager.varDiskPath(),
		VarDiskLabel:      m.manager.varDiskLabel(),
		RootfsArchivePath: strings.TrimSpace(m.cfg.WSLRootfsPath),
		ImageRef:          m.cfg.WSLImageRef,
	}

	if strings.TrimSpace(m.cfg.WSLInstallDir) == "" {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: "WSL_INSTALL_DIR is empty", Details: details}
	}
	if strings.TrimSpace(m.cfg.WSLDistroName) == "" {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: "WSL_DISTRO_NAME is empty", Details: details}
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
	defer cancel()

	distro, found, err := m.manager.probeDistro(ctx)
	if err != nil {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: err.Error(), Details: details}
	}
	if !found {
		return sandbox.ProviderStatus{Available: true, State: "not_installed", Message: fmt.Sprintf("managed WSL distro %q is not installed yet", m.cfg.WSLDistroName), Details: details}
	}

	details.DistroInstalled = true
	details.DistroState = distro.State
	details.DistroVersion = distro.Version

	if strings.EqualFold(distro.State, "Running") {
		return sandbox.ProviderStatus{Available: true, State: "ready", Message: "managed WSL distro is running", Details: details}
	}
	if strings.EqualFold(distro.State, "Stopped") {
		return sandbox.ProviderStatus{Available: true, State: "stopped", Message: "managed WSL distro is installed but currently stopped; it will be started on demand", Details: details}
	}
	return sandbox.ProviderStatus{Available: true, State: "starting", Message: fmt.Sprintf("managed WSL distro is currently %s", distro.State), Details: details}
}

func (m *VMManager) ensureManagedDistroRunning(ctx context.Context) error {
	if m.systemManager != nil {
		m.systemManager.RegisterTask(startupTaskWSLStartID, "Starting managed WSL distro")
		m.systemManager.StartTask(startupTaskWSLStartID)
	}

	progress := progressReporter{
		update: func(progress int, currentOperation string) {
			if m.systemManager != nil {
				m.systemManager.UpdateTaskProgress(startupTaskWSLStartID, progress, currentOperation)
			}
		},
	}

	err := m.manager.ensureVMRunningWithProgress(ctx, progress)
	if err != nil {
		if m.systemManager != nil {
			m.systemManager.FailTask(startupTaskWSLStartID, err)
		}
		return err
	}
	if m.systemManager != nil {
		m.systemManager.CompleteTask(startupTaskWSLStartID)
	}
	return nil
}

func (m *VMManager) resolveVMID() (guid.GUID, error) {
	m.vmIDOnce.Do(func() {
		value := strings.TrimSpace(os.Getenv("WSL_VM_ID"))
		if value == "" {
			value = strings.TrimSpace(os.Getenv("DISCOBOT_WSL_VM_ID"))
		}
		if value == "" {
			m.vmIDErr = fmt.Errorf("WSL VMID is required for hvsock; set WSL_VM_ID or DISCOBOT_WSL_VM_ID")
			return
		}
		m.vmID, m.vmIDErr = guid.FromString(value)
	})
	return m.vmID, m.vmIDErr
}

type projectVM struct {
	projectID string
	distro    string
	vmID      guid.GUID
}

func (p *projectVM) ProjectID() string {
	return p.projectID
}

func (p *projectVM) DockerDialer() func(context.Context, string, string) (net.Conn, error) {
	return p.hvsockDialer(dockerVsockPort)
}

func (p *projectVM) PortDialer(port uint32) func(context.Context, string, string) (net.Conn, error) {
	return p.hvsockDialer(port)
}

func (p *projectVM) Shutdown() error {
	return nil
}

func (p *projectVM) hvsockDialer(port uint32) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		dialer := &winio.HvsockDialer{
			Retries:   30,
			RetryWait: 200 * time.Millisecond,
		}
		return dialer.Dial(ctx, &winio.HvsockAddr{
			VMID:      p.vmID,
			ServiceID: winio.VsockServiceID(port),
		})
	}
}

func (m *Manager) ensureVMRunningWithProgress(ctx context.Context, progress progressReporter) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	progress.Update(5, "Ensuring managed WSL distro is installed")
	if err := m.ensureInstalledLocked(ctx, progressReporter{}); err != nil {
		return err
	}

	progress.Update(15, "Checking WSL host startup requirements")
	if err := m.checkHostStartupReadyWithPowerShell(ctx, progressReporter{
		update: func(childProgress int, currentOperation string) {
			progress.Update(15+(childProgress*20/100), currentOperation)
		},
	}); err != nil {
		return err
	}

	if _, err := m.ensureMainDistroReady(ctx, progress); err != nil {
		return err
	}

	progress.Update(100, "Managed WSL distro is running")
	return nil
}
