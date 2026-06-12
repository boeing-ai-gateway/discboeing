import type {
	Context,
	SessionViewState,
	ThreadViewState,
} from "$lib/context/context.types";
import {
	createInitialSessionViewState,
	createInitialThreadViewState,
} from "$lib/context/initial-state";
import { lastSessionWorkspaceStore } from "$lib/context/stores/last-session-workspace";

export function ensureSessionView(
	context: Context,
	sessionId: string,
): SessionViewState {
	context.view.sessions[sessionId] ??= createInitialSessionViewState(sessionId);
	return context.view.sessions[sessionId];
}

export async function mountSessionView(
	context: Context,
	sessionId: string,
): Promise<void> {
	ensureSessionView(context, sessionId);
}

export function ensureThreadView(
	context: Context,
	sessionId: string,
	threadId: string,
): ThreadViewState {
	const session = ensureSessionView(context, sessionId);
	session.threads[threadId] ??= createInitialThreadViewState(
		sessionId,
		threadId,
	);
	return session.threads[threadId];
}

export async function mountThreadView(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	ensureThreadView(context, sessionId, threadId);
}

export async function setSessionHooksExpanded(
	context: Context,
	sessionId: string,
	expanded: boolean,
): Promise<void> {
	const session = ensureSessionView(context, sessionId);
	session.hooks.expanded = expanded;
}

export async function setPendingWorkspaceSandboxProviderId(
	context: Context,
	sessionId: string,
	providerId: string,
): Promise<void> {
	const session = ensureSessionView(context, sessionId);
	session.pendingWorkspace.sandboxProviderId = providerId;
}

export async function setLastSessionWorkspaceSelection(
	context: Context,
	option: string,
): Promise<void> {
	context.view.app.lastSessionWorkspaceSelection =
		lastSessionWorkspaceStore.set(context.data.project.id, option);
}

export async function resetPendingWorkspaceSetup(
	context: Context,
	sessionId: string,
): Promise<void> {
	const session = ensureSessionView(context, sessionId);
	session.pendingWorkspace.option = "";
	session.pendingWorkspace.branch = "";
	session.pendingWorkspace.sourceInput = "";
	session.pendingWorkspace.validation = null;
	session.pendingWorkspace.validating = false;
	session.pendingWorkspace.setupMessage = null;
	session.pendingWorkspace.sandboxProviderId = "";
}
