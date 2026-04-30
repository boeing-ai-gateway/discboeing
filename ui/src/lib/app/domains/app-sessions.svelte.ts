import { generateId } from "ai";
import { SvelteMap } from "svelte/reactivity";

import { api } from "$lib/api-client";
import type { ChatMessage } from "$lib/api-types";
import { toSessionSummaries } from "$lib/app/app-helpers";
import type { AppSessions } from "$lib/app/app-context.types";
import type { SessionContextValue } from "$lib/session/session-context.types";
import type { RecentThreadSummary } from "$lib/shell-types";
import type { RecentThreadStore } from "$lib/store/recent-threads.store.svelte";
import type { SessionStore } from "$lib/store/sessions.store.svelte";

const INITIAL_SESSION_STATUS_RETRY_DELAYS_MS = [150, 300, 500, 1000];

type CreateAppSessionsDomainArgs = {
	store: SessionStore;
	recentThreadStore: RecentThreadStore;
	initialSelectedSessionId?: string;
	initialSelectedThreadId?: string;
};

export function createAppSessionsDomain(
	args: CreateAppSessionsDomainArgs,
): AppSessions {
	const { store, recentThreadStore } = args;
	let currentSelectedSessionId = $state<string | null>(
		args.initialSelectedSessionId ?? null,
	);
	let pendingSessionId = $state<string>(generateId());
	let awaitingInitialStatusId = $state<string | null>(null);
	let initialStatusRetryTimer: ReturnType<typeof setTimeout> | null = null;
	const requestedThreadIdBySession = new SvelteMap<string, string>();
	const optimisticMessagesByThread = new SvelteMap<string, ChatMessage[]>();
	const sessionContexts = new SvelteMap<string, SessionContextValue>();

	if (args.initialSelectedSessionId && args.initialSelectedThreadId) {
		requestedThreadIdBySession.set(
			args.initialSelectedSessionId,
			args.initialSelectedThreadId,
		);
	}

	function getOptimisticMessagesKey(
		sessionId: string,
		threadId: string,
	): string {
		return `${sessionId}:${threadId}`;
	}

	function clearInitialStatusRetry(): void {
		if (initialStatusRetryTimer !== null) {
			clearTimeout(initialStatusRetryTimer);
			initialStatusRetryTimer = null;
		}
	}

	function scheduleInitialStatusRetry(sessionId: string, attempt = 0): void {
		if (awaitingInitialStatusId !== sessionId) {
			return;
		}
		const delay = INITIAL_SESSION_STATUS_RETRY_DELAYS_MS[attempt];
		if (delay === undefined) {
			return;
		}

		clearInitialStatusRetry();
		initialStatusRetryTimer = setTimeout(() => {
			initialStatusRetryTimer = null;
			if (awaitingInitialStatusId !== sessionId) {
				return;
			}
			void reloadSession(sessionId, attempt + 1);
		}, delay);
	}

	const selectSession = (sessionId: string) => {
		currentSelectedSessionId = sessionId;
		store.ensure(sessionId);
		void reloadSession(sessionId);
	};

	const list = $derived.by(() => toSessionSummaries(store.list));

	const recentThreads = $derived.by(() => {
		const sessionsById = Object.fromEntries(
			store.list.map((session) => [session.id, session] as const),
		);

		return recentThreadStore.entries.flatMap((savedEntry) => {
			const session = sessionsById[savedEntry.sessionId];
			const liveThread = sessionContexts
				.get(savedEntry.sessionId)
				?.threads.list.find((thread) => thread.id === savedEntry.threadId);

			if (session && liveThread) {
				return [
					{
						sessionId: savedEntry.sessionId,
						sessionName: session.displayName || session.name,
						sessionStatus: session.status,
						threadId: liveThread.id,
						threadName: liveThread.name,
						...(liveThread.state ? { state: liveThread.state } : {}),
						lastMessage: liveThread.lastMessage ?? "",
						lastAccessedAt: savedEntry.lastAccessedAt,
					},
				];
			}

			const fallbackSessionName =
				session?.displayName || session?.name || savedEntry.sessionName;
			const fallbackSessionStatus = session?.status ?? savedEntry.sessionStatus;
			const fallbackThreadName = savedEntry.threadName ?? fallbackSessionName;
			if (
				!fallbackSessionName ||
				!fallbackSessionStatus ||
				!fallbackThreadName
			) {
				return [];
			}

			const fallbackSummary: RecentThreadSummary = {
				sessionId: savedEntry.sessionId,
				sessionName: fallbackSessionName,
				sessionStatus: fallbackSessionStatus,
				threadId: savedEntry.threadId,
				threadName: fallbackThreadName,
				lastAccessedAt: savedEntry.lastAccessedAt,
			};
			if (savedEntry.state) {
				fallbackSummary.state = savedEntry.state;
			}
			if (savedEntry.lastMessage !== undefined) {
				fallbackSummary.lastMessage = savedEntry.lastMessage;
			}
			return [fallbackSummary];
		});
	});
	const selected = $derived.by(
		() =>
			list.find((session) => session.id === currentSelectedSessionId) ?? null,
	);

	$effect(() => {
		const selectedSessionId = currentSelectedSessionId;
		if (!selectedSessionId) {
			recentThreadStore.clearTrackedSelection();
			return;
		}

		const session = store.peek(selectedSessionId);
		const selectedThreadId =
			sessionContexts.get(selectedSessionId)?.threads.selectedId ?? null;
		const threadId = selectedThreadId ?? selectedSessionId;

		const thread = sessionContexts
			.get(selectedSessionId)
			?.threads.list.find((item) => item.id === threadId);

		// Save whatever display data we have now. The sidebar can use live thread
		// data when a session is in memory, and fall back to this saved snapshot
		// without waking stopped sessions back up.
		recentThreadStore.recordSelection({
			sessionId: selectedSessionId,
			threadId,
			...(session
				? {
						sessionName: session.displayName || session.name,
						sessionStatus: session.status,
					}
				: {}),
			...(thread
				? {
						threadName: thread.name,
						...(thread.state ? { state: thread.state } : {}),
						lastMessage: thread.lastMessage ?? "",
					}
				: session
					? {
							threadName: session.displayName || session.name,
						}
					: {}),
		});
	});

	function purgeMissingRecentSessions(): void {
		const validSessionIds = Object.fromEntries(
			store.list.map((session) => [session.id, true] as const),
		);
		const staleSessionIds: string[] = [];
		const trackStaleSessionId = (sessionId: string) => {
			if (!staleSessionIds.includes(sessionId)) {
				staleSessionIds.push(sessionId);
			}
		};
		for (const entry of recentThreadStore.entries) {
			if (!validSessionIds[entry.sessionId]) {
				trackStaleSessionId(entry.sessionId);
			}
		}
		if (
			currentSelectedSessionId &&
			!validSessionIds[currentSelectedSessionId]
		) {
			trackStaleSessionId(currentSelectedSessionId);
		}
		for (const sessionId of staleSessionIds) {
			removeFromMemory(sessionId);
		}
	}

	const removeFromMemory = (sessionId: string): boolean => {
		sessionContexts.get(sessionId)?.dispose();
		sessionContexts.delete(sessionId);
		requestedThreadIdBySession.delete(sessionId);
		recentThreadStore.pruneSession(sessionId);

		if (sessionId === currentSelectedSessionId) {
			currentSelectedSessionId = null;
		}

		if (sessionId === pendingSessionId) {
			pendingSessionId = generateId();
		}

		if (sessionId === awaitingInitialStatusId) {
			awaitingInitialStatusId = null;
			clearInitialStatusRetry();
		}

		if (!store.list.some((session) => session.id === sessionId)) {
			return false;
		}

		store.evict(sessionId);
		return true;
	};

	const refresh = async () => {
		await store.fetch();
		purgeMissingRecentSessions();
	};

	const reloadSession = async (sessionId: string, attempt = 0) => {
		await store.fetchOne(sessionId);
		if (awaitingInitialStatusId !== sessionId) {
			return;
		}

		const session = store.peek(sessionId);
		if (session?.status) {
			awaitingInitialStatusId = null;
			clearInitialStatusRetry();
			return;
		}

		scheduleInitialStatusRetry(sessionId, attempt);
	};

	return {
		get sessions() {
			return store.list;
		},
		get list() {
			return list;
		},
		get recentThreads() {
			return recentThreads;
		},
		get selectedId() {
			return currentSelectedSessionId;
		},
		get pendingId() {
			return pendingSessionId;
		},
		get awaitingInitialStatusId() {
			return awaitingInitialStatusId;
		},
		get selected() {
			return selected;
		},
		peek: (sessionId) => store.peek(sessionId),
		sessionContexts,
		select: selectSession,
		openThread: (sessionId, threadId) => {
			const sessionContext = sessionContexts.get(sessionId);
			if (sessionContext) {
				sessionContext.threads.select(threadId);
			} else {
				requestedThreadIdBySession.set(sessionId, threadId);
			}
			selectSession(sessionId);
		},
		createThread: async (sessionId) => {
			if (!list.some((session) => session.id === sessionId)) {
				return null;
			}

			const { threads } = await api.getThreads(sessionId);
			const created = await api.createThread(sessionId, {
				id: threads.some((thread) => thread.id === sessionId)
					? generateId()
					: sessionId,
			});
			const thread = await api.getThread(sessionId, created.id);
			const sessionContext = sessionContexts.get(sessionId);
			if (sessionContext) {
				sessionContext.stores.threads.upsert(thread);
				sessionContext.threads.select(thread.id);
			} else {
				requestedThreadIdBySession.set(sessionId, thread.id);
			}
			selectSession(sessionId);
			return thread.id;
		},
		startNew: () => {
			pendingSessionId = generateId();
			currentSelectedSessionId = null;
			awaitingInitialStatusId = null;
			clearInitialStatusRetry();
		},
		setAwaitingInitialStatus: (sessionId) => {
			awaitingInitialStatusId = sessionId;
			clearInitialStatusRetry();
			if (sessionId) {
				void reloadSession(sessionId);
			}
		},
		refresh,
		reloadSession,
		create: async (workspaceId) => {
			const session = await store.create(
				workspaceId ? { workspaceId } : undefined,
			);
			currentSelectedSessionId = session.id;
			await refresh();
			return session.id;
		},
		rename: async (sessionId, nextName) => {
			const trimmedName = nextName.trim();
			if (!trimmedName || !list.some((session) => session.id === sessionId)) {
				return false;
			}
			await store.update(sessionId, {
				displayName: trimmedName,
			});

			try {
				const { threads } = await api.getThreads(sessionId);
				const primaryThread =
					threads.length === 1 && threads[0]?.id === sessionId
						? threads[0]
						: null;
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
				console.error(
					"[AppSessions] Failed to sync primary thread name:",
					sessionId,
					error,
				);
			}
			return true;
		},
		remove: async (sessionId) => {
			if (!list.some((session) => session.id === sessionId)) {
				return false;
			}
			await store.remove(sessionId);
			// remove already evicts from the store; clean up context lifecycle
			sessionContexts.get(sessionId)?.dispose();
			sessionContexts.delete(sessionId);
			requestedThreadIdBySession.delete(sessionId);
			recentThreadStore.pruneSession(sessionId);
			if (sessionId === currentSelectedSessionId) {
				currentSelectedSessionId = null;
			}
			if (sessionId === awaitingInitialStatusId) {
				awaitingInitialStatusId = null;
				clearInitialStatusRetry();
			}
			return true;
		},
		removeFromMemory,
		takeRequestedThreadId: (sessionId) => {
			const threadId = requestedThreadIdBySession.get(sessionId) ?? null;
			requestedThreadIdBySession.delete(sessionId);
			return threadId;
		},
		stageOptimisticMessages: (sessionId, threadId, messages) => {
			const key = getOptimisticMessagesKey(sessionId, threadId);
			if (messages.length === 0) {
				optimisticMessagesByThread.delete(key);
				return;
			}
			optimisticMessagesByThread.set(key, messages);
		},
		takeOptimisticMessages: (sessionId, threadId) => {
			const key = getOptimisticMessagesKey(sessionId, threadId);
			const messages = optimisticMessagesByThread.get(key) ?? [];
			optimisticMessagesByThread.delete(key);
			return messages;
		},
	};
}
