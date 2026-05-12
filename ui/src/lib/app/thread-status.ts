import type {
	SessionThreadActivityStatusValue,
	ThreadState,
} from "../api-types";
import type {
	SessionActivityStatusValue,
	SessionStatusValue,
} from "../shell-types";

type SidebarThreadStatusInput = {
	sessionStatus?: SessionStatusValue | null;
	sessionActivityStatus?: SessionThreadActivityStatusValue | null;
	threadActivityStatus?: SessionThreadActivityStatusValue | null;
	localActivityStatus?: SessionThreadActivityStatusValue | null;
	threadState?: ThreadState;
	pendingQuestion?: boolean;
	errorMessage?: string;
	promptQueueCount?: number;
	idleFallback?: "session" | "none";
};

type ThreadRunningStatusInput = {
	activityStatus?: {
		status: SessionThreadActivityStatusValue;
	};
	activeCommand?: string;
} | null;

export function isThreadSnapshotRunning<T extends ThreadRunningStatusInput>(
	thread: T,
): boolean {
	return (
		thread?.activityStatus?.status === "running" ||
		(thread?.activeCommand ?? "").trim().length > 0
	);
}

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

export function resolveSidebarThreadStatus({
	sessionStatus,
	sessionActivityStatus,
	threadActivityStatus,
	localActivityStatus,
	threadState,
	pendingQuestion,
	errorMessage,
	promptQueueCount,
	idleFallback = "session",
}: SidebarThreadStatusInput):
	| SessionActivityStatusValue
	| SessionStatusValue
	| null {
	if (
		pendingQuestion ||
		(errorMessage ?? "").trim().length > 0 ||
		threadState === "interrupted" ||
		threadState === "cancelled"
	) {
		return "needs_attention";
	}

	if (sessionActivityStatus && sessionActivityStatus !== "idle") {
		return sessionActivityStatus;
	}

	const fallbackStatus =
		idleFallback === "session" ? (sessionStatus ?? null) : null;
	if (sessionActivityStatus === "idle") {
		return fallbackStatus;
	}

	if (localActivityStatus && localActivityStatus !== "idle") {
		return localActivityStatus;
	}
	if (threadActivityStatus && threadActivityStatus !== "idle") {
		return threadActivityStatus;
	}
	if ((promptQueueCount ?? 0) > 0) {
		return "queued";
	}
	return fallbackStatus;
}
