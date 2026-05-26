//go:build windows

package wsl

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
	"github.com/obot-platform/discobot/server/internal/startup"
)

// VMManager adapts the managed WSL distro lifecycle to vm.ProjectVMManager.
type VMManager struct {
	cfg           *config.Config
	manager       *Manager
	systemManager *startup.SystemManager

	ready chan struct{}

	mu         sync.RWMutex
	projectVMs map[string]*projectVM

	bridgeMu     sync.Mutex
	dockerBridge dockerBridge
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
	if _, err := m.ensureDockerBridge(ctx); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if projectVM, ok := m.projectVMs[projectID]; ok {
		return projectVM, nil
	}

	projectVM := &projectVM{
		manager:   m,
		projectID: projectID,
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
		m.closeDockerBridge()
		return m.manager.Stop(context.Background())
	}
	return nil
}

// ResizeDataDisk grows the managed VHDX that is mounted at /var.
func (m *VMManager) ResizeDataDisk(ctx context.Context, _ string, sizeGB int) error {
	return m.manager.ResizeVarDisk(ctx, sizeGB)
}

func (m *VMManager) Shutdown() {
	m.mu.Lock()
	m.projectVMs = make(map[string]*projectVM)
	m.mu.Unlock()
	m.closeDockerBridge()
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

	if isRunningDistroState(distro.State) {
		return sandbox.ProviderStatus{Available: true, State: "ready", Message: "managed WSL distro is running", Details: details}
	}
	if isStoppedDistroState(distro.State) {
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

type projectVM struct {
	manager   *VMManager
	projectID string
}

func (p *projectVM) ProjectID() string {
	return p.projectID
}

func (p *projectVM) DockerDialer() func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		return p.manager.dialDockerBridge(ctx)
	}
}

func (p *projectVM) PortDialer(port uint32) func(context.Context, string, string) (net.Conn, error) {
	return p.tcpPortDialer(port)
}

func (p *projectVM) WorkspaceMountSource(source string) (string, error) {
	return TranslatePath(source)
}

func (p *projectVM) Shutdown() error {
	return nil
}

func (p *projectVM) tcpPortDialer(port uint32) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
	}
}

