//go:build windows

package wsl

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	dockerclient "github.com/docker/docker/client"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/docker"
	"github.com/obot-platform/discobot/server/internal/startup"
)

func TestProviderStartBackgroundBootstrapCompletesStartupTasks(t *testing.T) {
	t.Parallel()

	systemManager := startup.NewSystemManager(nil, "local")
	started := make(chan struct{})

	provider := &Provider{
		systemManager: systemManager,
		ensureInstalled: func(_ context.Context, progress progressReporter) error {
			progress.Update(100, "Managed WSL distro is installed")
			return nil
		},
		ensureRunning: func(_ context.Context, progress progressReporter) (*RuntimeInfo, error) {
			progress.Update(100, "Managed WSL distro and Docker bridge are ready")
			close(started)
			return &RuntimeInfo{
				DistroState:      "Running",
				BridgeDockerHost: "tcp://127.0.0.1:23755",
				BridgeReady:      true,
			}, nil
		},
	}

	provider.startBackgroundBootstrap()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for background bootstrap to finish")
	}

	waitForTaskState(t, systemManager, startupTaskWSLInstallID, startup.TaskStateCompleted)
	waitForTaskState(t, systemManager, startupTaskWSLStartID, startup.TaskStateCompleted)
	assertTaskOperation(t, systemManager, startupTaskWSLInstallID, "Managed WSL distro is installed")
	assertTaskOperation(t, systemManager, startupTaskWSLStartID, "Managed WSL distro and Docker bridge are ready")
}

func TestProviderStartBackgroundBootstrapFailsStartupTasksOnInstallError(t *testing.T) {
	t.Parallel()

	systemManager := startup.NewSystemManager(nil, "local")
	installErr := errors.New("install failed")
	failed := make(chan struct{})

	provider := &Provider{
		systemManager: systemManager,
		ensureInstalled: func(_ context.Context, progress progressReporter) error {
			progress.Update(45, "Preparing managed WSL install directory")
			close(failed)
			return installErr
		},
		ensureRunning: func(context.Context, progressReporter) (*RuntimeInfo, error) {
			t.Fatal("ensureRunning should not be called when install fails")
			return nil, nil
		},
	}

	provider.startBackgroundBootstrap()

	select {
	case <-failed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for background bootstrap failure")
	}

	waitForTaskState(t, systemManager, startupTaskWSLInstallID, startup.TaskStateFailed)
	waitForTaskState(t, systemManager, startupTaskWSLStartID, startup.TaskStateFailed)

	startTask, ok := systemManager.GetTask(startupTaskWSLStartID)
	if !ok {
		t.Fatalf("startup task %q was not registered", startupTaskWSLStartID)
	}
	if startTask.Error == "" {
		t.Fatalf("startup task %q error was empty", startupTaskWSLStartID)
	}
	assertTaskOperation(t, systemManager, startupTaskWSLInstallID, "Preparing managed WSL install directory")
}

func TestProviderEnsureRuntimeInfoWaitsForBackgroundInstall(t *testing.T) {
	t.Parallel()

	installStarted := make(chan struct{})
	unblockInstall := make(chan struct{})
	runtimeCalled := make(chan struct{}, 2)
	result := make(chan error, 1)

	provider := &Provider{
		ensureInstalled: func(_ context.Context, progress progressReporter) error {
			progress.Update(50, "Installing managed WSL distro")
			close(installStarted)
			<-unblockInstall
			return nil
		},
		ensureRunning: func(_ context.Context, progress progressReporter) (*RuntimeInfo, error) {
			progress.Update(100, "Managed WSL distro and Docker bridge are ready")
			runtimeCalled <- struct{}{}
			return &RuntimeInfo{BridgeReady: true}, nil
		},
	}

	provider.startBackgroundBootstrap()

	select {
	case <-installStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for background install to start")
	}

	go func() {
		_, err := provider.ensureRuntimeInfo(context.Background(), progressReporter{})
		result <- err
	}()

	select {
	case <-runtimeCalled:
		t.Fatal("ensureRunning was called before background install completed")
	case err := <-result:
		t.Fatalf("ensureRuntimeInfo() returned before background install completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(unblockInstall)

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("ensureRuntimeInfo() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ensureRuntimeInfo() to return")
	}
}

func TestProviderCloseCancelsBackgroundBootstrap(t *testing.T) {
	t.Parallel()

	installCanceled := make(chan struct{})
	provider := &Provider{
		ensureInstalled: func(ctx context.Context, progress progressReporter) error {
			progress.Update(50, "Installing managed WSL distro")
			<-ctx.Done()
			close(installCanceled)
			return ctx.Err()
		},
		ensureRunning: func(context.Context, progressReporter) (*RuntimeInfo, error) {
			t.Fatal("ensureRunning should not be called after Close cancels bootstrap")
			return nil, nil
		},
	}

	provider.startBackgroundBootstrap()

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- provider.Close()
	}()

	select {
	case <-installCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for background bootstrap cancellation")
	}

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Close() to return")
	}
}

