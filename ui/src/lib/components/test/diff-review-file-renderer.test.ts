import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const DIFF_REVIEW_FILE_RENDERER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/DiffReviewFileRenderer.svelte",
);
const PIERRE_DIFF_MODULE = path.resolve(
	import.meta.dirname,
	"../../pierre-diff.ts",
);

function readDiffReviewFileRendererSource() {
	return readFileSync(DIFF_REVIEW_FILE_RENDERER_COMPONENT, "utf-8");
}

function readPierreDiffSource() {
	return readFileSync(PIERRE_DIFF_MODULE, "utf-8");
}

test("diff review file renderer enables Pierre line selection and gutter affordances", () => {
	const source = readDiffReviewFileRendererSource();

	assert.match(source, /type SelectedLineRange/);
	assert.match(
		source,
		/onLineSelected\?: \(range: SelectedLineRange \| null\) => void/,
	);
	assert.match(source, /selectedLines\?: SelectedLineRange \| null/);
	assert.match(source, /enableGutterUtility: true/);
	assert.match(source, /enableLineSelection: true/);
	assert.match(source, /lineHoverHighlight: "number"/);
	assert.match(
		source,
		/currentInstance\.setSelectedLines\(currentSelectedLines \?\? null\)/,
	);
});

test("pierre diff overrides move the gutter utility button left of the line number", () => {
	const source = readPierreDiffSource();

	assert.match(source, /const DIFF_GUTTER_UTILITY_CSS = `/);
	assert.match(source, /\[data-gutter-utility-slot\][\s\S]*left: 0;/);
	assert.match(source, /\[data-gutter-utility-slot\][\s\S]*right: auto;/);
	assert.match(source, /\[data-utility-button\][\s\S]*margin-left: 0\.5ch;/);
	assert.match(source, /unsafeCSS: DIFF_GUTTER_UTILITY_CSS/);
});
