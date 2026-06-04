import type { ServiceOutputSubscription } from "$lib/thread/chat-stream-manager";
import {
	bindRuntimeServiceLocalhost,
	openRuntimeService,
	refreshRuntimeServices,
	runtime,
	startRuntimeService,
	stopRuntimeService,
	unbindRuntimeServiceLocalhost,
} from "$lib/app/app-runtime.svelte";

export async function refreshServices(sessionId: string): Promise<void> {
	await refreshRuntimeServices(sessionId);
}

export function openService(sessionId: string, serviceId: string): void {
	openRuntimeService(sessionId, serviceId);
}

export async function startService(
	sessionId: string,
	serviceId: string,
): Promise<void> {
	await startRuntimeService(sessionId, serviceId);
}

export async function stopService(
	sessionId: string,
	serviceId: string,
): Promise<void> {
	await stopRuntimeService(sessionId, serviceId);
}

export async function bindServiceLocalhost(
	sessionId: string,
	serviceId: string,
	port: number,
): Promise<void> {
	await bindRuntimeServiceLocalhost(sessionId, serviceId, port);
}

export async function unbindServiceLocalhost(
	sessionId: string,
	serviceId: string,
): Promise<void> {
	await unbindRuntimeServiceLocalhost(sessionId, serviceId);
}

export function subscribeServiceOutput(args: {
	sessionId: string;
	serviceId: string;
	onOpen?: () => void;
	onError?: (error: unknown) => void;
}): ServiceOutputSubscription {
	return runtime.chatStreams.subscribeServiceOutput(args);
}
