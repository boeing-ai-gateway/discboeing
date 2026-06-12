import { api } from "$lib/api-client";
import type {
	ListSessionFilesResponse,
	SessionDiffFilesResponse,
	SessionFileEntry,
	WriteSessionFileRequest,
} from "$lib/api-types";
import {
	createErrorStatus,
	createReadyStatus,
	createRefreshingStatus,
} from "$lib/context/cache";
import type { ResourceStatus } from "$lib/context/cache";
import { createIdleStatus } from "$lib/context/cache";
import type { CommandOptions, Context } from "$lib/context/context.types";
import { ensureSessionView } from "$lib/context/domains/view";
import {
	ensureSessionRecord,
	type SessionRecord,
} from "$lib/context/domains/sessions";

export type FileTreeNode = {
	path: string;
	parentPath: string | null;
	entry: SessionFileEntry | null;
	childrenPaths?: string[];
};

export type FilesState = {
	nodesByPath: Record<string, FileTreeNode>;
	activeSubtrees: Record<string, true>;
	statusBySubtree: Record<string, ResourceStatus>;
};

export function createFilesState(): FilesState {
	return {
		nodesByPath: {},
		activeSubtrees: {},
		statusBySubtree: {
			"": createIdleStatus(),
		},
	};
}

export function normalizeFilePath(path: string): string {
	const normalized = path.replace(/^\/+|\/+$/g, "");
	return normalized === "." ? "" : normalized;
}

function applyFileSubtreeSnapshotToCache(
	context: Context,
	sessionId: string,
	response: ListSessionFilesResponse,
): void {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	applyFileSubtreeSnapshotToRecord(record, response);
}

export function applyFileSubtreeSnapshotToRecord(
	record: SessionRecord,
	response: ListSessionFilesResponse,
): void {
	const normalizedPath = normalizeFilePath(response.path);
	if (fileSubtreeSnapshotEqual(record, normalizedPath, response)) {
		if (record.files.statusBySubtree[normalizedPath]?.state !== "ready") {
			record.files.statusBySubtree[normalizedPath] = createReadyStatus();
		}
		return;
	}
	const childPaths = response.entries.map((entry) =>
		normalizedPath ? `${normalizedPath}/${entry.name}` : entry.name,
	);
	record.files.nodesByPath[normalizedPath] = {
		path: normalizedPath,
		parentPath: null,
		entry: null,
		childrenPaths: childPaths,
	};
	for (const entry of response.entries) {
		const childPath = normalizedPath
			? `${normalizedPath}/${entry.name}`
			: entry.name;
		record.files.nodesByPath[childPath] = {
			path: childPath,
			parentPath: normalizedPath,
			entry,
		};
	}
	record.files.statusBySubtree[normalizedPath] = createReadyStatus();
}

function fileSubtreeSnapshotEqual(
	record: SessionRecord,
	normalizedPath: string,
	response: ListSessionFilesResponse,
): boolean {
	const current = record.files.nodesByPath[normalizedPath];
	if (!current?.childrenPaths) {
		return false;
	}
	if (current.childrenPaths.length !== response.entries.length) {
		return false;
	}
	return response.entries.every((entry, index) => {
		const childPath = normalizedPath
			? `${normalizedPath}/${entry.name}`
			: entry.name;
		if (current.childrenPaths?.[index] !== childPath) {
			return false;
		}
		const currentChild = record.files.nodesByPath[childPath];
		return (
			currentChild?.parentPath === normalizedPath &&
			currentChild.entry?.name === entry.name &&
			currentChild.entry.type === entry.type &&
			currentChild.entry.size === entry.size
		);
	});
}

export async function loadFileSubtreeIntoCache(
	context: Context,
	sessionId: string,
	path: string,
): Promise<void> {
	const normalizedPath = normalizeFilePath(path);
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.files.activeSubtrees[normalizedPath] = true;
	record.files.statusBySubtree[normalizedPath] = record.files.nodesByPath[
		normalizedPath
	]
		? createRefreshingStatus()
		: { state: "loading" };

	try {
		const response = await api.listSessionFiles(sessionId, normalizedPath);
		applyFileSubtreeSnapshotToCache(context, sessionId, response);
	} catch (error) {
		record.files.statusBySubtree[normalizedPath] = createErrorStatus(error);
		throw error;
	}
}

