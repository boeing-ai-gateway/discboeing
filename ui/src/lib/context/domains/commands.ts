import { api } from "$lib/api-client";
import type { AgentCommand } from "$lib/api-types";
import type { CollectionCache } from "$lib/context/cache";
import {
	createCollectionCache,
	createErrorStatus,
	createReadyStatus,
	createRefreshingStatus,
	upsertById,
} from "$lib/context/cache";
import type { Context } from "$lib/context/context.types";
import {
	ensureSessionRecord,
	type SessionRecord,
} from "$lib/context/domains/sessions";

export type CommandsState = CollectionCache<AgentCommand> & {
	isSubmitting: boolean;
};

export function createCommandsState(): CommandsState {
	return {
		...createCollectionCache<AgentCommand>(),
		isSubmitting: false,
	};
}

function applyCommandsSnapshotToCache(
	context: Context,
	sessionId: string,
	commands: AgentCommand[],
): void {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	applyCommandsSnapshotToRecord(record, commands);
}

export function applyCommandsSnapshotToRecord(
	record: SessionRecord,
	commands: AgentCommand[],
): void {
	record.commands.byId = {};
	record.commands.allIds = [];
	for (const command of commands) {
		upsertById(record.commands, command.name, command);
	}
	record.commands.status = createReadyStatus();
}

export async function loadCommandsIntoCache(
	context: Context,
	sessionId: string,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.commands.status =
		record.commands.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };

	try {
		const response = await api.getSessionCommands(sessionId);
		applyCommandsSnapshotToCache(context, sessionId, response.commands);
	} catch (error) {
		record.commands.status = createErrorStatus(error);
		throw error;
	}
}
