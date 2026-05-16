import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const VSCODE_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/VSCodePanel.svelte",
);
const EDITOR_CONTROL_MODULE = path.resolve(
	import.meta.dirname,
	"../../editor-control.ts",
);
const VSCODE_THEME_EXTENSION = path.resolve(
	import.meta.dirname,
	"../../../../../container-assets/code-server/extensions/discobot.discobot-theme/dist/web-extension.js",
);

const DOCKERFILE = path.resolve(
	import.meta.dirname,
	"../../../../../Dockerfile",
);

const VSCODE_SYSTEMD_SERVICE = path.resolve(
	import.meta.dirname,
	"../../../../../container-assets/systemd/discobot-vscode-backend.service",
);

function readDockerfileSource() {
	return readFileSync(DOCKERFILE, "utf-8");
}

function readVSCodePanelSource() {
	return readFileSync(VSCODE_PANEL_COMPONENT, "utf-8");
}

function readEditorControlSource() {
	return readFileSync(EDITOR_CONTROL_MODULE, "utf-8");
}

function readVSCodeThemeExtensionSource() {
	return readFileSync(VSCODE_THEME_EXTENSION, "utf-8");
}

function readVSCodeSystemdServiceSource() {
	return readFileSync(VSCODE_SYSTEMD_SERVICE, "utf-8");
}

test("vscode panel syncs the theme file for the embedded editor", () => {
	const panelSource = readVSCodePanelSource();
	const controlSource = readEditorControlSource();

	assert.match(panelSource, /syncEditorTheme\(sessionId, resolvedTheme\)/);
	assert.match(
		controlSource,
		/export const VSCODE_THEME_FILE_PATH = "~\/\.discobot\/editor\/\.vscode-theme\.json";/,
	);
	assert.match(
		controlSource,
		/export const VSCODE_CONTROL_FILE_PATH =\s*"~\/\.discobot\/editor\/\.vscode-control\.json";/,
	);
	assert.match(
		controlSource,
		/function buildThemePayload\(nextTheme: ResolvedTheme\): string/,
	);
	assert.match(
		controlSource,
		/path: VSCODE_THEME_FILE_PATH,[\s\S]*content: buildThemePayload\(resolvedTheme\),/m,
	);
});

test("diff open file requests vscode through the editor control file", () => {
	const source = readEditorControlSource();

	assert.match(source, /type: "openFile"/);
	assert.match(source, /path: VSCODE_CONTROL_FILE_PATH/);
	assert.match(source, /content: buildOpenFilePayload\(path\)/);
});

test("vscode theme extension watches and retries theme sync", () => {
	const source = readVSCodeThemeExtensionSource();

	assert.match(source, /const workspaceFolder = getWorkspaceFolder\(\);/);
	assert.match(source, /workspaceFolder\.uri\.with\(\{ path: homeDir \}\)/);
	assert.match(
		source,
		/new vscode\.RelativePattern\([\s\S]*getHomeDirUri\(\),[\s\S]*`\$\{editorStateDirRelativeToHome\}\/\$\{relativePath\}`/m,
	);
	assert.match(
		source,
		/vscode\.workspace\.createFileSystemWatcher\([\s\S]*getEditorStateFilePattern\(themeSyncFilePath\)/m,
	);
	assert.match(source, /for \(const retryDelay of \[0, 250, 1000, 2500\]\)/);
	assert.match(source, /await syncThemeFromFile\(\);/);
});

test("vscode theme extension consumes open-file control commands", () => {
	const source = readVSCodeThemeExtensionSource();

	assert.match(
		source,
		/const editorStateDirRelativeToHome = "\.discobot\/editor";/,
	);
	assert.match(source, /const controlFilePath = "\.vscode-control\.json";/);
	assert.match(
		source,
		/vscode\.workspace\.createFileSystemWatcher\([\s\S]*getEditorStateFilePattern\(controlFilePath\)/m,
	);
	assert.match(source, /for \(const retryDelay of \[0, 250, 1000, 2500\]\)/);
	assert.match(source, /processControlCommand\("process initial command"\)/);
	assert.match(source, /payload\.type !== "openFile"/);
	assert.match(
		source,
		/vscode\.window\.showTextDocument\(fileUri, \{ preview: false \}\)/,
	);
	assert.match(
		source,
		/vscode\.workspace\.fs\.delete\(controlUri, \{ useTrash: false \}\)/,
	);
});

test("dockerfile does not ship stale code-server extension registry", () => {
	const source = readDockerfileSource();

	assert.match(
		source,
		/code-server --install-extension svelte\.svelte-vscode --extensions-dir \/opt\/discobot\/code-server-defaults\/extensions \\\n\s+&& rm -f \/opt\/discobot\/code-server-defaults\/extensions\/extensions\.json \\/,
	);
	assert.match(
		source,
		/COPY --chown=1000:1000 container-assets\/code-server\/ \/opt\/discobot\/code-server-defaults\//,
	);
});

test("vscode service forces an extension rescan after seeding bundled theme", () => {
	const source = readVSCodeSystemdServiceSource();

	assert.match(
		source,
		/rm -rf \/home\/discobot\/\.local\/share\/discobot-code-server\/extensions\/discobot\.discobot-theme && cp -r \/opt\/discobot\/code-server-defaults\/extensions\/discobot\.discobot-theme/,
	);
	assert.match(
		source,
		/rm -f \/home\/discobot\/\.local\/share\/discobot-code-server\/extensions\/extensions\.json/,
	);
	assert.match(
		source,
		/\/home\/discobot\/\.local\/share\/discobot-code-server\/extensions\/\.obsolete/,
	);
	assert.match(
		source,
		/\/home\/discobot\/\.local\/share\/discobot-code-server\/CachedProfilesData\/__default__profile__\/extensions\.user\.cache/,
	);
});
