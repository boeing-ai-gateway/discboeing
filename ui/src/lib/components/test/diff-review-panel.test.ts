import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const DIFF_REVIEW_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/DiffReviewPanel.svelte",
);

function readDiffReviewPanelSource() {
	return readFileSync(DIFF_REVIEW_PANEL_COMPONENT, "utf-8");
}

test("diff review panel captures selected diff text and exposes a comment prompt", () => {
	const source = readDiffReviewPanelSource();

	assert.doesNotMatch(source, /onQueueSelectionComment/);
	assert.match(source, /onSubmitSelectionComment: \(payload: \{/);
	assert.match(source, /let selectedDiffTextByPath = \$state</);
	assert.match(source, /Record<string, DiffSelectionState \| null>/);
	assert.match(source, /type DiffSelectionState = \{/);
	assert.match(source, /function handleLineSelection\(/);
	assert.match(source, /function buildSelectedDiffText\(/);
	assert.match(source, /async function submitSelectionComment\(path: string\)/);
	assert.match(
		source,
		/<Textarea[\s\S]*placeholder="Add a comment for the assistant"/,
	);
	assert.doesNotMatch(source, /Queue/);
	assert.match(source, /Submit/);
	assert.match(source, /selectedLines=\{getSelectedLines\(file\.path\)\}/);
	assert.match(
		source,
		/onLineSelected=\{\(range\) =>[\s\S]*handleLineSelection\(file\.path, state, range\)/,
	);
});
