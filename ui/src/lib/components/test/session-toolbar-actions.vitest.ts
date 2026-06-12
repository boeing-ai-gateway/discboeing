import assert from "node:assert/strict";
import { test } from "vitest";

import type { Session } from "../../api-types";

import { getSessionToolbarOperationState } from "../app/session-toolbar-actions";

function makeSession(overrides: Partial<Session> = {}): Session {
	return {
		id: "session-1",
		projectId: "project-1",
		workspaceId: "workspace-1",
		name: "Session",
		description: "",
		createdAt: new Date().toISOString(),
		updatedAt: new Date().toISOString(),
		sandboxStatus: "ready",
		commitStatus: "",
		...overrides,
	};
}

test("shows commit as the primary action when changes exist", () => {
	const state = getSessionToolbarOperationState({
		filesChanged: 3,
		session: makeSession(),
		startingOperation: null,
	});

	assert.equal(state.showSplitButton, true);
	assert.equal(state.primaryAction, "commit");
	assert.equal(state.primaryLabel, "Commit");
	assert.equal(state.secondaryAction, "rebase");
	assert.equal(state.secondaryLabel, "Rebase");
	assert.equal(state.buttonLabel, "Commit");
	assert.equal(state.showBusy, false);
});

test("keeps commit and rebase available even when there are no changes", () => {
	const state = getSessionToolbarOperationState({
		filesChanged: 0,
		session: makeSession(),
		startingOperation: null,
	});

	assert.equal(state.showSplitButton, true);
	assert.equal(state.primaryAction, "commit");
	assert.equal(state.primaryLabel, "Commit");
	assert.equal(state.secondaryAction, "rebase");
	assert.equal(state.secondaryLabel, "Rebase");
	assert.equal(state.buttonLabel, "Commit");
});

test("keeps commit as the primary action while a dropdown-triggered rebase is starting", () => {
	const state = getSessionToolbarOperationState({
		filesChanged: 2,
		session: makeSession(),
		startingOperation: "rebase",
	});

	assert.equal(state.primaryAction, "commit");
	assert.equal(state.activeOperation, "rebase");
	assert.equal(state.showBusy, true);
	assert.equal(state.buttonLabel, "Rebasing...");
});

test("shows pending state from the session commit status", () => {
	const state = getSessionToolbarOperationState({
		filesChanged: 1,
		session: makeSession({
			commitStatus: "pending",
		}),
		startingOperation: null,
	});

	assert.equal(state.showPending, true);
	assert.equal(state.showBusy, true);
	assert.equal(state.buttonLabel, "Pending...");
});

test("does not show command progress for committing status without an active operation hint", () => {
	const state = getSessionToolbarOperationState({
		filesChanged: 1,
		session: makeSession({
			commitStatus: "committing",
		}),
		startingOperation: null,
	});

	assert.equal(state.showPending, false);
	assert.equal(state.showBusy, false);
	assert.equal(state.buttonLabel, "Commit");
});
