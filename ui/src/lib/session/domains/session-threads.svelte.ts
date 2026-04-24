import { generateId } from "ai";

import type { Session, Thread } from "$lib/api-types";
import {
	buildImplicitThread,
	getNextSelectedThreadId,
} from "$lib/session/domains/session-domain.helpers";
import type { SessionThreadsService } from "$lib/session/session-context.types";
import type { ThreadStore } from "$lib/store/threads.store.svelte";

type CreateSessionThreadsDomainArgs = {
	store: ThreadStore;
	sessionId: string;
	hasSession: () => boolean;
	getSession: () => Session | null;
	getSelectedId: () => string | null;
	setSelectedId: (threadId: string | null) => void;
	takeRequestedId?: () => string | null;
	onThreadUpdated?: (thread: Thread) => void;
	onThreadRenamed?: (thread: Thread) => void;
	onThreadRemoved?: (threadId: string) => void;
};

export function createSessionThreadsDomain(
	args: CreateSessionThreadsDomainArgs,
): SessionThreadsService {
	const { store } = args;

	function currentList() {
		if (!args.hasSession()) {
			return buildImplicitThread(args.getSession());
		}
		return store.list.length > 0
			? store.list
			: buildImplicitThread(args.getSession());
	}

	function applyRequestedThreadSelection(nextList = currentList()) {
		const requestedId = args.takeRequestedId?.();
		if (!requestedId) {
			return;
		}
		if (nextList.some((thread) => thread.id === requestedId)) {
			args.setSelectedId(requestedId);
		}
	}

	function syncSelectedThread(nextList = currentList()) {
		if (nextList.length === 0) {
			args.setSelectedId(null);
			return;
		}

		const selectedId = args.getSelectedId();
		if (selectedId && nextList.some((thread) => thread.id === selectedId)) {
			return;
		}

		if (nextList.length === 1) {
			args.setSelectedId(nextList[0]?.id ?? null);
			return;
		}

		args.setSelectedId(null);
	}

	function notifyThreadsUpdated(nextList = currentList()) {
		for (const thread of nextList) {
			args.onThreadUpdated?.(thread);
		}
	}

	let loadScheduled = false;

	function scheduleEnsureLoaded() {
		if (
			loadScheduled ||
			!args.hasSession() ||
			store.status === "loading" ||
			store.status === "ready"
		) {
			return;
		}
		loadScheduled = true;
		queueMicrotask(() => {
			loadScheduled = false;
			void ensureLoaded();
		});
	}

	async function ensureLoaded() {
		if (!args.hasSession()) {
			store.reset();
			applyRequestedThreadSelection([]);
			syncSelectedThread([]);
			return;
		}
		if (store.status === "loading" || store.status === "ready") {
			const nextList = currentList();
			applyRequestedThreadSelection(nextList);
			syncSelectedThread(nextList);
			return;
		}
		await store.fetch();
		const nextList = currentList();
		applyRequestedThreadSelection(nextList);
		syncSelectedThread(nextList);
		notifyThreadsUpdated(nextList);
	}

	const list = $derived.by(() => currentList());

	return {
		get list() {
			scheduleEnsureLoaded();
			return list;
		},
		get status() {
			return args.hasSession() ? store.status : "idle";
		},
		get selectedId() {
			scheduleEnsureLoaded();
			return args.getSelectedId();
		},
		get selected() {
			scheduleEnsureLoaded();
			return list.find((thread) => thread.id === args.getSelectedId()) ?? null;
		},
		refresh: async () => {
			if (!args.hasSession()) {
				store.reset();
				applyRequestedThreadSelection([]);
				syncSelectedThread([]);
				return;
			}
			await store.fetch();
			const nextList = currentList();
			applyRequestedThreadSelection(nextList);
			syncSelectedThread(nextList);
			notifyThreadsUpdated(nextList);
		},
		select: (threadId: string | null) => {
			if (threadId === null) {
				args.setSelectedId(null);
				return;
			}
			const selectedThread = list.find((thread) => thread.id === threadId);
			if (selectedThread) {
				args.setSelectedId(threadId);
			}
		},
		create: (name?: string) => {
			void (async () => {
				if (!args.hasSession()) {
					return;
				}

				const trimmedName = name?.trim();
				const threadId =
					store.list.length === 0 ? args.sessionId : generateId();
				const created = await store.create({
					id: threadId,
					name: trimmedName && trimmedName.length > 0 ? trimmedName : undefined,
				});
				args.setSelectedId(created.id);
			})();
		},
		rename: async (threadId: string, nextName: string): Promise<boolean> => {
			const trimmedName = nextName.trim();
			if (!trimmedName || !list.some((thread) => thread.id === threadId)) {
				return false;
			}
			if (!args.hasSession()) {
				return false;
			}

			if (store.list.length === 0 && threadId === args.sessionId) {
				const created = await store.create({
					id: threadId,
					name: trimmedName,
				});
				args.onThreadRenamed?.(created);
				return true;
			}

			const updated = await store.update(threadId, {
				name: trimmedName,
			});
			args.onThreadRenamed?.(updated);
			return true;
		},
		remove: async (threadId: string): Promise<boolean> => {
			if (!args.hasSession()) {
				return false;
			}
			if (threadId === args.sessionId) {
				return false;
			}
			if (!store.list.some((thread) => thread.id === threadId)) {
				return false;
			}
			await store.remove(threadId);
			args.onThreadRemoved?.(threadId);
			args.setSelectedId(
				getNextSelectedThreadId(currentList(), threadId, args.getSelectedId()),
			);
			syncSelectedThread();
			return true;
		},
		refreshThread: async (threadId: string) => {
			if (!args.hasSession()) {
				return;
			}
			await store.fetchOne(threadId);
			const updatedThread = currentList().find(
				(thread) => thread.id === threadId,
			);
			if (updatedThread) {
				args.onThreadUpdated?.(updatedThread);
			}
			syncSelectedThread();
		},
	};
}
