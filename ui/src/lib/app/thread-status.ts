import { CommitOperation, CommitStatus } from "../api-constants";
import type {
	Session,
	SessionDisplayStatus,
	SessionThreadStatus,
	Thread,
	ThreadActivityStatus,
} from "../api-types";
import type { ThreadContextValue } from "$lib/session/session-context.types";

export function isThreadSnapshotRunning(
	thread: Partial<Thread> | null | undefined,
): boolean {
	return (
		thread?.activityStatus?.status === "running" ||
		(thread?.activeCommand ?? "").trim().length > 0
	);
}

export function resolveSessionDisplayStatus(
	session: Partial<
		Pick<
			Session,
			"status" | "threadStatus" | "commitStatus" | "commitOperation"
		>
	> | null,
): SessionDisplayStatus {
	const sessionStatus = session?.sandboxStatus;
	const sessionActivityStatus = session?.threadStatus?.status;
	const commitStatus = session?.commitStatus;
	const commitOperation = session?.commitOperation;

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
	threadContext:
		| Pick<ThreadContextValue, "status" | "isStreaming" | "hasPendingQuestion">
		| null
		| undefined,
): ThreadActivityStatus | null {
	if (threadContext?.isStreaming) {
		return { status: "running" };
	}
	if (threadContext?.hasPendingQuestion) {
		return { status: "needs_attention" };
	}
	return null;
}

export function getThreadStateLabel(state: Thread["state"] | undefined) {
	if (state === "interrupted") {
		return "Interrupted";
	}
	if (state === "cancelled") {
		return "Cancelled";
	}
	return null;
}

export function getThreadStateTone(state: Thread["state"] | undefined) {
	if (state === "interrupted") {
		return "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300";
	}
	if (state === "cancelled") {
		return "border-current/15 bg-current/10 text-current/75";
	}
	return "";
}

export function resolveSidebarThreadStatus({
	sessionThreadStatus,
	thread,
	localActivityStatus,
}: {
	sessionThreadStatus?: SessionThreadStatus | null;
	thread?: Pick<
		Thread,
		| "activityStatus"
		| "state"
		| "pendingQuestion"
		| "errorMessage"
		| "promptQueue"
	> | null;
	localActivityStatus?: ThreadActivityStatus | null;
}): ThreadActivityStatus["status"] | null {
	const sessionActivityStatus = sessionThreadStatus?.status;

	if (
		thread?.pendingQuestion ||
		(thread?.errorMessage ?? "").trim().length > 0 ||
		thread?.state === "interrupted" ||
		thread?.state === "cancelled"
	) {
		return "needs_attention";
	}

	if (sessionActivityStatus && sessionActivityStatus !== "idle") {
		return sessionActivityStatus;
	}

	if (sessionActivityStatus === "idle") {
		return null;
	}

	if (localActivityStatus?.status && localActivityStatus.status !== "idle") {
		return localActivityStatus.status;
	}
	if (
		thread?.activityStatus?.status &&
		thread.activityStatus.status !== "idle"
	) {
		return thread.activityStatus.status;
	}
	if ((thread?.promptQueue?.length ?? 0) > 0) {
		return "queued";
	}
	return null;
}

export function resolveThreadDisplayStatus({
	session,
	sessionThreadStatus,
	thread,
	localActivityStatus,
}: {
	session?: Pick<
		Session,
		"status" | "threadStatus" | "commitStatus" | "commitOperation"
	> | null;
	sessionThreadStatus?: SessionThreadStatus | null;
	thread?: Pick<
		Thread,
		| "activityStatus"
		| "state"
		| "pendingQuestion"
		| "errorMessage"
		| "promptQueue"
	> | null;
	localActivityStatus?: ThreadActivityStatus | null;
}): SessionDisplayStatus {
	const displayedSessionStatus = resolveSessionDisplayStatus({
		...session,
		threadStatus: sessionThreadStatus ?? undefined,
	});
	if (displayedSessionStatus === "committed") {
		return "committed";
	}

	const status = resolveSidebarThreadStatus({
		sessionThreadStatus,
		thread,
		localActivityStatus,
	});

	if (displayedSessionStatus === "stopped" && status === "running") {
		return "stopped";
	}
	return status ?? "idle";
}
