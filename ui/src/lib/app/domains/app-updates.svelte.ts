import {
	IGNORED_UPDATE_VERSION_STORAGE_KEY,
	readIgnoredUpdateVersion,
	writeStorage,
} from "$lib/app/app-helpers";
import type {
	DownloadEvent,
	Update as TauriUpdate,
} from "@tauri-apps/plugin-updater";
import type { AppUpdates, UpdateStatus } from "$lib/app/app-context.types";
import { isTauriShell } from "$lib/environment";

export function createAppUpdatesDomain(): AppUpdates {
	let updateStatus = $state<UpdateStatus>("idle");
	let availableVersion = $state<string | null>(null);
	let updateError = $state<string | null>(null);
	let downloadedBytes = $state(0);
	let totalBytes = $state<number | null>(null);
	let ignoredUpdateVersion = $state<string | null>(readIgnoredUpdateVersion());

	const isUpdateIgnored = $derived.by(
		() =>
			availableVersion !== null && ignoredUpdateVersion === availableVersion,
	);
	const showUpdateBadge = $derived.by(
		() =>
			updateStatus === "ready" && availableVersion !== null && !isUpdateIgnored,
	);

	let updateCheckInFlight = false;
	let pendingUpdate: TauriUpdate | null = null;

	async function closePendingUpdate(): Promise<void> {
		if (!pendingUpdate) return;

		const update = pendingUpdate;
		pendingUpdate = null;

		try {
			await update.close();
		} catch {
			// Ignore cleanup failures.
		}
	}

	function resetProgress(): void {
		downloadedBytes = 0;
		totalBytes = null;
	}

	function logUpdateError(action: "check" | "install", error: unknown): void {
		console.error(`[updates] ${action} failed`, {
			error,
			message: error instanceof Error ? error.message : String(error),
			stack: error instanceof Error ? error.stack : undefined,
			status: updateStatus,
			availableVersion,
			isTauri: isTauriShell(),
		});
	}

	return {
		get status() {
			return updateStatus;
		},
		get availableVersion() {
			return availableVersion;
		},
		get error() {
			return updateError;
		},
		get downloadedBytes() {
			return downloadedBytes;
		},
		get totalBytes() {
			return totalBytes;
		},
		get isIgnored() {
			return isUpdateIgnored;
		},
		get showBadge() {
			return showUpdateBadge;
		},
		check: async () => {
			if (updateCheckInFlight) return;
			if (updateStatus === "downloading" || updateStatus === "installing") {
				return;
			}

			if (!isTauriShell()) {
				updateStatus = "error";
				updateError = "App updates are only available in the desktop app.";
				return;
			}

			updateCheckInFlight = true;
			updateStatus = "checking";
			updateError = null;
			resetProgress();

			try {
				const { check } = await import("@tauri-apps/plugin-updater");

				await closePendingUpdate();

				const nextUpdate = await check();
				if (!nextUpdate) {
					availableVersion = null;
					updateStatus = "idle";
					return;
				}

				availableVersion = nextUpdate.version;

				if (ignoredUpdateVersion === nextUpdate.version) {
					await nextUpdate.close();
					updateStatus = "ready";
					return;
				}

				pendingUpdate = nextUpdate;
				updateStatus = "downloading";
				await nextUpdate.download((event: DownloadEvent) => {
					switch (event.event) {
						case "Started":
							totalBytes = event.data?.contentLength ?? null;
							downloadedBytes = 0;
							break;
						case "Progress":
							downloadedBytes += event.data?.chunkLength ?? 0;
							break;
						case "Finished":
							if (totalBytes !== null) {
								downloadedBytes = totalBytes;
							}
							break;
					}
				});
				updateStatus = "ready";
			} catch (error) {
				logUpdateError("check", error);
				await closePendingUpdate();
				resetProgress();
				updateStatus = "error";
				updateError =
					error instanceof Error
						? error.message
						: "Failed to check for updates";
			} finally {
				updateCheckInFlight = false;
			}
		},
		installAndRelaunch: async () => {
			if (updateStatus !== "ready" || !pendingUpdate) return;
			if (!isTauriShell()) {
				updateStatus = "error";
				updateError = "App updates are only available in the desktop app.";
				return;
			}

			updateStatus = "installing";
			updateError = null;
			try {
				const { relaunch } = await import("@tauri-apps/plugin-process");
				await pendingUpdate.install();
				await relaunch();
			} catch (error) {
				logUpdateError("install", error);
				updateStatus = "error";
				updateError = error instanceof Error ? error.message : "Install failed";
			} finally {
				await closePendingUpdate();
			}
		},
		ignore: () => {
			if (!availableVersion) return;
			void closePendingUpdate();
			resetProgress();
			ignoredUpdateVersion = availableVersion;
			writeStorage(IGNORED_UPDATE_VERSION_STORAGE_KEY, availableVersion);
		},
	};
}
