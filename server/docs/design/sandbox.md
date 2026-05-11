# Sandbox Module

This module provides the sandbox runtime abstraction for managing execution environments (Docker containers, VMs, or hybrid).

## Files

| File                                       | Description                                              |
| ------------------------------------------ | -------------------------------------------------------- |
| `internal/sandbox/runtime.go`              | Provider interface definition                            |
| `internal/sandbox/idle_runtime_monitor.go` | Shared idle host-runtime shutdown monitor                |
| `internal/sandbox/errors.go`               | Error types                                              |
| `internal/sandbox/manager.go`              | Provider manager and proxy                               |
| `internal/sandbox/docker/provider.go`      | Docker implementation                                    |
| `internal/sandbox/docker/cache.go`         | Cache volume management                                  |
| `internal/sandbox/exedev/provider.go`      | exe.dev VM implementation                                |
| `internal/sandbox/vm/manager.go`           | VM abstraction layer (interfaces for VZ, KVM, WSL2)      |
| `internal/sandbox/vz/vz_vm_manager.go`     | Apple Virtualization.framework VM manager (macOS)        |
| `internal/sandbox/vz/vz_docker.go`         | Hybrid provider: VZ VMs with Docker containers (macOS)   |
| `internal/sandbox/vz/vsock.go`             | VSOCK communication types                                |
| `internal/sandbox/wsl/`                    | Windows WSL2 provider and lifecycle management           |
| `server/docs/design/wsl2-sandbox-plan.md`  | Working implementation plan for the WSL2 sandbox backend |
| `internal/sandbox/vz/provider_stub.go`     | Stub for non-darwin platforms                            |
| `internal/sandbox/local/provider.go`       | Local process provider (development)                     |
| `internal/sandbox/mock/provider.go`        | Mock implementation for testing                          |

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                       Sandbox Abstraction                              │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │                     Provider Interface                           │  │
│  │   Create, Start, Stop, Remove, Get, List, Exec, Attach, Watch   │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│                                │                                        │
│      ┌─────────────────────────┼────────────────────────┐             │
│      ▼                         ▼                         ▼             │
│  ┌──────────┐          ┌──────────────┐          ┌──────────┐         │
│  │  Docker  │          │   VZ+Docker  │          │   Mock   │         │
│  │ Provider │          │   Provider   │          │ Provider │         │
│  └──────────┘          │   (darwin)   │          └──────────┘         │
│      │                 └──────┬───────┘                 │              │
│      │                        │                         │              │
│      │                        ▼                         │              │
│      │              ┌─────────────────────┐            │              │
│      │              │  VM Abstraction     │            │              │
│      │              │  (vm.Manager)       │            │              │
│      │              └──────────┬──────────┘            │              │
│      │                         │                        │              │
│      │           ┌─────────────┼─────────────┐         │              │
│      │           ▼             ▼             ▼         │              │
│      │      ┌────────┐    ┌────────┐    ┌────────┐    │              │
│      │      │   VZ   │    │  KVM   │    │  WSL2  │    │              │
│      │      │ (macOS)│    │(Linux) │    │(Win11) │    │              │
│      │      └────┬───┘    └────────┘    └────────┘    │              │
│      │           │                                      │              │
│      ▼           ▼                                      ▼              │
│  Docker API   VSOCK → Docker                    In-Memory State       │
│  (local)      (VM-hosted Docker)                                      │
└────────────────────────────────────────────────────────────────────────┘
```

### Provider Types

Provider types are the runtime capabilities known to the sandbox manager, such
as `docker`, `vz`, `wsl`, `local`, and `exedev`. Some types are registered as
process-wide runtime providers, while others may be registered only as
definitions that can be instantiated from project configuration. User
configuration is represented separately as sandbox provider instances stored in
`sandbox_provider_instances`. Instance `config` contains only non-secret
settings; infrastructure secrets such as an exe.dev API key live in the normal
credential store and are referenced from config by credential ID. New sessions
may persist `sessions.sandbox_provider_id`; routing resolves that ID to either
a built-in provider type or a configured instance type. Sessions without a
provider ID use the project default provider when `projects.default_sandbox_provider_id`
is set, otherwise they fall back to the process-wide default selected by the
sandbox manager (`SANDBOX_PROVIDER` or the platform default). Workspaces do not
own sandbox provider selection.

Provider definitions expose config field metadata. Credential fields may include
the expected credential provider and auth type, such as `exedev` plus `api_key`,
so the UI can filter credential choices and create correctly scoped credentials
inline.

Built-in provider rows may also be stored in `sandbox_provider_instances` as
project-level overrides. These rows use the provider type as the ID, set
`built_in=true`, and can set `disabled=true` to hide that built-in provider from
new session selection and prevent project sessions from routing to it.

Provider icons are returned as UI display metadata. Built-in provider types have
hard-coded defaults; custom provider instances may set `config.icon` through the
API's top-level `icon` request field. Icon values may be `simple-icons:<name>`,
an inline SVG string, a data URL, or an external image URL.

Operational controls are provider capabilities, not global project features.
Provider type and instance responses expose `capabilities.resources`,
`capabilities.inspection`, and `capabilities.clearCache` so the UI can show only
supported controls. Provider-scoped routes resolve either a built-in provider ID
or a configured provider instance ID before calling the runtime provider:

- `GET /api/projects/{projectId}/sandbox-providers/{providerId}/resources`
- `PATCH /api/projects/{projectId}/sandbox-providers/{providerId}/resources`
- `GET /api/projects/{projectId}/sandbox-providers/{providerId}/inspection`
- `GET /api/projects/{projectId}/sandbox-providers/{providerId}/inspection/terminal/ws`

The legacy project-level resources and inspection endpoints remain compatibility
wrappers around the default provider.

1. **Docker Provider**: Standard Docker containers (cross-platform)
   - Direct connection to Docker daemon
   - One container per session
   - Fast startup, good for Linux/Windows
   - Starts a daemon-scoped privileged `discobot-host-inspect` container
     from the sandbox image for host troubleshooting

2. **VZ+Docker Provider**: VM isolation with container efficiency (macOS only)
   - One VM per project (shared across sessions)
   - One Docker container per session inside the VM
   - VSOCK communication for Docker API access
   - Automatic VM lifecycle management with idle timeout
   - Best resource efficiency on macOS
   - Starts the same privileged inspection container inside each project VM's
     Docker daemon after the sandbox image is available

3. **VM Abstraction Layer**: Platform-agnostic VM management
   - Interface-based design supporting multiple hypervisors
   - VZ implementation (macOS) - Apple Virtualization.framework
   - Future: KVM implementation (Linux)
   - WSL2 is implemented as a dedicated Windows provider layered on top of a
     managed shared distro rather than the current per-project VM abstraction
   - The WSL2 provider creates its persistent `/var` VHDX with the native
     Windows Virtual Disk API and attaches it with `wsl.exe --mount`, so it
     does not depend on Hyper-V PowerShell modules such as `New-VHD`
   - Project-level VMs with session reference counting
   - Configurable console logging and resource allocation

4. **Shared Idle Runtime Monitor**: Generic host-runtime shutdown logic
   - Separate from the session-scoped sandbox `Provider` interface
   - Watches shared runtimes such as project VMs or the managed WSL distro
   - Stops a runtime only after it has no running Discobot sandboxes for the configured idle period

5. **Local Provider**: Direct process execution (development only)
   - No container/VM overhead
   - Runs agent-api as local process
   - Not recommended for production

6. **exe.dev Provider**: Remote exe.dev VMs (opt-in)
   - Uses the exe.dev HTTPS command endpoint (`POST /exec`) for VM lifecycle
   - Creates one exe.dev VM per Discobot session
   - Routes sandbox agent HTTP traffic through the VM's `*.exe.xyz` hostname
   - Enable by creating an exe.dev sandbox provider instance that references an
     exe.dev API credential

Non-local providers use `SANDBOX_IMAGE_REMOTE` by default so remote runtimes can
pull a published image even when local development uses a local-only
`SANDBOX_IMAGE`. Provider implementations can report locality through the
optional `sandbox.LocalityProvider` capability. Docker is local when
`DOCKER_HOST` is empty or points at a local socket (`unix://`, `npipe://`, or
`fd://`), and remote when it is explicitly configured with a remote host such as
`tcp://...` or `ssh://...`. Provider instances can override their configured
image directly when they need something other than the locality-based default.

