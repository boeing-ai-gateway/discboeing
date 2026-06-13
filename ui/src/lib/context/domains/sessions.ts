import { ApiError, api } from "$lib/api-client";
import type {
	AgentCommand,
	HooksStateResponse,
	ListSessionFilesResponse,
	Service,
	Session,
	SessionDiffFilesResponse,
	SessionDiffResponse,
	SessionStreamMessage,
	Thread,
} from "$lib/api-types";
import type { CollectionCache, ResourceStatus } from "$lib/context/cache";
import {
	createCollectionCache,
	createErrorStatus,
	createIdleStatus,
	createMissingStatus,
	createReadyStatus,
	createRefreshingStatus,
	removeById,
	upsertById,
} from "$lib/context/cache";
import type {
	CommandOptions,
	CreateSessionInput,
	Context,
} from "$lib/context/context.types";
import type { CommandsState } from "$lib/context/domains/commands";
import {
	applyCommandsSnapshotToRecord,
	createCommandsState,
} from "$lib/context/domains/commands";
import type { DiffState } from "$lib/context/domains/diff";
import {
	applyDiffSnapshotToRecord,
	applyDiffStatusSnapshotToRecord,
	createDiffState,
} from "$lib/context/domains/diff";
import type { FilesState } from "$lib/context/domains/files";
import {
	applyFileSubtreeSnapshotToRecord,
	createFilesState,
} from "$lib/context/domains/files";
import type { HooksState } from "$lib/context/domains/hooks";
import {
	applyHooksSnapshotToRecord,
	createHooksState,
} from "$lib/context/domains/hooks";
import type { ServicesState } from "$lib/context/domains/services";
import {
	applyServicesSnapshotToRecord,
	createServicesState,
} from "$lib/context/domains/services";
import type { ThreadsState } from "$lib/context/domains/threads";
import {
	applyThreadsSnapshotToRecord,
	createThreadsState,
	deactivateThread,
	stopThreadWatchesForSession,
} from "$lib/context/domains/threads";
import { getProjectEventSocket } from "$lib/project-events";
import type { ProjectEventSocket } from "$lib/context/project-subscription";
import type { SessionSubscription } from "$lib/context/session-subscription";
import {
	createSessionCredentialsState,
	type SessionCredentialsState,
} from "$lib/context/domains/session-credentials";

const sessionActivationPromises = new WeakMap<
	Context,
	Map<string, Promise<void>>
>();

export type SessionRecord = {
	id: string;
	value: Session | null;
	status: ResourceStatus;
	threads: ThreadsState;
	files: FilesState;
	commands: CommandsState;
	hooks: HooksState;
	services: ServicesState;
	diff: DiffState;
	credentials: SessionCredentialsState;
	subscription: SessionSubscription | null;
};

export type SessionsState = CollectionCache<SessionRecord>;

export function createSessionsState(): SessionsState {
	return createCollectionCache<SessionRecord>();
}

export function createSessionRecord(
	sessionId: string,
	session: Session | null = null,
): SessionRecord {
	return {
		id: sessionId,
		value: session,
		status: session
			? { state: "ready", lastLoadedAt: Date.now() }
			: createIdleStatus(),
		threads: createThreadsState(),
		files: createFilesState(),
		commands: createCommandsState(),
		hooks: createHooksState(),
		services: createServicesState(),
		diff: createDiffState(),
		credentials: createSessionCredentialsState(),
		subscription: null,
	};
}

export function ensureSessionRecord(
	state: SessionsState,
	sessionId: string,
): SessionRecord {
	state.byId[sessionId] ??= createSessionRecord(sessionId);
	if (!state.allIds.includes(sessionId)) {
		state.allIds.push(sessionId);
	}
	return state.byId[sessionId];
}

export function peekSessionInCache(
	context: Context,
	sessionId: string,
): Session | null {
	return context.data.sessions.byId[sessionId]?.value ?? null;
}

export function listCachedSessions(context: Context): Session[] {
	const state = context.data.sessions;
	return state.allIds
		.map((id) => state.byId[id]?.value)
		.filter((session): session is Session => !!session);
}

export function removeSessionFromCache(
	context: Context,
	sessionId: string,
): boolean {
	const existed = peekSessionInCache(context, sessionId) !== null;
	removeById(context.data.sessions, sessionId);
	return existed;
}

export async function listSessionsCache(): Promise<SessionsState> {
	const response = await api.getSessions();
	const state = createSessionsState();
	for (const session of response.sessions) {
		upsertById(state, session.id, createSessionRecord(session.id, session));
	}
	state.status = createReadyStatus();
	return state;
}

