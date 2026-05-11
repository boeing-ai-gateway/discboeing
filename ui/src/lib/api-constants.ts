// API Constants - shared string constants that must match the public REST API

// Session status constants representing the lifecycle of a session plus
// commit progress states surfaced through the public status field.
export const SessionStatus = {
	INITIALIZING: "initializing",
	REINITIALIZING: "reinitializing",
	CLONING: "cloning",
	PULLING_IMAGE: "pulling_image",
	CREATING_SANDBOX: "creating_sandbox",
	CREATE_FAILED: "create_failed",
	READY: "ready",
	STOPPED: "stopped",
	PENDING: "pending",
	COMMITTING: "committing",
	COMPLETED: "completed",
	COMMITTED: "committed",
	ERROR: "error",
	REMOVING: "removing",
	REMOVED: "removed",
} as const;

type SessionStatusValue = (typeof SessionStatus)[keyof typeof SessionStatus];

const SESSION_TRANSITIONING_STATUSES = new Set<SessionStatusValue>([
	SessionStatus.INITIALIZING,
	SessionStatus.REINITIALIZING,
	SessionStatus.CLONING,
	SessionStatus.PULLING_IMAGE,
	SessionStatus.CREATING_SANDBOX,
	SessionStatus.PENDING,
	SessionStatus.COMMITTING,
	SessionStatus.REMOVING,
]);

export function isSessionTransitioningStatus(
	status: SessionStatusValue | null | undefined,
): boolean {
	return status !== null && status !== undefined
		? SESSION_TRANSITIONING_STATUSES.has(status)
		: false;
}

// Workspace status constants representing the lifecycle of a workspace
export const WorkspaceStatus = {
	INITIALIZING: "initializing",
	CLONING: "cloning",
	READY: "ready",
	ERROR: "error",
} as const;