7. **Mock Provider**: In-memory testing
   - No real sandboxes created
   - Used for unit tests

## Provider Interface

```go
type Provider interface {
    // Create a new sandbox
    Create(ctx context.Context, sessionID string, opts Options) (*Sandbox, error)

    // Start a sandbox
    Start(ctx context.Context, sessionID string) error

    // Stop a sandbox
    Stop(ctx context.Context, sessionID string, timeout time.Duration) error

    // Remove a sandbox and optionally its data volumes
    // Pass sandbox.RemoveVolumes() to delete volumes
    Remove(ctx context.Context, sessionID string, opts ...RemoveOption) error

    // Get sandbox info
    Get(ctx context.Context, sessionID string) (*Sandbox, error)

    // List all sandboxes
    List(ctx context.Context) ([]*Sandbox, error)

    // Execute command in sandbox
    Exec(ctx context.Context, sessionID string, cmd []string, opts ExecOptions) (*ExecResult, error)

    // Attach to sandbox (PTY)
    Attach(ctx context.Context, sessionID string, opts AttachOptions) (PTY, error)
}
```

## Types

### Sandbox

```go
type Sandbox struct {
    ID        string            // Docker container ID
    SessionID string            // Discobot session ID
    Status    string            // created, running, stopped
    Address   string            // HTTP address (host:port)
    Labels    map[string]string
    CreatedAt time.Time
}
```

