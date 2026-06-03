import { generateId } from "ai";
import { SvelteMap } from "svelte/reactivity";

import { api } from "$lib/api-client";
import type {
	CredentialInfo,
	CredentialType,
	CreateWorkspaceRequest,
	Session,
	StartChatRequest,
	StartChatResponse,
	StartupTask,
	Workspace,
} from "$lib/api-types";
import { sortSessionsByCreatedAt } from "$lib/app/domains/app-sessions.helpers";
import type { RecentThreadEntry } from "$lib/app/thread-switcher";
import { getCommandContext } from "$lib/context/commands";
import { createSessionState } from "$lib/session/create-session-state.svelte";
import type { SessionContextValue } from "$lib/session/session-context.types";
import { createChatStreamManager } from "$lib/thread/chat-stream-manager";
import type { ChatStreamManager } from "$lib/thread/chat-stream-manager";
import { SessionStore } from "$lib/store/sessions.store.svelte";
import { RecentThreadStore } from "$lib/store/recent-threads.store.svelte";

export type AppChatRequest = Omit<StartChatRequest, "sessionId"> & {
	sessionId?: string | null;
	workspaceType?: CreateWorkspaceRequest["sourceType"] | null;
	workspacePath?: string | null;
};

export type StartChat = (data: AppChatRequest) => Promise<StartChatResponse>;

export type AppRuntime = {
	stores: {
		sessions: SessionStore;
		recentThreads: RecentThreadStore;
	};
	chatStreams: ChatStreamManager;
	sessionContexts: SvelteMap<string, SessionContextValue>;
	getDefaultModel: () => string;
	peekSession: (sessionId: string) => Session | null;
	reloadSession: (sessionId: string) => Promise<void>;
	upsertSession: (session: Session) => void;
	takeRequestedThreadId: (sessionId: string) => string | null;
	pruneRecentThread: (sessionId: string, threadId: string) => void;
	recordRecentThread: (
		entry: Omit<RecentThreadEntry, "lastAccessedAt">,
	) => void;
	refreshCredentials: () => Promise<void>;
	getCredentials: () => CredentialInfo[];
	getCredentialTypes: () => CredentialType[];
	openCredentialFlow: (provider: "github-git" | "codex") => void;
	startChat: StartChat;
};

type AppRuntimeBootstrap = {
	selectedSessionId?: string;
	selectedThreadId?: string;
};

type ProjectEvent<TData> = {
	id: string;
	type: string;
	timestamp: string;
	data: TData;
};

type SessionUpdatedEventData = {
	sessionId: string;
	sandboxStatus?: string;
	sandboxStatusMessage?: string;
};

type ThreadUpdatedEventData = {
	sessionId: string;
	threadId?: string;
	name?: string;
};

type WorkspaceUpdatedEventData = {
	workspaceId: string;
	status: string;
};

type AppRuntimeState = {
	stores: AppRuntime["stores"];
	chatStreams: ChatStreamManager;
	sessionContexts: SvelteMap<string, SessionContextValue>;
	requestedThreadIdBySession: SvelteMap<string, string>;
};

let runtimeState: AppRuntimeState | null = null;
let stores: AppRuntime["stores"];
let chatStreams: ChatStreamManager;
let sessionContexts: SvelteMap<string, SessionContextValue>;
let requestedThreadIdBySession: SvelteMap<string, string>;

let runtimeInitialized = false;
let stopProjectEvents: (() => void) | null = null;

function createRuntimeState(): AppRuntimeState {
	return {
		stores: {
			sessions: new SessionStore(),
			recentThreads: new RecentThreadStore(),
		},
		chatStreams: createChatStreamManager(),
		sessionContexts: new SvelteMap<string, SessionContextValue>(),
		requestedThreadIdBySession: new SvelteMap<string, string>(),
	};
}

function initializeRuntimeState(): AppRuntimeState {
	if (!runtimeState) {
		runtimeState = createRuntimeState();
		stores = runtimeState.stores;
		chatStreams = runtimeState.chatStreams;
		sessionContexts = runtimeState.sessionContexts;
		requestedThreadIdBySession = runtimeState.requestedThreadIdBySession;
	}
	return runtimeState;
}

function context() {
	return getCommandContext();
}

