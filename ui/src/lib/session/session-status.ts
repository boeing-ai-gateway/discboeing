import { SessionStatus } from "../api-constants";
import type { SessionStatus as SessionStatusValue } from "../api-types";

const SESSION_TRANSITIONING_STATUSES = new Set<SessionStatusValue>([
	SessionStatus.INITIALIZING,
	SessionStatus.REINITIALIZING,
	SessionStatus.CLONING,
	SessionStatus.PULLING_IMAGE,
	SessionStatus.CREATING_SANDBOX,
	SessionStatus.REMOVING,
]);

export function isSessionTransitioningStatus(
	status: SessionStatusValue | null | undefined,
): boolean {
	return status !== null && status !== undefined
		? SESSION_TRANSITIONING_STATUSES.has(status)
		: false;
}