### Options

```go
type Options struct {
    Image       string            // Container image
    Cmd         []string          // Command to run
    Env         []string          // Environment variables
    Binds       []string          // Volume mounts
    NetworkMode string            // Docker network
    Labels      map[string]string // Container labels
    PortBindings map[string]string // Port mappings
}
```

### ExecOptions

```go
type ExecOptions struct {
    Env        []string // Additional environment
    WorkingDir string   // Working directory
    Tty        bool     // Allocate TTY
}
```

### AttachOptions

```go
type AttachOptions struct {
    Cmd  []string // Command to run (empty = auto-detect shell)
    Rows int      // Terminal rows
    Cols int      // Terminal columns
    Env  map[string]string // Environment variables
    User string   // User to run as (empty = sandbox default, "root" = root, or "UID:GID")
}
```

### ExecResult

```go
type ExecResult struct {
    ExitCode int
    Stdout   string
    Stderr   string
}
```

### PTY Interface

```go
type PTY interface {
    io.ReadWriteCloser
    Resize(height, width uint) error
}
```

## Docker Provider

### Implementation

```go
type Provider struct {
    client *client.Client
    config *config.Config
}

func NewProvider(cfg *config.Config) (*Provider, error) {
    cli, err := client.NewClientWithOpts(
        client.FromEnv,
        client.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return nil, err
    }

    return &Provider{
        client: cli,
        config: cfg,
    }, nil
}
```

### Sandbox Naming

```go
func (p *Provider) sandboxName(sessionID string) string {
    return fmt.Sprintf("discobot-session-%s", sessionID)
}
```

### Create

