import {
	downloadFile as downloadBrowserFile,
	openExternalUrl as openBrowserExternalUrl,
	pickDirectory as pickBrowserDirectory,
	readClipboardText as readBrowserClipboardText,
	writeClipboardText as writeBrowserClipboardText,
} from "$lib/desktop/browser-adapter";
import {
	checkForAppUpdate as checkForElectronAppUpdate,
	closeAppUpdate as closeElectronAppUpdate,
	detectElectronRuntime,
	downloadAppUpdate as downloadElectronAppUpdate,
	downloadFile as downloadElectronFile,
	getServerConfig as getElectronServerConfig,
	initServerConfig as initElectronServerConfig,
	installAppUpdate as installElectronAppUpdate,
	openExternalUrl as openElectronExternalUrl,
	pickDirectory as pickElectronDirectory,
	readClipboardText as readElectronClipboardText,
	relaunchApp as relaunchElectronApp,
	withCurrentWindow as withCurrentElectronWindow,
	writeClipboardText as writeElectronClipboardText,
} from "$lib/desktop/electron-adapter";
import type {
	DesktopDownloadEvent,
	DesktopRuntimeKind,
	DesktopServerConfig,
	DesktopUpdateMetadata,
	DesktopWindowCallback,
	DownloadFileOptions,
} from "$lib/desktop/types";

export type {
	DesktopDownloadEvent,
	DesktopRuntimeKind,
	DesktopServerConfig,
	DesktopUpdateMetadata,
	DesktopWindow,
	DownloadFileOptions,
} from "$lib/desktop/types";

function toUint8Array(content: DownloadFileOptions["content"]): Uint8Array {
	if (typeof content === "string") {
		return new TextEncoder().encode(content);
	}
	if (content instanceof Uint8Array) {
		return content;
	}
	return new Uint8Array(content);
}

function unsupportedFeature(feature: string): never {
	throw new Error(`${feature} is not available in this environment`);
}

export function getDesktopRuntimeKind(): DesktopRuntimeKind {
	if (detectElectronRuntime()) {
		return "electron";
	}
	return "browser";
}

export function isDesktopShell(): boolean {
	return getDesktopRuntimeKind() !== "browser";
}

export function supportsNativeWindowControls(): boolean {
	return isDesktopShell();
}

export function supportsAppUpdates(): boolean {
	return getDesktopRuntimeKind() !== "browser";
}

export async function initDesktopConfig(): Promise<void> {
	if (getDesktopRuntimeKind() === "electron") {
		await initElectronServerConfig();
	}
}

export function getDesktopServerConfig(): DesktopServerConfig | null {
	if (getDesktopRuntimeKind() === "electron") {
		return getElectronServerConfig();
	}
	return null;
}

export function getDesktopAuthToken(): string | null {
	return getDesktopServerConfig()?.secret ?? null;
}

export async function downloadFile({
	filename,
	content,
	mimeType = "application/octet-stream",
}: DownloadFileOptions): Promise<void> {
	const bytes = toUint8Array(content);

	if (getDesktopRuntimeKind() === "electron") {
		const { toast } = await import("svelte-sonner");
		await downloadElectronFile(filename, bytes);
		toast.success(`${filename} saved to Downloads`);
		return;
	}

	await downloadBrowserFile(filename, bytes, mimeType);
}

export async function readClipboardText(): Promise<string> {
	if (getDesktopRuntimeKind() === "electron") {
		return readElectronClipboardText();
	}
	return readBrowserClipboardText();
}

export async function writeClipboardText(text: string): Promise<void> {
	if (getDesktopRuntimeKind() === "electron") {
		await writeElectronClipboardText(text);
		return;
	}
	await writeBrowserClipboardText(text);
}

export async function openUrl(url: string): Promise<void> {
	if (getDesktopRuntimeKind() === "electron") {
		await openElectronExternalUrl(url);
		return;
	}
	await openBrowserExternalUrl(url);
}

export async function pickDirectory(): Promise<string | null> {
	if (getDesktopRuntimeKind() === "electron") {
		return pickElectronDirectory();
	}
	return pickBrowserDirectory();
}

export async function withCurrentDesktopWindow<T>(
	callback: DesktopWindowCallback<T>,
): Promise<T | undefined> {
	if (getDesktopRuntimeKind() === "electron") {
		return withCurrentElectronWindow(callback);
	}
	return undefined;
}

export async function checkForAppUpdate(
	endpoint?: string | null,
): Promise<DesktopUpdateMetadata | null> {
	if (getDesktopRuntimeKind() === "electron") {
		return checkForElectronAppUpdate(endpoint);
	}
	return null;
}

export async function downloadAppUpdate(
	rid: number,
	onEvent: (event: DesktopDownloadEvent) => void,
): Promise<number> {
	if (getDesktopRuntimeKind() === "electron") {
		return downloadElectronAppUpdate(rid, onEvent);
	}
	return unsupportedFeature("App updates");
}

export async function installAppUpdate(
	updateRid: number,
	bytesRid: number,
): Promise<void> {
	if (getDesktopRuntimeKind() === "electron") {
		await installElectronAppUpdate(updateRid, bytesRid);
		return;
	}
	unsupportedFeature("App updates");
}

export async function closeAppUpdate(
	updateRid?: number | null,
	bytesRid?: number | null,
): Promise<void> {
	if (getDesktopRuntimeKind() === "electron") {
		await closeElectronAppUpdate(updateRid, bytesRid);
	}
}

export async function relaunchApp(): Promise<void> {
	if (getDesktopRuntimeKind() === "electron") {
		await relaunchElectronApp();
		return;
	}
	unsupportedFeature("App relaunch");
}
