"use strict";

Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;

const vscode = require("vscode");

const editorStateDirRelativeToHome = ".discboeing/editor";
const themeSyncFilePath = ".vscode-theme.json";
const controlFilePath = ".vscode-control.json";
const themeNames = {
	dark: "Discboeing Dark",
	light: "Discboeing Light",
};

function log(...args) {
	console.log("[discboeing-theme]", ...args);
}

function getWorkspaceFolder() {
	return vscode.workspace.workspaceFolders?.[0] ?? null;
}

function getWorkspaceFileUri(relativePath) {
	const workspaceFolder = getWorkspaceFolder();
	if (!workspaceFolder) {
		return null;
	}

	return vscode.Uri.joinPath(workspaceFolder.uri, relativePath);
}

function getHomeDirUri() {
	const homeDir =
		typeof process !== "undefined" && process.env?.HOME
			? process.env.HOME
			: "/home/discboeing";
	const workspaceFolder = getWorkspaceFolder();
	if (workspaceFolder) {
		return workspaceFolder.uri.with({ path: homeDir });
	}
	return vscode.Uri.file(homeDir);
}

function getEditorStateFileUri(relativePath) {
	return vscode.Uri.joinPath(
		getHomeDirUri(),
		editorStateDirRelativeToHome,
		relativePath,
	);
}

function getEditorStateFilePattern(relativePath) {
	return new vscode.RelativePattern(
		getHomeDirUri(),
		`${editorStateDirRelativeToHome}/${relativePath}`,
	);
}

function delay(ms) {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

async function readJsonFile(uri, label) {
	let rawContent;
	try {
		rawContent = await vscode.workspace.fs.readFile(uri);
	} catch (error) {
		log(`${label} file not readable yet`, error);
		return null;
	}

	try {
		return JSON.parse(new TextDecoder().decode(rawContent));
	} catch (error) {
		console.error(`[discboeing-theme] failed to parse ${label} file`, error);
		return null;
	}
}

function getWorkspaceRelativeUri(path) {
	if (typeof path !== "string") {
		return null;
	}

	const trimmed = path.trim();
	if (!trimmed || trimmed.startsWith("/") || trimmed.includes("\0")) {
		return null;
	}

	const segments = trimmed.split("/");
	if (
		segments.some((segment) => !segment || segment === "." || segment === "..")
	) {
		return null;
	}

	return getWorkspaceFileUri(trimmed);
}

function activate(context) {
	log("activate called");
	const applyTheme = async (themeMode) => {
		const nextTheme = themeNames[themeMode];
		log("applyTheme requested", {
			themeMode,
			nextTheme,
		});
		if (!nextTheme) {
			log("ignoring unknown theme mode", themeMode);
			return;
		}

		const config = vscode.workspace.getConfiguration("workbench");
		const currentTheme = config.get("colorTheme");
		log("current theme before update", currentTheme);
		if (currentTheme === nextTheme) {
			log("theme already applied", nextTheme);
			return;
		}

		await config.update(
			"colorTheme",
			nextTheme,
			vscode.ConfigurationTarget.Global,
		);
		log("theme updated", nextTheme);
	};

	const syncThemeFromFile = async () => {
		const themeSyncUri = getEditorStateFileUri(themeSyncFilePath);

		log("reading theme sync file", themeSyncUri.toString());
		const payload = await readJsonFile(themeSyncUri, "theme sync");
		if (!payload) {
			return;
		}

		log("theme sync payload received", payload);
		await applyTheme(payload.theme);
	};

	const openFileFromCommand = async () => {
		const controlUri = getEditorStateFileUri(controlFilePath);

		log("reading editor control file", controlUri.toString());
		const payload = await readJsonFile(controlUri, "editor control");
		if (!payload) {
			return;
		}

		log("editor control payload received", payload);
		if (payload.type !== "openFile") {
			log("ignoring unknown editor control command", payload.type);
			return;
		}

		const fileUri = getWorkspaceRelativeUri(payload.path);
		if (!fileUri) {
			log("ignoring invalid open file path", payload.path);
			return;
		}

		await vscode.window.showTextDocument(fileUri, { preview: false });
		await vscode.workspace.fs.delete(controlUri, { useTrash: false }).then(
			() => log("editor control command consumed", payload.id),
			(error) => log("failed to delete consumed editor control file", error),
		);
	};

	let controlCommandInFlight = false;
	const processControlCommand = async (label) => {
		if (controlCommandInFlight) {
			return;
		}

		controlCommandInFlight = true;
		try {
			await openFileFromCommand();
		} catch (error) {
			console.error(`[discboeing-theme] failed to ${label}`, error);
		} finally {
			controlCommandInFlight = false;
		}
	};

	const themeWatcher = vscode.workspace.createFileSystemWatcher(
		getEditorStateFilePattern(themeSyncFilePath),
	);
	themeWatcher.onDidCreate((uri) => {
		log("theme sync file created", uri.toString());
		void syncThemeFromFile().catch((error) => {
			console.error(
				"[discboeing-theme] failed to sync created theme file",
				error,
			);
		});
	});
	themeWatcher.onDidChange((uri) => {
		log("theme sync file changed", uri.toString());
		void syncThemeFromFile().catch((error) => {
			console.error(
				"[discboeing-theme] failed to sync changed theme file",
				error,
			);
		});
	});
	themeWatcher.onDidDelete((uri) => {
		log("theme sync file deleted", uri.toString());
	});
	context.subscriptions.push(themeWatcher);

	const controlWatcher = vscode.workspace.createFileSystemWatcher(
		getEditorStateFilePattern(controlFilePath),
	);
	controlWatcher.onDidCreate((uri) => {
		log("editor control file created", uri.toString());
		void processControlCommand("process created command");
	});
	controlWatcher.onDidChange((uri) => {
		log("editor control file changed", uri.toString());
		void processControlCommand("process changed command");
	});
	controlWatcher.onDidDelete((uri) => {
		log("editor control file deleted", uri.toString());
	});
	context.subscriptions.push(controlWatcher);

	void (async () => {
		for (const retryDelay of [0, 250, 1000, 2500]) {
			if (retryDelay > 0) {
				await delay(retryDelay);
			}
			await syncThemeFromFile();
			await processControlCommand("process initial command");
		}
	})().catch((error) => {
		console.error("[discboeing-theme] initial editor sync failed", error);
	});
}

function deactivate() {
	log("deactivate called");
}
