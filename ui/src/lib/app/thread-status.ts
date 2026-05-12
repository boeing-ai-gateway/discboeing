import type { ThreadState } from "$lib/api-types";
import type {
	RecentThreadSummary,
	SessionActivityStatusValue,
	SessionStatusValue,
} from "$lib/shell-types";

export function getThreadStateLabel(state: ThreadState | undefined) {
	if (state === "interrupted") {
		return "Interrupted";
	}
	if (state === "cancelled") {
		return "Cancelled";
	}
	return null;
}

export function getThreadStateTone(state: ThreadState | undefined) {
	if (state === "interrupted") {
		return "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300";
	}
	if (state === "cancelled") {
		return "border-current/15 bg-current/10 text-current/75";
	}
	return "";
}

export function getRecentThreadDisplayStatus(
	thread: RecentThreadSummary,
): SessionActivityStatusValue | SessionStatusValue {
	const activityStatus = thread.activityStatus?.status;
	if (activityStatus && activityStatus !== "idle") {
		return activityStatus;
	}
	const sessionThreadStatus = thread.sessionThreadStatus?.status;
	if (sessionThreadStatus && sessionThreadStatus !== "idle") {
		return sessionThreadStatus;
	}
	if (thread.state === "interrupted" || thread.state === "cancelled") {
		return "needs_attention";
	}
	return thread.sessionStatus;
}