function publishSessionsCache(context: Context, state: SessionsState): void {
	context.data.sessions = state;
}

export async function applySessionChangedToCache(
	state: SessionsState,
	sessionId: string,
	options: { removed?: boolean } = {},
): Promise<void> {
	if (options.removed) {
		removeById(state, sessionId);
		return;
	}

	try {
		const session = await api.getSession(sessionId);
		const existing = state.byId[sessionId];
		upsertById(state, sessionId, {
			...(existing ?? createSessionRecord(sessionId)),
			value: session,
			status: createReadyStatus(),
		});
	} catch (error) {
		if (error instanceof ApiError && error.status === 404) {
			removeById(state, sessionId);
			return;
		}
		throw error;
	}
}

export function applySessionSnapshotToCache(
	context: Context,
	session: Session,
): void {
	const existing = context.data.sessions.byId[session.id];
	upsertById(context.data.sessions, session.id, {
		...(existing ?? createSessionRecord(session.id)),
		value: session,
		status: createReadyStatus(),
	});
}

export function applySessionSnapshotToRecord(
	record: SessionRecord,
	session: Session,
): void {
	record.value = session;
	record.status = createReadyStatus();
}

export function applySessionEvent(
	record: SessionRecord,
	message: SessionStreamMessage,
): void {
	switch (message.event) {
		case "history-start":
		case "history-end":
			return;
		case "session_updated":
			applySessionSnapshotToRecord(record, message.data as Session);
			return;
		case "threads_updated":
			applyThreadsSnapshotToRecord(
				record,
				(message.data as { threads: Thread[] }).threads,
			);
			return;
		case "files_updated":
			applyFileSubtreeSnapshotToRecord(
				record,
				message.data as unknown as ListSessionFilesResponse,
			);
			return;
		case "commands_updated":
			applyCommandsSnapshotToRecord(
				record,
				(message.data as { commands: AgentCommand[] }).commands,
			);
			return;
		case "hooks_updated":
			applyHooksSnapshotToRecord(
				record,
				message.data as unknown as HooksStateResponse,
			);
			return;
		case "services_updated":
			applyServicesSnapshotToRecord(
				record,
				(message.data as { services: Service[] }).services,
			);
			return;
		case "diff_status_updated":
			applyDiffStatusSnapshotToRecord(
				record,
				message.data as unknown as SessionDiffFilesResponse,
			);
			return;
		case "diff_updated":
			applyDiffSnapshotToRecord(
				record,
				message.data as unknown as SessionDiffResponse,
			);
			return;
	}
}

export async function loadSessionsIntoCache(context: Context): Promise<void> {
	context.data.sessions.status =
		context.data.sessions.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };

	try {
		publishSessionsCache(context, await listSessionsCache());
	} catch (error) {
		context.data.sessions.status = createErrorStatus(error);
		throw error;
	}
}

export async function loadSessionIntoCache(
	context: Context,
	sessionId: string,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.status = record.value
		? createRefreshingStatus()
		: { state: "loading" };

	try {
		record.value = await api.getSession(sessionId);
		record.status = createReadyStatus();
	} catch (error) {
		if (error instanceof ApiError && error.status === 404) {
			record.value = null;
			record.status = createMissingStatus();
			return;
		}
		record.status = createErrorStatus(error);
		throw error;
	}
}

export async function activateSession(
	context: Context,
	sessionId: string,
	socket: ProjectEventSocket | null,
	options: CommandOptions = {},
): Promise<void> {
	if (!socket) {
		throw new Error("Project watch must be active before activating a session");
	}

	const existingActivation = sessionActivationPromises
		.get(context)
		?.get(sessionId);
	if (existingActivation) {
		if (options.wait) await existingActivation;
		else void existingActivation.catch(() => undefined);
		return;
	}

	const record = ensureSessionRecord(context.data.sessions, sessionId);
	if (record.subscription) {
		const work = record.subscription.open();
		if (options.wait) await work;
		else void work.catch(() => undefined);
		return;
	}
	record.status = record.value
		? createRefreshingStatus()
		: { state: "loading" };
	record.threads.status =
		record.threads.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };
	record.files.activeSubtrees[""] = true;
	record.files.statusBySubtree[""] = record.files.nodesByPath[""]
		? createRefreshingStatus()
		: { state: "loading" };
	record.commands.status =
		record.commands.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };
	record.hooks.status =
		record.hooks.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };
	record.services.status =
		record.services.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };
	record.diff.filesStatus = record.diff.files
		? createRefreshingStatus()
		: { state: "loading" };
	record.diff.status = record.diff.value
		? createRefreshingStatus()
		: { state: "loading" };

	const work = startSessionActivation(context, sessionId, record, socket);
	storeSessionActivation(context, sessionId, work);
	if (options.wait) await work;
	else void work.catch(() => undefined);
}

