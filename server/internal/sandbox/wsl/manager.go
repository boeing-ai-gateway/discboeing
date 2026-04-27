//go:build windows

package wsl

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/klauspost/compress/zstd"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
)

const (
	defaultProbeTimeout   = 5 * time.Second
	defaultReadyTimeout   = 30 * time.Second
	defaultReadyPollDelay = 500 * time.Millisecond
	dockerSockPath        = "/var/run/docker.sock"
	bridgeLogPath         = "/tmp/discobot-docker-bridge.log"
)

// RuntimeInfo contains the current runtime connection details returned by EnsureRunning.
type RuntimeInfo struct {
	DistroName       string `json:"distro_name"`
	DistroInstalled  bool   `json:"distro_installed"`
	DistroState      string `json:"distro_state,omitempty"`
	DistroVersion    int    `json:"distro_version,omitempty"`
	BridgeType       string `json:"bridge_type"`
	BridgePort       int    `json:"bridge_port,omitempty"`
	BridgePipeName   string `json:"bridge_pipe_name,omitempty"`
	BridgeDockerHost string `json:"bridge_docker_host,omitempty"`
	InstallDir       string `json:"install_dir,omitempty"`
	StateDir         string `json:"state_dir,omitempty"`
	ImageRef         string `json:"image_ref,omitempty"`
	BridgeReady      bool   `json:"bridge_ready"`
}

// StatusDetails contains WSL-specific provider details.
type StatusDetails struct {
	DistroName       string `json:"distro_name"`
	DistroInstalled  bool   `json:"distro_installed"`
	DistroState      string `json:"distro_state,omitempty"`
	DistroVersion    int    `json:"distro_version,omitempty"`
	InstallDir       string `json:"install_dir,omitempty"`
	StateDir         string `json:"state_dir,omitempty"`
	StatePath        string `json:"state_path,omitempty"`
	ImageRef         string `json:"image_ref,omitempty"`
	BridgeType       string `json:"bridge_type,omitempty"`
	BridgePort       int    `json:"bridge_port,omitempty"`
	BridgePipeName   string `json:"bridge_pipe_name,omitempty"`
	BridgeDockerHost string `json:"bridge_docker_host,omitempty"`
	UpgradeStrategy  string `json:"upgrade_strategy,omitempty"`
}

// Manager owns managed WSL distro lifecycle for the Windows sandbox backend.
type Manager struct {
	cfg        *config.Config
	state      *StateStore
	downloader *ImageDownloader

	mu                sync.RWMutex
	pipeListener      net.Listener
	pipeListenerName  string
	pipeListenerClose chan struct{}
}

// NewManager creates a new WSL lifecycle manager.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:   cfg,
		state: NewStateStore(cfg.WSLStateDir),
		downloader: NewImageDownloader(ImageDownloadConfig{
			ImageRef: cfg.WSLImageRef,
			DataDir:  cfg.WSLStateDir,
		}),
	}
}

// EnsureInstalled verifies that WSL tooling is available and reserves the managed distro identity.
func (m *Manager) EnsureInstalled(ctx context.Context) error {
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return fmt.Errorf("wsl.exe not found: %w", err)
	}

	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return err
	}
	if found {
		_ = distro
		return nil
	}
	return m.importDistro(ctx)
}

