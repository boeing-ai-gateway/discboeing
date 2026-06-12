import { generateId } from "ai";

import { getContext } from "$lib/context/context.svelte";

export type AppBootstrap = {
	selectedSessionId?: string;
	selectedThreadId?: string;
};

export function initializeApp(bootstrap: AppBootstrap = {}): void {
	const context = getContext();
	const selection = context.view.selection;
	selection.pendingSessionId ||= generateId();
	selection.sessionId = bootstrap.selectedSessionId ?? selection.sessionId;
	selection.threadId = bootstrap.selectedThreadId ?? selection.threadId;
	if (selection.sessionId && selection.threadId) {
		selection.requestedThreadIdBySessionId[selection.sessionId] =
			selection.threadId;
	}
}