func (m *VMManager) dialDockerBridge(ctx context.Context) (net.Conn, error) {
	bridge, err := m.ensureDockerBridge(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := bridge.Dial(ctx)
	if err == nil {
		return conn, nil
	}
	if bridge.Running() {
		return nil, err
	}

	m.closeDockerBridge()
	bridge, restartErr := m.ensureDockerBridge(ctx)
	if restartErr != nil {
		return nil, restartErr
	}
	return bridge.Dial(ctx)
}

func (m *VMManager) ensureDockerBridge(ctx context.Context) (dockerBridge, error) {
	m.bridgeMu.Lock()
	defer m.bridgeMu.Unlock()

	if m.dockerBridge != nil && m.dockerBridge.Running() {
		return m.dockerBridge, nil
	}
	if m.dockerBridge != nil {
		_ = m.dockerBridge.Close()
		m.dockerBridge = nil
	}

	bridge, err := startWSLDockerBridge(ctx, strings.TrimSpace(m.cfg.WSLDistroName))
	if err != nil {
		return nil, err
	}
	m.dockerBridge = bridge
	return bridge, nil
}

func (m *VMManager) closeDockerBridge() {
	m.bridgeMu.Lock()
	defer m.bridgeMu.Unlock()
	if m.dockerBridge != nil {
		_ = m.dockerBridge.Close()
		m.dockerBridge = nil
	}
}

func (m *Manager) ensureVMRunningWithProgress(ctx context.Context, progress progressReporter) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	progress.Update(5, "Verifying managed WSL distro installation")
	if err := m.verifyInstalledLocked(ctx, progressReporter{}); err != nil {
		return err
	}

	progress.Update(15, "Ensuring WSL host startup requirements")
	if err := m.ensureHostStartupWithPowerShell(ctx, progressReporter{
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

// ResizeVarDisk grows the managed WSL /var VHDX backing file.
func (m *Manager) ResizeVarDisk(ctx context.Context, sizeGB int) error {
	if sizeGB <= 0 {
		return fmt.Errorf("var disk size must be greater than 0")
	}

	varDiskPath := m.varDiskPath()
	createDisk := false
	info, err := os.Stat(varDiskPath)
	if err != nil {
		if os.IsNotExist(err) {
			createDisk = true
		} else {
			return fmt.Errorf("stat WSL /var disk %q: %w", varDiskPath, err)
		}
	}

	targetSize := int64(sizeGB) * 1024 * 1024 * 1024
	if !createDisk && info.Size() > targetSize {
		return fmt.Errorf("cannot shrink WSL /var disk from %d GB to %d GB", info.Size()/(1024*1024*1024), sizeGB)
	}
	if !createDisk && info.Size() == targetSize {
		return nil
	}

	if !createDisk {
		if err := m.unmountVarDiskForResize(ctx, varDiskPath); err != nil {
			return err
		}
	}

	return m.applyVarDiskSize(ctx, varDiskPath, sizeGB, createDisk)
}

// RequestVarDiskResize records the intended /var disk size for the next WSL
// startup. The startup script performs the resize later while the managed WSL
// runtime is stopped.
func (m *Manager) RequestVarDiskResize(_ context.Context, sizeGB int) error {
	if sizeGB <= 0 {
		return fmt.Errorf("var disk size must be greater than 0")
	}
	return m.state.RequestVarDiskSize(sizeGB, m.varDiskSizeGB(), m.runtimeID)
}

func (m *Manager) applyVarDiskSize(ctx context.Context, varDiskPath string, sizeGB int, createDisk bool) error {
	if err := os.MkdirAll(filepath.Dir(varDiskPath), 0755); err != nil {
		return fmt.Errorf("create WSL /var disk parent directory: %w", err)
	}

	diskPartScript, err := os.CreateTemp("", "discobot-wsl-resize-*.txt")
	if err != nil {
		return fmt.Errorf("create WSL /var disk resize script: %w", err)
	}
	diskPartScriptPath := diskPartScript.Name()
	defer func() {
		_ = os.Remove(diskPartScriptPath)
	}()

	maximumMB := int64(sizeGB) * 1024
	content := fmt.Sprintf("select vdisk file=\"%s\"\nexpand vdisk maximum=%d\nexit\n", varDiskPath, maximumMB)
	action := "resize"
	if createDisk {
		content = fmt.Sprintf("create vdisk file=\"%s\" maximum=%d type=expandable\nexit\n", varDiskPath, maximumMB)
		action = "create"
	}
	if _, err := diskPartScript.WriteString(content); err != nil {
		_ = diskPartScript.Close()
		return fmt.Errorf("write WSL /var disk resize script: %w", err)
	}
	if err := diskPartScript.Close(); err != nil {
		return fmt.Errorf("close WSL /var disk resize script: %w", err)
	}

	if _, err := m.runCommand(ctx, "diskpart.exe", "/s", diskPartScriptPath); err != nil {
		return fmt.Errorf("%s WSL /var disk %q at %d GB: %w", action, varDiskPath, sizeGB, err)
	}
	return nil
}

func (m *Manager) unmountVarDiskForResize(ctx context.Context, varDiskPath string) error {
	if _, err := m.runCommand(ctx, "wsl.exe", "--unmount", varDiskPath); err != nil {
		if isStaleVarDiskUnmountError(err.Error()) {
			return nil
		}
		return fmt.Errorf("unmount WSL /var disk %q before resize: %w", varDiskPath, err)
	}
	return nil
}

func isStaleVarDiskUnmountError(message string) bool {
	text := strings.ToLower(message)
	return strings.Contains(text, "failed to detach") ||
		strings.Contains(text, "invalid argument") ||
		strings.Contains(text, "not mounted") ||
		strings.Contains(text, "not attached") ||
		strings.Contains(text, "cannot find the path specified")
}
