import { getContext, setContext } from "svelte";
import { SvelteMap } from "svelte/reactivity";

import type { AppContext } from "$lib/context/app-context.svelte";
import { useAppContext } from "$lib/context/app-context.svelte";
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
import { DESKTOP_SERVICE_ID } from "$lib/shell-types";
import { createSessionViewState } from "$lib/session/view/create-session-view-state.svelte";
import { ThreadStore } from "$lib/store/threads.store.svelte";

const SESSION_CONTEXT_KEY = Symbol.for("discobot-ui-session-context");

function createSessionContext(
	app: AppContext,
	sessionId: string,
): SessionContextValue {
	let selectedThreadId = $state<string | null>(null);

	const current = $derived.by(() => {
		return app.sessions.peek(sessionId);
	});

	const hasSession = $derived.by(() => current !== null);
	const isPending = $derived.by(() => !hasSession);

	const ui = createSessionViewState({
		getFiles: () => filesDomain.list,
		getServices: () =>
			services.list
				.filter((service) => service.id !== DESKTOP_SERVICE_ID)
				.map((service) => service.id),
	});

	const stores: SessionStores = {
		threads: new ThreadStore(),
	};

	const filesDomain = createSessionFilesDomain({
		sessionId,
		hasSession: () => hasSession,
		getSelectedFile: () => ui.selectedFile,
		openFile: ui.openFile,
	});

	const threads = createSessionThreadsDomain({
		store: stores.threads,
		sessionId,
		hasSession: () => hasSession,
		getSession: () => current,
		getSelectedId: () => selectedThreadId,
		setSelectedId: (threadId) => {
			selectedThreadId = threadId;
		},
		takeRequestedId: () => app.sessions.takeRequestedThreadId(sessionId),
	});

	const hooks = createSessionHooksDomain({
		sessionId,
		hasSession: () => hasSession,
	});

	const services = createSessionServicesDomain({
		sessionId,
		hasSession: () => hasSession,
		getActiveServiceId: () => ui.activeServiceId,
		openService: ui.openService,
	});

	const commands = createSessionCommandsDomain({
		app,
		sessionId,
		hasSession: () => hasSession,
		getSelectedThreadId: () => threads.selectedId ?? sessionId,
	});

	const threadContexts = new SvelteMap<string, ThreadContextValue>();
	const conversationScrollTopByThreadId = new SvelteMap<string, number>();

	function dispose() {
		filesDomain.dispose();
		for (const context of threadContexts.values()) {
			context.dispose();
		}
		threadContexts.clear();
	}

	return {
		get sessionId() {
			return sessionId;
		},
		get isPending() {
			return isPending;
		},
		get current() {
			return current;
		},
		dispose,
		stores,
		ui,
		threads,
		hooks,
		files: filesDomain,
		services,
		commands,
		threadContexts,
		conversationScrollTopByThreadId,
	};
}

export function ensureSessionContext(
	app: AppContext,
	sessionId: string,
): SessionContextValue {
	let context = app.sessions.sessionContexts.get(sessionId);
	if (!context) {
		context = createSessionContext(app, sessionId);
		app.sessions.sessionContexts.set(sessionId, context);
	}
	return context;
}

export function setSessionContext(sessionId?: string): SessionContextValue {
	const app = useAppContext();
	const resolvedSessionId =
		sessionId ?? app.sessions.selectedId ?? app.sessions.pendingId;
	const context = ensureSessionContext(app, resolvedSessionId);
	setContext(SESSION_CONTEXT_KEY, context);
	return context;
}

export function useSessionContext(sessionId?: string): SessionContextValue {
	if (sessionId !== undefined) {
		return setSessionContext(sessionId);
	}
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