```go
func (p *Provider) Create(
    ctx context.Context,
    sessionID string,
    opts Options,
) (*Sandbox, error) {
    name := p.sandboxName(sessionID)

    // Container config
    containerConfig := &dockercontainer.Config{
        Image: opts.Image,
        Cmd:   opts.Cmd,
        Env:   opts.Env,
        Labels: map[string]string{
            "discobot.session": sessionID,
        },
        ExposedPorts: nat.PortSet{
            "3002/tcp": struct{}{},
        },
    }

    // Host config
    hostConfig := &dockercontainer.HostConfig{
        Binds:       opts.Binds,
        NetworkMode: dockercontainer.NetworkMode(opts.NetworkMode),
        PortBindings: nat.PortMap{
            "3002/tcp": []nat.PortBinding{
                {HostIP: "127.0.0.1", HostPort: "0"}, // Random port
            },
        },
    }

    // Create container
    resp, err := p.client.ContainerCreate(
        ctx,
        containerConfig,
        hostConfig,
        nil, nil,
        name,
    )
    if err != nil {
        return nil, err
    }

    return &Sandbox{
        ID:        resp.ID,
        SessionID: sessionID,
        Status:    "created",
    }, nil
}
```

### Start

```go
func (p *Provider) Start(ctx context.Context, sessionID string) error {
    name := p.sandboxName(sessionID)
    return p.client.ContainerStart(ctx, name, dockercontainer.StartOptions{})
}
```

### Get with Address

```go
func (p *Provider) Get(ctx context.Context, sessionID string) (*Sandbox, error) {
    name := p.sandboxName(sessionID)

    info, err := p.client.ContainerInspect(ctx, name)
    if err != nil {
        return nil, err
    }

    // Get assigned port
    bindings := info.NetworkSettings.Ports["3002/tcp"]
    address := ""
    if len(bindings) > 0 {
        address = fmt.Sprintf("http://127.0.0.1:%s", bindings[0].HostPort)
    }

    return &Sandbox{
        ID:        info.ID,
        SessionID: sessionID,
        Status:    info.State.Status,
        Address:   address,
        CreatedAt: info.Created,
    }, nil
}
```

### Attach (PTY)

Creates an interactive PTY session with automatic shell detection:

```go
func (p *Provider) Attach(
    ctx context.Context,
    sessionID string,
    opts AttachOptions,
) (PTY, error) {
    name := p.sandboxName(sessionID)

    // Detect shell if not specified
    cmd := opts.Cmd
    if len(cmd) == 0 {
        cmd = p.detectShell(ctx, name)
    }

    // Create exec with PTY
    execConfig := container.ExecOptions{
        Cmd:          cmd,
        User:         opts.User,  // "root", "UID:GID", or empty for default
        Tty:          true,
        AttachStdin:  true,
        AttachStdout: true,
        AttachStderr: true,
    }
    // ... create exec and attach
}
```

#### Shell Detection

When no command is specified, the provider detects the appropriate shell:

```go
func (p *Provider) detectShell(ctx context.Context, containerID string) []string {
    // 1. Try $SHELL environment variable
    result, err := p.execSimple(ctx, containerID, []string{"sh", "-c", "echo $SHELL"})
    if err == nil && result != "" && result != "/bin/false" {
        return []string{result}
    }

    // 2. Try /bin/bash
    if p.commandExists(ctx, containerID, "/bin/bash") {
        return []string{"/bin/bash"}
    }

    // 3. Fall back to /bin/sh
    return []string{"/bin/sh"}
}
```

This ensures the terminal uses the user's preferred shell when available.

### Exec

```go
func (p *Provider) Exec(
    ctx context.Context,
    sessionID string,
    cmd []string,
    opts ExecOptions,
) (*ExecResult, error) {
    name := p.sandboxName(sessionID)

    execConfig := container.ExecOptions{
        Cmd:          cmd,
        AttachStdout: true,
        AttachStderr: true,
        Env:          opts.Env,
        WorkingDir:   opts.WorkingDir,
    }

    execID, err := p.client.ContainerExecCreate(ctx, name, execConfig)
    if err != nil {
        return nil, err
    }

    resp, err := p.client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
    if err != nil {
        return nil, err
    }
    defer resp.Close()

    // Read output
    var stdout, stderr bytes.Buffer
    _, _ = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)

    // Get exit code
    inspect, _ := p.client.ContainerExecInspect(ctx, execID.ID)

    return &ExecResult{
        ExitCode: inspect.ExitCode,
        Stdout:   stdout.String(),
        Stderr:   stderr.String(),
    }, nil
}
```

