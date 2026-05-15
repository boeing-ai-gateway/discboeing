import { getContext, setContext } from "svelte";
import { SvelteMap } from "svelte/reactivity";

import { SessionStatus } from "$lib/api-constants";
import type { AppContext, StartChat } from "$lib/context/app-context.svelte";
import { createThreadContext } from "$lib/context/thread-context.svelte";
import { createSessionCommandsDomain } from "$lib/session/domains/session-commands.svelte";
import { createSessionFilesDomain } from "$lib/session/domains/session-files.svelte";
import { createSessionHooksDomain } from "$lib/session/domains/session-hooks.svelte";
import { createSessionServicesDomain } from "$lib/session/domains/session-services.svelte";
import { createSessionThreadsDomain } from "$lib/session/domains/session-threads";
import type {
	SessionContextValue,
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

	function getCurrent() {
		return app.sessions.peek(sessionId);
	}

	function isPendingSession() {
		return getCurrent() === null;
	}

	function canLoadSessionData() {
		return getCurrent()?.sandboxStatus === SessionStatus.READY;
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

	const threadStore = new ThreadStore({
		sessionId,
		enabled: canLoadSessionData,
	});

	const filesDomain = createSessionFilesDomain({
		sessionId,
		canLoadSessionData,
		getSelectedFile: () => ui.selectedFile,
		openFile: ui.openFile,
	});

	const threads = createSessionThreadsDomain({
		store: threadStore,
		sessionId,
		canLoadSessionData,
		getSession: getCurrent,
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
		canLoadSessionData,
	});

	const services = createSessionServicesDomain({
		sessionId,
		canLoadSessionData,
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
		const threadId = options.threadId ?? threads.selectedId;
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
		canLoadSessionData,
		getSelectedThreadId: () => threads.selectedId,
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
			return isPendingSession();
		},
		get current() {
			return getCurrent();
		},
		dispose,
		ensureThread,
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
	const context = getSessionContextIfPresent();
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
