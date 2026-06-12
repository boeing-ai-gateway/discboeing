import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const DIFF_REVIEW_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/DiffReviewPanel.svelte",
);

const DIFF_REVIEW_SELECTION_COMMENT_POPOVER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/DiffReviewSelectionCommentPopover.svelte",
);

function readDiffReviewPanelSource() {
	return readFileSync(DIFF_REVIEW_PANEL_COMPONENT, "utf-8");
}

function readDiffReviewSelectionCommentPopoverSource() {
	return readFileSync(DIFF_REVIEW_SELECTION_COMMENT_POPOVER_COMPONENT, "utf-8");
}

test("diff review panel captures selected diff text and exposes a comment prompt", () => {
	const source = readDiffReviewPanelSource();

	assert.match(source, /onQueueSelectionComment: \(payload: \{/);
	assert.match(source, /onSubmitSelectionComment: \(payload: \{/);
	assert.match(source, /let selectedDiffTextByPath = \$state</);
	assert.match(source, /Record<string, DiffSelectionState \| null>/);
	assert.match(source, /type DiffSelectionState = \{/);
	assert.match(source, /function handleLineSelection\(/);
	assert.match(source, /function buildSelectedDiffText\(/);
	assert.match(
		source,
		/async function saveSelectionComment\(\s*path: string,\s*action: "queue" \| "submit",\s*\)/,
	);
	assert.match(source, /DiffReviewSelectionCommentPopover/);
	assert.match(source, /selectedText=\{getSelectedDiffText\(file\.path\)\}/);
	assert.match(source, /onDraftChange=\{updateCommentDraft\}/);
	assert.match(source, /onSave=\{saveSelectionComment\}/);
	assert.match(source, /onClear=\{resetSelectionComment\}/);
	assert.match(source, /"queue"/);
	assert.match(source, /"submit"/);
	assert.match(source, /selectedLines=\{getSelectedLines\(file\.path\)\}/);
	assert.match(
		source,
		/onLineSelected=\{\(range\) =>[\s\S]*handleLineSelection\(file\.path, state, range\)/,
	);
});

test("diff review panel keeps diff target draft synced with prop changes", () => {
	const source = readDiffReviewPanelSource();

	assert.match(
		source,
		/let diffTargetDraft = \$derived\(diffTarget === "HEAD" \? "" : diffTarget\);/,
	);
});

test("diff review panel does not automatically retry failed diff loads", () => {
	const source = readDiffReviewPanelSource();

	assert.match(source, /if \(state && state\.status !== "idle"\) \{/);
	assert.match(
		source,
		/async function refreshPanel\(\) \{[\s\S]*loadGeneration \+= 1;[\s\S]*clearDiffStates\(\);[\s\S]*await onRefresh\(\);/,
	);
});

test("diff review selection comment popover exposes accessible comment actions", () => {
	const source = readDiffReviewSelectionCommentPopoverSource();

	assert.match(source, /role="dialog"/);
	assert.match(source, /tabindex="-1"/);
	assert.match(source, /<Label[\s\S]*Comment for the assistant/);
	assert.match(
		source,
		/<Textarea[\s\S]*placeholder="Add a comment for the assistant"/,
	);
	assert.match(source, /\{queueing \? "Queueing…" : "Queue"\}/);
	assert.match(source, /\{submitting \? "Submitting…" : "Submit"\}/);
	assert.match(source, /Clear selection/);
});