## VZ+Docker Hybrid Provider (macOS)

The VZ+Docker provider combines Apple Virtualization framework VMs with Docker containers for optimal resource efficiency on macOS. It uses the VM abstraction layer (`vm.ProjectVMManager` interface) to provide platform-agnostic VM management.

### Architecture

```
Project 1                                Project 2
┌─────────────────────────────┐         ┌─────────────────────────────┐
│   Linux VM (Project-level)  │         │   Linux VM (Project-level)  │
│                              │         │                              │
│  ┌───────────────────────┐  │         │  ┌───────────────────────┐  │
│  │   Docker Daemon       │  │         │  │   Docker Daemon       │  │
│  └──────┬────────────────┘  │         │  └──────┬────────────────┘  │
│         │                    │         │         │                    │
│    ┌────┴────┐  ┌─────────┐ │         │    ┌────┴────┐              │
│    │Container│  │Container│ │         │    │Container│              │
│    │Session 1│  │Session 2│ │         │    │Session 3│              │
│    └─────────┘  └─────────┘ │         │    └─────────┘              │
│         ▲            ▲       │         │         ▲                    │
└─────────┼────────────┼───────┘         └─────────┼────────────────────┘
          │            │                           │
          │            │                           │
          └────────────┴───────────────────────────┘
                       │
                    VSOCK:2375
                       │
              ┌────────▼────────┐
              │  VzDockerProvider│
              │   (Host Process) │
              └──────────────────┘
```

### Key Features

1. **Project-Level VMs**: One VM per project, shared by all sessions
2. **Session-Level Containers**: Each session gets its own Docker container inside the VM
3. **VSOCK Communication**: Docker API accessed via VSOCK (port 2375)
4. **Automatic Lifecycle**:
   - VMs created on-demand when first session starts
   - VMs stay alive while sessions exist
   - VMs automatically shut down after 30 minutes of inactivity
5. **Resource Efficiency**: Better than one VM per session

### Implementation

```go
type VzDockerProvider struct {
    vmManager        vm.ProjectVMManager          // VM abstraction interface
    dockerProviders  map[string]*docker.Provider  // projectID -> Docker provider
    sessionToProject map[string]string            // sessionID -> projectID
}

func NewProvider(cfg *config.Config, vmConfig *vm.Config) (*VzDockerProvider, error) {
    // Create VZ VM manager (implements vm.ProjectVMManager)
    vmManager, err := vz.NewVMManager(*vmConfig)
    if err != nil {
        return nil, err
    }

    return &VzDockerProvider{
        vmManager:        vmManager,
        dockerProviders:  make(map[string]*docker.Provider),
        sessionToProject: make(map[string]string),
    }, nil
}
```

The provider uses the `vm.ProjectVMManager` interface, making it easy to swap VZ for KVM or WSL2 in the future.

### Create Flow

1. **Get/Create Project VM**:

   ```go
   pvm, err := p.vmManager.GetOrCreateVM(ctx, opts.ProjectID, sessionID)
   ```

   - Creates new VM if first session in project
   - Reuses existing VM if project already has one
   - Waits for Docker daemon to be ready (max 60s)
   - VM manager handles console logging, resource allocation, and lifecycle

2. **Get/Create Docker Provider**:

   ```go
   dockerProv, err := p.getOrCreateDockerProvider(opts.ProjectID, pvm)
   ```

   - Creates Docker client with VSOCK transport
   - VSOCK dialer connects to port 2375 in the VM
   - Docker client communicates as if local