// EnsureRunning ensures the managed distro exists, starts it if needed, and waits
// for basic in-guest readiness before returning runtime connection details.
func (m *Manager) EnsureRunning(ctx context.Context) (*RuntimeInfo, error) {
	if err := m.EnsureInstalled(ctx); err != nil {
		return nil, err
	}

	bridgeInfo, err := m.resolveBridgeInfo()
	if err != nil {
		return nil, err
	}

	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("managed WSL distro %q is not installed", m.cfg.WSLDistroName)
	}

	if strings.EqualFold(distro.State, "Stopped") {
		if err := m.startDistro(ctx); err != nil {
			return nil, err
		}

		if err := m.waitForSystemdReady(ctx); err != nil {
			return nil, err
		}
		if err := m.waitForDockerReady(ctx); err != nil {
			return nil, err
		}

		distro, found, err = m.probeDistro(ctx)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("managed WSL distro %q disappeared after startup", m.cfg.WSLDistroName)
		}
	} else if strings.EqualFold(distro.State, "Running") {
		if err := m.waitForSystemdReady(ctx); err != nil {
			return nil, err
		}
		if err := m.waitForDockerReady(ctx); err != nil {
			return nil, err
		}
	}

	bridgeInfo, bridgeReady, err := m.ensureBridgeReady(ctx, bridgeInfo)
	if err != nil {
		return nil, err
	}

	return &RuntimeInfo{
		DistroName:       m.cfg.WSLDistroName,
		DistroInstalled:  true,
		DistroState:      distro.State,
		DistroVersion:    distro.Version,
		BridgeType:       bridgeInfo.Type,
		BridgePort:       bridgeInfo.Port,
		BridgePipeName:   bridgeInfo.PipeName,
		BridgeDockerHost: bridgeInfo.DockerHost,
		InstallDir:       m.cfg.WSLInstallDir,
		StateDir:         m.cfg.WSLStateDir,
		ImageRef:         m.cfg.WSLImageRef,
		BridgeReady:      bridgeReady,
	}, nil
}

// Stop terminates the managed distro if it is currently running.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if strings.EqualFold(distro.State, "Stopped") {
		m.stopNamedPipeBridgeLocked()
		return nil
	}

	if _, err := m.runCommand(ctx, "wsl.exe", "--terminate", m.cfg.WSLDistroName); err != nil {
		return fmt.Errorf("terminate managed WSL distro %q: %w", m.cfg.WSLDistroName, err)
	}
	m.stopNamedPipeBridgeLocked()
	return nil
}

// UpgradeIfNeeded performs the currently supported upgrade strategy.
func (m *Manager) UpgradeIfNeeded(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	strategy := strings.TrimSpace(strings.ToLower(m.cfg.WSLUpgradeStrategy))
	switch strategy {
	case "", "inplace":
	default:
		return fmt.Errorf("unsupported WSL upgrade strategy %q", m.cfg.WSLUpgradeStrategy)
	}

	state, err := m.state.Load()
	if err != nil {
		return err
	}

	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return err
	}
	if !found {
		return m.importDistro(ctx)
	}

	if strings.EqualFold(state.DistroName, m.cfg.WSLDistroName) &&
		strings.TrimSpace(state.ImageRef) != "" &&
		strings.TrimSpace(state.ImageRef) != strings.TrimSpace(m.cfg.WSLImageRef) {
		if strings.EqualFold(distro.State, "Running") {
			if _, err := m.runCommand(ctx, "wsl.exe", "--terminate", m.cfg.WSLDistroName); err != nil {
				return fmt.Errorf("terminate managed WSL distro %q before upgrade: %w", m.cfg.WSLDistroName, err)
			}
		}
		if err := m.unregisterDistro(ctx); err != nil {
			return err
		}
		if err := m.removeInstallDir(); err != nil {
			return err
		}
		if err := m.state.Clear(); err != nil {
			return err
		}
		return m.importDistro(ctx)
	}

	if state == (RuntimeState{}) || !strings.EqualFold(state.DistroName, m.cfg.WSLDistroName) || strings.TrimSpace(state.ImageRef) == "" {
		bridgeInfo, err := m.resolveBridgeInfo()
		if err != nil {
			return err
		}
		return m.state.Save(RuntimeState{
			DistroName: m.cfg.WSLDistroName,
			BridgeType: bridgeInfo.Type,
			BridgePort: bridgeInfo.Port,
			ImageRef:   m.cfg.WSLImageRef,
		})
	}

	return nil
}

// Uninstall removes the managed distro registration, install directory, and runtime state.
func (m *Manager) Uninstall(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, found, err := m.probeDistro(ctx); err != nil {
		return err
	} else if found {
		m.stopNamedPipeBridgeLocked()
		if err := m.unregisterDistro(ctx); err != nil {
			return err
		}
	}

	if err := m.removeInstallDir(); err != nil {
		return err
	}
	if err := m.state.Clear(); err != nil {
		return err
	}
	return nil
}

