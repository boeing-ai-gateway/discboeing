import { api } from "$lib/api-client";
import type { Service } from "$lib/api-types";
import {
	createErrorStatus,
	createReadyStatus,
	createRefreshingStatus,
	upsertById,
} from "$lib/context/cache";
import type { CollectionCache } from "$lib/context/cache";
import { createCollectionCache } from "$lib/context/cache";
import type { CommandOptions, Context } from "$lib/context/context.types";
import { ensureSessionView } from "$lib/context/domains/view";
import {
	ensureSessionRecord,
	type SessionRecord,
} from "$lib/context/domains/sessions";

export type ServicesState = CollectionCache<Service>;

export function createServicesState(): ServicesState {
	return createCollectionCache<Service>();
}

function applyServicesSnapshotToCache(
	context: Context,
	sessionId: string,
	services: Service[],
): void {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	applyServicesSnapshotToRecord(record, services);
}

export function applyServicesSnapshotToRecord(
	record: SessionRecord,
	services: Service[],
): void {
	record.services.byId = {};
	record.services.allIds = [];
	for (const service of services) {
		upsertById(record.services, service.id, service);
	}
	record.services.status = createReadyStatus();
}

export async function loadServicesIntoCache(
	context: Context,
	sessionId: string,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.services.status =
		record.services.allIds.length > 0
			? createRefreshingStatus()
			: { state: "loading" };

	try {
		const response = await api.getServices(sessionId);
		applyServicesSnapshotToCache(context, sessionId, response.services);
	} catch (error) {
		record.services.status = createErrorStatus(error);
		throw error;
	}
}

export async function startService(
	context: Context,
	sessionId: string,
	serviceId: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.startService(sessionId, serviceId);
	if (options.wait) await loadServicesIntoCache(context, sessionId);
}

export async function openServicePanel(
	context: Context,
	sessionId: string,
	serviceId: string,
	viewMode?: "preview" | "logs",
): Promise<void> {
	const view = ensureSessionView(context, sessionId);
	view.workspace.activeView = "services";
	view.workspace.activeServiceId = serviceId;
	view.services.activeServiceId = serviceId;
	if (viewMode) {
		view.services.activeViewMode = viewMode;
	}
}

export async function stopService(
	context: Context,
	sessionId: string,
	serviceId: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.stopService(sessionId, serviceId);
	if (options.wait) await loadServicesIntoCache(context, sessionId);
}

export async function bindServiceLocalhost(
	context: Context,
	sessionId: string,
	serviceId: string,
	port: number,
	options: CommandOptions = {},
): Promise<void> {
	await api.bindServiceLocalhost(sessionId, serviceId, { port });
	if (options.wait) await loadServicesIntoCache(context, sessionId);
}

export async function unbindServiceLocalhost(
	context: Context,
	sessionId: string,
	serviceId: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.unbindServiceLocalhost(sessionId, serviceId);
	if (options.wait) await loadServicesIntoCache(context, sessionId);
}