func TestProviderEnsureRuntimeInfoReconcilesFailedStartupTask(t *testing.T) {
	t.Parallel()

	systemManager := startup.NewSystemManager(nil, "local")
	systemManager.RegisterTask(startupTaskWSLStartID, "Starting managed WSL distro")
	systemManager.FailTask(startupTaskWSLStartID, errors.New("earlier bootstrap failed"))

	provider := &Provider{
		systemManager: systemManager,
		ensureRunning: func(_ context.Context, progress progressReporter) (*RuntimeInfo, error) {
			progress.Update(100, "Managed WSL distro and Docker bridge are ready")
			return &RuntimeInfo{BridgeReady: true}, nil
		},
	}

	if _, err := provider.ensureRuntimeInfo(context.Background(), progressReporter{}); err != nil {
		t.Fatalf("ensureRuntimeInfo() error = %v", err)
	}

	waitForTaskState(t, systemManager, startupTaskWSLStartID, startup.TaskStateCompleted)
	assertTaskOperation(t, systemManager, startupTaskWSLStartID, "Managed WSL distro and Docker bridge are ready")
}

func TestProviderStatusReconcilesFailedStartupTaskWhenReady(t *testing.T) {
	t.Parallel()

	systemManager := startup.NewSystemManager(nil, "local")
	systemManager.RegisterTask(startupTaskWSLStartID, "Starting managed WSL distro")
	systemManager.FailTask(startupTaskWSLStartID, errors.New("earlier bootstrap failed"))

	provider := &Provider{
		systemManager: systemManager,
		status: func() sandbox.ProviderStatus {
			return sandbox.ProviderStatus{Available: true, State: "ready"}
		},
	}

	status := provider.Status()
	if status.State != "ready" {
		t.Fatalf("Status().State = %q, want %q", status.State, "ready")
	}

	waitForTaskState(t, systemManager, startupTaskWSLStartID, startup.TaskStateCompleted)
	assertTaskOperation(t, systemManager, startupTaskWSLStartID, "Managed WSL distro and Docker bridge are ready")
}

