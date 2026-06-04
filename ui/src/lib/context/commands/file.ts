import {
	acceptRuntimeFileConflict,
	closeRuntimeFile,
	collapseRuntimeFileTree,
	discardRuntimeFileBuffer,
	expandRuntimeFileTree,
	forceSaveRuntimeFile,
	getRuntimeFileEditorModel,
	getRuntimeFileEditorViewState,
	openRuntimeFile,
	refreshRuntimeFiles,
	removeRuntimeFile,
	renameRuntimeFile,
	saveRuntimeFile,
	setRuntimeFileDiffTarget,
	setRuntimeFileEditorModel,
	setRuntimeFileEditorViewState,
	toggleRuntimeFileDirectory,
	toggleRuntimeFilesChangedOnly,
	updateRuntimeFileBuffer,
} from "$lib/app/app-runtime.svelte";

export async function openFile(
	sessionId: string,
	path?: string,
): Promise<void> {
	await openRuntimeFile(sessionId, path);
}

export async function refreshFiles(sessionId: string): Promise<void> {
	await refreshRuntimeFiles(sessionId);
}

export async function setFileDiffTarget(
	sessionId: string,
	target: string,
): Promise<void> {
	await setRuntimeFileDiffTarget(sessionId, target);
}

export async function saveFile(
	sessionId: string,
	path: string,
): Promise<boolean> {
	return saveRuntimeFile(sessionId, path);
}

export async function renameFile(
	sessionId: string,
	path: string,
	nextName: string,
): Promise<boolean> {
	return renameRuntimeFile(sessionId, path, nextName);
}

export async function deleteFile(
	sessionId: string,
	path: string,
): Promise<boolean> {
	return removeRuntimeFile(sessionId, path);
}

export function closeFile(sessionId: string, path: string): void {
	closeRuntimeFile(sessionId, path);
}

export async function toggleFilesChangedOnly(sessionId: string): Promise<void> {
	await toggleRuntimeFilesChangedOnly(sessionId);
}

export async function toggleFileDirectory(
	sessionId: string,
	path: string,
): Promise<void> {
	await toggleRuntimeFileDirectory(sessionId, path);
}

export async function expandFileTree(sessionId: string): Promise<void> {
	await expandRuntimeFileTree(sessionId);
}

export function collapseFileTree(sessionId: string): void {
	collapseRuntimeFileTree(sessionId);
}

export function updateFileBuffer(
	sessionId: string,
	path: string,
	content: string,
): void {
	updateRuntimeFileBuffer(sessionId, path, content);
}

export function discardFileBuffer(sessionId: string, path: string): void {
	discardRuntimeFileBuffer(sessionId, path);
}

export function acceptFileConflict(sessionId: string, path: string): void {
	acceptRuntimeFileConflict(sessionId, path);
}

export async function forceSaveFile(
	sessionId: string,
	path: string,
): Promise<boolean> {
	return forceSaveRuntimeFile(sessionId, path);
}

export function getFileEditorModel(
	sessionId: string,
	path: string,
): unknown | null {
	return getRuntimeFileEditorModel(sessionId, path);
}

export function setFileEditorModel(
	sessionId: string,
	path: string,
	model: unknown | null,
): void {
	setRuntimeFileEditorModel(sessionId, path, model);
}

export function getFileEditorViewState(
	sessionId: string,
	path: string,
): unknown | null {
	return getRuntimeFileEditorViewState(sessionId, path);
}

export function setFileEditorViewState(
	sessionId: string,
	path: string,
	viewState: unknown | null,
): void {
	setRuntimeFileEditorViewState(sessionId, path, viewState);
}
