"use strict";

Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;

const vscode = require("vscode");

const themeSyncFilePath = ".discobot/.vscode-theme.json";
const themeNames = {
	dark: "Discobot Dark",
	light: "Discobot Light",
};

function log(...args) {
	console.log("[discobot-theme]", ...args);
}

function getThemeSyncUri() {
	const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
	if (!workspaceFolder) {
		return null;
	}

	return vscode.Uri.joinPath(workspaceFolder.uri, themeSyncFilePath);
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
		const themeSyncUri = getThemeSyncUri();
		if (!themeSyncUri) {
			log("workspace folder unavailable for theme sync");
			return;
		}

		log("reading theme sync file", themeSyncUri.toString());
		let rawContent;
		try {
			rawContent = await vscode.workspace.fs.readFile(themeSyncUri);
		} catch (error) {
			log("theme sync file not readable yet", error);
			return;
		}

		let payload;
		try {
			payload = JSON.parse(new TextDecoder().decode(rawContent));
		} catch (error) {
			console.error("[discobot-theme] failed to parse theme sync file", error);
			return;
		}

		log("theme sync payload received", payload);
		await applyTheme(payload.theme);
	};

	const watcher = vscode.workspace.createFileSystemWatcher(themeSyncFilePath);
	watcher.onDidCreate(() => {
		void syncThemeFromFile().catch((error) => {
			console.error("[discobot-theme] failed to sync created theme file", error);
		});
	});
	watcher.onDidChange(() => {
		void syncThemeFromFile().catch((error) => {
			console.error("[discobot-theme] failed to sync changed theme file", error);
		});
	});
	watcher.onDidDelete(() => {
		log("theme sync file deleted");
	});
	context.subscriptions.push(watcher);

	void syncThemeFromFile().catch((error) => {
		console.error("[discobot-theme] initial theme sync failed", error);
	});
}

function deactivate() {
	log("deactivate called");
}