func TestBridgeNotReadyErrorForDynamicTCPPort(t *testing.T) {
	t.Parallel()

	err := bridgeNotReadyError(&RuntimeInfo{BridgeType: BridgeTypeTCP})
	if err == nil {
		t.Fatal("bridgeNotReadyError() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "assigned a loopback port on startup") {
		t.Fatalf("bridgeNotReadyError() = %q, want dynamic port message", err.Error())
	}
	if strings.Contains(err.Error(), "not implemented yet") {
		t.Fatalf("bridgeNotReadyError() = %q, should not mention unimplemented behavior", err.Error())
	}
}

func TestProviderDockerProviderForRuntimeLoadsLocalImageOnce(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{SandboxImage: "discobot-local/agent-api:latest"}
	runtimeInfo := &RuntimeInfo{
		BridgeDockerHost: "tcp://127.0.0.1:23755",
		BridgeReady:      true,
	}

	dockerProvider := &docker.Provider{}
	var newCalls int
	var loadCalls int

	provider := &Provider{
		cfg: cfg,
		newDockerProvider: func(cfg *config.Config) (*docker.Provider, error) {
			newCalls++
			if cfg.DockerHost != runtimeInfo.BridgeDockerHost {
				t.Fatalf("newDockerProvider() DockerHost = %q, want %q", cfg.DockerHost, runtimeInfo.BridgeDockerHost)
			}
			return dockerProvider, nil
		},
		ensureLocalImageLoad: func(_ context.Context, got *docker.Provider) error {
			loadCalls++
			if got != dockerProvider {
				t.Fatalf("ensureLocalImageLoad() provider = %p, want %p", got, dockerProvider)
			}
			return nil
		},
	}

	got, err := provider.dockerProviderForRuntime(context.Background(), runtimeInfo)
	if err != nil {
		t.Fatalf("dockerProviderForRuntime() error = %v", err)
	}
	if got != dockerProvider {
		t.Fatalf("dockerProviderForRuntime() provider = %p, want %p", got, dockerProvider)
	}

	got, err = provider.dockerProviderForRuntime(context.Background(), runtimeInfo)
	if err != nil {
		t.Fatalf("dockerProviderForRuntime() cached error = %v", err)
	}
	if got != dockerProvider {
		t.Fatalf("dockerProviderForRuntime() cached provider = %p, want %p", got, dockerProvider)
	}
	if newCalls != 1 {
		t.Fatalf("newDockerProvider() calls = %d, want 1", newCalls)
	}
	if loadCalls != 1 {
		t.Fatalf("ensureLocalImageLoad() calls = %d, want 1", loadCalls)
	}
}

func TestProviderDockerProviderForRuntimeReturnsLocalImageLoadError(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("load failed")
	provider := &Provider{
		cfg: &config.Config{SandboxImage: "discobot-local/agent-api:latest"},
		newDockerProvider: func(*config.Config) (*docker.Provider, error) {
			return &docker.Provider{}, nil
		},
		ensureLocalImageLoad: func(context.Context, *docker.Provider) error {
			return loadErr
		},
	}

	_, err := provider.dockerProviderForRuntime(context.Background(), &RuntimeInfo{
		BridgeDockerHost: "tcp://127.0.0.1:23755",
		BridgeReady:      true,
	})
	if err == nil {
		t.Fatal("dockerProviderForRuntime() error = nil, want error")
	}
	if !strings.Contains(err.Error(), loadErr.Error()) {
		t.Fatalf("dockerProviderForRuntime() error = %q, want to contain %q", err.Error(), loadErr.Error())
	}
	if provider.dockerProvider != nil {
		t.Fatal("dockerProviderForRuntime() cached provider on local image load error")
	}
	if provider.dockerHost != "" {
		t.Fatalf("dockerProviderForRuntime() cached docker host = %q, want empty", provider.dockerHost)
	}
}

func TestProviderRequireDockerProviderRetriesAfterStaleTCPBridgeFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
	}
	manager := NewManager(cfg)
	if err := manager.state.Save(RuntimeState{
		DistroName: cfg.WSLDistroName,
		BridgeType: BridgeTypeTCP,
		BridgePort: 1111,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	refreshedRuntimeInfo := &RuntimeInfo{
		BridgeType:       BridgeTypeTCP,
		BridgePort:       2222,
		BridgeDockerHost: "tcp://127.0.0.1:2222",
		BridgeReady:      true,
	}
	dockerProvider := &docker.Provider{}
	ensureRunningCalls := 0

	provider := &Provider{
		cfg:     cfg,
		manager: manager,
		ensureRunning: func(_ context.Context, progress progressReporter) (*RuntimeInfo, error) {
			ensureRunningCalls++
			progress.Update(100, "Managed WSL distro and Docker bridge are ready")
			return refreshedRuntimeInfo, nil
		},
		newDockerProvider: func(cfg *config.Config) (*docker.Provider, error) {
			switch cfg.DockerHost {
			case "tcp://127.0.0.1:1111":
				return nil, errors.New(`failed to connect to docker daemon: Head "http://127.0.0.1:1111/_ping": connectex: No connection could be made because the target machine actively refused it`)
			case refreshedRuntimeInfo.BridgeDockerHost:
				return dockerProvider, nil
			default:
				t.Fatalf("newDockerProvider() DockerHost = %q, want stale or refreshed host", cfg.DockerHost)
				return nil, nil
			}
		},
		ensureLocalImageLoad: func(_ context.Context, got *docker.Provider) error {
			if got != dockerProvider {
				t.Fatalf("ensureLocalImageLoad() provider = %p, want %p", got, dockerProvider)
			}
			return nil
		},
	}

	got, err := provider.requireDockerProvider(context.Background(), &RuntimeInfo{
		BridgeType:       BridgeTypeTCP,
		BridgePort:       1111,
		BridgeDockerHost: "tcp://127.0.0.1:1111",
		BridgeReady:      true,
	})
	if err != nil {
		t.Fatalf("requireDockerProvider() error = %v", err)
	}
	if got != dockerProvider {
		t.Fatalf("requireDockerProvider() provider = %p, want %p", got, dockerProvider)
	}
	if ensureRunningCalls != 1 {
		t.Fatalf("ensureRunning() calls = %d, want 1 retry", ensureRunningCalls)
	}

	state, err := manager.state.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state != (RuntimeState{}) {
		t.Fatalf("Load() = %#v, want cleared persisted state after stale bridge retry", state)
	}

	runtimeInfo := provider.loadRuntimeInfo()
	if runtimeInfo == nil || runtimeInfo.BridgeDockerHost != refreshedRuntimeInfo.BridgeDockerHost {
		t.Fatalf("loadRuntimeInfo() = %#v, want refreshed host %q", runtimeInfo, refreshedRuntimeInfo.BridgeDockerHost)
	}
}

func TestProviderEnsureRuntimeInfoUsesCachedBridgeInsteadOfReenteringWSL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_ping" {
			t.Fatalf("request path = %q, want %q", r.URL.Path, "/_ping")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", server.URL, err)
	}
	port, err := strconv.Atoi(serverURL.Port())
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", serverURL.Port(), err)
	}

	var ensureRunningCalls int
	provider := &Provider{
		manager: NewManager(&config.Config{}),
		ensureRunning: func(_ context.Context, progress progressReporter) (*RuntimeInfo, error) {
			ensureRunningCalls++
			progress.Update(100, "Managed WSL distro and Docker bridge are ready")
			return &RuntimeInfo{
				BridgeType:       BridgeTypeTCP,
				BridgePort:       port,
				BridgeDockerHost: "tcp://127.0.0.1:" + serverURL.Port(),
				BridgeReady:      true,
			}, nil
		},
	}

	if _, err := provider.ensureRuntimeInfo(context.Background(), progressReporter{}); err != nil {
		t.Fatalf("ensureRuntimeInfo() first call error = %v", err)
	}
	if _, err := provider.ensureRuntimeInfo(context.Background(), progressReporter{}); err != nil {
		t.Fatalf("ensureRuntimeInfo() second call error = %v", err)
	}
	if ensureRunningCalls != 1 {
		t.Fatalf("ensureRunning() calls = %d, want 1", ensureRunningCalls)
	}
}

