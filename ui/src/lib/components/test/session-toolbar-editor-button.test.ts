import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(
	new URL("../app/SessionToolbar.svelte", import.meta.url),
	"utf8",
);

test("session toolbar gates the editor button behind the user preference", () => {
	assert.match(
		source,
		/const showEditorButton = \$derived\.by\(\(\) => preferences\.showEditorButton\);/,
	);
	assert.match(source, /\{#if showEditorButton\}[\s\S]*Editor[\s\S]*\{\/if\}/);
	assert.match(source, /disabled=\{!vscodeAvailable\}/);
});