function ensurePendingSessionId(): string {
	const selection = context().view.app.selection;
	if (!selection.pendingSessionId) {
		selection.pendingSessionId = generateId();
	}
	return selection.pendingSessionId;
}

function setSessionsData(sessions: Session[]): void {
	const ctx = context();
	const sorted = sortSessionsByCreatedAt(sessions);
	ctx.data.sessions.items = sorted;
	ctx.data.sessions.byId = Object.fromEntries(
		sorted.map((session) => [session.id, session]),
	);
}

function setWorkspacesData(workspaces: Workspace[]): void {
	const ctx = context();
	ctx.data.workspaces.items = workspaces;
	ctx.data.workspaces.byId = Object.fromEntries(
		workspaces.map((workspace) => [workspace.id, workspace]),
	);
}

function syncRecentThreads(): void {
	const ctx = context();
	ctx.data.sessions.recentThreads = stores.recentThreads.entries.map(
		(entry) => {
			const liveThread = sessionContexts
				.get(entry.sessionId)
				?.threads.list.find((thread) => thread.id === entry.threadId);
			return {
				sessionId: entry.sessionId,
				threadId: entry.threadId,
				name: liveThread?.name || entry.name,
				lastAccessedAt: entry.lastAccessedAt,
			};
		},
	);
}

function refreshMountedSessionProjections(): void {
	const ctx = context();
	for (const [sessionId, sessionContext] of sessionContexts) {
		ctx.data.threads.bySessionId[sessionId] = {
			items: sessionContext.threads.list,
			byId: Object.fromEntries(
				sessionContext.threads.list.map((thread) => [thread.id, thread]),
			),
			status: sessionContext.threads.status,
			error: null,
		};
		for (const [threadId, threadContext] of sessionContext.threadContexts) {
			ctx.data.conversations.byThreadId[threadId] = {
				sessionId,
				threadId,
				messages: threadContext.messages,
				browserEventsByTurnId: threadContext.browserEventsByTurnId,
				status: threadContext.status,
				error: threadContext.error,
				isStreaming: threadContext.isStreaming,
				hasPendingQuestion: threadContext.hasPendingQuestion,
				pendingQuestionId: threadContext.pendingQuestionId,
				promptQueue: threadContext.promptQueue,
			};
		}
	}
}

function findWorkspaceBySourceAndPath(
	path: string,
	sourceType: "local" | "git",
): Workspace | null {
	const normalizedPath = path.trim();
	if (!normalizedPath) {
		return null;
	}
	return (
		context().data.workspaces.items.find(
			(workspace) =>
				workspace.sourceType === sourceType &&
				workspace.path.trim() === normalizedPath,
		) ?? null
	);
}

async function startChat({
	sessionId,
	threadId,
	workspaceId,
	workspaceType,
	workspacePath,
	...rest
}: AppChatRequest): Promise<StartChatResponse> {
	const ctx = context();
	const resolvedSessionId = sessionId || ensurePendingSessionId();
	let resolvedWorkspaceId = workspaceId ?? undefined;

	if (
		!resolvedWorkspaceId &&
		resolvedSessionId === ctx.view.app.selection.pendingSessionId &&
		workspaceType &&
		workspacePath
	) {
		const normalizedWorkspacePath = workspacePath.trim();
		if (normalizedWorkspacePath) {
			let workspace = findWorkspaceBySourceAndPath(
				normalizedWorkspacePath,
				workspaceType,
			);
			if (!workspace) {
				await refreshWorkspacesData();
				workspace = findWorkspaceBySourceAndPath(
					normalizedWorkspacePath,
					workspaceType,
				);
			}
			if (!workspace) {
				workspace = await api.createWorkspace({
					path: normalizedWorkspacePath,
					sourceType: workspaceType,
				});
				context().data.workspaces.items = [
					workspace,
					...context().data.workspaces.items,
				];
				context().data.workspaces.byId[workspace.id] = workspace;
			}
			resolvedWorkspaceId = workspace.id;
		}
	}

	return api.startChat({
		...rest,
		sessionId: resolvedSessionId,
		threadId,
		...(resolvedWorkspaceId ? { workspaceId: resolvedWorkspaceId } : {}),
	});
}

