import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const SESSION_TOOLBAR_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/SessionToolbar.svelte",
);
const APP_SHELL_CONFIG = path.resolve(
	import.meta.dirname,
	"../../app/app-shell-config.ts",
);

function readSessionToolbarSource() {
	return readFileSync(SESSION_TOOLBAR_COMPONENT, "utf-8");
}

function readAppShellConfigSource() {
	return readFileSync(APP_SHELL_CONFIG, "utf-8");
}

test("session toolbar reserves a dedicated editor pane and hides its service from the generic list", () => {
	const source = readSessionToolbarSource();

	assert.match(
		source,
		/service\.id !== DESKTOP_SERVICE_ID && service\.id !== VSCODE_SERVICE_ID/,
	);
	assert.match(source, /const vscodeAvailable = \$derived\.by\(\(\) =>/);
	assert.match(source, /function toggleVSCode\(\)/);
	assert.match(source, /if \(sessionView\.activeView\.kind === "vscode"\) \{/);
	assert.match(source, /sessionView\.openVSCode\(\);/);
	assert.match(source, /disabled=\{!vscodeAvailable\}/);
	assert.match(
		source,
		/<CodeIcon class="size-3\.5" \/>\s*\{#if !preferences\.topBarIconOnly\}\s*Editor\s*\{\/if\}\s*<\/Button>/,
	);
	assert.match(source, /iconOnly=\{preferences\.topBarIconOnly\}/);
	assert.match(source, /aria-label=\{operationState\.buttonLabel\}/);
	assert.match(source, /<path d=\{selectedIdeOption\.icon\.path\} \/>/);
	assert.match(source, /<path d=\{option\.icon\.path\} \/>/);
});

test("session toolbar preferred IDE picker uses branded icon metadata", () => {
	const source = readAppShellConfigSource();

	assert.match(source, /siCursor/);
	assert.match(source, /siZedindustries/);
	assert.match(source, /siJetbrains/);
	assert.match(source, /const vscodeIcon: IdeIcon = \{/);
	assert.match(
		source,
		/\{ id: "vscode", label: "VS Code", family: "standard", icon: vscodeIcon \}/,
	);
	assert.match(
		source,
		/\{ id: "cursor", label: "Cursor", family: "standard", icon: cursorIcon \}/,
	);
	assert.match(
		source,
		/\{ id: "zed", label: "Zed", family: "standard", icon: zedIcon \}/,
	);
	assert.match(source, /icon: jetbrainsIcon/);
});

test("session toolbar preserves a desktop drag region while keeping controls interactive", () => {
	const source = readSessionToolbarSource();

	assert.match(source, /data-desktop-drag-region/);
	assert.doesNotMatch(source, /data-tauri-drag-region/);
	assert.match(
		source,
		/class="desktop-no-drag inline-flex items-center overflow-hidden rounded-md bg-background p-0\.5 shadow-xs"/,
	);
	assert.match(
		source,
		/class="desktop-no-drag group inline-flex items-center overflow-hidden rounded-md bg-background p-0\.5 text-sidebar-foreground\/70 shadow-xs"/,
	);
});
