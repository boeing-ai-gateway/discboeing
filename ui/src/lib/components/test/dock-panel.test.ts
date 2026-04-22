import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const DOCK_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/DockPanel.svelte",
);

function readDockPanelSource() {
	return readFileSync(DOCK_PANEL_COMPONENT, "utf-8");
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
		/class=\{sessionView\.activeView\.kind === "services" \? "contents" : "hidden"\}/,
	);
	assert.match(source, /const thread = useThreadContext\(\)/);
	assert.match(
		source,
		/onSubmitSelectionComment=\{handleSubmitDiffSelectionComment\}/,
	);
	assert.match(source, /buildUserMessageParts\(prompt, \[\]\)/);
});
