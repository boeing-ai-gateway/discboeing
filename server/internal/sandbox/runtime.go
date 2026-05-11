// Package sandbox provides an abstraction for sandbox execution environments.
// It supports multiple backends including Docker, Kubernetes, and Cloudflare sandboxes.
package sandbox

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Provider abstracts sandbox execution environments (Docker, K8s, Cloudflare, etc.)
// Each session gets one dedicated sandbox, managed through this interface.
type Provider interface {
	// ImageExists checks if the configured sandbox image is available locally.
	// Returns true if the image exists, false if it needs to be pulled.
	ImageExists(ctx context.Context) bool

	// Image returns the configured sandbox image name.
	Image() string

	// List returns all sandboxes managed by discobot.
	// This includes sandboxes in any state (running, stopped, failed).
	List(ctx context.Context) ([]*Sandbox, error)

	// Watch returns a channel that receives sandbox state change events.
	// On subscription, it replays the current state of all existing sandboxes,
	// then streams state changes as they occur.
	//
	// The channel is closed when the context is cancelled or when an
	// unrecoverable error occurs. Callers should watch for channel closure.
	//
	// Events include: created, running, stopped, failed, removed.
	// The "removed" status indicates a sandbox was deleted (possibly externally).
	//
	// For Docker, this watches the Docker events API for container lifecycle events.
	// For VZ, this uses the VM state change notifications.
	Watch(ctx context.Context) (<-chan StateEvent, error)

	// Reconcile performs provider-specific reconciliation on startup.
	// This handles tasks like cleaning up old images, removing outdated
	// infrastructure containers (e.g., BuildKit), and other housekeeping.
	// Called after sandbox image reconciliation completes.
	Reconcile(ctx context.Context) error

	// RemoveProject cleans up all provider-managed resources for a project.
	// This includes cache volumes, BuildKit containers, networks, etc.
	// Called when a project is deleted.
	RemoveProject(ctx context.Context, projectID string) error

	// PrepareState returns opaque provider-specific state for a sandbox before it
	// is created. The service stores non-empty state encrypted and passes it back
	// on subsequent provider calls. Providers that do not need persistent state
	// should return nil.
	PrepareState(ctx context.Context, sessionID string, opts CreateOptions) ([]byte, error)

	// Create creates a new sandbox for the given session.
	// The returned state replaces the input state when it differs.
	Create(ctx context.Context, state []byte, sessionID string, opts CreateOptions) (*Sandbox, []byte, error)

	// Start starts a previously created sandbox and returns updated state.
	Start(ctx context.Context, state []byte, sessionID string) ([]byte, error)

	// Stop stops a running sandbox gracefully and returns updated state.
	Stop(ctx context.Context, state []byte, sessionID string, timeout time.Duration) ([]byte, error)

	// Remove removes a sandbox and optionally its associated data volumes, and
	// returns updated state. Returning nil/empty state clears persisted state.
	Remove(ctx context.Context, state []byte, sessionID string, opts ...RemoveOption) ([]byte, error)

	// Get returns the current state of a sandbox.
	Get(ctx context.Context, state []byte, sessionID string) (*Sandbox, error)

	// GetSecret returns the raw shared secret from provider state/runtime.
	GetSecret(ctx context.Context, state []byte, sessionID string) (string, error)

	// AcquireHTTPClient returns a leased HTTP client configured to communicate
	// with the sandbox.
	AcquireHTTPClient(ctx context.Context, state []byte, sessionID string) (*HTTPClientLease, error)
}

const MetadataImageID = "image_id"

// CurrentImageIDProvider is an optional provider capability that returns the
// immutable image ID for the configured sandbox image.
type CurrentImageIDProvider interface {
	CurrentImageID(ctx context.Context) (string, error)
}

// CleanupUnusedImagesProvider is an optional provider capability for cleaning up
// sandbox images that are no longer referenced after reconciliation.
type CleanupUnusedImagesProvider interface {
	CleanupUnusedImages(ctx context.Context) error
}

