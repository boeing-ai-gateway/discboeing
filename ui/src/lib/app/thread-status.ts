import { CommitOperation, CommitStatus } from "../api-constants";
import type {
	CommitOperation as CommitOperationValue,
	CommitStatus as CommitStatusValue,
	SessionStatus,
	SessionThreadActivityStatusValue,
	ThreadState,
} from "../api-types";

type SidebarThreadStatusInput = {
	sessionStatus?: SessionStatus | null;
	sessionActivityStatus?: SessionThreadActivityStatusValue | null;
	threadActivityStatus?: SessionThreadActivityStatusValue | null;
	localActivityStatus?: SessionThreadActivityStatusValue | null;
	threadState?: ThreadState;
	pendingQuestion?: boolean;
	errorMessage?: string;
	promptQueueCount?: number;
	idleFallback?: "session" | "none";
};

type SessionDisplayStatusInput = Pick<
	SidebarThreadStatusInput,
	"sessionStatus" | "sessionActivityStatus"
> & {
	commitStatus?: CommitStatusValue | null;
	commitOperation?: CommitOperationValue | null;
};

type ThreadRunningStatusInput = {
	activityStatus?: {
		status: SessionThreadActivityStatusValue;
	};
	activeCommand?: string;
} | null;

type ThreadContextStatusInput =
	| {
			status?: string;
			isStreaming?: boolean;
			hasPendingQuestion?: boolean;
	  }
	| null
	| undefined;

export function isThreadSnapshotRunning<T extends ThreadRunningStatusInput>(
	thread: T,
): boolean {
	return (
		thread?.activityStatus?.status === "running" ||
		(thread?.activeCommand ?? "").trim().length > 0
	);
}

type ThreadDisplayStatusInput = Omit<SidebarThreadStatusInput, "idleFallback"> &
	SessionDisplayStatusInput;

export type DisplayStatusValue =
	| SessionStatus
	| SessionThreadActivityStatusValue
	| "pending"
	| "committing"
	| "completed"
	| "committed"
	| "unknown";

export type ThreadDisplayStatusValue = DisplayStatusValue;

export function resolveSessionDisplayStatus({
	sessionStatus,
	sessionActivityStatus,
	commitStatus,
	commitOperation,
}: SessionDisplayStatusInput): ThreadDisplayStatusValue {
	if (sessionStatus === "removing" || sessionStatus === "removed") {
		return sessionStatus;
	}
	if (sessionStatus === "error" || sessionStatus === "create_failed") {
		return sessionStatus;
	}
	if (commitStatus === CommitStatus.PENDING) {
		return "pending";
	}
	if (commitStatus === CommitStatus.COMMITTING) {
		return "committing";
	}
	if (
		commitStatus === CommitStatus.COMPLETED &&
		commitOperation === CommitOperation.COMMIT
	) {
		return "committed";
	}
	if (commitStatus === CommitStatus.COMPLETED) {
		return "completed";
	}
	if (sessionActivityStatus && sessionActivityStatus !== "idle") {
		return sessionActivityStatus;
	}
	return sessionStatus === "ready" || !sessionStatus ? "idle" : sessionStatus;
}

export function resolveThreadContextDisplayStatus(
	threadContext: ThreadContextStatusInput,
): SessionThreadActivityStatusValue | null {
	if (threadContext?.isStreaming || threadContext?.status === "streaming") {
		return "running";
	}
	if (threadContext?.hasPendingQuestion) {
		return "needs_attention";
	}
	return null;
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
	| SessionThreadActivityStatusValue
	| SessionStatus
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

export function resolveThreadDisplayStatus({
	sessionStatus,
	sessionActivityStatus,
	commitStatus,
	commitOperation,
	threadActivityStatus,
	localActivityStatus,
	threadState,
	pendingQuestion,
	errorMessage,
	promptQueueCount,
}: ThreadDisplayStatusInput): ThreadDisplayStatusValue {
	const displayedSessionStatus = resolveSessionDisplayStatus({
		sessionStatus,
		sessionActivityStatus,
		commitStatus,
		commitOperation,
	});
	if (displayedSessionStatus === "committed") {
		return "committed";
	}

	const status = resolveSidebarThreadStatus({
		localActivityStatus,
		threadActivityStatus,
		threadState,
		pendingQuestion,
		errorMessage,
		promptQueueCount,
		idleFallback: "none",
	});

	if (displayedSessionStatus === "stopped" && status === "running") {
		return "stopped";
	}
	return status ?? "idle";
}