3. **Create Container**:
   ```go
   sb, err := dockerProv.Create(ctx, sessionID, opts)
   ```

   - Standard Docker container creation
   - Container runs inside the project VM
   - Isolated from other sessions via Docker

### VSOCK Communication

The Docker daemon inside the VM exposes its socket via VSOCK:

```bash
# Inside VM init script
socat VSOCK-LISTEN:2375,reuseaddr,fork UNIX-CONNECT:/var/run/docker.sock &
```

The host connects via VSOCK dialer:

```go
func (pvm *projectVM) DockerDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
    return func(ctx context.Context, network, addr string) (net.Conn, error) {
        conn, err := pvm.socketDevice.Connect(dockerSockPort)
        if err != nil {
            return nil, err
        }
        return &vsockConn{
            VirtioSocketConnection: conn,
            localAddr:  &vsockAddr{cid: 2, port: 0},
            remoteAddr: &vsockAddr{cid: 3, port: dockerSockPort},
        }, nil
    }
}
```

### VM Base Image Requirements

The base disk image must include:

- Linux kernel with virtio drivers (vsock, net, blk)
- Docker daemon
- socat for VSOCK-to-socket bridging
- Init script that:
  1. Starts Docker daemon
  2. Starts socat bridge: `socat VSOCK-LISTEN:2375 → /var/run/docker.sock`

See [VZ README](../../internal/sandbox/vz/README.md) for complete image requirements.

### Configuration

```go
// Configure VM settings
vmConfig := &vm.Config{
    DataDir:       "/var/lib/discobot/vz",
    ConsoleLogDir: "/var/log/discobot/vz",
    KernelPath:    "/path/to/vmlinuz",
    InitrdPath:    "/path/to/initrd.img",
    BaseDiskPath:  "/path/to/base.img",
    IdleTimeout:   "30m",
    CPUCount:      2,
    MemoryMB:      2048,
}

// Create VZ+Docker provider
provider, err := vz.NewProvider(cfg, vmConfig)

// Create sandbox (requires ProjectID)
sandbox, err := provider.Create(ctx, sessionID, sandbox.CreateOptions{
    ProjectID:       "project-abc123",  // Required
    SharedSecret:    "secret",
    WorkspacePath:   "/path/to/workspace",
    WorkspaceSource: "https://github.com/user/repo.git",
})
```

**Important**: `ProjectID` is required for VZ+Docker provider. Sessions with the same `ProjectID` will share a VM.

### Platform-Specific Build

The VZ provider uses build tags to ensure it only compiles on macOS:

```go
//go:build darwin

// vz_vm_manager.go, vz_docker.go, vsock.go
```

A stub implementation (`provider_stub.go`) returns errors on non-darwin platforms:

```go
//go:build !darwin

func NewProvider(_ *config.Config, _ *vm.Config) (*VzDockerProvider, error) {
    return nil, fmt.Errorf("vz sandbox provider is only available on macOS (darwin), current platform: %s", runtime.GOOS)
}
```

This allows the main server code to reference the VZ provider on all platforms without compilation errors.

## Mock Provider

### Implementation

```go
type MockProvider struct {
    sandboxes map[string]*Sandbox
    mu        sync.RWMutex
}

func NewMockProvider() *MockProvider {
    return &MockProvider{
        sandboxes: make(map[string]*Sandbox),
    }
}

func (m *MockProvider) Create(
    ctx context.Context,
    sessionID string,
    opts Options,
) (*Sandbox, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    s := &Sandbox{
        ID:        uuid.New().String(),
        SessionID: sessionID,
        Status:    "created",
        Address:   "http://mock:3002",
        CreatedAt: time.Now(),
    }

    m.sandboxes[sessionID] = s
    return s, nil
}
```

## Error Types

```go
var (
    ErrNotFound = errors.New("sandbox not found")
    ErrStopped  = errors.New("sandbox is stopped")
    ErrExecFailed = errors.New("exec failed")
)
```

## Sandbox Labels

Labels are used to identify Discobot sandboxes:

```go
labels := map[string]string{
    "discobot":         "true",
    "discobot.session": sessionID,
    "discobot.project": projectID,
}
```

## Container Removal with Optional Volume Cleanup

The sandbox provider's `Remove()` method accepts optional `RemoveOption` parameters:

### Default behavior (no options)

- **Purpose**: Remove container for rebuild scenarios (e.g., image updates)
- **Behavior**: Deletes the container but preserves data volumes
- **Use case**: Image reconciliation, container recreation, failed container recovery
- **Docker**: Removes container only, leaves `discobot-data-{sessionID}` volume intact
- **VZ**: Removes VM (always removes disk)

```go
// Used during sandbox reconciliation to rebuild outdated containers
// No options = preserves volumes by default
if err := provider.Remove(ctx, sessionID); err != nil {
    return err
}
```

### With sandbox.RemoveVolumes() option

- **Purpose**: Complete cleanup when deleting a session or explicitly purging retained data
- **Behavior**: Deletes both container and all associated data volumes
- **Use case**: Immediate permanent cleanup, explicit cleanup jobs
- **Docker**: Removes container AND explicitly deletes the `discobot-data-{sessionID}` volume
- **VZ**: Removes VM and all associated storage (same as default)

```go
// Used during session deletion to clean up all resources
// Pass sandbox.RemoveVolumes() to delete volumes
if err := provider.Remove(ctx, sessionID, sandbox.RemoveVolumes()); err != nil {
    return err
}
```

### Session Deletion Retention Window

When a session is deleted, Discobot stops the sandbox immediately but keeps the sandbox and its data for a configurable recovery window before final cleanup. The default is **1 minute**, controlled by `SESSION_SANDBOX_CLEANUP_DELAY`.

- The session record is removed right away, but the stopped sandbox is retained during the recovery window.
- A delayed background job later removes the sandbox with `sandbox.RemoveVolumes()`, which performs the provider's normal full cleanup.
- If a session with the same ID exists again before the delayed job runs, the job skips deletion.
- Startup reconciliation preserves retained orphaned sandboxes while their delayed delete job is still pending.

### Docker Volume Management

Docker containers use named data volumes for persistent storage:

```go
// Volume naming
dataVolName := fmt.Sprintf("discobot-data-%s", sessionID)

// Volume is mounted at /.data inside container
Mounts: []mount.Mount{
    {
        Type:   mount.TypeVolume,
        Source: dataVolName,
        Target: "/.data",
    },
}
```

**Important**: Docker's `RemoveVolumes: true` flag only removes anonymous volumes, not named volumes. Named volumes must be explicitly deleted with `VolumeRemove()`.

## Sandbox Reconciliation

On server startup, sandbox reconciliation does more than prune orphaned containers:

1. List all managed sandboxes.
2. Compare each sandbox against the configured sandbox image.
   - For Docker, this comparison uses the resolved image ID when available, not just the configured image ref string.
3. For sandboxes using an outdated image:
   - running sandboxes are recreated with the new image while preserving their named data volumes.
   - stopped or never-started sandboxes are removed but not restarted; they are recreated later on demand.
4. After sandbox migration completes, clean up labeled sandbox images that are no longer referenced by either:
   - the currently configured sandbox image, or
   - any managed sandbox container.

Cleanup happens **after** reconciliation so startup does not race with image pulls or delete images that are still needed by existing sandboxes.

## Testing

```go
func TestDockerProvider_Create(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping Docker test in short mode")
    }

    provider, err := NewProvider(&config.Config{})
    require.NoError(t, err)

    ctx := context.Background()
    sessionID := uuid.New().String()

    sb, err := provider.Create(ctx, sessionID, Options{
        Image: "alpine:latest",
        Cmd:   []string{"sleep", "30"},
    })
    require.NoError(t, err)
    defer provider.Remove(ctx, sessionID)

    assert.NotEmpty(t, sb.ID)
    assert.Equal(t, "created", sb.Status)
}
```