func TestProviderEnsureRuntimeInfoFallsBackWhenCachedBridgeIsUnavailable(t *testing.T) {
	t.Parallel()

	var ensureRunningCalls int
	provider := &Provider{
		manager: NewManager(&config.Config{}),
		ensureRunning: func(_ context.Context, progress progressReporter) (*RuntimeInfo, error) {
			ensureRunningCalls++
			progress.Update(100, "Managed WSL distro and Docker bridge are ready")
			return &RuntimeInfo{
				BridgeType:       BridgeTypeTCP,
				BridgePort:       9,
				BridgeDockerHost: "tcp://127.0.0.1:9",
				BridgeReady:      true,
			}, nil
		},
	}

	if _, err := provider.ensureRuntimeInfo(context.Background(), progressReporter{}); err != nil {
		t.Fatalf("ensureRuntimeInfo() first call error = %v", err)
	}
	if _, err := provider.ensureRuntimeInfo(context.Background(), progressReporter{}); err != nil {
		t.Fatalf("ensureRuntimeInfo() second call error = %v", err)
	}
	if ensureRunningCalls != 2 {
		t.Fatalf("ensureRunning() calls = %d, want 2", ensureRunningCalls)
	}
}

func TestProviderEnsureRuntimeInfoDeduplicatesConcurrentEnsureRunning(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	result := make(chan error, 2)

	var ensureRunningCalls int
	provider := &Provider{
		ensureRunning: func(_ context.Context, progress progressReporter) (*RuntimeInfo, error) {
			ensureRunningCalls++
			progress.Update(100, "Managed WSL distro and Docker bridge are ready")
			started <- struct{}{}
			<-release
			return &RuntimeInfo{BridgeReady: true}, nil
		},
	}

	go func() {
		_, err := provider.ensureRuntimeInfo(context.Background(), progressReporter{})
		result <- err
	}()
	go func() {
		_, err := provider.ensureRuntimeInfo(context.Background(), progressReporter{})
		result <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ensureRunning() to start")
	}

	select {
	case <-started:
		t.Fatal("ensureRunning() started more than once concurrently")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	for range 2 {
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("ensureRuntimeInfo() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for ensureRuntimeInfo() to return")
		}
	}

	if ensureRunningCalls != 1 {
		t.Fatalf("ensureRunning() calls = %d, want 1", ensureRunningCalls)
	}
}

