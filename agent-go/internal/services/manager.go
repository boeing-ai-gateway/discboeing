package services

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/processes"
	"github.com/obot-platform/discobot/agent-go/internal/workspaceenv"
)

// subscriber receives live output events from a managed service.
type subscriber struct {
	ch chan OutputEvent
}

// managedService tracks a running service process and its subscribers.
type managedService struct {
	mu          sync.Mutex
	service     ServiceInfo
	sessionID   string
	closeCh     chan struct{} // closed when process exits
	closeOnce   sync.Once
	subscribers []*subscriber
}

// broadcast sends an event to all subscribers.
func (m *managedService) broadcast(event OutputEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sub := range m.subscribers {
		select {
		case sub.ch <- event:
		default:
			// drop if subscriber is slow
		}
	}
}

// addSubscriber registers a new subscriber and returns its channel and unsubscribe func.
func (m *managedService) addSubscriber() (ch <-chan OutputEvent, unsubscribe func()) {
	sub := &subscriber{ch: make(chan OutputEvent, 256)}
	m.mu.Lock()
	m.subscribers = append(m.subscribers, sub)
	m.mu.Unlock()

	return sub.ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, s := range m.subscribers {
			if s == sub {
				m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
				break
			}
		}
	}
}

// Manager handles service discovery, process lifecycle, and output streaming.
type Manager struct {
	mu          sync.RWMutex
	services    map[string]*managedService // keyed by service ID
	envSnapshot func() map[string]string
	processes   *processes.Manager
}

// NewManager creates a new service Manager.
func NewManager(defaultWorkDir string, processManager ...*processes.Manager) *Manager {
	var procMgr *processes.Manager
	if len(processManager) > 0 && processManager[0] != nil {
		procMgr = processManager[0]
	} else {
		procMgr = processes.NewManager(defaultWorkDir)
	}
	return &Manager{
		services:  make(map[string]*managedService),
		processes: procMgr,
	}
}

// SetEnvSnapshot sets an optional function that returns request-scoped
// environment variables to inject into launched services.
func (mgr *Manager) SetEnvSnapshot(fn func() map[string]string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.envSnapshot = fn
}

// desktopService is the built-in VNC desktop service.
var desktopService = ServiceInfo{
	ID:      "discobot-desktop",
	Name:    "Desktop",
	HTTP:    6080,
	Path:    "",
	Status:  "running",
	Passive: true,
}

// vscodeService is the built-in code-server service.
var vscodeService = ServiceInfo{
	ID:          "discobot-vscode",
	Name:        "VS Code",
	Description: "Browser-based VS Code workspace",
	HTTP:        13337,
	Path:        "",
	URLPath:     "/",
	Status:      "running",
	Passive:     true,
}

var lookPath = exec.LookPath

// isDesktopAvailable checks if x11vnc is installed and executable.
func isDesktopAvailable() bool {
	info, err := os.Stat("/usr/bin/x11vnc")
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
}

// isVSCodeAvailable checks if code-server is installed and executable.
func isVSCodeAvailable() bool {
	_, err := lookPath("code-server")
	return err == nil
}