export async function refreshSessionsData(): Promise<void> {
	const ctx = context();
	ctx.data.sessions.status = "loading";
	ctx.data.sessions.error = null;
	try {
		await stores.sessions.fetch();
		setSessionsData(stores.sessions.list);
		ctx.data.sessions.status = "ready";
	} catch (error) {
		ctx.data.sessions.error =
			error instanceof Error ? error.message : "Failed to load sessions.";
		ctx.data.sessions.status = "error";
	}
	purgeMissingRecentSessions();
	syncRecentThreads();
}

export async function refreshWorkspacesData(): Promise<void> {
	const ctx = context();
	ctx.data.workspaces.status = "loading";
	ctx.data.workspaces.error = null;
	try {
		const { workspaces } = await api.getWorkspaces();
		setWorkspacesData(workspaces);
		ctx.data.workspaces.status = "ready";
	} catch (error) {
		ctx.data.workspaces.error =
			error instanceof Error ? error.message : "Failed to load workspaces.";
		ctx.data.workspaces.status = "error";
	}
}

export async function refreshModelsData(): Promise<void> {
	const ctx = context();
	ctx.data.models.status = "loading";
	ctx.data.models.error = null;
	try {
		const { models } = await api.getProjectModels();
		ctx.data.models.items = models;
		ctx.data.models.byId = Object.fromEntries(
			models.map((model) => [model.id, model]),
		);
		ctx.data.models.status = "ready";
	} catch (error) {
		ctx.data.models.error =
			error instanceof Error ? error.message : "Failed to load models.";
		ctx.data.models.status = "error";
	}
}

export async function refreshStartupTasksData(): Promise<void> {
	const ctx = context();
	ctx.data.startupTasks.status = "loading";
	ctx.data.startupTasks.error = null;
	try {
		const startupTasks = (await api.getSystemStatus()).startupTasks ?? [];
		ctx.data.startupTasks.items = startupTasks;
		ctx.data.startupTasks.byId = Object.fromEntries(
			startupTasks.map((task) => [task.id, task]),
		);
		ctx.view.app.startupTasks.visibleIds = startupTasks
			.filter((task) => task.state !== "completed")
			.map((task) => task.id);
		ctx.view.app.startupTasks.hasActiveTasks = startupTasks.some(
			(task) => task.state === "pending" || task.state === "in_progress",
		);
		ctx.data.startupTasks.status = "ready";
	} catch (error) {
		ctx.data.startupTasks.error =
			error instanceof Error ? error.message : "Failed to load startup tasks.";
		ctx.data.startupTasks.status = "error";
	}
}

export async function refreshCredentialsData(): Promise<void> {
	const ctx = context();
	ctx.data.credentials.status = "loading";
	ctx.data.credentials.error = null;
	try {
		const [credentialsResult, typesResult] = await Promise.all([
			api.getCredentials(),
			api.getCredentialTypes(),
		]);
		ctx.data.credentials.items = credentialsResult.credentials;
		ctx.data.credentials.byId = Object.fromEntries(
			credentialsResult.credentials.map((credential) => [
				credential.id,
				credential,
			]),
		);
		ctx.data.credentials.types = typesResult.credentialTypes;
		ctx.data.credentials.status = "ready";
	} catch (error) {
		ctx.data.credentials.error =
			error instanceof Error ? error.message : "Failed to load credentials.";
		ctx.data.credentials.status = "error";
	}
}

function purgeMissingRecentSessions(): void {
	const validSessionIds = new Set(
		stores.sessions.list.map((session) => session.id),
	);
	for (const entry of stores.recentThreads.entries) {
		if (!validSessionIds.has(entry.sessionId)) {
			removeSessionFromMemory(entry.sessionId);
		}
	}
	const selectedSessionId = context().view.app.selection.sessionId;
	if (selectedSessionId && !validSessionIds.has(selectedSessionId)) {
		removeSessionFromMemory(selectedSessionId);
	}
}

export function initializeAppRuntime(bootstrap: AppRuntimeBootstrap): void {
	initializeRuntimeState();
	if (runtimeInitialized) {
		return;
	}
	runtimeInitialized = true;
	ensurePendingSessionId();
	if (bootstrap.selectedSessionId && bootstrap.selectedThreadId) {
		requestedThreadIdBySession.set(
			bootstrap.selectedSessionId,
			bootstrap.selectedThreadId,
		);
	}
}