func TestProviderRunningSandboxCountTreatsActiveWatchAsActivity(t *testing.T) {
	cfg := &config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
	}
	provider := &Provider{
		cfg:     cfg,
		manager: NewManager(cfg),
	}

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})
	runCommandOutput = func(_ context.Context, name string, _ ...string) ([]byte, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %q", name)
		}
		return []byte(distroListForTest("Running")), nil
	}

	releaseWatch := provider.retainActiveWatch()
	defer releaseWatch()

	count, err := provider.RunningSandboxCount(context.Background(), "discobot")
	if err != nil {
		t.Fatalf("RunningSandboxCount() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("RunningSandboxCount() = %d, want 1 while a watch is active", count)
	}
}

func TestProviderStopRuntimeClosesHostDockerClient(t *testing.T) {
	t.Parallel()

	const runtimeID = "discobot"
	var closed int
	provider := &Provider{
		cfg: &config.Config{WSLDistroName: runtimeID},
	}
	provider.hostDockerClient = &dockerclient.Client{}
	provider.hostDockerClientOnce.Do(func() {})
	provider.hostDockerClientErr = errors.New("stale error")

	originalClose := dockerClientClose
	t.Cleanup(func() {
		dockerClientClose = originalClose
	})
	dockerClientClose = func(cli *dockerclient.Client) error {
		if cli == nil {
			t.Fatal("dockerClientClose() got nil client")
		}
		closed++
		return nil
	}

	if err := provider.StopRuntime(context.Background(), runtimeID); err != nil {
		t.Fatalf("StopRuntime() error = %v", err)
	}
	if closed != 1 {
		t.Fatalf("dockerClientClose() calls = %d, want 1", closed)
	}
	if provider.hostDockerClient != nil {
		t.Fatal("StopRuntime() did not clear cached host Docker client")
	}
	if provider.hostDockerClientErr != nil {
		t.Fatalf("StopRuntime() cached host Docker client error = %v, want nil", provider.hostDockerClientErr)
	}
}

