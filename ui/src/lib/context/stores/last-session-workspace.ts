import { readStorage, writeStorage } from "$lib/local-storage";

const LAST_SESSION_WORKSPACE_STORAGE_PREFIX =
	"discboeing:last-session-workspace:";

export function isStoredWorkspaceOption(value: string): boolean {
	return (
		value === "new-workspace" ||
		value === "local-directory" ||
		value === "git-repo" ||
		value.startsWith("existing:")
	);
}

function storageKey(projectId: string): string {
	return `${LAST_SESSION_WORKSPACE_STORAGE_PREFIX}${projectId}`;
}

function read(projectId: string): string | null {
	const value = readStorage(storageKey(projectId));
	return value && isStoredWorkspaceOption(value) ? value : null;
}

function write(projectId: string, value: string | null): string | null {
	const nextValue = value && isStoredWorkspaceOption(value) ? value : null;
	writeStorage(storageKey(projectId), nextValue);
	return nextValue;
}

export const lastSessionWorkspaceStore = {
	readInitial(projectId: string): string | null {
		try {
			return read(projectId);
		} catch {
			return null;
		}
	},
	set(projectId: string, value: string): string | null {
		try {
			return write(projectId, value);
		} catch {
			return isStoredWorkspaceOption(value) ? value : null;
		}
	},
};