export function syncRuntimeProjections(): void {
	const ctx = context();
	setSessionsData(stores.sessions.list);
	syncRecentThreads();
	refreshMountedSessionProjections();
	ctx.view.app.navigation.mountedSessionIds = [
		ctx.view.app.selection.sessionId,
		...sessionContexts.keys(),
		ctx.view.app.selection.pendingSessionId,
	]
		.filter((sessionId): sessionId is string => !!sessionId)
		.filter((sessionId, index, values) => values.indexOf(sessionId) === index);
}

export function shouldLoadRuntimeSession(
	sessionId: string,
	options?: { includePending?: boolean },
): boolean {
	if (!sessionId) {
		return false;
	}
	const ctx = context();
	const session = stores.sessions.peek(sessionId);
	return (
		sessionId === ctx.view.app.selection.sessionId ||
		(!!options?.includePending &&
			sessionId === ctx.view.app.selection.pendingSessionId) ||
		(!!session && session.sandboxStatus !== "stopped")
	);
}

export function selectRuntimeSession(sessionId: string): void {
	const ctx = context();
	ctx.view.app.selection.sessionId = sessionId;
	ctx.view.app.selection.threadId =
		sessionContexts.get(sessionId)?.threads.selectedId ?? null;
	stores.sessions.ensure(sessionId);
	void reloadRuntimeSession(sessionId);
	syncRuntimeProjections();
}

export function openRuntimeThread(sessionId: string, threadId: string): void {
	requestedThreadIdBySession.set(sessionId, threadId);
	const sessionContext = sessionContexts.get(sessionId);
	if (sessionContext) {
		sessionContext.threads.select(threadId);
	}
	selectRuntimeSession(sessionId);
	context().view.app.selection.threadId = threadId;
}

export function startNewRuntimeSession(): void {
	const ctx = context();
	ctx.view.app.selection.pendingSessionId = generateId();
	ctx.view.app.selection.sessionId = null;
	ctx.view.app.selection.threadId = null;
	stores.recentThreads.clearTrackedSelection();
	syncRuntimeProjections();
}

export async function createRuntimeThread(
	sessionId: string,
): Promise<string | null> {
	if (!stores.sessions.peek(sessionId)) {
		selectRuntimeSession(sessionId);
		return null;
	}
	const sessionContext = sessionContexts.get(sessionId);
	if (!sessionContext) {
		selectRuntimeSession(sessionId);
		return null;
	}
	const threadId = await sessionContext.threads.create();
	if (!threadId) {
		return null;
	}
	openRuntimeThread(sessionId, threadId);
	return threadId;
}

export async function renameRuntimeSession(
	sessionId: string,
	nextName: string,
): Promise<boolean> {
	const trimmedName = nextName.trim();
	if (!trimmedName || !stores.sessions.peek(sessionId)) {
		return false;
	}
	const session = await stores.sessions.update(sessionId, {
		displayName: trimmedName,
	});
	context().data.sessions.byId[session.id] = session;
	setSessionsData(stores.sessions.list);
	try {
		const { threads } = await api.getThreads(sessionId);
		const primaryThread =
			threads.length === 1 && threads[0]?.id === sessionId ? threads[0] : null;
		if (primaryThread) {
			const updatedThread = await api.updateThread(
				sessionId,
				primaryThread.id,
				{
					name: trimmedName,
				},
			);
			sessionContexts.get(sessionId)?.stores.threads.upsert(updatedThread);
		}
	} catch (error) {
		console.error("[AppRuntime] Failed to sync primary thread name:", error);
	}
	syncRuntimeProjections();
	return true;
}

export async function stopRuntimeSession(sessionId: string): Promise<boolean> {
	if (!stores.sessions.peek(sessionId)) {
		return false;
	}
	const session = await stores.sessions.stop(sessionId);
	context().data.sessions.byId[session.id] = session;
	setSessionsData(stores.sessions.list);
	return true;
}

export async function deleteRuntimeSession(
	sessionId: string,
): Promise<boolean> {
	if (!stores.sessions.peek(sessionId)) {
		return false;
	}
	await stores.sessions.remove(sessionId);
	removeSessionFromMemory(sessionId);
	syncRuntimeProjections();
	return true;
}

export async function renameRuntimeThread(
	sessionId: string,
	threadId: string,
	nextName: string,
): Promise<boolean> {
	const renamed =
		(await sessionContexts
			.get(sessionId)
			?.threads.rename(threadId, nextName)) ?? false;
	syncRuntimeProjections();
	return renamed;
}

