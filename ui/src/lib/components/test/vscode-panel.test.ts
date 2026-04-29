import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const VSCODE_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/VSCodePanel.svelte",
);

function readVSCodePanelSource() {
	return readFileSync(VSCODE_PANEL_COMPONENT, "utf-8");
}

test("vscode panel syncs the theme file for the embedded editor", () => {
	const source = readVSCodePanelSource();

	assert.match(
		source,
		/const VSCODE_THEME_FILE_PATH = "\.discobot\/\.vscode-theme\.json";/,
	);
	assert.match(
		source,
		/const GIT_EXCLUDE_ENTRY = `\$\{VSCODE_THEME_FILE_PATH\}\\n`;/,
	);
	assert.match(
		source,
		/function buildThemePayload\(nextTheme: ResolvedTheme\): string/,
	);
	assert.match(
		source,
		/path: VSCODE_THEME_FILE_PATH,[\s\S]*content: buildThemePayload\(resolvedTheme\),/m,
	);
	assert.doesNotMatch(source, /openPath\?: string;/);
	assert.doesNotMatch(source, /VSCODE_OPEN_FILE_PATH/);
});
