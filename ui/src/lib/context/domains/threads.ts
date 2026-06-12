import { api } from "$lib/api-client";
import type {
	BrowserEventChunkData,
	ChatMessage,
	ChatStreamMessage,
	CreateThreadRequest,
	Thread,
	UpdateThreadRequest,
} from "$lib/api-types";
import { getPendingQuestionApprovalId } from "$lib/conversation-helpers";
import {
	createErrorStatus,
	createReadyStatus,
	createRefreshingStatus,
	removeById,
	upsertById,
} from "$lib/context/cache";
import type { CollectionCache, ResourceStatus } from "$lib/context/cache";
import { createCollectionCache, createIdleStatus } from "$lib/context/cache";
import type {
	CommandOptions,
	Context,
	SendMessageInput,
} from "$lib/context/context.types";
import {
	ensureSessionRecord,
	type SessionRecord,
} from "$lib/context/domains/sessions";
import type { ProjectEventSocket } from "$lib/context/project-subscription";
import type { ThreadSubscription } from "$lib/context/thread-subscription";
import type { AsyncStatus } from "$lib/resource/types";
import { getProjectEventSocket } from "$lib/project-events";
import { createChatStreamState } from "$lib/thread-stream";
import type { ChatStreamEvent } from "$lib/thread-stream-events";

const threadActivationPromises = new WeakMap<
	Context,
	Map<string, Promise<void>>
>();

export type ThreadContentState = {
	messages: ChatMessage[];
	browserEventsByTurnId: Record<string, BrowserEventChunkData[]>;
	historyReplayVersion: number;
	isStreaming: boolean;
	error: string | null;
	pendingQuestionId: string | null;
	status: ResourceStatus;
	subscription: ThreadSubscription | null;
};

export type ThreadRecord = {
	id: string;
	value: Thread | null;
	status: ResourceStatus;
	content: ThreadContentState;
};

export type ThreadsState = CollectionCache<ThreadRecord>;

type ThreadReadyControls = {
	resolveReady(): void;
	rejectReady(error: unknown): void;
};

export type ThreadEventTarget = {
	controls: ThreadReadyControls;
	streamState: ReturnType<typeof createChatStreamState>;
};

function createThreadContentState(): ThreadContentState {
	return {
		messages: [],
		browserEventsByTurnId: {},
		historyReplayVersion: 0,
		isStreaming: false,
		error: null,
		pendingQuestionId: null,
		status: createIdleStatus(),
		subscription: null,
	};
}

export function createThreadRecord(
	threadId: string,
	thread: Thread | null = null,
): ThreadRecord {
	return {
		id: threadId,
		value: thread,
		status: thread ? createReadyStatus() : createIdleStatus(),
		content: createThreadContentState(),
	};
}

export function createThreadsState(): ThreadsState {
	return createCollectionCache<ThreadRecord>();
}

function ensureThreadRecord(
	state: ThreadsState,
	threadId: string,
): ThreadRecord {
	state.byId[threadId] ??= createThreadRecord(threadId);
	if (!state.allIds.includes(threadId)) {
		state.allIds.push(threadId);
	}
	return state.byId[threadId];
}

export function ensureThreadContentState(
	state: ThreadsState,
	threadId: string,
): ThreadContentState {
	return ensureThreadRecord(state, threadId).content;
}