export async function deleteRuntimeThread(
	sessionId: string,
	threadId: string,
): Promise<boolean> {
	const deleted =
		(await sessionContexts.get(sessionId)?.threads.remove(threadId)) ?? false;
	if (deleted) {
		stores.recentThreads.pruneThread(sessionId, threadId);
	}
	syncRuntimeProjections();
	return deleted;
}

export async function reloadRuntimeSession(sessionId: string): Promise<void> {
	await stores.sessions.fetchOne(sessionId);
	setSessionsData(stores.sessions.list);
}

export function removeSessionFromMemory(sessionId: string): boolean {
	sessionContexts.get(sessionId)?.dispose();
	sessionContexts.delete(sessionId);
	requestedThreadIdBySession.delete(sessionId);
	stores.recentThreads.pruneSession(sessionId);
	const ctx = context();
	if (ctx.view.app.selection.sessionId === sessionId) {
		ctx.view.app.selection.sessionId = null;
		ctx.view.app.selection.threadId = null;
	}
	if (ctx.view.app.selection.pendingSessionId === sessionId) {
		ctx.view.app.selection.pendingSessionId = generateId();
	}
	const existed = stores.sessions.peek(sessionId) !== null;
	stores.sessions.evict(sessionId);
	setSessionsData(stores.sessions.list);
	return existed;
}

export function ensureRuntimeSessionState(
	sessionId?: string | null,
): SessionContextValue {
	const ctx = context();
	const resolvedSessionId =
		sessionId || ctx.view.app.selection.sessionId || ensurePendingSessionId();
	let sessionContext = sessionContexts.get(resolvedSessionId);
	if (!sessionContext) {
		sessionContext = createSessionState(runtime, startChat, resolvedSessionId);
		sessionContexts.set(resolvedSessionId, sessionContext);
	}
	syncRuntimeProjections();
	return sessionContext;
}

export function releaseRuntimeSessionState(session: SessionContextValue): void {
	if (sessionContexts.get(session.sessionId) === session) {
		sessionContexts.delete(session.sessionId);
		session.dispose();
		syncRuntimeProjections();
	}
}

function refreshSessionArtifacts(sessionId: string): void {
	const sessionContext = sessionContexts.get(sessionId);
	if (!sessionContext) {
		return;
	}
	sessionContext.services.invalidate();
	sessionContext.hooks.invalidate();
	void sessionContext.files.refresh().catch((error) => {
		console.error("[WS] Failed to refresh session files:", error);
	});
}