// LocalityProvider is an optional provider capability that reports whether a
// provider runs sandboxes on the same host as Discobot. Local providers can use
// developer-built local images; remote providers need a remotely pullable image.
type LocalityProvider interface {
	IsLocal() bool
}

// ProjectResourceInfo describes the effective VM resources for a project.
type ProjectResourceInfo struct {
	Provider   string `json:"provider"`
	CPUCount   int    `json:"cpuCount"`
	MemoryMB   int    `json:"memoryMB"`
	DataDiskGB int    `json:"dataDiskGB"`
}

// UpdateProjectResourcesRequest describes project-scoped VM resource changes.
type UpdateProjectResourcesRequest struct {
	MemoryMB   *int `json:"memoryMB,omitempty"`
	DataDiskGB *int `json:"dataDiskGB,omitempty"`
}

// ProjectResourceManager is an optional provider capability for managing
// project-scoped VM resources such as memory and data disk size.
type ProjectResourceManager interface {
	GetProjectResourceInfo(ctx context.Context, projectID string) (*ProjectResourceInfo, error)
	ApplyProjectResourceUpdate(ctx context.Context, projectID string, req UpdateProjectResourcesRequest) error
}

// ProjectInspectionInfo describes host-inspection container access for a project.
type ProjectInspectionInfo struct {
	Provider      string `json:"provider"`
	Available     bool   `json:"available"`
	ContainerName string `json:"containerName"`
	Scope         string `json:"scope"`
}

// ProjectInspectionManager is an optional provider capability for exposing the
// troubleshooting inspection container for a project.
type ProjectInspectionManager interface {
	GetProjectInspectionInfo(ctx context.Context, projectID string) (*ProjectInspectionInfo, error)
	AttachProjectInspection(ctx context.Context, projectID string, opts AttachOptions) (PTY, error)
}

// ProjectCacheManager is an optional provider capability for clearing
// provider-managed cache for a project.
type ProjectCacheManager interface {
	// ClearCache removes the provider-managed cache for a project.
	// For Docker this deletes the project cache volume and any containers
	// currently attached to it, without deleting any other named volumes.
	ClearCache(ctx context.Context, projectID string) error
}

// ProviderConfigField describes one configurable value accepted by a registered
// sandbox driver. These fields configure provider instances, not individual
// session sandboxes.
type ProviderConfigField struct {
	Key                string `json:"key"`
	Label              string `json:"label"`
	Type               string `json:"type"`
	Description        string `json:"description,omitempty"`
	Placeholder        string `json:"placeholder,omitempty"`
	Required           bool   `json:"required,omitempty"`
	Advanced           bool   `json:"advanced,omitempty"`
	CredentialProvider string `json:"credentialProvider,omitempty"`
	CredentialAuthType string `json:"credentialAuthType,omitempty"`
}

// ProviderDefinition describes a registered sandbox driver/adapter. A driver
// can have many configured provider instances.
type ProviderDefinition struct {
	Name         string                `json:"name,omitempty"`
	Icon         string                `json:"icon,omitempty"`
	Description  string                `json:"description,omitempty"`
	ConfigFields []ProviderConfigField `json:"configFields,omitempty"`
}

// DefinitionProvider is an optional provider capability for reporting driver
// metadata used to configure provider instances.
type DefinitionProvider interface {
	Definition() ProviderDefinition
}

// DockerProxyProvider is an optional interface that sandbox providers can implement
// to expose the Docker daemon for debugging. This is used by the debug Docker proxy
// to forward Docker API requests to the sandbox runtime (e.g., inside a VZ VM).
type DockerProxyProvider interface {
	// DockerTransport returns an http.RoundTripper that communicates with the Docker
	// daemon for the given project. Returns an error if the project VM doesn't exist.
	DockerTransport(projectID string) (http.RoundTripper, error)
}

// ProviderStatus represents the current status of a sandbox provider.
type ProviderStatus struct {
	Available          bool   `json:"available"`
	State              string `json:"state"` // "ready", "downloading", "failed", "not_available"
	Message            string `json:"message,omitempty"`
	SupportsResources  bool   `json:"supportsResources"`
	SupportsInspection bool   `json:"supportsInspection"`
	SupportsClearCache bool   `json:"supportsClearCache"`
	// Details contains provider-specific status information (e.g., download progress, config).
	Details any `json:"details,omitempty"`
}

