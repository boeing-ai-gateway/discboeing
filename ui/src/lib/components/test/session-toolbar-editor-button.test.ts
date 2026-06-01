import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(
	new URL("../app/SessionToolbar.svelte", import.meta.url),
	"utf8",
);

test("session toolbar always shows the editor button", () => {
	assert.doesNotMatch(source, /showEditorButton/);
	assert.match(source, /<Button[\s\S]*Editor[\s\S]*<\/Button>/);
	assert.match(source, /disabled=\{!vscodeAvailable\}/);
});