async function startSessionActivation(
	context: Context,
	sessionId: string,
	record: SessionRecord,
	socket: ProjectEventSocket,
): Promise<void> {
	const { connectSessionEvents } =
		await import("$lib/context/session-subscription");
	const subscription = connectSessionEvents(context, sessionId, socket);
	record.subscription = subscription;
	await subscription.open().catch((error) => {
		if (record.subscription === subscription) {
			setSessionActivationError(record, error);
		}
		throw error;
	});
}

function storeSessionActivation(
	context: Context,
	sessionId: string,
	work: Promise<void>,
): void {
	let activations = sessionActivationPromises.get(context);
	if (!activations) {
		activations = new Map();
		sessionActivationPromises.set(context, activations);
	}
	activations.set(sessionId, work);
	void work.then(
		() => {
			if (activations.get(sessionId) === work) {
				activations.delete(sessionId);
			}
		},
		() => {
			if (activations.get(sessionId) === work) {
				activations.delete(sessionId);
			}
		},
	);
}

function clearSessionActivation(context: Context, sessionId: string): void {
	sessionActivationPromises.get(context)?.delete(sessionId);
}

export async function activateSessionUsingProjectSocket(
	context: Context,
	sessionId: string,
	options?: CommandOptions,
): Promise<void> {
	await activateSession(
		context,
		sessionId,
		getProjectEventSocket(context),
		options,
	);
}

export async function deactivateSession(
	context: Context,
	sessionId: string,
): Promise<void> {
	clearSessionActivation(context, sessionId);
	const record = context.data.sessions.byId[sessionId];
	if (!record) return;
	record.subscription?.close();
	record.subscription = null;
	stopThreadWatchesForSession(context, sessionId);
}

export function stopSessionWatches(
	context: Context,
	exceptSessionId?: string,
): void {
	const activations = sessionActivationPromises.get(context);
	if (!exceptSessionId) {
		sessionActivationPromises.delete(context);
	} else if (activations) {
		for (const sessionId of activations.keys()) {
			if (sessionId !== exceptSessionId) {
				activations.delete(sessionId);
			}
		}
	}
	for (const id of context.data.sessions.allIds) {
		if (id === exceptSessionId) continue;
		const record = context.data.sessions.byId[id];
		record?.subscription?.close();
		if (record) {
			record.subscription = null;
			stopThreadWatchesForSession(context, id);
		}
	}
}

function setSessionActivationError(
	record: SessionRecord,
	error: unknown,
): void {
	const status = createErrorStatus(error);
	record.status = status;
	record.threads.status = status;
	record.files.statusBySubtree[""] = status;
	record.commands.status = status;
	record.hooks.status = status;
	record.services.status = status;
	record.diff.filesStatus = status;
	record.diff.status = status;
}

export async function createSession(
	context: Context,
	input: CreateSessionInput,
	options: CommandOptions = {},
): Promise<void> {
	await api.createSession(input);
	if (options.wait) await loadSessionsIntoCache(context);
}

export async function renameSession(
	context: Context,
	sessionId: string,
	name: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.updateSession(sessionId, { displayName: name });
	if (options.wait) await loadSessionIntoCache(context, sessionId);
}

export async function stopSession(
	context: Context,
	sessionId: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.stopSession(sessionId);
	if (options.wait) await loadSessionIntoCache(context, sessionId);
}

export async function deleteSession(
	context: Context,
	sessionId: string,
	options: CommandOptions = {},
): Promise<void> {
	await deactivateSession(context, sessionId);
	await api.deleteSession(sessionId);
	if (options.wait) await loadSessionsIntoCache(context);
}

export async function deleteSessionWithThreadDeactivation(
	context: Context,
	sessionId: string,
	options?: CommandOptions,
): Promise<void> {
	const threadIds = context.data.sessions.byId[sessionId]?.threads.allIds ?? [];
	for (const threadId of threadIds) {
		await deactivateThread(context, sessionId, threadId);
	}
	await deleteSession(context, sessionId, options);
}
