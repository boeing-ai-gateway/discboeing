import { api } from "$lib/api-client";
import type { ResolvedTheme } from "$lib/theme";

export const VSCODE_THEME_FILE_PATH = "~/.discboeing/editor/.vscode-theme.json";
export const VSCODE_CONTROL_FILE_PATH =
	"~/.discboeing/editor/.vscode-control.json";

type VSCodeOpenFileCommand = {
	id: string;
	type: "openFile";
	path: string;
	createdAt: string;
};

export function buildThemePayload(nextTheme: ResolvedTheme): string {
	return JSON.stringify({ theme: nextTheme }, null, "\t");
}

export function buildOpenFilePayload(path: string): string {
	const command: VSCodeOpenFileCommand = {
		id:
			typeof crypto !== "undefined" && "randomUUID" in crypto
				? crypto.randomUUID()
				: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
		type: "openFile",
		path,
		createdAt: new Date().toISOString(),
	};
	return JSON.stringify(command, null, "\t");
}

export async function syncEditorTheme(
	sessionId: string,
	resolvedTheme: ResolvedTheme,
) {
	await api.writeSessionFile(sessionId, {
		path: VSCODE_THEME_FILE_PATH,
		content: buildThemePayload(resolvedTheme),
	});
}

export async function requestVSCodeOpenFile(sessionId: string, path: string) {
	await api.writeSessionFile(sessionId, {
		path: VSCODE_CONTROL_FILE_PATH,
		content: buildOpenFilePayload(path),
	});
}