export async function openFile(
	context: Context,
	sessionId: string,
	path?: string,
	options: CommandOptions = {},
): Promise<void> {
	const view = ensureSessionView(context, sessionId);
	view.workspace.activeView = "file";
	if (!path) return;

	view.files.activePath = path;
	if (!view.files.openPaths.includes(path)) {
		view.files.openPaths = [...view.files.openPaths, path];
	}

	await loadFileSubtreeIntoCache(context, sessionId, parentPath(path));
	if (view.files.buffers[path]) {
		if (options.wait) return;
		return;
	}

	view.files.loadingPaths[path] = true;
	try {
		const record = ensureSessionRecord(context.data.sessions, sessionId);
		const fromBase =
			record.diff.files?.files.find((entry) => entry.path === path)?.status ===
			"deleted";
		const file = await api.readSessionFile(sessionId, path, { fromBase });
		view.files.buffers[path] = {
			content: file.content,
			originalContent: file.content,
			encoding: file.encoding,
			isDirty: false,
			isSaving: false,
			saveError: null,
			hasConflict: false,
			conflictContent: null,
			fromBase,
		};
	} finally {
		view.files.loadingPaths[path] = false;
	}
	if (options.wait) return;
}

export async function openFilesPanel(
	context: Context,
	sessionId: string,
): Promise<void> {
	const view = ensureSessionView(context, sessionId);
	view.workspace.activeView = "file";
	view.files.selected = "";
	view.files.activePath = "";
}

export async function setDiffTarget(
	context: Context,
	sessionId: string,
	target: string,
): Promise<void> {
	const view = ensureSessionView(context, sessionId);
	const normalizedTarget = target.trim();
	view.files.diffTarget = normalizedTarget;
	if (!normalizedTarget) {
		return;
	}

	const response = (await api.getSessionDiff(sessionId, {
		format: "files",
		target: normalizedTarget,
	})) as SessionDiffFilesResponse;
	if (view.files.diffTarget === normalizedTarget) {
		view.files.diffFilesByTarget[normalizedTarget] = response;
	}
}

function clearDiffTargetSummaries(context: Context, sessionId: string): void {
	const view = ensureSessionView(context, sessionId);
	view.files.diffFilesByTarget = {};
}

export async function saveFile(
	context: Context,
	sessionId: string,
	path: string,
	content: string,
	options: CommandOptions &
		Pick<WriteSessionFileRequest, "encoding" | "originalContent"> = {},
): Promise<void> {
	const request: WriteSessionFileRequest = { path, content };
	if (options.encoding) {
		request.encoding = options.encoding;
	}
	if (options.originalContent !== undefined) {
		request.originalContent = options.originalContent;
	}
	await api.writeSessionFile(sessionId, request);
	clearDiffTargetSummaries(context, sessionId);
	if (options.wait)
		await loadFileSubtreeIntoCache(context, sessionId, parentPath(path));
}

export async function renameFile(
	context: Context,
	sessionId: string,
	from: string,
	to: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.renameSessionFile(sessionId, { oldPath: from, newPath: to });
	clearDiffTargetSummaries(context, sessionId);
	if (options.wait)
		await loadFileSubtreeIntoCache(context, sessionId, parentPath(to));
}

export async function deleteFile(
	context: Context,
	sessionId: string,
	path: string,
	options: CommandOptions = {},
): Promise<void> {
	await api.deleteSessionFile(sessionId, { path });
	clearDiffTargetSummaries(context, sessionId);
	if (options.wait)
		await loadFileSubtreeIntoCache(context, sessionId, parentPath(path));
}

function parentPath(path: string): string {
	return path.split("/").slice(0, -1).join("/");
}
