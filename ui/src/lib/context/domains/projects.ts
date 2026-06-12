import type {
	ProjectEventsStreamMessage,
	SessionUpdatedEvent,
	StartupTaskUpdatedEvent,
	WorkspaceUpdatedEvent,
} from "$lib/api-types";
import {
	createErrorStatus,
	createLoadingStatus,
	createReadyStatus,
	createRefreshingStatus,
	removeById,
	upsertById,
} from "$lib/context/cache";
import type { CommandOptions, Context } from "$lib/context/context.types";
import {
	applySessionSnapshotToRecord,
	createSessionRecord,
	createSessionsState,
	type SessionsState,
} from "$lib/context/domains/sessions";
import {
	createStartupTasksState,
	type StartupTasksState,
} from "$lib/context/domains/startup-tasks";
import {
	createWorkspacesState,
	type WorkspacesState,
} from "$lib/context/domains/workspaces";
import {
	connectProjectEvents,
	type ProjectEventSocket,
	type ProjectSocketRequest,
} from "$lib/context/project-subscription";
import {
	activateDebugSubscription,
	closeDebugSubscription,
	logDebugSocketMessage,
	logDebugSubscriptionEvent,
	openDebugSubscription,
} from "$lib/context/debug";
import { lastSessionWorkspaceStore } from "$lib/context/stores/last-session-workspace";
import { setProjectEventSocket } from "$lib/project-events";

type ActivationPhase = "idle" | "initializing" | "active" | "stopped";
type ProjectEventTarget = {
	sessions: SessionsState;
	workspaces: WorkspacesState;
	startupTasks: StartupTasksState;
};
type ProjectRuntime = {
	activationId: number;
	phase: ActivationPhase;
	socket: ProjectEventSocket | null;
	historyTarget: ProjectEventTarget | null;
	historyResolve: (() => void) | null;
	historyReject: ((error: unknown) => void) | null;
};

const runtimes = new WeakMap<Context, ProjectRuntime>();
const PROJECT_EVENTS_REQUEST = {
	type: "subscribe",
	stream: "project-events",
} satisfies ProjectSocketRequest;

export async function activateProject(
	context: Context,
	projectId: string,
	options: CommandOptions = {},
): Promise<void> {
	context.data.project.id = projectId;
	context.view.app.lastSessionWorkspaceSelection =
		lastSessionWorkspaceStore.readInitial(projectId);
	const work = startProjectActivation(context);
	if (options.wait) await work;
	else void work.catch(() => undefined);
}

export function stopProjectWatch(context: Context): void {
	const runtime = runtimes.get(context);
	if (!runtime) return;

	runtime.phase = "stopped";
	runtime.activationId += 1;
	runtime.historyTarget = null;
	clearHistoryReadyPromise(runtime, new Error("Project watch stopped"));
	closeDebugSubscription(context, PROJECT_EVENTS_REQUEST);
	runtime.socket?.close();
	runtime.socket = null;
	setProjectEventSocket(context, null);
	runtimes.delete(context);
}

async function startProjectActivation(context: Context): Promise<void> {
	const runtime = getRuntime(context);
	const activationId = runtime.activationId + 1;
	runtime.activationId = activationId;
	runtime.phase = "initializing";
	runtime.historyTarget = null;
	runtime.socket?.close();
	runtime.socket = null;
	setProjectEventSocket(context, null);

	context.data.project.status = createLoadingStatus();
	context.data.sessions.status =
		context.data.sessions.allIds.length > 0
			? createRefreshingStatus()
			: createLoadingStatus();
	context.data.workspaces.status =
		context.data.workspaces.allIds.length > 0
			? createRefreshingStatus()
			: createLoadingStatus();
	context.data.startupTasks.status =
		context.data.startupTasks.allIds.length > 0
			? createRefreshingStatus()
			: createLoadingStatus();

	try {
		const historyReady = createHistoryReadyPromise(runtime);
		openDebugSubscription(context, PROJECT_EVENTS_REQUEST);
		runtime.socket = connectProjectEvents({
			onEvent: (message) =>
				handleProjectStreamEvent(context, runtime, activationId, message),
			onError: (error) =>
				handleProjectStreamError(context, runtime, activationId, error),
			onSocketMessage: (direction, message) =>
				logDebugSocketMessage(context, direction, message),
		});
		setProjectEventSocket(context, runtime.socket);
		await runtime.socket.open();
		await historyReady;
	} catch (error) {
		if (activationId !== runtime.activationId) return;
		runtime.phase = "idle";
		setProjectEventSocket(context, null);
		closeDebugSubscription(context, PROJECT_EVENTS_REQUEST, error);
		context.data.project.status = createErrorStatus(error);
		context.data.sessions.status = createErrorStatus(error);
		context.data.workspaces.status = createErrorStatus(error);
		context.data.startupTasks.status = createErrorStatus(error);
		throw error;
	}
}