func TestProviderWatchRestartsWhenBridgeDisappears(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var mu sync.Mutex
	firstHost := ""
	breakFirstHost := false
	streams := make(map[string]chan sandbox.StateEvent)
	started := make(chan string, 4)
	ensureRunningCalls := 0

	provider := &Provider{
		cfg:                     &config.Config{WSLDistroName: "discobot"},
		watchBridgePollInterval: 10 * time.Millisecond,
		ensureRunning: func(_ context.Context, progress progressReporter) (*RuntimeInfo, error) {
			mu.Lock()
			defer mu.Unlock()

			ensureRunningCalls++
			host := "tcp://127.0.0.1:23" + strconv.Itoa(700+ensureRunningCalls)
			if firstHost == "" {
				firstHost = host
			}
			progress.Update(100, "Managed WSL distro and Docker bridge are ready")
			return &RuntimeInfo{
				BridgeType:       BridgeTypeTCP,
				BridgePort:       23700 + ensureRunningCalls,
				BridgeDockerHost: host,
				BridgeReady:      true,
			}, nil
		},
		startDockerWatch: func(ctx context.Context, runtimeInfo *RuntimeInfo) (<-chan sandbox.StateEvent, error) {
			ch := make(chan sandbox.StateEvent, 4)
			mu.Lock()
			streams[runtimeInfo.BridgeDockerHost] = ch
			mu.Unlock()
			started <- runtimeInfo.BridgeDockerHost

			go func() {
				<-ctx.Done()
			}()

			return ch, nil
		},
		probeBridgeReady: func(_ context.Context, runtimeInfo *RuntimeInfo) (bool, error) {
			mu.Lock()
			defer mu.Unlock()
			return !breakFirstHost || runtimeInfo.BridgeDockerHost != firstHost, nil
		},
	}

	eventCh, err := provider.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	var initialHost string
	select {
	case initialHost = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial watch startup")
	}

	mu.Lock()
	initialStream := streams[initialHost]
	mu.Unlock()
	initialStream <- sandbox.StateEvent{SessionID: "session-1", Status: sandbox.StatusRunning}

	select {
	case event := <-eventCh:
		if event.SessionID != "session-1" || event.Status != sandbox.StatusRunning {
			t.Fatalf("initial watch event = %#v, want running session-1 event", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial watch event")
	}

	mu.Lock()
	breakFirstHost = true
	mu.Unlock()

	var restartedHost string
	select {
	case restartedHost = <-started:
		if restartedHost == initialHost {
			t.Fatalf("watch restarted with host %q, want a new bridge host", restartedHost)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watch to restart after bridge loss")
	}

	mu.Lock()
	restartedStream := streams[restartedHost]
	breakFirstHost = false
	calls := ensureRunningCalls
	mu.Unlock()

	if calls < 2 {
		t.Fatalf("ensureRunning() calls = %d, want at least 2 after bridge loss", calls)
	}

	restartedStream <- sandbox.StateEvent{SessionID: "session-2", Status: sandbox.StatusStopped}

	select {
	case event := <-eventCh:
		if event.SessionID != "session-2" || event.Status != sandbox.StatusStopped {
			t.Fatalf("restarted watch event = %#v, want stopped session-2 event", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for restarted watch event")
	}
}

func waitForTaskState(t *testing.T, systemManager *startup.SystemManager, taskID string, want startup.TaskState) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, ok := systemManager.GetTask(taskID)
		if ok && task.State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	task, ok := systemManager.GetTask(taskID)
	if !ok {
		t.Fatalf("startup task %q was not registered", taskID)
	}
	t.Fatalf("startup task %q state = %q, want %q", taskID, task.State, want)
}

func assertTaskOperation(t *testing.T, systemManager *startup.SystemManager, taskID string, want string) {
	t.Helper()

	task, ok := systemManager.GetTask(taskID)
	if !ok {
		t.Fatalf("startup task %q was not registered", taskID)
	}
	if task.CurrentOperation != want {
		t.Fatalf("startup task %q current operation = %q, want %q", taskID, task.CurrentOperation, want)
	}
}
