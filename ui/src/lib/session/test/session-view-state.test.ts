import assert from "node:assert/strict";
import test from "node:test";

import { resolveOpenFileState } from "../view/create-session-view-state.svelte";

test("resolveOpenFileState clears the active file when given an empty string", () => {
	const nextState = resolveOpenFileState("", "src/app.ts");

	assert.deepEqual(nextState.activeView, { kind: "file", path: "" });
	assert.equal(nextState.selectedFile, "");
});

test("resolveOpenFileState without an argument opens an empty files panel when no file is selected", () => {
	const nextState = resolveOpenFileState(undefined, "");

	assert.deepEqual(nextState.activeView, { kind: "file", path: "" });
	assert.equal(nextState.selectedFile, "");
});

test("resolveOpenFileState without an argument keeps the selected file", () => {
	const nextState = resolveOpenFileState(undefined, "src/app.ts");

	assert.deepEqual(nextState.activeView, { kind: "file", path: "src/app.ts" });
	assert.equal(nextState.selectedFile, "src/app.ts");
});