function handleProjectStreamEvent(
	context: Context,
	runtime: ProjectRuntime,
	activationId: number,
	message: ProjectEventsStreamMessage,
): void {
	if (activationId !== runtime.activationId || runtime.phase === "stopped")
		return;

	logDebugSubscriptionEvent(context, PROJECT_EVENTS_REQUEST, message.event);

	if (message.event === "history-start") {
		runtime.historyTarget = {
			sessions: createSessionsState(),
			workspaces: createWorkspacesState(),
			startupTasks: createStartupTasksState(),
		};
		return;
	}

	if (message.event === "history-end") {
		if (runtime.historyTarget) {
			context.data.sessions = runtime.historyTarget.sessions;
			context.data.workspaces = runtime.historyTarget.workspaces;
			context.data.startupTasks = runtime.historyTarget.startupTasks;
			runtime.historyTarget = null;
		}
		runtime.phase = "active";
		activateDebugSubscription(context, PROJECT_EVENTS_REQUEST);
		context.data.project.status = createReadyStatus();
		context.data.sessions.status = createReadyStatus();
		context.data.workspaces.status = createReadyStatus();
		context.data.startupTasks.status = createReadyStatus();
		clearHistoryReadyPromise(runtime);
		return;
	}

	const target = runtime.historyTarget ?? context.data;
	applyProjectEvent(context, target, message);
}

function handleProjectStreamError(
	context: Context,
	runtime: ProjectRuntime,
	activationId: number,
	error: unknown,
): void {
	if (activationId !== runtime.activationId || runtime.phase === "stopped")
		return;
	context.data.project.status = createErrorStatus(error);
	closeDebugSubscription(context, PROJECT_EVENTS_REQUEST, error);
	clearHistoryReadyPromise(runtime, error);
}

export function applyProjectEvent(
	context: Context,
	target: ProjectEventTarget,
	message: ProjectEventsStreamMessage,
): void {
	switch (message.event) {
		case "connected":
		case "history-start":
		case "history-end":
			return;
		case "session_updated": {
			const session = isSessionUpdatedEvent(message.data)
				? message.data.data
				: undefined;
			if (session?.id) {
				if (session.sandboxStatus === "removed") {
					removeById(target.sessions, session.id);
					clearRemovedSessionSelection(context, session.id);
					return;
				}
				const existing = target.sessions.byId[session.id];
				if (existing) {
					applySessionSnapshotToRecord(existing, session);
				} else {
					upsertById(
						target.sessions,
						session.id,
						createSessionRecord(session.id, session),
					);
				}
			}
			return;
		}
		case "workspace_updated": {
			const workspace = isWorkspaceUpdatedEvent(message.data)
				? message.data.data
				: undefined;
			if (workspace?.id) {
				upsertById(target.workspaces, workspace.id, workspace);
			}
			return;
		}
		case "startup_task_updated": {
			const task = isStartupTaskUpdatedEvent(message.data)
				? message.data.data
				: undefined;
			if (task?.id) {
				upsertById(target.startupTasks, task.id, task);
			}
			return;
		}
		case "thread_updated":
			return;
	}
}

function clearRemovedSessionSelection(
	context: Context,
	sessionId: string,
): void {
	if (context.view.selection.sessionId !== sessionId) {
		return;
	}
	context.view.selection.sessionId = null;
	context.view.selection.threadId = null;
	delete context.view.selection.requestedThreadIdBySessionId[sessionId];
}

function isSessionUpdatedEvent(
	data: ProjectEventsStreamMessage["data"] | undefined,
): data is SessionUpdatedEvent {
	return !!data && "type" in data && data.type === "session_updated";
}

function isWorkspaceUpdatedEvent(
	data: ProjectEventsStreamMessage["data"] | undefined,
): data is WorkspaceUpdatedEvent {
	return !!data && "type" in data && data.type === "workspace_updated";
}

function isStartupTaskUpdatedEvent(
	data: ProjectEventsStreamMessage["data"] | undefined,
): data is StartupTaskUpdatedEvent {
	return !!data && "type" in data && data.type === "startup_task_updated";
}

function getRuntime(context: Context): ProjectRuntime {
	let runtime = runtimes.get(context);
	if (!runtime) {
		runtime = {
			activationId: 0,
			phase: "idle",
			socket: null,
			historyTarget: null,
			historyResolve: null,
			historyReject: null,
		};
		runtimes.set(context, runtime);
	}
	return runtime;
}

function createHistoryReadyPromise(runtime: ProjectRuntime): Promise<void> {
	clearHistoryReadyPromise(runtime, new Error("Project activation replaced"));
	return new Promise((resolve, reject) => {
		runtime.historyResolve = resolve;
		runtime.historyReject = reject;
	});
}

function clearHistoryReadyPromise(
	runtime: ProjectRuntime,
	error?: unknown,
): void {
	const resolve = runtime.historyResolve;
	const reject = runtime.historyReject;
	runtime.historyResolve = null;
	runtime.historyReject = null;
	if (error) reject?.(error);
	else resolve?.();
}