export function createThreadEventTarget(
	context: Context,
	sessionId: string,
	threadId: string,
): ThreadEventTarget {
	const target: ThreadEventTarget = {
		controls: {
			resolveReady: () => undefined,
			rejectReady: () => undefined,
		},
		streamState: createChatStreamState({
			getMessages: () => content().messages,
			setMessages: (messages) => {
				content().messages = messages;
			},
			onStart: () => {
				const state = content();
				state.isStreaming = true;
				state.error = null;
			},
			onCompletionStatus: ({ isRunning }) => {
				content().isStreaming = isRunning;
			},
			onFinish: () => {
				content().isStreaming = false;
			},
			onHistoryReplayStart: () => {
				const state = content();
				state.browserEventsByTurnId = {};
				state.status = { state: "loading" } satisfies ResourceStatus;
			},
			onHistoryReplayEnd: () => {
				const state = content();
				state.status = createReadyStatus();
				state.historyReplayVersion += 1;
				state.pendingQuestionId = getPendingQuestionApprovalId(state.messages);
				target.controls.resolveReady();
			},
			onChunkError: (errorText) => {
				const state = content();
				state.isStreaming = false;
				state.error = errorText;
				state.status = {
					state: "error",
					error: errorText,
				} satisfies ResourceStatus;
				target.controls.rejectReady(new Error(errorText));
			},
			onThreadUpdate: (thread) => {
				const threads = ensureSessionRecord(
					context.data.sessions,
					sessionId,
				).threads;
				threads.byId[thread.id] = {
					...(threads.byId[thread.id] ?? createThreadRecord(thread.id)),
					value: thread,
					status: createReadyStatus(),
				};
				if (!threads.allIds.includes(thread.id)) {
					threads.allIds.push(thread.id);
				}
				threads.status = createReadyStatus();
			},
			onHooksStatusUpdate: (status) => {
				const record = ensureSessionRecord(context.data.sessions, sessionId);
				record.hooks.byId = status.hooks;
				record.hooks.allIds = Object.keys(status.hooks);
				record.hooks.pendingHookIds = status.pendingHooks;
				record.hooks.lastEvaluatedAt = status.lastEvaluatedAt;
				record.hooks.executionPaused = status.executionPaused;
				record.hooks.status = createReadyStatus();
			},
			onBrowserEvent: (event) => {
				applyBrowserEvent(content(), event);
			},
		}),
	};

	return target;

	function content(): ThreadContentState {
		return ensureThreadContentState(
			ensureSessionRecord(context.data.sessions, sessionId).threads,
			threadId,
		);
	}
}

export async function applyThreadEvent(
	target: ThreadEventTarget,
	content: ThreadContentState,
	message: ChatStreamMessage,
	controls: ThreadReadyControls,
): Promise<void> {
	target.controls = controls;
	if (!message.event || typeof message.data !== "string") return;
	try {
		await target.streamState.handleStreamEvent({
			event: message.event,
			data: message.data,
		} as ChatStreamEvent);
	} catch (error) {
		content.error =
			error instanceof Error ? error.message : "Thread stream failed";
		content.status = {
			state: "error",
			error: content.error,
		} satisfies ResourceStatus;
		throw error;
	}
}

function applyBrowserEvent(
	state: ThreadContentState,
	event: BrowserEventChunkData,
): void {
	const turnId = event.turnId?.trim();
	if (!turnId) return;
	const current = state.browserEventsByTurnId[turnId] ?? [];
	const existingIndex = current.findIndex(
		(candidate) => candidate.event.eventId === event.event.eventId,
	);
	const next = [...current];
	if (existingIndex === -1) next.push(event);
	else next[existingIndex] = event;
	state.browserEventsByTurnId = {
		...state.browserEventsByTurnId,
		[turnId]: sortBrowserEvents(next),
	};
}

function sortBrowserEvents(
	events: BrowserEventChunkData[],
): BrowserEventChunkData[] {
	return [...events].sort((left, right) => {
		if (left.stepIndex !== right.stepIndex) {
			return left.stepIndex - right.stepIndex;
		}
		const leftTime = left.event.recordedAt
			? Date.parse(left.event.recordedAt)
			: Number.NaN;
		const rightTime = right.event.recordedAt
			? Date.parse(right.event.recordedAt)
			: Number.NaN;
		if (
			!Number.isNaN(leftTime) &&
			!Number.isNaN(rightTime) &&
			leftTime !== rightTime
		) {
			return leftTime - rightTime;
		}
		return left.event.eventId.localeCompare(right.event.eventId);
	});
}

function toAsyncStatus(status: ResourceStatus): AsyncStatus {
	switch (status.state) {
		case "loading":
		case "refreshing":
			return "loading";
		case "ready":
			return "ready";
		case "error":
			return "error";
		default:
			return "idle";
	}
}

function threadsRecord(context: Context, sessionId: string): ThreadsState {
	return ensureSessionRecord(context.data.sessions, sessionId).threads;
}

export function listCachedThreads(
	context: Context,
	sessionId: string,
): Thread[] {
	const threads = threadsRecord(context, sessionId);
	return threads.allIds
		.map((threadId) => threads.byId[threadId]?.value)
		.filter((thread): thread is Thread => !!thread);
}

