import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const source = readFileSync(
	path.resolve(import.meta.dirname, "../app/SessionToolbar.svelte"),
	"utf8",
);

test("session toolbar always shows the editor button", () => {
	assert.doesNotMatch(source, /showEditorButton/);
	assert.match(source, /<Button[\s\S]*Editor[\s\S]*<\/Button>/);
	assert.match(source, /disabled=\{!vscodeAvailable\}/);
});
