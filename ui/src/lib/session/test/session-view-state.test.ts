import assert from "node:assert/strict";
import test from "node:test";

import { resolveOpenFileState } from "../view/create-session-view-state.svelte";

test("resolveOpenFileState clears the active file when given an empty string", () => {
	const nextState = resolveOpenFileState("", "src/app.ts", ["src/app.ts"]);

	assert.deepEqual(nextState.activeView, { kind: "file", path: "" });
	assert.equal(nextState.selectedFile, "");
});

test("resolveOpenFileState without an argument opens an empty files panel when no files exist", () => {
	const nextState = resolveOpenFileState(undefined, "", []);

	assert.deepEqual(nextState.activeView, { kind: "file", path: "" });
	assert.equal(nextState.selectedFile, "");
});
