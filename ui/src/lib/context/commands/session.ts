import type {
	SessionContextValue,
	ThreadContextValue,
} from "$lib/session/session-context.types";
import {
	connectRuntimeProjectEvents,
	connectRuntimeThread,
	createRuntimeThread,
	deleteRuntimeSession,
	deleteRuntimeThread,
	ensureRuntimeSessionState,
	ensureRuntimeThreadState,
	openRuntimeThread,
	refreshRuntimeData,
	releaseRuntimeSessionState,
	releaseRuntimeThreadState,
	renameRuntimeSession,
	renameRuntimeThread,
	selectRuntimeSession,
	shouldLoadRuntimeSession,
	startNewRuntimeSession,
	stopRuntimeSession,
	syncRuntimeProjections,
} from "$lib/app/app-runtime.svelte";

export function syncAppNavigationFromBridge(): void {
	syncRuntimeProjections();
}

export function shouldLoadSessionWorkspace(
	sessionId: string,
	options?: { includePending?: boolean },
): boolean {
	return shouldLoadRuntimeSession(sessionId, options);
}

export function shouldLoadSessionToolbar(sessionId: string): boolean {
	return shouldLoadRuntimeSession(sessionId);
}

export function selectSession(sessionId: string): void {
	selectRuntimeSession(sessionId);
	syncAppNavigationFromBridge();
}

export function openThread(sessionId: string, threadId: string): void {
	openRuntimeThread(sessionId, threadId);
	syncAppNavigationFromBridge();
}

export function createSession(): void {
	startNewRuntimeSession();
	syncAppNavigationFromBridge();
}

export async function createThread(sessionId: string): Promise<string | null> {
	const threadId = await createRuntimeThread(sessionId);
	syncAppNavigationFromBridge();
	return threadId;
}

export async function renameSession(
	sessionId: string,
	nextName: string,
): Promise<boolean> {
	const renamed = await renameRuntimeSession(sessionId, nextName);
	syncAppNavigationFromBridge();
	return renamed;
}

export async function stopSession(sessionId: string): Promise<boolean> {
	const stopped = await stopRuntimeSession(sessionId);
	syncAppNavigationFromBridge();
	return stopped;
}

export async function deleteSession(sessionId: string): Promise<boolean> {
	const deleted = await deleteRuntimeSession(sessionId);
	syncAppNavigationFromBridge();
	return deleted;
}

export async function renameThread(
	sessionId: string,
	threadId: string,
	nextName: string,
): Promise<boolean> {
	const renamed = await renameRuntimeThread(sessionId, threadId, nextName);
	syncAppNavigationFromBridge();
	return renamed;
}

export async function deleteThread(
	sessionId: string,
	threadId: string,
): Promise<boolean> {
	const deleted = await deleteRuntimeThread(sessionId, threadId);
	syncAppNavigationFromBridge();
	return deleted;
}

export function ensureSessionState(sessionId: string): SessionContextValue {
	return ensureRuntimeSessionState(sessionId);
}

export function releaseSessionState(session: SessionContextValue): void {
	releaseRuntimeSessionState(session);
}

export function ensureThreadState(
	sessionId: string,
	threadId: string,
): ThreadContextValue {
	return ensureRuntimeThreadState(sessionId, threadId);
}

export function connectThread(sessionId: string, threadId: string): void {
	connectRuntimeThread(sessionId, threadId);
}

export function releaseThreadState(
	sessionId: string,
	thread: ThreadContextValue,
): void {
	releaseRuntimeThreadState(sessionId, thread);
}

export async function refreshAppData(): Promise<void> {
	await refreshRuntimeData();
	syncAppNavigationFromBridge();
}

export function connectProjectEvents(): () => void {
	return connectRuntimeProjectEvents();
}