// Status returns the current provider status for UI and diagnostics.
func (m *Manager) Status() sandbox.ProviderStatus {
	details := StatusDetails{
		DistroName:      m.cfg.WSLDistroName,
		InstallDir:      m.cfg.WSLInstallDir,
		StateDir:        m.cfg.WSLStateDir,
		StatePath:       m.state.Path(),
		ImageRef:        m.cfg.WSLImageRef,
		BridgeType:      m.cfg.WSLBridgeType,
		BridgePort:      m.cfg.WSLBridgePort,
		UpgradeStrategy: m.cfg.WSLUpgradeStrategy,
	}

	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return sandbox.ProviderStatus{Available: false, State: "not_available", Message: "wsl.exe is not available on PATH", Details: details}
	}
	if strings.TrimSpace(m.cfg.WSLDistroName) == "" {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: "WSL_DISTRO_NAME is empty", Details: details}
	}

	bridgeInfo, err := m.resolveBridgeInfo()
	if err != nil {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: err.Error(), Details: details}
	}
	details.BridgeType = bridgeInfo.Type
	details.BridgePort = bridgeInfo.Port
	details.BridgePipeName = bridgeInfo.PipeName
	details.BridgeDockerHost = bridgeInfo.DockerHost

	ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
	defer cancel()

	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: err.Error(), Details: details}
	}
	if !found {
		return sandbox.ProviderStatus{Available: true, State: "not_installed", Message: fmt.Sprintf("managed WSL distro %q is not installed yet", m.cfg.WSLDistroName), Details: details}
	}

	details.DistroInstalled = true
	details.DistroState = distro.State
	details.DistroVersion = distro.Version

	bridgeReady, err := m.probeBridgeReady(ctx, bridgeInfo)
	if err == nil && bridgeReady {
		return sandbox.ProviderStatus{
			Available: true,
			State:     "ready",
			Message:   "managed WSL distro and Docker bridge are ready",
			Details:   details,
		}
	}

	message := "managed WSL distro is running; bridge startup is not implemented yet"
	state := "starting"
	if strings.EqualFold(distro.State, "Stopped") {
		message = "managed WSL distro is installed but currently stopped; it will be started on demand"
		state = "stopped"
	} else if strings.EqualFold(bridgeInfo.Type, BridgeTypeNamedPipe) {
		message = "managed WSL distro is running; named-pipe Docker bridge will be started on demand"
	} else if strings.EqualFold(bridgeInfo.Type, BridgeTypeTCP) {
		message = "managed WSL distro is running; TCP Docker bridge will be started on demand"
	}

	return sandbox.ProviderStatus{Available: true, State: state, Message: message, Details: details}
}

func (m *Manager) resolveBridgeInfo() (BridgeInfo, error) {
	port := m.cfg.WSLBridgePort
	if port == 0 && strings.EqualFold(m.cfg.WSLBridgeType, BridgeTypeTCP) {
		state, err := m.state.Load()
		if err != nil {
			return BridgeInfo{}, err
		}
		if strings.EqualFold(state.BridgeType, BridgeTypeTCP) && state.BridgePort > 0 {
			port = state.BridgePort
		}
	}
	return ResolveBridgeInfo(m.cfg.WSLBridgeType, m.cfg.WSLDistroName, port)
}

func (m *Manager) ensureBridgeReady(ctx context.Context, bridgeInfo BridgeInfo) (BridgeInfo, bool, error) {
	switch bridgeInfo.Type {
	case BridgeTypeNamedPipe:
		ready, err := m.probeBridgeReady(ctx, bridgeInfo)
		if err == nil && ready {
			return bridgeInfo, true, nil
		}
		if err := m.startNamedPipeBridge(ctx, bridgeInfo.PipeName); err != nil {
			return BridgeInfo{}, false, err
		}
		if err := m.waitForNamedPipeBridgeReady(ctx, bridgeInfo); err != nil {
			return BridgeInfo{}, false, err
		}
		if err := m.saveBridgeRuntimeState(bridgeInfo); err != nil {
			return BridgeInfo{}, false, err
		}
		return bridgeInfo, true, nil
	case BridgeTypeTCP:
		if bridgeInfo.Port == 0 {
			port, err := allocateLoopbackPort()
			if err != nil {
				return BridgeInfo{}, false, err
			}
			bridgeInfo.Port = port
			bridgeInfo.DockerHost = fmt.Sprintf("tcp://127.0.0.1:%d", port)
			if err := m.saveBridgeRuntimeState(bridgeInfo); err != nil {
				return BridgeInfo{}, false, err
			}
		}

		ready, err := m.probeBridgeReady(ctx, bridgeInfo)
		if err == nil && ready {
			return bridgeInfo, true, nil
		}

		if err := m.startTCPBridge(ctx, bridgeInfo.Port); err != nil {
			return BridgeInfo{}, false, err
		}
		if err := m.waitForTCPBridgeReady(ctx, bridgeInfo); err != nil {
			return BridgeInfo{}, false, err
		}
		if err := m.saveBridgeRuntimeState(bridgeInfo); err != nil {
			return BridgeInfo{}, false, err
		}
		return bridgeInfo, true, nil
	default:
		return BridgeInfo{}, false, fmt.Errorf("unsupported WSL bridge type %q", bridgeInfo.Type)
	}
}

