import type { Context, SessionDockViewKind } from "$lib/context/context.types";
import { ensureSessionView, ensureThreadView } from "$lib/context/domains/view";
import { threadSelectionStore } from "$lib/context/stores/thread-selection";
import { generateId } from "ai";

export async function setDesktopSidebarOpen(
	context: Context,
	open: boolean,
): Promise<void> {
	context.view.navigation.desktopSidebarOpen = open;
}

export async function setMobileSidebarOpen(
	context: Context,
	open: boolean,
): Promise<void> {
	context.view.navigation.mobileSidebarOpen = open;
}

export async function toggleMobileSidebarOpen(context: Context): Promise<void> {
	context.view.navigation.mobileSidebarOpen =
		!context.view.navigation.mobileSidebarOpen;
}

export async function startNewSession(context: Context): Promise<void> {
	const sessionId = generateId();
	context.view.selection.pendingSessionId = sessionId;
	context.view.selection.sessionId = null;
	context.view.selection.threadId = null;
	threadSelectionStore.clear();
	if (!context.view.navigation.mountedSessionIds.includes(sessionId)) {
		context.view.navigation.mountedSessionIds.push(sessionId);
	}
}

export async function selectSession(
	context: Context,
	sessionId: string,
): Promise<void> {
	const storedSelection = threadSelectionStore.set({
		sessionId,
		threadId: sessionId,
	});
	context.view.selection.sessionId = storedSelection.sessionId;
	context.view.selection.threadId = storedSelection.threadId;
	context.view.selection.requestedThreadIdBySessionId[sessionId] =
		storedSelection.threadId;
	ensureThreadView(context, sessionId, storedSelection.threadId);
	if (!context.view.navigation.mountedSessionIds.includes(sessionId)) {
		context.view.navigation.mountedSessionIds.push(sessionId);
	}
}

export async function openThread(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	await selectSession(context, sessionId);
	context.view.selection.threadId = threadId;
	context.view.selection.requestedThreadIdBySessionId[sessionId] = threadId;
	ensureThreadView(context, sessionId, threadId);
	const storedSelection = threadSelectionStore.set({ sessionId, threadId });
	context.view.selection.sessionId = storedSelection.sessionId;
	context.view.selection.threadId = storedSelection.threadId;
}

export async function toggleSelectedSessionView(
	context: Context,
	viewKind: SessionDockViewKind,
): Promise<void> {
	const sessionId =
		context.view.selection.sessionId ?? context.view.selection.pendingSessionId;
	if (!sessionId) {
		return;
	}
	const sessionView = ensureSessionView(context, sessionId);
	sessionView.workspace.activeView =
		sessionView.workspace.activeView === viewKind ? "chat" : viewKind;
}
