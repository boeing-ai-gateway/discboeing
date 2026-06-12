import { api } from "$lib/api-client";
import type {
	SessionDiffFilesResponse,
	SessionDiffResponse,
	SessionSingleFileDiffResponse,
} from "$lib/api-types";
import type { ResourceStatus } from "$lib/context/cache";
import {
	createErrorStatus,
	createIdleStatus,
	createReadyStatus,
	createRefreshingStatus,
} from "$lib/context/cache";
import type { Context } from "$lib/context/context.types";
import {
	ensureSessionRecord,
	type SessionRecord,
} from "$lib/context/domains/sessions";

export type DiffState = {
	status: ResourceStatus;
	value: SessionDiffResponse | null;
	filesStatus: ResourceStatus;
	files: SessionDiffFilesResponse | null;
	byPath: Record<string, SessionSingleFileDiffResponse>;
	statusByPath: Record<string, ResourceStatus>;
};

export function createDiffState(): DiffState {
	return {
		status: createIdleStatus(),
		value: null,
		filesStatus: createIdleStatus(),
		files: null,
		byPath: {},
		statusByPath: {},
	};
}

function applyDiffSnapshotToCache(
	context: Context,
	sessionId: string,
	diff: SessionDiffResponse,
): void {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	applyDiffSnapshotToRecord(record, diff);
}

export function applyDiffSnapshotToRecord(
	record: SessionRecord,
	diff: SessionDiffResponse,
): void {
	if (diffSnapshotsEqual(record.diff.value, diff)) {
		if (record.diff.status.state !== "ready") {
			record.diff.status = createReadyStatus();
		}
		return;
	}
	record.diff.value = diff;
	record.diff.status = createReadyStatus();
}

function applyDiffStatusSnapshotToCache(
	context: Context,
	sessionId: string,
	diff: SessionDiffFilesResponse,
): void {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	applyDiffStatusSnapshotToRecord(record, diff);
}

export function applyDiffStatusSnapshotToRecord(
	record: SessionRecord,
	diff: SessionDiffFilesResponse,
): void {
	if (diffStatusSnapshotsEqual(record.diff.files, diff)) {
		if (record.diff.filesStatus.state !== "ready") {
			record.diff.filesStatus = createReadyStatus();
		}
		return;
	}
	record.diff.files = diff;
	record.diff.filesStatus = createReadyStatus();
}

function diffSnapshotsEqual(
	current: SessionDiffResponse | null,
	next: SessionDiffResponse,
): boolean {
	if (!current || !diffStatsEqual(current.stats, next.stats)) {
		return false;
	}
	if (current.files.length !== next.files.length) {
		return false;
	}
	return current.files.every((file, index) => {
		const nextFile = next.files[index];
		return (
			file.path === nextFile.path &&
			file.status === nextFile.status &&
			file.oldPath === nextFile.oldPath &&
			file.additions === nextFile.additions &&
			file.deletions === nextFile.deletions &&
			file.binary === nextFile.binary &&
			file.patch === nextFile.patch
		);
	});
}

function diffStatusSnapshotsEqual(
	current: SessionDiffFilesResponse | null,
	next: SessionDiffFilesResponse,
): boolean {
	if (!current || !diffStatsEqual(current.stats, next.stats)) {
		return false;
	}
	if (current.files.length !== next.files.length) {
		return false;
	}
	return current.files.every((file, index) => {
		const nextFile = next.files[index];
		return (
			file.path === nextFile.path &&
			file.status === nextFile.status &&
			file.oldPath === nextFile.oldPath
		);
	});
}

function diffStatsEqual(
	current: SessionDiffResponse["stats"],
	next: SessionDiffResponse["stats"],
): boolean {
	return (
		current.filesChanged === next.filesChanged &&
		current.additions === next.additions &&
		current.deletions === next.deletions
	);
}

export async function loadDiffIntoCache(
	context: Context,
	sessionId: string,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.diff.status = record.diff.value
		? createRefreshingStatus()
		: { state: "loading" };

	try {
		const response = await api.getSessionDiff(sessionId);
		applyDiffSnapshotToCache(
			context,
			sessionId,
			response as SessionDiffResponse,
		);
	} catch (error) {
		record.diff.status = createErrorStatus(error);
		throw error;
	}
}

export async function loadDiffStatusIntoCache(
	context: Context,
	sessionId: string,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.diff.filesStatus = record.diff.files
		? createRefreshingStatus()
		: { state: "loading" };

	try {
		const response = await api.getSessionDiff(sessionId, { format: "files" });
		applyDiffStatusSnapshotToCache(
			context,
			sessionId,
			response as SessionDiffFilesResponse,
		);
	} catch (error) {
		record.diff.filesStatus = createErrorStatus(error);
		throw error;
	}
}
