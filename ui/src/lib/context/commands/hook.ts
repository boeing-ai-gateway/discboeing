import {
	refreshRuntimeHooks,
	rerunRuntimeHook,
	setRuntimeHookPaused,
	setRuntimeHooksPaused,
} from "$lib/app/app-runtime.svelte";

export async function refreshHooks(sessionId: string): Promise<void> {
	await refreshRuntimeHooks(sessionId);
}

export function rerunHook(sessionId: string, hookId: string): void {
	rerunRuntimeHook(sessionId, hookId);
}

export async function setHooksPaused(
	sessionId: string,
	paused: boolean,
): Promise<void> {
	await setRuntimeHooksPaused(sessionId, paused);
}

export async function setHookPaused(
	sessionId: string,
	hookId: string,
	paused: boolean,
): Promise<void> {
	await setRuntimeHookPaused(sessionId, hookId, paused);
}