// GetServices returns all discovered services merged with running state.
func (mgr *Manager) GetServices(workspaceRoot string) ([]ServiceInfo, error) {
	servicesDir := filepath.Join(workspaceRoot, ServicesDir)
	discovered, err := DiscoverServices(servicesDir)
	if err != nil {
		return nil, err
	}

	var result []ServiceInfo
	seen := make(map[string]struct{})

	// Add built-in passive services if available.
	if isDesktopAvailable() {
		result = append(result, desktopService)
		seen[desktopService.ID] = struct{}{}
	}
	if isVSCodeAvailable() {
		result = append(result, vscodeService)
		seen[vscodeService.ID] = struct{}{}
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	for _, svc := range discovered {
		if _, exists := seen[svc.ID]; exists {
			continue
		}
		if managed, ok := mgr.services[svc.ID]; ok {
			managed.mu.Lock()
			result = append(result, managed.service)
			managed.mu.Unlock()
		} else {
			result = append(result, svc)
		}
	}

	return result, nil
}

// GetService returns a single service by ID.
func (mgr *Manager) GetService(workspaceRoot, serviceID string) (*ServiceInfo, error) {
	if serviceID == desktopService.ID {
		if isDesktopAvailable() {
			svc := desktopService
			return &svc, nil
		}
		return nil, nil
	}
	if serviceID == vscodeService.ID {
		if isVSCodeAvailable() {
			svc := vscodeService
			return &svc, nil
		}
		return nil, nil
	}

	// Check running services first
	mgr.mu.RLock()
	if managed, ok := mgr.services[serviceID]; ok {
		managed.mu.Lock()
		svc := managed.service
		managed.mu.Unlock()
		mgr.mu.RUnlock()
		return &svc, nil
	}
	mgr.mu.RUnlock()

	// Fall back to discovery
	services, err := mgr.GetServices(workspaceRoot)
	if err != nil {
		return nil, err
	}
	for i := range services {
		if services[i].ID == serviceID {
			return &services[i], nil
		}
	}
	return nil, nil
}

// StartService starts a service by ID. Returns the service info or an error code.
func (mgr *Manager) StartService(workspaceRoot, serviceID string) (*ServiceInfo, string, error) {
	// Check if already running
	mgr.mu.RLock()
	if managed, ok := mgr.services[serviceID]; ok {
		managed.mu.Lock()
		status := managed.service.Status
		pid := managed.service.PID
		managed.mu.Unlock()
		mgr.mu.RUnlock()

		if status == "running" || status == "starting" {
			return nil, "service_already_running", fmt.Errorf("service %s already running (pid %d)", serviceID, pid)
		}
	} else {
		mgr.mu.RUnlock()
	}

	// Discover service
	servicesDir := filepath.Join(workspaceRoot, ServicesDir)
	discovered, err := DiscoverServices(servicesDir)
	if err != nil {
		return nil, "", err
	}

	var svcTemplate *ServiceInfo
	for i := range discovered {
		if discovered[i].ID == serviceID {
			svcTemplate = &discovered[i]
			break
		}
	}
	if svcTemplate == nil {
		return nil, "service_not_found", fmt.Errorf("service %s not found", serviceID)
	}

	// Clear previous output
	clearOutput(serviceID)

	// Spawn the service with the visible environment snapshot from this request.
	requestEnv := mgr.visibleEnvSnapshot(workspaceRoot)
	mgr.spawnService(workspaceRoot, *svcTemplate, requestEnv)

	svc := ServiceInfo{
		ID:     serviceID,
		Status: "starting",
	}
	return &svc, "", nil
}

// StopService stops a running service.
func (mgr *Manager) StopService(serviceID string) (string, error) {
	mgr.mu.RLock()
	managed, ok := mgr.services[serviceID]
	mgr.mu.RUnlock()

	if !ok {
		return "service_not_found", fmt.Errorf("service %s not found", serviceID)
	}

	managed.mu.Lock()
	status := managed.service.Status
	managed.mu.Unlock()

	if status != "running" && status != "starting" {
		return "service_not_running", fmt.Errorf("service %s is not running", serviceID)
	}

	managed.mu.Lock()
	managed.service.Status = "stopping"
	sessionID := managed.sessionID
	managed.mu.Unlock()

	if sessionID != "" {
		_ = mgr.processes.Kill(sessionID)

		go func() {
			time.Sleep(5 * time.Second)
			managed.mu.Lock()
			s := managed.service.Status
			managed.mu.Unlock()
			if s == "stopping" {
				_ = mgr.processes.Kill(sessionID)
			}
		}()
	}

	return "", nil
}

// Subscribe returns a channel of live output events for a service and an unsubscribe func.
// Returns nil channels if the service is not managed (not running).
func (mgr *Manager) Subscribe(serviceID string) (<-chan OutputEvent, func(), <-chan struct{}) {
	mgr.mu.RLock()
	managed, ok := mgr.services[serviceID]
	mgr.mu.RUnlock()

	if !ok {
		return nil, func() {}, nil
	}

	ch, unsub := managed.addSubscriber()
	return ch, unsub, managed.closeCh
}

// GetServiceOutput returns stored output events from the JSONL file.
func (mgr *Manager) GetServiceOutput(serviceID string) []OutputEvent {
	return readEvents(serviceID)
}

// IsManaged returns whether a service is currently in the managed services map.
func (mgr *Manager) IsManaged(serviceID string) bool {
	mgr.mu.RLock()
	_, ok := mgr.services[serviceID]
	mgr.mu.RUnlock()
	return ok
}

// spawnService starts a service process and registers it in the manager.
func (mgr *Manager) spawnService(workspaceRoot string, svcTemplate ServiceInfo, requestEnv map[string]string) {
	svc := svcTemplate
	svc.Status = "starting"
	svc.StartedAt = time.Now().UTC().Format(time.RFC3339)

	command, args := buildServiceCommand(svcTemplate.Path)
	session, err := mgr.processes.Start(context.Background(), processes.CreateRequest{
		Kind:     processes.KindService,
		Name:     svc.Name,
		ReuseKey: "service:" + svc.ID,
		Cmd:      append([]string{command}, args...),
		WorkDir:  workspaceRoot,
		Env:      mergedEnvMap(requestEnv),
		Metadata: map[string]string{"serviceId": svc.ID},
	})
	if err != nil {
		appendEvent(svc.ID, newErrorEvent(err.Error()))
		return
	}

	svc.PID = session.PID
	svc.Status = "running"

	managed := &managedService{
		service:   svc,
		sessionID: session.ID,
		closeCh:   make(chan struct{}),
	}

	mgr.mu.Lock()
	mgr.services[svc.ID] = managed
	mgr.mu.Unlock()

	emitEvent := func(event OutputEvent) {
		appendEvent(svc.ID, event)
		managed.broadcast(event)
	}

	go func() {
		events, unsubscribe, done, err := mgr.processes.Subscribe(session.ID)
		if err != nil {
			emitEvent(newErrorEvent(err.Error()))
			close(managed.closeCh)
			return
		}
		defer unsubscribe()

		for {
			select {
			case event, ok := <-events:
				if !ok {
					mgr.finishService(svc.ID, managed)
					return
				}
				switch event.Type {
				case "stdout":
					emitEvent(newStdoutEvent(event.Data))
				case "stderr":
					emitEvent(newStderrEvent(event.Data))
				case "exit":
					emitEvent(newExitEvent(event.ExitCode))
				case "error":
					emitEvent(newErrorEvent(event.Error))
				}
			case <-done:
				mgr.finishService(svc.ID, managed)
				return
			}
		}
	}()
}

func (mgr *Manager) finishService(serviceID string, managed *managedService) {
	managed.closeOnce.Do(func() {
		session, err := mgr.processes.Get(managed.sessionID)
		managed.mu.Lock()
		managed.service.Status = "stopped"
		if err == nil {
			managed.service.ExitCode = session.ExitCode
		}
		managed.mu.Unlock()

		close(managed.closeCh)

		time.AfterFunc(30*time.Second, func() {
			mgr.mu.Lock()
			if current, ok := mgr.services[serviceID]; ok && current == managed {
				delete(mgr.services, serviceID)
			}
			mgr.mu.Unlock()
		})
	})
}

func buildServiceCommand(path string) (string, []string) {
	if runtime.GOOS != "windows" {
		return path, nil
	}

	interpreter, args := parseServiceShebang(path)
	switch interpreter {
	case "bash", "sh", "zsh":
		return "bash", append(args, path)
	case "":
		return path, nil
	default:
		return interpreter, append(args, path)
	}
}

func (mgr *Manager) visibleEnvSnapshot(workspaceRoot string) map[string]string {
	env := workspaceenv.FileSnapshot(workspaceRoot)
	if env == nil {
		env = map[string]string{}
	}
	mgr.mu.RLock()
	fn := mgr.envSnapshot
	mgr.mu.RUnlock()
	if fn == nil {
		return env
	}
	maps.Copy(env, fn())
	return env
}

func mergedEnvMap(requestEnv map[string]string) map[string]string {
	return workspaceenv.MergeProcessSnapshot(requestEnv)
}

func parseServiceShebang(path string) (string, []string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}

	line, _, _ := strings.Cut(string(content), "\n")
	if !strings.HasPrefix(line, "#!") {
		return "", nil
	}

	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "#!")))
	if len(fields) == 0 {
		return "", nil
	}

	interpreter := filepath.Base(fields[0])
	args := fields[1:]
	if interpreter == "env" {
		for len(args) > 0 && args[0] == "-S" {
			args = args[1:]
		}
		if len(args) == 0 {
			return "", nil
		}
		interpreter = filepath.Base(args[0])
		args = args[1:]
	}

	return strings.ToLower(interpreter), args
}
