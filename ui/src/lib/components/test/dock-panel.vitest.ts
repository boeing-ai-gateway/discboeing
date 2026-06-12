import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

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
		/type DockPanelKind = Exclude<SessionActiveViewKind, "chat">/,
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
		/class=\{activeView\.kind === "terminal" \? "contents" : "hidden"\}/,
	);
	assert.match(
		source,
		/class=\{activeView\.kind === "vscode" \? "contents" : "hidden"\}/,
	);
	assert.doesNotMatch(source, /editorEnabled/);
	assert.match(source, /async function handleOpenDiffFile\(path: string\)/);
	assert.match(source, /await requestVSCodeOpenFile\(sessionId, path\);/);
	assert.match(source, /openVSCode\(\);/);
	assert.match(source, /await openFile\(path\);/);
	assert.match(source, /onOpenFile=\{handleOpenDiffFile\}/);
	assert.match(
		source,
		/class=\{activeView\.kind === "services" \? "contents" : "hidden"\}/,
	);
	assert.doesNotMatch(source, /SessionRuntimeState|ThreadRuntimeController/);
	assert.match(source, /sessionId: string;/);
	assert.match(source, /threadId: string;/);
	assert.doesNotMatch(source, /sessionView: SessionViewFacade;/);
	assert.doesNotMatch(source, /\$lib\/context\/runtime/);
	assert.doesNotMatch(source, /legacy-context-bridge/);
	assert.doesNotMatch(source, /useThreadBridge/);
	assert.match(
		source,
		/onQueueSelectionComment=\{handleQueueDiffSelectionComment\}/,
	);
	assert.match(
		source,
		/context\.commands\.threadComposer\.addThreadPendingComment\(\s*sessionId,\s*threadId,\s*\{/,
	);
	assert.match(
		source,
		/await context\.commands\.files\.saveFile\(sessionId, path, buffer\.content, \{[\s\S]*wait: true,[\s\S]*encoding: buffer\.encoding,[\s\S]*originalContent: options\.force \? undefined : buffer\.originalContent,/,
	);
	assert.doesNotMatch(
		source,
		/async function saveFile[\s\S]*api\.writeSessionFile/,
	);
	assert.doesNotMatch(source, /diffTarget: "working"/);
	assert.match(source, /diffTarget: ""/);
	assert.match(source, /diffTarget: fileView\.diffTarget/);
	assert.match(
		source,
		/fileView\.diffTarget === ""[\s\S]*sessionRecord\.diff\.files[\s\S]*fileView\.diffFilesByTarget\[fileView\.diffTarget\]/,
	);
	assert.match(source, /const diff = diffFiles\?\.files \?\? \[\]/);
	assert.match(
		source,
		/diffStats: diffFiles\?\.stats \?\? emptyFileData\.diffStats/,
	);
	assert.match(
		source,
		/function handleDiffTargetChange\(target: string\) \{[\s\S]*context\.commands\.files\.setDiffTarget\(sessionId, target\)/,
	);
	assert.match(
		source,
		/function refreshDiffReview\(\) \{[\s\S]*return context\.commands\.files\.setDiffTarget\(sessionId, fileView\.diffTarget\);/,
	);
	assert.match(source, /onDiffTargetChange=\{handleDiffTargetChange\}/);
	assert.match(source, /onRefresh=\{refreshDiffReview\}/);
});

test("files panel receives root files data/view and callbacks", () => {
	const dockSource = readDockPanelSource();
	const filesSource = readFilesPanelSource();

	assert.doesNotMatch(
		dockSource,
		/getSessionFilesDomain|filesDomain|SessionFilesDomain|SessionRuntimeState|ThreadRuntimeController/,
	);
	assert.match(dockSource, /sessionRecord\?\.files/);
	assert.match(dockSource, /sessionView\?\.files/);
	assert.match(dockSource, /fileView\.buffers\[path\] = \{/);
	assert.match(dockSource, /<FilesPanel\s+\{fileData\}\s+\{fileView\}/);
	assert.match(dockSource, /actions=\{filePanelActions\}/);
	assert.match(dockSource, /const filePanelActions: FilesPanelActions = \{/);
	assert.match(dockSource, /openFile: \(path\) => openFile\(path\)/);
	assert.match(
		dockSource,
		/updateBuffer: \(path, content\) => updateFileBuffer\(path, content\)/,
	);

	assert.doesNotMatch(
		filesSource,
		/SessionFilesDomain|getSessionFilesDomain|useContext|context\/commands/,
	);
	assert.match(filesSource, /fileData: SessionFilesData;/);
	assert.match(filesSource, /fileView: FilesPanelView;/);
	assert.match(filesSource, /export type FilesPanelView = \{/);
	assert.match(filesSource, /export type FilesPanelActions = \{/);
	assert.match(filesSource, /openFile: \(path\?: string\) => Promise<void>;/);
});
