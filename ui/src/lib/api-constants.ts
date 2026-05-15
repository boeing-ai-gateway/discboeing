// API Constants - shared string constants that must match the public REST API

// Session lifecycle constants. These values must match
// server/internal/model SessionStatus* constants.
export const SessionStatus = {
	INITIALIZING: "initializing",
	REINITIALIZING: "reinitializing",
	CLONING: "cloning",
	PULLING_IMAGE: "pulling_image",
	CREATING_SANDBOX: "creating_sandbox",
	CREATE_FAILED: "create_failed",
	READY: "ready",
	STOPPED: "stopped",
	ERROR: "error",
	REMOVING: "removing",
	REMOVED: "removed",
} as const;

export const SessionSandboxStatus = SessionStatus;

export const CommitStatus = {
	NONE: "",
	PENDING: "pending",
	COMMITTING: "committing",
	COMPLETED: "completed",
	FAILED: "failed",
} as const;

export const CommitOperation = {
	COMMIT: "commit",
} as const;

// Workspace status constants representing the lifecycle of a workspace
export const WorkspaceStatus = {
	INITIALIZING: "initializing",
	CLONING: "cloning",
	READY: "ready",
	REMOVING: "removing",
	ERROR: "error",
} as const;
