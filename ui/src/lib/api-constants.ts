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

export const SessionDisplayStatus = {
	...SessionStatus,
	IDLE: "idle",
	QUEUED: "queued",
	RUNNING: "running",
	NEEDS_ATTENTION: "needs_attention",
	UNKNOWN: "unknown",
	PENDING: "pending",
	COMMITTING: "committing",
	COMPLETED: "completed",
	COMMITTED: "committed",
} as const;

type SessionStatusValue = (typeof SessionStatus)[keyof typeof SessionStatus];

const SESSION_TRANSITIONING_STATUSES = new Set<SessionStatusValue>([
	SessionStatus.INITIALIZING,
	SessionStatus.REINITIALIZING,
	SessionStatus.CLONING,
	SessionStatus.PULLING_IMAGE,
	SessionStatus.CREATING_SANDBOX,
	SessionStatus.REMOVING,
]);

const SESSION_THREAD_ACCESSIBLE_STATUSES = new Set<SessionStatusValue>([
	SessionStatus.READY,
	SessionStatus.STOPPED,
	SessionStatus.ERROR,
]);

export function isSessionTransitioningStatus(
	status: string | null | undefined,
): boolean {
	return status !== null && status !== undefined
		? SESSION_TRANSITIONING_STATUSES.has(status as SessionStatusValue)
		: false;
}

export function canLoadSessionThreads(
	status: string | null | undefined,
): boolean {
	return status !== null && status !== undefined
		? SESSION_THREAD_ACCESSIBLE_STATUSES.has(status as SessionStatusValue)
		: false;
}

// Workspace status constants representing the lifecycle of a workspace
export const WorkspaceStatus = {
	INITIALIZING: "initializing",
	CLONING: "cloning",
	READY: "ready",
	REMOVING: "removing",
	ERROR: "error",
} as const;
