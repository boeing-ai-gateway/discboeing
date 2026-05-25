package processes

import (
	"context"
	"errors"
	"io"
	"time"
)

// Kind classifies why a process session exists.
type Kind string

const (
	// KindUserExec is an interactive or one-off user exec session.
	KindUserExec Kind = "user"
	// KindService is a long-running workspace service process.
	KindService Kind = "service"
	// KindHook is a workspace hook process.
	KindHook Kind = "hook"
)

// Status is the lifecycle state of a process session.
type Status string

const (
	// StatusStarting means the session is being created but is not yet usable.
	StatusStarting Status = "starting"
	// StatusRunning means the process has started and can be attached to.
	StatusRunning Status = "running"
	// StatusExited means the process ended without an explicit manager kill.
	StatusExited Status = "exited"
	// StatusKilling means the manager has requested process termination.
	StatusKilling Status = "killing"
	// StatusKilled means the process exited after an explicit manager kill.
	StatusKilled Status = "killed"
	// StatusFailed means the manager could not start or supervise the process.
	StatusFailed Status = "failed"
)

// CreateRequest describes a process session to start or reuse.
type CreateRequest struct {
	// Kind separates user exec sessions from service sessions.
	Kind Kind `json:"kind"`
	// Name is an optional human-readable label for UI and logs.
	Name string `json:"name,omitempty"`
	// ReuseKey is an optional stable key for singleton-like sessions. If a
	// running session with the same key already exists, Start returns that
	// session instead of creating another process. The terminal uses this to
	// reconnect to the same shell across browser WebSocket reconnects.
	ReuseKey string `json:"reuseKey,omitempty"`
	// Cmd is the argv to execute. If empty, the platform default shell is used.
	Cmd []string `json:"cmd,omitempty"`
	// WorkDir is the process working directory. If empty, the manager default is used.
	WorkDir string `json:"workDir,omitempty"`
	// HomeDir starts the process in the requested user's home directory.
	HomeDir bool `json:"homeDir,omitempty"`
	// Env contains environment variable overrides added to the process environment.
	Env map[string]string `json:"env,omitempty"`
	// User requests the OS user to run as. Unsupported user switches fail clearly.
	User string `json:"user,omitempty"`
	// TTY requests a pseudo-terminal with stdin/stdout/stderr attached to it.
	TTY bool `json:"tty,omitempty"`
	// Rows is the initial terminal height for TTY sessions.
	Rows int `json:"rows,omitempty"`
	// Cols is the initial terminal width for TTY sessions.
	Cols int `json:"cols,omitempty"`
	// Metadata stores caller-defined key/value data, such as a service ID.
	Metadata map[string]string `json:"metadata,omitempty"`
	// LogDir overrides the directory used for process sidecar log files.
	LogDir string `json:"logDir,omitempty"`
	// LogPath overrides the combined output log path.
	LogPath string `json:"logPath,omitempty"`
}

// Session is the persisted and API-visible state for a managed process.
type Session struct {
	// ID is the manager-assigned unique session identifier.
	ID string `json:"id"`
	// Kind separates user exec sessions from service sessions.
	Kind Kind `json:"kind"`
	// Name is an optional human-readable label for UI and logs.
	Name string `json:"name,omitempty"`
	// ReuseKey is the stable key used to return an existing running session
	// instead of starting a duplicate process.
	ReuseKey string `json:"reuseKey,omitempty"`
	// Cmd is the argv that was started.
	Cmd []string `json:"cmd,omitempty"`
	// WorkDir is the process working directory.
	WorkDir string `json:"workDir,omitempty"`
	// User is the requested OS user, if any.
	User string `json:"user,omitempty"`
	// TTY records whether the session was started with a pseudo-terminal.
	TTY bool `json:"tty"`
	// Status is the current lifecycle state.
	Status Status `json:"status"`
	// PID is the root process ID for the session.
	PID int `json:"pid,omitempty"`
	// PGID is the process group ID used for tree cleanup on Unix platforms.
	PGID int `json:"pgid,omitempty"`
	// ExitCode is set after the process exits or is killed.
	ExitCode *int `json:"exitCode,omitempty"`
	// StartedAt is when the process successfully started.
	StartedAt time.Time `json:"startedAt"`
	// ExitedAt is set when the manager observes process exit.
	ExitedAt *time.Time `json:"exitedAt,omitempty"`
	// LogPath is the combined output log path for this session.
	LogPath string `json:"logPath,omitempty"`
	// Metadata stores caller-defined key/value data, such as a service ID.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OutputEvent is one persisted or streamed process output/lifecycle event.
type OutputEvent struct {
	// Seq is the monotonically increasing session-local event number.
	Seq int64 `json:"seq"`
	// Type is stdout, stderr, output, exit, or error.
	Type string `json:"type"`
	// Data contains output bytes decoded as a string for stdout/stderr/output.
	Data string `json:"data,omitempty"`
	// ExitCode is populated for exit events.
	ExitCode *int `json:"exitCode,omitempty"`
	// Error is populated for error events.
	Error string `json:"error,omitempty"`
	// Timestamp is when the manager emitted the event.
	Timestamp time.Time `json:"timestamp"`
}

// EventQuery filters a session's historical event log.
type EventQuery struct {
	// Limit caps the number of historical events returned. With no After or
	// Since filter, the latest Limit events are returned.
	Limit int
	// After returns events with Seq greater than this value.
	After *int64
	// Since returns events emitted after this timestamp.
	Since *time.Time
}

// Capabilities describes the platform process features available at runtime.
type Capabilities struct {
	// Platform is the Go runtime OS, such as linux, darwin, or windows.
	Platform string `json:"platform"`
	// Runtime names the supervisor implementation, such as process-group.
	Runtime string `json:"runtime"`
	// TTY reports whether pseudo-terminal sessions are supported.
	TTY bool `json:"tty"`
	// Resize reports whether TTY resize is supported.
	Resize bool `json:"resize"`
	// ProcessTreeKill reports whether killing a session cleans up children.
	ProcessTreeKill bool `json:"processTreeKill"`
	// UserSwitching reports whether running as another OS user is supported.
	UserSwitching bool `json:"userSwitching"`
	// SupportedUsers optionally lists known users the runtime can switch to.
	SupportedUsers []string `json:"supportedUsers"`
}

// Stream is the live I/O attachment to a managed process session.
type Stream interface {
	io.Reader
	io.Writer
	// Stderr returns the stderr reader for non-TTY sessions. It returns nil for TTY sessions.
	Stderr() io.Reader
	// Resize changes the terminal dimensions for TTY sessions.
	Resize(ctx context.Context, rows, cols int) error
	// CloseWrite closes stdin for non-TTY sessions when supported.
	CloseWrite() error
	// Close closes the stream transport.
	Close() error
	// Wait blocks until the process exits and returns its exit code.
	Wait(ctx context.Context) (int, error)
}

var (
	ErrNotFound              = errors.New("process session not found")
	ErrTTYUnsupported        = errors.New("tty is not supported on this platform")
	ErrUserSwitchUnsupported = errors.New("unsupported user switch")
)