// StatusProvider is an optional interface that sandbox providers can implement
// to report their status. Providers that don't implement this are assumed ready.
type StatusProvider interface {
	Status() ProviderStatus
}

// RemoveOption configures sandbox removal behavior.
type RemoveOption func(*RemoveConfig)

// RemoveConfig holds the parsed remove options.
type RemoveConfig struct {
	RemoveVolumes bool
}

// RemoveVolumes returns an option that enables volume deletion during removal.
// By default, volumes are preserved. Use this option for session deletion.
func RemoveVolumes() RemoveOption {
	return func(cfg *RemoveConfig) {
		cfg.RemoveVolumes = true
	}
}

// ParseRemoveOptions parses remove options with defaults.
// This is exported for provider implementations to use.
func ParseRemoveOptions(opts []RemoveOption) RemoveConfig {
	cfg := RemoveConfig{
		RemoveVolumes: false, // Default: preserve volumes
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Sandbox represents a running or stopped sandbox instance.
type Sandbox struct {
	ID        string            // Runtime-specific sandbox ID
	SessionID string            // Discobot session ID (1:1 mapping)
	Status    Status            // created, running, stopped, failed
	Image     string            // Sandbox image used
	CreatedAt time.Time         // When the sandbox was created
	StartedAt *time.Time        // When the sandbox was started (nil if never started)
	StoppedAt *time.Time        // When the sandbox was stopped (nil if still running)
	Error     string            // Error message if status == failed
	Metadata  map[string]string // Runtime-specific metadata
	Ports     []AssignedPort    // Assigned port mappings after sandbox creation
	Env       map[string]string // Environment variables set on the sandbox
}

// AssignedPort represents a port mapping that was assigned after sandbox creation.
type AssignedPort struct {
	ContainerPort int    // Port inside the sandbox
	HostPort      int    // Actual port assigned on the host
	HostIP        string // Host IP address (typically "0.0.0.0" or "127.0.0.1")
	Protocol      string // Protocol: "tcp" or "udp"
}

// Status represents the current state of a sandbox.
type Status string

const (
	StatusCreated Status = "created" // Sandbox exists but not started
	StatusRunning Status = "running" // Sandbox is running
	StatusStopped Status = "stopped" // Sandbox has stopped
	StatusFailed  Status = "failed"  // Sandbox failed to start or crashed
)

// StateEvent represents a sandbox state change event.
// These events are emitted when sandboxes are created, started, stopped, or removed.
type StateEvent struct {
	SessionID string    // The session ID associated with the sandbox
	Status    Status    // The new status (or StatusRemoved for deletion)
	Timestamp time.Time // When the event occurred
	Error     string    // Error message if status is StatusFailed
}

// StateEventType indicates what kind of state change occurred.
// This is used to distinguish between a sandbox being removed vs just stopped.
const (
	// StatusRemoved is a pseudo-status indicating the sandbox was deleted.
	// This is only used in StateEvent, not in Sandbox.Status.
	StatusRemoved Status = "removed"
)

// SSHKeyProvision contains SSH key material that should be staged into a sandbox.
// The private key must only be used for runtime provisioning and must never be
// exposed via environment variables.
type SSHKeyProvision struct {
	Filename   string
	PrivateKey string
	PublicKey  string
	Algorithm  string
}

// CreateOptions configures sandbox creation.
// Note: The sandbox image is configured globally via SANDBOX_IMAGE env var,
// not per-sandbox. The provider uses its configured image for all sandboxes.
type CreateOptions struct {
	Labels map[string]string // Sandbox labels/tags for identification

	// SharedSecret is the secret used for authenticating requests to the sandbox.
	// Providers that need the raw secret for host-side authentication metadata can
	// store it, but agent-facing secret env vars are passed through Env.
	SharedSecret string

	// SSHKey contains optional SSH identity material to provision into the sandbox.
	SSHKey *SSHKeyProvision

	// Env contains environment variables providers must set on the created
	// sandbox instance.
	Env map[string]string

	// WorkspacePath is an optional local directory to mount inside the sandbox at
	// /.workspace. Local workspaces use this; git URL workspaces may leave it empty
	// and let the agent clone from WorkspaceSource instead.
	WorkspacePath string

	// WorkspaceSource is the original workspace source (local path or git URL).
	// For local workspaces, this is the local directory path.
	// For git workspaces, this is the git URL (e.g., https://github.com/user/repo.git).
	WorkspaceSource string

	// WorkspaceCommit is an optional workspace commit to check out while bootstrapping
	// the sandbox repository. It is a sandbox creation input, not persisted session
	// commit state.
	WorkspaceCommit string

	// WorkspaceTargetRef is the git ref the sandbox should clone or check out when
	// bootstrapping a git URL workspace. Examples: HEAD, main, refs/heads/main.
	WorkspaceTargetRef string

	// ProjectID is the project this session belongs to.
	ProjectID string

	// MCPOAuthRedirectBase is the base URL for MCP OAuth callbacks.
	MCPOAuthRedirectBase string

	// AgentServerURL is the URL the agent uses to reach the Discobot server
	// (e.g. for posting MCP tokens after OAuth).
	AgentServerURL string

	// Resources defines resource limits for the sandbox.
	Resources ResourceConfig
}

// ResourceConfig defines resource limits for the sandbox.
type ResourceConfig struct {
	MemoryMB int           // Memory limit in MB (0 = no limit)
	CPUCores float64       // CPU cores (0 = no limit)
	DiskMB   int           // Disk space in MB (0 = no limit)
	Timeout  time.Duration // Max sandbox lifetime (0 = no limit)
}

// AttachOptions configures interactive PTY session creation.
type AttachOptions struct {
	Cmd     []string          // Command to run (empty = default shell)
	Rows    int               // Terminal rows
	Cols    int               // Terminal columns
	WorkDir string            // Working directory for command
	Env     map[string]string // Additional environment variables
	User    string            // User to run as (empty = default sandbox user)
}

// PTY represents an interactive terminal session to a sandbox.
// It implements io.ReadWriteCloser for terminal I/O.
type PTY interface {
	// Read reads output from the PTY.
	// Implements io.Reader.
	Read(p []byte) (n int, err error)

	// Write sends input to the PTY.
	// Implements io.Writer.
	Write(p []byte) (n int, err error)

	// Resize changes the terminal dimensions.
	Resize(ctx context.Context, rows, cols int) error

	// Close terminates the PTY session.
	// Implements io.Closer.
	Close() error

	// Wait blocks until the PTY command exits and returns the exit code.
	// The context can be used to cancel the wait.
	Wait(ctx context.Context) (int, error)
}

// ExecStreamOptions configures streaming command execution.
type ExecStreamOptions struct {
	WorkDir string            // Working directory for command
	Env     map[string]string // Additional environment variables
	User    string            // User to run as (empty = default)
	TTY     bool              // Allocate a pseudo-terminal for the command
}

// Stream represents a bidirectional stream to a command.
// When created with TTY=true, stdout and stderr are merged and
// Resize can be used to change terminal dimensions.
// This is used for SFTP, port forwarding, and exec.
type Stream interface {
	// Read reads stdout from the command.
	// Implements io.Reader.
	Read(p []byte) (n int, err error)

	// Stderr returns a reader for the command's stderr.
	// Returns nil if stderr is not available (e.g., TTY mode merges streams).
	Stderr() io.Reader

	// Write sends input to the command's stdin.
	// Implements io.Writer.
	Write(p []byte) (n int, err error)

	// Resize changes the terminal dimensions.
	// Only effective when the stream was created with TTY=true.
	Resize(ctx context.Context, rows, cols int) error

	// CloseWrite signals EOF to the command's stdin.
	// The stream can still be read after calling this.
	CloseWrite() error

	// Close terminates the stream and the command.
	// Implements io.Closer.
	Close() error

	// Wait blocks until the command exits and returns the exit code.
	// The context can be used to cancel the wait.
	Wait(ctx context.Context) (int, error)
}
