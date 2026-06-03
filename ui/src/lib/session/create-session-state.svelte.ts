import { SvelteMap } from "svelte/reactivity";

import { canLoadSessionThreads, SessionStatus } from "$lib/api-constants";
import type { AppRuntime, StartChat } from "$lib/app/app-runtime.svelte";
import { createThreadState } from "$lib/thread/create-thread-state.svelte";
import { createSessionCommandsDomain } from "$lib/session/domains/session-commands.svelte";
import { createSessionFilesDomain } from "$lib/session/domains/session-files.svelte";
import { createSessionHooksDomain } from "$lib/session/domains/session-hooks.svelte";
import { createSessionServicesDomain } from "$lib/session/domains/session-services.svelte";
import { createSessionThreadsDomain } from "$lib/session/domains/session-threads.svelte";
import type {
	SessionContextValue,
	SessionStores,
	ThreadContextValue,
} from "$lib/session/session-context.types";
import {
	DESKTOP_SERVICE_ID,
	VSCODE_SERVICE_ID,
} from "$lib/session/service-ids";
import { createSessionViewState } from "$lib/session/view/create-session-view-state.svelte";
import { ThreadStore } from "$lib/store/threads.store.svelte";
import {
	buildUserMessageParts,
	createUserMessage,
} from "$lib/session/domains/session-domain.helpers";

export function createSessionState(
	runtime: AppRuntime,
	startChat: StartChat,
	sessionId: string,
): SessionContextValue {
	let selectedThreadId = $state<string | null>(null);

	function getCurrentSession() {
		return runtime.peekSession(sessionId);
	}

	function hasCurrentSession() {
		return getCurrentSession() !== null;
	}

	function canLoadCurrentSandboxData() {
		return getCurrentSession()?.sandboxStatus === SessionStatus.READY;
	}

	function canLoadCurrentThreadData() {
		return canLoadSessionThreads(getCurrentSession()?.sandboxStatus);
	}

	const ui = createSessionViewState({
		getFiles: () => filesDomain.list,
		getServices: () =>
			services.list
				.filter(
					(service) =>
						service.id !== DESKTOP_SERVICE_ID &&
						service.id !== VSCODE_SERVICE_ID,
				)
				.map((service) => service.id),
	});

	const stores: SessionStores = {
		threads: new ThreadStore({
			sessionId,
			enabled: canLoadCurrentThreadData,
		}),
	};

	const filesDomain = createSessionFilesDomain({
		sessionId,
		hasSession: canLoadCurrentSandboxData,
		getSelectedFile: () => ui.selectedFile,
		openFile: ui.openFile,
	});

	const threads = createSessionThreadsDomain({
		store: stores.threads,
		sessionId,
		hasSession: canLoadCurrentThreadData,
		getSession: getCurrentSession,
		getSelectedId: () => selectedThreadId,
		setSelectedId: (threadId) => {
			selectedThreadId = threadId;
		},
		takeRequestedId: () => runtime.takeRequestedThreadId(sessionId),
		onThreadRemoved: (threadId) => {
			runtime.pruneRecentThread(sessionId, threadId);
		},
	});

	const hooks = createSessionHooksDomain({
		sessionId,
		hasSession: canLoadCurrentSandboxData,
	});

	const services = createSessionServicesDomain({
		sessionId,
		hasSession: canLoadCurrentSandboxData,
		getActiveServiceId: () => ui.activeServiceId,
		openService: ui.openService,
	});

	const threadContexts = new SvelteMap<string, ThreadContextValue>();
	const conversationScrollTopByThreadId = new SvelteMap<string, number>();

	const ensureThread: SessionContextValue["ensureThread"] = (threadId) => {
		const existing = threadContexts.get(threadId);
		if (existing) {
			return existing;
		}

		const thread = createThreadState(runtime, startChat, context, threadId);
		threadContexts.set(threadId, thread);
		return thread;
	};

	const submit: SessionContextValue["submit"] = async (text, options = {}) => {
		const threadId = options.threadId ?? threads.selectedId ?? sessionId;
		const thread = threadContexts.get(threadId);
		if (thread) {
			return thread.submit({ parts: buildUserMessageParts(text) });
		}

		return startChat({
			sessionId,
			threadId,
			messages: [createUserMessage(text)],
		});
	};

	const commands = createSessionCommandsDomain({
		runtime,
		sessionId,
		hasSession: canLoadCurrentThreadData,
		getSelectedThreadId: () => threads.selectedId ?? sessionId,
		submit,
	});

	function dispose() {
		filesDomain.dispose();
		for (const context of threadContexts.values()) {
			context.dispose();
		}
		threadContexts.clear();
	}

	const context: SessionContextValue = {
		get sessionId() {
			return sessionId;
		},
		get isPending() {
			return !hasCurrentSession();
		},
		get current() {
			return getCurrentSession();
		},
		dispose,
		ensureThread,
		stores,
		ui,
		threads,
		hooks,
		files: filesDomain,
		services,
		commands,
		submit,
		threadContexts,
		conversationScrollTopByThreadId,
	};

	return context;
}