func (m *Manager) probeDistro(ctx context.Context) (DistroInfo, bool, error) {
	output, err := m.runCommand(ctx, "wsl.exe", "--list", "--verbose")
	if err != nil {
		return DistroInfo{}, false, err
	}

	distros, err := ParseDistroList(output)
	if err != nil {
		return DistroInfo{}, false, err
	}
	distro, found := FindDistro(distros, m.cfg.WSLDistroName)
	return distro, found, nil
}

func (m *Manager) importDistro(ctx context.Context) error {
	artifact, err := m.downloader.EnsureRootfs(ctx)
	if err != nil {
		return fmt.Errorf("prepare WSL rootfs artifact: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.cfg.WSLInstallDir), 0755); err != nil {
		return fmt.Errorf("create WSL install parent dir: %w", err)
	}
	if _, err := os.Stat(m.cfg.WSLInstallDir); err == nil {
		return fmt.Errorf("WSL install dir %q already exists but distro is not registered", m.cfg.WSLInstallDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat WSL install dir %q: %w", m.cfg.WSLInstallDir, err)
	}

	rootfsTarPath, cleanup, err := m.decompressRootfsArchive(artifact.RootfsArchive)
	if err != nil {
		return err
	}
	defer cleanup()

	if _, err := m.runCommand(ctx, "wsl.exe", "--import", m.cfg.WSLDistroName, m.cfg.WSLInstallDir, rootfsTarPath, "--version", "2"); err != nil {
		return fmt.Errorf("import managed WSL distro %q: %w", m.cfg.WSLDistroName, err)
	}

	bridgeInfo, err := m.resolveBridgeInfo()
	if err != nil {
		return err
	}
	return m.state.Save(RuntimeState{
		DistroName: m.cfg.WSLDistroName,
		BridgeType: bridgeInfo.Type,
		BridgePort: bridgeInfo.Port,
		ImageRef:   m.cfg.WSLImageRef,
	})
}

func (m *Manager) decompressRootfsArchive(rootfsArchivePath string) (string, func(), error) {
	if err := os.MkdirAll(m.cfg.WSLStateDir, 0755); err != nil {
		return "", nil, fmt.Errorf("create WSL state dir: %w", err)
	}

	src, err := os.Open(rootfsArchivePath)
	if err != nil {
		return "", nil, fmt.Errorf("open rootfs archive %q: %w", rootfsArchivePath, err)
	}
	defer src.Close()

	decoder, err := zstd.NewReader(src)
	if err != nil {
		return "", nil, fmt.Errorf("open zstd decoder for %q: %w", rootfsArchivePath, err)
	}
	defer decoder.Close()

	tempFile, err := os.CreateTemp(m.cfg.WSLStateDir, "discobot-rootfs-*.tar")
	if err != nil {
		return "", nil, fmt.Errorf("create temp rootfs tar: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := func() { _ = os.Remove(tempPath) }

	if _, err := io.Copy(tempFile, decoder); err != nil {
		tempFile.Close()
		cleanup()
		return "", nil, fmt.Errorf("decompress rootfs archive %q: %w", rootfsArchivePath, err)
	}
	if err := tempFile.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close temp rootfs tar %q: %w", tempPath, err)
	}
	return tempPath, cleanup, nil
}

func (m *Manager) startDistro(ctx context.Context) error {
	_, err := m.runCommand(ctx, "wsl.exe", "-d", m.cfg.WSLDistroName, "--", "true")
	if err != nil {
		return fmt.Errorf("start managed WSL distro %q: %w", m.cfg.WSLDistroName, err)
	}
	return nil
}

func (m *Manager) waitForSystemdReady(ctx context.Context) error {
	return m.waitForCommandSuccess(ctx, "wait for systemd readiness", func(ctx context.Context) error {
		output, err := m.runInDistro(ctx, "systemctl", "is-system-running")
		if err != nil {
			return err
		}
		state := strings.TrimSpace(output)
		if state == "running" || state == "degraded" {
			return nil
		}
		return fmt.Errorf("systemd state is %q", state)
	})
}

func (m *Manager) waitForDockerReady(ctx context.Context) error {
	return m.waitForCommandSuccess(ctx, "wait for docker.service readiness", func(ctx context.Context) error {
		output, err := m.runInDistro(ctx, "systemctl", "is-active", "docker.service")
		if err != nil {
			return err
		}
		if strings.TrimSpace(output) != "active" {
			return fmt.Errorf("docker.service state is %q", strings.TrimSpace(output))
		}
		return nil
	})
}

func (m *Manager) waitForTCPBridgeReady(ctx context.Context, bridgeInfo BridgeInfo) error {
	return m.waitForCommandSuccess(ctx, "wait for WSL Docker bridge readiness", func(ctx context.Context) error {
		ready, err := m.probeBridgeReady(ctx, bridgeInfo)
		if err != nil {
			return err
		}
		if !ready {
			return fmt.Errorf("docker bridge is not responding on %s", bridgeInfo.DockerHost)
		}
		return nil
	})
}

func (m *Manager) waitForNamedPipeBridgeReady(ctx context.Context, bridgeInfo BridgeInfo) error {
	return m.waitForCommandSuccess(ctx, "wait for WSL named-pipe Docker bridge readiness", func(ctx context.Context) error {
		ready, err := m.probeBridgeReady(ctx, bridgeInfo)
		if err != nil {
			return err
		}
		if !ready {
			return fmt.Errorf("docker bridge is not responding on %s", bridgeInfo.DockerHost)
		}
		return nil
	})
}

func (m *Manager) waitForCommandSuccess(ctx context.Context, description string, fn func(context.Context) error) error {
	deadlineCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		deadlineCtx, cancel = context.WithTimeout(ctx, defaultReadyTimeout)
		defer cancel()
	}

	ticker := time.NewTicker(defaultReadyPollDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(deadlineCtx, defaultProbeTimeout)
		lastErr = fn(attemptCtx)
		cancel()
		if lastErr == nil {
			return nil
		}

		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("%s: %w (last error: %v)", description, deadlineCtx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func (m *Manager) runInDistro(ctx context.Context, args ...string) (string, error) {
	base := []string{"-d", m.cfg.WSLDistroName, "--"}
	base = append(base, args...)
	return m.runCommand(ctx, "wsl.exe", base...)
}

func (m *Manager) startTCPBridge(ctx context.Context, port int) error {
	if port <= 0 {
		return fmt.Errorf("tcp bridge port must be greater than zero")
	}

	command := fmt.Sprintf(
		"command -v socat >/dev/null 2>&1 || { echo 'socat is required for WSL TCP bridge startup' >&2; exit 127; }; "+
			"( ss -ltnH '( sport = :%d )' 2>/dev/null || netstat -ltn 2>/dev/null ) | grep -q ':%d ' || "+
			"nohup socat TCP-LISTEN:%d,bind=127.0.0.1,reuseaddr,fork UNIX-CONNECT:%s >%s 2>&1 </dev/null &",
		port,
		port,
		port,
		dockerSockPath,
		bridgeLogPath,
	)
	if _, err := m.runInDistro(ctx, "sh", "-lc", command); err != nil {
		return fmt.Errorf("start WSL TCP Docker bridge on port %d: %w", port, err)
	}
	return nil
}

func (m *Manager) startNamedPipeBridge(_ context.Context, pipeName string) error {
	if strings.TrimSpace(pipeName) == "" {
		return fmt.Errorf("named-pipe bridge pipe name is empty")
	}

	pipePath := bridgePipePath(pipeName)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pipeListener != nil && m.pipeListenerName == pipeName {
		return nil
	}
	if m.pipeListener != nil {
		m.stopNamedPipeBridgeLocked()
	}

	listener, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		return fmt.Errorf("listen on WSL named pipe %q: %w", pipePath, err)
	}

	closeCh := make(chan struct{})
	m.pipeListener = listener
	m.pipeListenerName = pipeName
	m.pipeListenerClose = closeCh

	go m.serveNamedPipeBridge(listener, pipeName, closeCh)
	return nil
}

func (m *Manager) probeBridgeReady(ctx context.Context, bridgeInfo BridgeInfo) (bool, error) {
	switch bridgeInfo.Type {
	case BridgeTypeNamedPipe:
		if bridgeInfo.PipeName == "" {
			return false, nil
		}
		return m.probeNamedPipeBridgeReady(ctx, bridgeInfo.PipeName)
	case BridgeTypeTCP:
		if bridgeInfo.DockerHost == "" || bridgeInfo.Port <= 0 {
			return false, nil
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(bridgeInfo.DockerHost, "/")+"/_ping", nil)
		if err != nil {
			return false, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK, nil
	default:
		return false, fmt.Errorf("unsupported WSL bridge type %q", bridgeInfo.Type)
	}
}

func (m *Manager) saveBridgeRuntimeState(bridgeInfo BridgeInfo) error {
	return m.state.Save(RuntimeState{
		DistroName: m.cfg.WSLDistroName,
		BridgeType: bridgeInfo.Type,
		BridgePort: bridgeInfo.Port,
		ImageRef:   m.cfg.WSLImageRef,
	})
}

func (m *Manager) unregisterDistro(ctx context.Context) error {
	if _, err := m.runCommand(ctx, "wsl.exe", "--unregister", m.cfg.WSLDistroName); err != nil {
		return fmt.Errorf("unregister managed WSL distro %q: %w", m.cfg.WSLDistroName, err)
	}
	return nil
}

func (m *Manager) removeInstallDir() error {
	installDir := strings.TrimSpace(m.cfg.WSLInstallDir)
	if installDir == "" {
		return nil
	}
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("remove WSL install dir %q: %w", installDir, err)
	}
	return nil
}

func (m *Manager) stopNamedPipeBridgeLocked() {
	if m.pipeListener != nil {
		_ = m.pipeListener.Close()
		m.pipeListener = nil
	}
	if m.pipeListenerClose != nil {
		close(m.pipeListenerClose)
		m.pipeListenerClose = nil
	}
	m.pipeListenerName = ""
}

func (m *Manager) serveNamedPipeBridge(listener net.Listener, pipeName string, closeCh <-chan struct{}) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-closeCh:
				return
			default:
			}
			return
		}

		go m.handleNamedPipeBridgeConn(conn)
	}
}

func (m *Manager) handleNamedPipeBridgeConn(conn net.Conn) {
	defer conn.Close()

	cmd := exec.Command("wsl.exe", "-d", m.cfg.WSLDistroName, "--", "sh", "-lc", fmt.Sprintf("command -v socat >/dev/null 2>&1 || { echo 'socat is required for WSL named-pipe bridge' >&2; exit 127; }; exec socat STDIO UNIX-CONNECT:%s", dockerSockPath))

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return
	}

	copyDone := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(stdin, conn)
		_ = stdin.Close()
		copyDone <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(conn, stdout)
		_ = stdout.Close()
		copyDone <- struct{}{}
	}()

	<-copyDone
	_ = conn.Close()
	<-copyDone
	_ = cmd.Wait()
}

func (m *Manager) probeNamedPipeBridgeReady(ctx context.Context, pipeName string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://discobot/_ping", nil)
	if err != nil {
		return false, err
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return winio.DialPipeContext(ctx, bridgePipePath(pipeName))
			},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

func (m *Manager) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, trimmed)
	}
	return trimmed, nil
}

func allocateLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate loopback port: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return 0, fmt.Errorf("allocate loopback port: unexpected listener addr %T", listener.Addr())
	}
	return addr.Port, nil
}
