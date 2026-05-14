import { generateId } from "ai";
import { SvelteMap } from "svelte/reactivity";

import { api } from "$lib/api-client";
import type { AppSessions } from "$lib/app/app-context.types";
import { sortSessionsByCreatedAt } from "$lib/app/domains/app-sessions.helpers";
import type { SessionContextValue } from "$lib/session/session-context.types";
import type { RecentThreadEntry } from "$lib/app/thread-switcher";
import type { RecentThreadStore } from "$lib/store/recent-threads.store.svelte";
import type { SessionStore } from "$lib/store/sessions.store.svelte";

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
	const requestedThreadIdBySession = new SvelteMap<string, string>();
	const sessionContexts = new SvelteMap<string, SessionContextValue>();

	if (args.initialSelectedSessionId && args.initialSelectedThreadId) {
		requestedThreadIdBySession.set(
			args.initialSelectedSessionId,
			args.initialSelectedThreadId,
		);
	}

	const selectSession = (sessionId: string) => {
		currentSelectedSessionId = sessionId;
		store.ensure(sessionId);
		void reloadSession(sessionId);
	};

	const list = $derived.by(() => sortSessionsByCreatedAt(store.list));

	const recentThreads = $derived.by(() => {
		return recentThreadStore.entries.flatMap((savedEntry) => {
			const liveThread = sessionContexts
				.get(savedEntry.sessionId)
				?.threads.list.find((thread) => thread.id === savedEntry.threadId);

			return [
				{
					sessionId: savedEntry.sessionId,
					threadId: savedEntry.threadId,
					name: liveThread?.name || savedEntry.name,
					lastAccessedAt: savedEntry.lastAccessedAt,
				} satisfies RecentThreadEntry,
			];
		});
	});
	const selected = $derived.by(
		() =>
			list.find((session) => session.id === currentSelectedSessionId) ?? null,
	);

	function shouldLoadSession(
		sessionId: string,
		options?: { includePending?: boolean },
	): boolean {
		const session = store.peek(sessionId);
		return (
			sessionId === currentSelectedSessionId ||
			(!!options?.includePending && sessionId === pendingSessionId) ||
			(!!session && session.sandboxStatus !== "stopped")
		);
	}

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

	const reloadSession = async (sessionId: string) => {
		await store.fetchOne(sessionId);
	};

	const openThread = (sessionId: string, threadId: string) => {
		requestedThreadIdBySession.set(sessionId, threadId);
		const sessionContext = sessionContexts.get(sessionId);
		if (sessionContext) {
			sessionContext.threads.select(threadId);
		}
		selectSession(sessionId);
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
		get selected() {
			return selected;
		},
		peek: (sessionId) => store.peek(sessionId),
		shouldLoadSession,
		sessionContexts,
		select: selectSession,
		openThread,
		createThread: async (sessionId) => {
			if (!list.some((session) => session.id === sessionId)) {
				return null;
			}

			const sessionContext = sessionContexts.get(sessionId);
			if (!sessionContext) {
				selectSession(sessionId);
				return null;
			}

			const threadId = await sessionContext.threads.create();
			if (!threadId) {
				return null;
			}
			openThread(sessionId, threadId);
			return threadId;
		},
		startNew: () => {
			pendingSessionId = generateId();
			currentSelectedSessionId = null;
			recentThreadStore.clearTrackedSelection();
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
		stop: async (sessionId) => {
			if (!list.some((session) => session.id === sessionId)) {
				return false;
			}
			await store.stop(sessionId);
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
			return true;
		},
		removeFromMemory,
		takeRequestedThreadId: (sessionId) => {
			const threadId = requestedThreadIdBySession.get(sessionId) ?? null;
			requestedThreadIdBySession.delete(sessionId);
			return threadId;
		},
	};
}
