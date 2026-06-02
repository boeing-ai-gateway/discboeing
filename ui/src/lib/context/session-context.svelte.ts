import { getContext, setContext } from "svelte";
import { SvelteMap } from "svelte/reactivity";

import { canLoadSessionThreads, SessionStatus } from "$lib/api-constants";
import type { AppContext, StartChat } from "$lib/context/app-context.svelte";
import { createThreadContext } from "$lib/context/thread-context.svelte";
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

const SESSION_CONTEXT_KEY = Symbol.for("discobot-ui-session-context");

export function createSessionContext(
	app: AppContext,
	startChat: StartChat,
	sessionId: string,
): SessionContextValue {
	let selectedThreadId = $state<string | null>(null);

	function getCurrentSession() {
		return app.sessions.peek(sessionId);
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
		takeRequestedId: () => app.sessions.takeRequestedThreadId(sessionId),
		onThreadRemoved: (threadId) => {
			app.stores.recentThreads.pruneThread(sessionId, threadId);
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

		const thread = createThreadContext(app, startChat, context, threadId);
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
		app,
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

export function setSessionContext(
	context: SessionContextValue,
): SessionContextValue {
	setContext(SESSION_CONTEXT_KEY, context);
	return context;
}

export function useSessionContext(): SessionContextValue {
	const context = getContext<SessionContextValue | undefined>(
		SESSION_CONTEXT_KEY,
	);
	if (!context) {
		throw new Error(
			"useSessionContext must be used within SessionContext provider",
		);
	}
	return context;
}

export function getSessionContextIfPresent(): SessionContextValue | undefined {
	return getContext<SessionContextValue | undefined>(SESSION_CONTEXT_KEY);
}