export function getThreadCacheStatus(
	context: Context,
	sessionId: string,
): AsyncStatus {
	return toAsyncStatus(threadsRecord(context, sessionId).status);
}

export function peekThreadInCache(
	context: Context,
	sessionId: string,
	threadId: string,
): Thread | null {
	return threadsRecord(context, sessionId).byId[threadId]?.value ?? null;
}

export function applyThreadSnapshotToCache(
	context: Context,
	sessionId: string,
	thread: Thread,
): void {
	const threads = threadsRecord(context, sessionId);
	upsertById(threads, thread.id, {
		...(threads.byId[thread.id] ?? createThreadRecord(thread.id)),
		value: thread,
		status: createReadyStatus(),
	});
	threads.status = createReadyStatus();
}

export function resetThreadsCache(context: Context, sessionId: string): void {
	const threads = threadsRecord(context, sessionId);
	threads.byId = {};
	threads.allIds = [];
	threads.status = createIdleStatus();
}

function applyThreadsSnapshotToCache(
	context: Context,
	sessionId: string,
	threads: Thread[],
): void {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	applyThreadsSnapshotToRecord(record, threads);
}

export function applyThreadsSnapshotToRecord(
	record: SessionRecord,
	threads: Thread[],
): void {
	const existingById = record.threads.byId;
	record.threads.byId = {};
	record.threads.allIds = [];
	for (const thread of threads) {
		upsertById(record.threads, thread.id, {
			...(existingById[thread.id] ?? createThreadRecord(thread.id)),
			value: thread,
			status: createReadyStatus(),
		});
	}
	record.threads.status = createReadyStatus();
}

export function preserveThreadRuntimeState(
	target: SessionRecord,
	source: SessionRecord,
): void {
	for (const threadId of target.threads.allIds) {
		const sourceContent = source.threads.byId[threadId]?.content;
		const targetRecord = target.threads.byId[threadId];
		if (!sourceContent || !targetRecord) {
			continue;
		}
		targetRecord.content = sourceContent;
	}
}

export async function loadThreadsIntoCache(
	context: Context,
	sessionId: string,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.threads.status =
		record.threads.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };

	try {
		const response = await api.getThreads(sessionId);
		applyThreadsSnapshotToCache(context, sessionId, response.threads);
	} catch (error) {
		record.threads.status = createErrorStatus(error);
		throw error;
	}
}

export async function loadThreadIntoCache(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.threads.status = createRefreshingStatus();

	try {
		const thread = await api.getThread(sessionId, threadId);
		upsertById(record.threads, thread.id, {
			...(record.threads.byId[thread.id] ?? createThreadRecord(thread.id)),
			value: thread,
			status: createReadyStatus(),
		});
		record.threads.status = createReadyStatus();
	} catch (error) {
		record.threads.status = createErrorStatus(error);
		throw error;
	}
}

export async function createThreadInCache(
	context: Context,
	sessionId: string,
	data: CreateThreadRequest,
): Promise<Thread> {
	const created = await api.createThread(sessionId, data);
	const thread = await api.getThread(sessionId, created.id);
	applyThreadSnapshotToCache(context, sessionId, thread);
	return thread;
}

export async function updateThreadInCache(
	context: Context,
	sessionId: string,
	threadId: string,
	data: UpdateThreadRequest,
): Promise<Thread> {
	await api.updateThread(sessionId, threadId, data);
	const thread = await api.getThread(sessionId, threadId);
	applyThreadSnapshotToCache(context, sessionId, thread);
	return thread;
}

export async function deleteThreadFromCache(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	await api.deleteThread(sessionId, threadId);
	const threads = threadsRecord(context, sessionId);
	removeById(threads, threadId);
	threads.status = createReadyStatus();
}

export async function createThread(
	context: Context,
	sessionId: string,
	input: CreateThreadRequest,
	options: CommandOptions = {},
): Promise<void> {
	await api.createThread(sessionId, input);
	if (options.wait) await loadThreadsIntoCache(context, sessionId);
}

export async function renameThread(
	context: Context,
	sessionId: string,
	threadId: string,
	name: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.updateThread(sessionId, threadId, { name });
	if (options.wait) await loadThreadIntoCache(context, sessionId, threadId);
}

export async function updateThread(
	context: Context,
	sessionId: string,
	threadId: string,
	input: UpdateThreadRequest,
	options: CommandOptions = {},
): Promise<void> {
	await api.updateThread(sessionId, threadId, input);
	if (options.wait) await loadThreadIntoCache(context, sessionId, threadId);
}

