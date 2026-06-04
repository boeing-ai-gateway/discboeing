import { getCommandContext } from "$lib/context/commands";
import {
	checkForAppUpdate,
	closeAppUpdate,
	downloadAppUpdate,
	installAppUpdate,
	relaunchApp,
	type DesktopDownloadEvent,
} from "$lib/shell";

import { uiStateStore } from "./shared";

let pendingUpdate: {
	updateRid: number;
	bytesRid: number | null;
} | null = null;

export async function checkForUpdates(): Promise<void> {
	const context = getCommandContext();
	context.data.updates.status = "checking";
	context.data.updates.error = null;
	context.data.updates.downloadedBytes = 0;
	context.data.updates.totalBytes = null;
	try {
		if (pendingUpdate) {
			await closeAppUpdate(pendingUpdate.updateRid, pendingUpdate.bytesRid);
			pendingUpdate = null;
		}
		const nextUpdate = await checkForAppUpdate(null);
		if (!nextUpdate) {
			context.data.updates.availableVersion = null;
			context.data.updates.status = "idle";
			context.view.app.updates.showBadge = false;
			return;
		}
		context.data.updates.availableVersion = nextUpdate.version;
		if (uiStateStore.ignoredUpdateVersion === nextUpdate.version) {
			await closeAppUpdate(nextUpdate.rid, null);
			context.data.updates.isIgnored = true;
			context.data.updates.status = "ready";
			context.view.app.updates.showBadge = false;
			return;
		}
		pendingUpdate = { updateRid: nextUpdate.rid, bytesRid: null };
		context.data.updates.status = "downloading";
		const bytesRid = await downloadAppUpdate(
			nextUpdate.rid,
			(event: DesktopDownloadEvent) => {
				if (event.event === "Started") {
					context.data.updates.totalBytes = event.data?.contentLength ?? null;
					context.data.updates.downloadedBytes = 0;
				}
				if (event.event === "Progress") {
					context.data.updates.downloadedBytes += event.data?.chunkLength ?? 0;
				}
				if (
					event.event === "Finished" &&
					context.data.updates.totalBytes !== null
				) {
					context.data.updates.downloadedBytes =
						context.data.updates.totalBytes;
				}
			},
		);
		pendingUpdate.bytesRid = bytesRid;
		context.data.updates.isIgnored = false;
		context.data.updates.status = "ready";
		context.view.app.updates.showBadge = true;
	} catch (error) {
		context.data.updates.status = "error";
		context.data.updates.error =
			error instanceof Error ? error.message : "Failed to check for updates";
		context.view.app.updates.showBadge = false;
	}
}

export async function setTrackPrereleases(track: boolean): Promise<void> {
	getCommandContext().view.app.preferences.trackPrereleases = track;
	uiStateStore.setTrackPrereleases(track);
	await checkForUpdates();
}

export async function installUpdateAndRelaunch(): Promise<void> {
	if (!pendingUpdate || pendingUpdate.bytesRid === null) {
		return;
	}
	getCommandContext().data.updates.status = "installing";
	await installAppUpdate(pendingUpdate.updateRid, pendingUpdate.bytesRid);
	await relaunchApp();
}

export function ignoreUpdate(): void {
	const context = getCommandContext();
	uiStateStore.ignoreUpdateVersion(context.data.updates.availableVersion);
	context.data.updates.isIgnored = true;
	context.view.app.updates.showBadge = false;
}
