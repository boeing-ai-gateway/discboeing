import {
	checkForAppUpdate,
	closeAppUpdate,
	downloadAppUpdate,
	installAppUpdate,
	relaunchApp,
	type DesktopDownloadEvent,
} from "$lib/shell";
import type { Context } from "$lib/context/context.types";
import { uiStateStore } from "$lib/context/domains/preferences";

let pendingUpdate: {
	updateRid: number;
	bytesRid: number | null;
} | null = null;

export async function checkForUpdates(context: Context): Promise<void> {
	const updates = context.view.app.updates;
	updates.status = "checking";
	updates.error = null;
	updates.downloadedBytes = 0;
	updates.totalBytes = null;
	try {
		if (pendingUpdate) {
			await closeAppUpdate(pendingUpdate.updateRid, pendingUpdate.bytesRid);
			pendingUpdate = null;
		}
		const nextUpdate = await checkForAppUpdate(null);
		if (!nextUpdate) {
			updates.availableVersion = null;
			updates.status = "idle";
			updates.showBadge = false;
			return;
		}
		updates.availableVersion = nextUpdate.version;
		if (uiStateStore.ignoredUpdateVersion === nextUpdate.version) {
			await closeAppUpdate(nextUpdate.rid, null);
			updates.isIgnored = true;
			updates.status = "ready";
			updates.showBadge = false;
			return;
		}
		pendingUpdate = { updateRid: nextUpdate.rid, bytesRid: null };
		updates.status = "downloading";
		const bytesRid = await downloadAppUpdate(
			nextUpdate.rid,
			(event: DesktopDownloadEvent) => {
				if (event.event === "Started") {
					updates.totalBytes = event.data?.contentLength ?? null;
					updates.downloadedBytes = 0;
				}
				if (event.event === "Progress") {
					updates.downloadedBytes += event.data?.chunkLength ?? 0;
				}
				if (event.event === "Finished" && updates.totalBytes !== null) {
					updates.downloadedBytes = updates.totalBytes;
				}
			},
		);
		pendingUpdate.bytesRid = bytesRid;
		updates.isIgnored = false;
		updates.status = "ready";
		updates.showBadge = true;
	} catch (error) {
		updates.status = "error";
		updates.error =
			error instanceof Error ? error.message : "Failed to check for updates";
		updates.showBadge = false;
	}
}

export async function setTrackPrereleases(
	context: Context,
	track: boolean,
): Promise<void> {
	context.view.app.preferences.trackPrereleases = track;
	uiStateStore.setTrackPrereleases(track);
	await checkForUpdates(context);
}

export async function installUpdateAndRelaunch(
	context: Context,
): Promise<void> {
	if (!pendingUpdate || pendingUpdate.bytesRid === null) {
		return;
	}
	context.view.app.updates.status = "installing";
	await installAppUpdate(pendingUpdate.updateRid, pendingUpdate.bytesRid);
	await relaunchApp();
}

export async function ignoreUpdate(context: Context): Promise<void> {
	const updates = context.view.app.updates;
	uiStateStore.ignoreUpdateVersion(updates.availableVersion);
	updates.isIgnored = true;
	updates.showBadge = false;
}
