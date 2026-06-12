import assert from "node:assert/strict";
import { test } from "vitest";

import {
	canLoadSessionThreads,
	CommitOperation,
	CommitStatus,
	isSessionTransitioningStatus,
	SessionDisplayStatus,
	SessionStatus,
} from "../api-constants";

test("backend session constants only contain lifecycle states", () => {
	assert.equal(SessionStatus.READY, "ready");
	assert.equal(SessionStatus.STOPPED, "stopped");
	assert.equal(SessionStatus.REMOVED, "removed");
	assert.equal(CommitStatus.PENDING, "pending");
	assert.equal(CommitOperation.COMMIT, "commit");
	assert.equal(SessionDisplayStatus.COMMITTED, "committed");
});

test("canLoadSessionThreads accepts canonical lifecycle states", () => {
	assert.equal(canLoadSessionThreads(SessionStatus.READY), true);
	assert.equal(canLoadSessionThreads(SessionStatus.STOPPED), true);
	assert.equal(canLoadSessionThreads(SessionStatus.ERROR), true);
	assert.equal(canLoadSessionThreads(SessionStatus.CREATING_SANDBOX), false);
	assert.equal(canLoadSessionThreads(SessionStatus.REMOVING), false);
	assert.equal(canLoadSessionThreads(null), false);
	assert.equal(canLoadSessionThreads(undefined), false);
});

test("isSessionTransitioningStatus only marks lifecycle transitions", () => {
	assert.equal(isSessionTransitioningStatus(SessionStatus.INITIALIZING), true);
	assert.equal(
		isSessionTransitioningStatus(SessionStatus.CREATING_SANDBOX),
		true,
	);
	assert.equal(isSessionTransitioningStatus(SessionStatus.REMOVING), true);
	assert.equal(isSessionTransitioningStatus(SessionStatus.STOPPED), false);
	assert.equal(isSessionTransitioningStatus(SessionStatus.READY), false);
});