function startProjectEventsSubscription(): () => void {
	const subscription = chatStreams.subscribeProjectEvents({
		onError: (error) => {
			console.error("[WS] Project events connection error:", error);
		},
	});
	const handleSessionUpdated = (event: MessageEvent<string>) => {
		try {
			const payload = JSON.parse(
				event.data,
			) as ProjectEvent<SessionUpdatedEventData>;
			const sessionData = payload.data;
			if (!sessionData?.sessionId) {
				return;
			}
			if (sessionData.sandboxStatus === "removed") {
				removeSessionFromMemory(sessionData.sessionId);
				return;
			}
			void reloadRuntimeSession(sessionData.sessionId).then(
				syncRuntimeProjections,
			);
		} catch (error) {
			console.error("[WS] Failed to parse session_updated event:", error);
		}
	};
	const handleConnected = () => {
		console.debug("[WS] Connected to project events stream");
		void refreshSessionsData();
		void refreshStartupTasksData();
	};
	const handleWorkspaceUpdated = (event: MessageEvent<string>) => {
		try {
			const payload = JSON.parse(
				event.data,
			) as ProjectEvent<WorkspaceUpdatedEventData>;
			if (!payload.data?.workspaceId) {
				return;
			}
			void api.getWorkspace(payload.data.workspaceId).then((workspace) => {
				context().data.workspaces.byId[workspace.id] = workspace;
				context().data.workspaces.items = context().data.workspaces.items.some(
					(item) => item.id === workspace.id,
				)
					? context().data.workspaces.items.map((item) =>
							item.id === workspace.id ? workspace : item,
						)
					: [workspace, ...context().data.workspaces.items];
			});
		} catch (error) {
			console.error("[WS] Failed to parse workspace_updated event:", error);
		}
	};
	const handleThreadUpdated = (event: MessageEvent<string>) => {
		try {
			const payload = JSON.parse(
				event.data,
			) as ProjectEvent<ThreadUpdatedEventData>;
			const threadData = payload.data;
			if (!threadData?.sessionId) {
				return;
			}
			void reloadRuntimeSession(threadData.sessionId);
			const sessionContext = sessionContexts.get(threadData.sessionId);
			if (!sessionContext) {
				return;
			}
			if (threadData.threadId) {
				void sessionContext.threads
					.refreshThread(threadData.threadId)
					.then(syncRuntimeProjections);
				return;
			}
			refreshSessionArtifacts(threadData.sessionId);
			void sessionContext.threads.refresh().then(syncRuntimeProjections);
		} catch (error) {
			console.error("[WS] Failed to parse thread_updated event:", error);
		}
	};
	const handleStartupTaskUpdated = (event: MessageEvent<string>) => {
		try {
			const payload = JSON.parse(event.data) as ProjectEvent<StartupTask>;
			if (!payload.data?.id) {
				return;
			}
			void refreshStartupTasksData();
		} catch (error) {
			console.error("[WS] Failed to parse startup_task_updated event:", error);
		}
	};
	subscription.eventSource.addEventListener(
		"session_updated",
		handleSessionUpdated,
	);
	subscription.eventSource.addEventListener("connected", handleConnected);
	subscription.eventSource.addEventListener(
		"workspace_updated",
		handleWorkspaceUpdated,
	);
	subscription.eventSource.addEventListener(
		"thread_updated",
		handleThreadUpdated,
	);
	subscription.eventSource.addEventListener(
		"startup_task_updated",
		handleStartupTaskUpdated,
	);
	return () => {
		subscription.eventSource.removeEventListener(
			"session_updated",
			handleSessionUpdated,
		);
		subscription.eventSource.removeEventListener("connected", handleConnected);
		subscription.eventSource.removeEventListener(
			"workspace_updated",
			handleWorkspaceUpdated,
		);
		subscription.eventSource.removeEventListener(
			"thread_updated",
			handleThreadUpdated,
		);
		subscription.eventSource.removeEventListener(
			"startup_task_updated",
			handleStartupTaskUpdated,
		);
		subscription.unsubscribe();
	};
}

export async function refreshRuntimeData(): Promise<void> {
	await Promise.all([
		refreshSessionsData(),
		refreshWorkspacesData(),
		refreshStartupTasksData(),
		refreshModelsData(),
		refreshCredentialsData(),
	]);
	syncRuntimeProjections();
}

export function connectRuntimeProjectEvents(): () => void {
	if (!stopProjectEvents) {
		stopProjectEvents = startProjectEventsSubscription();
	}
	return () => {
		stopProjectEvents?.();
		stopProjectEvents = null;
	};
}

export const runtime: AppRuntime = {
	get stores() {
		return initializeRuntimeState().stores;
	},
	get chatStreams() {
		return initializeRuntimeState().chatStreams;
	},
	get sessionContexts() {
		return initializeRuntimeState().sessionContexts;
	},
	getDefaultModel: () => context().view.app.preferences.defaultModel,
	peekSession: (sessionId) => stores.sessions.peek(sessionId),
	reloadSession: reloadRuntimeSession,
	upsertSession: (session) => {
		stores.sessions.upsert(session);
		setSessionsData(stores.sessions.list);
	},
	takeRequestedThreadId: (sessionId) => {
		const threadId = requestedThreadIdBySession.get(sessionId) ?? null;
		requestedThreadIdBySession.delete(sessionId);
		return threadId;
	},
	pruneRecentThread: (sessionId, threadId) => {
		stores.recentThreads.pruneThread(sessionId, threadId);
		syncRecentThreads();
	},
	recordRecentThread: (entry) => {
		stores.recentThreads.recordSelection(entry);
		syncRecentThreads();
	},
	refreshCredentials: refreshCredentialsData,
	getCredentials: () => context().data.credentials.items,
	getCredentialTypes: () => context().data.credentials.types,
	openCredentialFlow: (provider) => {
		context().view.app.dialogs.settings.open = true;
		context().view.app.dialogs.settings.tab = "credentials";
		context().view.app.dialogs.credentials.flowIntent = provider;
		context().view.app.dialogs.credentials.open = true;
	},
	startChat,
};