export async function deleteThread(
	context: Context,
	sessionId: string,
	threadId: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.deleteThread(sessionId, threadId);
	if (options.wait) await loadThreadsIntoCache(context, sessionId);
}

export async function deleteThreadWithDeactivation(
	context: Context,
	sessionId: string,
	threadId: string,
	options?: CommandOptions,
): Promise<void> {
	await deactivateThread(context, sessionId, threadId);
	await deleteThread(context, sessionId, threadId, options);
}

export async function sendMessage(
	context: Context,
	sessionId: string,
	threadId: string,
	input: SendMessageInput,
	options: CommandOptions = {},
): Promise<void> {
	await api.startChat({ ...input, sessionId, threadId });
	if (options.wait) await loadThreadIntoCache(context, sessionId, threadId);
}

export async function activateThread(
	context: Context,
	sessionId: string,
	threadId: string,
	socket: ProjectEventSocket | null,
): Promise<void> {
	if (!socket) {
		throw new Error("Project watch must be active before activating a thread");
	}

	const activationKey = threadActivationKey(sessionId, threadId);
	const existingActivation = threadActivationPromises
		.get(context)
		?.get(activationKey);
	if (existingActivation) {
		await existingActivation;
		return;
	}

	const content = ensureThreadContentState(
		ensureSessionRecord(context.data.sessions, sessionId).threads,
		threadId,
	);
	if (content.subscription) {
		await content.subscription.open();
		return;
	}
	content.status =
		content.messages.length > 0
			? createRefreshingStatus()
			: { state: "loading" };
	content.error = null;

	const work = startThreadActivation(
		context,
		sessionId,
		threadId,
		content,
		socket,
	);
	storeThreadActivation(context, activationKey, work);
	await work;
}

async function startThreadActivation(
	context: Context,
	sessionId: string,
	threadId: string,
	content: ThreadContentState,
	socket: ProjectEventSocket,
): Promise<void> {
	const { connectThreadEvents } =
		await import("$lib/context/thread-subscription");
	const subscription = connectThreadEvents(
		context,
		sessionId,
		threadId,
		socket,
	);
	content.subscription = subscription;
	await subscription.open().catch((error) => {
		if (content.subscription === subscription) {
			content.status = createErrorStatus(error);
			content.error =
				error instanceof Error ? error.message : "Thread subscription failed";
		}
		throw error;
	});
}

function storeThreadActivation(
	context: Context,
	key: string,
	work: Promise<void>,
): void {
	let activations = threadActivationPromises.get(context);
	if (!activations) {
		activations = new Map();
		threadActivationPromises.set(context, activations);
	}
	activations.set(key, work);
	void work.then(
		() => {
			if (activations.get(key) === work) {
				activations.delete(key);
			}
		},
		() => {
			if (activations.get(key) === work) {
				activations.delete(key);
			}
		},
	);
}

function clearThreadActivation(
	context: Context,
	sessionId: string,
	threadId: string,
): void {
	threadActivationPromises
		.get(context)
		?.delete(threadActivationKey(sessionId, threadId));
}

function threadActivationKey(sessionId: string, threadId: string): string {
	return `${sessionId}:${threadId}`;
}

export async function activateThreadUsingProjectSocket(
	context: Context,
	sessionId: string,
	threadId: string,
	options?: CommandOptions,
): Promise<void> {
	void options;
	await activateThread(
		context,
		sessionId,
		threadId,
		getProjectEventSocket(context),
	);
}

export async function deactivateThread(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	clearThreadActivation(context, sessionId, threadId);
	const content =
		context.data.sessions.byId[sessionId]?.threads.byId[threadId]?.content;
	if (!content) return;
	content.subscription?.close();
	content.subscription = null;
	content.isStreaming = false;
}

export function stopThreadWatches(context: Context): void {
	threadActivationPromises.delete(context);
	for (const sessionId of context.data.sessions.allIds) {
		const threads = context.data.sessions.byId[sessionId]?.threads;
		if (!threads) continue;
		for (const threadId of threads.allIds) {
			const content = threads.byId[threadId]?.content;
			content?.subscription?.close();
			if (content) content.subscription = null;
		}
	}
}
