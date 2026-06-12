import type { Context } from "$lib/context/context.types";

function shouldLoadSession(
	context: Context,
	sessionId: string,
	options?: { includePending?: boolean },
): boolean {
	if (!sessionId) {
		return false;
	}
	const session = context.data.sessions.byId[sessionId]?.value;
	return (
		sessionId === context.view.selection.sessionId ||
		(!!options?.includePending &&
			sessionId === context.view.selection.pendingSessionId) ||
		(!!session && session.sandboxStatus !== "stopped")
	);
}

export function shouldLoadSessionToolbar(
	context: Context,
	sessionId: string,
): boolean {
	return shouldLoadSession(context, sessionId);
}

export function shouldLoadSessionWorkspace(
	context: Context,
	sessionId: string,
	options?: { includePending?: boolean },
): boolean {
	return shouldLoadSession(context, sessionId, options);
}
