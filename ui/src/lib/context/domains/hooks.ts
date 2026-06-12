import { api } from "$lib/api-client";
import type {
	HookOutputResponse,
	HookRunStatus,
	HooksStateResponse,
} from "$lib/api-types";
import {
	createErrorStatus,
	createReadyStatus,
	createRefreshingStatus,
} from "$lib/context/cache";
import type { CollectionCache, ResourceStatus } from "$lib/context/cache";
import { createCollectionCache } from "$lib/context/cache";
import type { CommandOptions, Context } from "$lib/context/context.types";
import {
	ensureSessionRecord,
	type SessionRecord,
} from "$lib/context/domains/sessions";

export type HooksState = CollectionCache<HookRunStatus> & {
	outputsById: Record<string, HookOutputResponse>;
	pendingHookIds: string[];
	lastEvaluatedAt: string | null;
	executionPaused: boolean;
	outputStatusById: Record<string, ResourceStatus>;
};

export function createHooksState(): HooksState {
	return {
		...createCollectionCache<HookRunStatus>(),
		outputsById: {},
		pendingHookIds: [],
		lastEvaluatedAt: null,
		executionPaused: false,
		outputStatusById: {},
	};
}

function applyHooksSnapshotToCache(
	context: Context,
	sessionId: string,
	response: HooksStateResponse,
): void {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	applyHooksSnapshotToRecord(record, response);
}

export function applyHooksSnapshotToRecord(
	record: SessionRecord,
	response: HooksStateResponse,
): void {
	record.hooks.byId = response.hooks;
	record.hooks.allIds = Object.keys(response.hooks);
	record.hooks.outputsById = response.outputs;
	record.hooks.pendingHookIds = response.pendingHooks;
	record.hooks.lastEvaluatedAt = response.lastEvaluatedAt;
	record.hooks.executionPaused = response.executionPaused;
	record.hooks.status = createReadyStatus();
}

export async function loadHooksIntoCache(
	context: Context,
	sessionId: string,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.hooks.status =
		record.hooks.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };

	try {
		const response = await api.getHooksState(sessionId);
		applyHooksSnapshotToCache(context, sessionId, response);
	} catch (error) {
		record.hooks.status = createErrorStatus(error);
		throw error;
	}
}

export async function rerunHook(
	context: Context,
	sessionId: string,
	hookId: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.rerunHook(sessionId, hookId);
	if (options.wait) await loadHooksIntoCache(context, sessionId);
}

export async function pauseHooks(
	context: Context,
	sessionId: string,
	paused: boolean,
	options: CommandOptions = {},
): Promise<void> {
	await api.updateHooksExecution(sessionId, paused);
	if (options.wait) await loadHooksIntoCache(context, sessionId);
}

export async function pauseHook(
	context: Context,
	sessionId: string,
	hookId: string,
	paused: boolean,
	options: CommandOptions = {},
): Promise<void> {
	await api.updateHookExecution(sessionId, hookId, paused);
	if (options.wait) await loadHooksIntoCache(context, sessionId);
}
