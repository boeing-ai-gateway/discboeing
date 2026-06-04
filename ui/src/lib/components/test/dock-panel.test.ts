import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const DOCK_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/DockPanel.svelte",
);
const FILES_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/FilesPanel.svelte",
);

function readDockPanelSource() {
	return readFileSync(DOCK_PANEL_COMPONENT, "utf-8");
}

function readFilesPanelSource() {
	return readFileSync(FILES_PANEL_COMPONENT, "utf-8");
}

test("dock panel lazy-mounts panes on first open and keeps them mounted afterward", () => {
	const source = readDockPanelSource();

	assert.match(
		source,
		/type DockPanelKind = Exclude<SessionActiveView\["kind"\], "chat">/,
	);
	assert.match(
		source,
		/let mountedDockPanelKinds = \$state<DockPanelKind\[]>\(\[\]\)/,
	);
	assert.match(
		source,
		/if \(!activeKind \|\| mountedDockPanelKinds\.includes\(activeKind\)\) \{/,
	);
	assert.match(
		source,
		/mountedDockPanelKinds = \[\.\.\.mountedDockPanelKinds, activeKind\]/,
	);
	assert.match(source, /\{#if mountedDockPanelKinds\.includes\("terminal"\)\}/);
	assert.match(source, /\{#if mountedDockPanelKinds\.includes\("desktop"\)\}/);
	assert.match(source, /\{#if mountedDockPanelKinds\.includes\("vscode"\)\}/);
	assert.match(source, /\{#if mountedDockPanelKinds\.includes\("file"\)\}/);
	assert.match(
		source,
		/\{#if mountedDockPanelKinds\.includes\("diff-review"\)\}/,
	);
	assert.match(
		source,
		/\{#if visibleServices.length > 0 && mountedDockPanelKinds\.includes\("services"\)\}/,
	);
	assert.match(
		source,
		/class=\{sessionView\.activeView\.kind === "terminal" \? "contents" : "hidden"\}/,
	);
	assert.match(
		source,
		/class=\{sessionView\.activeView\.kind === "vscode" \? "contents" : "hidden"\}/,
	);
	assert.doesNotMatch(source, /editorEnabled/);
	assert.match(source, /async function handleOpenDiffFile\(path: string\)/);
	assert.match(source, /await requestVSCodeOpenFile\(sessionId, path\);/);
	assert.match(source, /sessionView\.openVSCode\(\);/);
	assert.match(source, /await openFile\(sessionId, path\);/);
	assert.match(source, /onOpenFile=\{handleOpenDiffFile\}/);
	assert.match(
		source,
		/class=\{sessionView\.activeView\.kind === "services" \? "contents" : "hidden"\}/,
	);
	assert.doesNotMatch(source, /SessionContextValue|ThreadContextValue/);
	assert.match(source, /sessionId: string;/);
	assert.match(source, /threadId: string;/);
	assert.match(source, /sessionView: SessionViewState;/);
	assert.doesNotMatch(source, /legacy-context-bridge/);
	assert.doesNotMatch(source, /useThreadBridge/);
	assert.match(
		source,
		/onQueueSelectionComment=\{handleQueueDiffSelectionComment\}/,
	);
	assert.match(source, /addThreadPendingComment\(sessionId, threadId, \{/);
});

test("files panel receives root files data/view and callbacks", () => {
	const dockSource = readDockPanelSource();
	const filesSource = readFilesPanelSource();

	assert.doesNotMatch(
		dockSource,
		/getSessionFilesDomain|filesDomain|SessionFilesDomain|SessionContextValue|ThreadContextValue/,
	);
	assert.match(dockSource, /context\.data\.files\.bySessionId\[sessionId\]/);
	assert.match(dockSource, /context\.view\.sessions\[sessionId\]\?\.files/);
	assert.match(
		dockSource,
		/fileView\.buffers\[path\]\?\.content \?\? record\.content/,
	);
	assert.match(dockSource, /<FilesPanel\s+\{fileData\}\s+\{fileView\}/);
	assert.match(dockSource, /actions=\{filePanelActions\}/);
	assert.match(dockSource, /const filePanelActions: FilesPanelActions = \{/);
	assert.match(dockSource, /openFile: \(path\) => openFile\(sessionId, path\)/);
	assert.match(
		dockSource,
		/updateBuffer: \(path, content\) => updateFileBuffer\(sessionId, path, content\)/,
	);

	assert.doesNotMatch(
		filesSource,
		/SessionFilesDomain|getSessionFilesDomain|useContext|context\/commands|app-view/,
	);
	assert.match(filesSource, /fileData: SessionFilesData;/);
	assert.match(filesSource, /fileView: FilesPanelView;/);
	assert.match(filesSource, /export type FilesPanelView = Pick</);
	assert.match(filesSource, /export type FilesPanelActions = \{/);
	assert.match(filesSource, /openFile: \(path\?: string\) => Promise<void>;/);
});
