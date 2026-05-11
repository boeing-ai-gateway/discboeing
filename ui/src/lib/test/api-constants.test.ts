import assert from "node:assert/strict";
import test from "node:test";

import {
	canLoadSessionThreads,
	isSessionTransitioningStatus,
	SessionStatus,
} from "../api-constants";

test("canLoadSessionThreads restores replayable session states", () => {
	assert.equal(canLoadSessionThreads(SessionStatus.READY), true);
	assert.equal(canLoadSessionThreads(SessionStatus.STOPPED), true);
	assert.equal(canLoadSessionThreads(SessionStatus.COMMITTED), true);
	assert.equal(canLoadSessionThreads(SessionStatus.COMPLETED), true);
	assert.equal(canLoadSessionThreads(SessionStatus.COMMITTING), true);
	assert.equal(canLoadSessionThreads(SessionStatus.ERROR), true);
	assert.equal(canLoadSessionThreads(SessionStatus.PENDING), false);
	assert.equal(canLoadSessionThreads(SessionStatus.CREATING_SANDBOX), false);
	assert.equal(canLoadSessionThreads(SessionStatus.REMOVING), false);
	assert.equal(canLoadSessionThreads(null), false);
	assert.equal(canLoadSessionThreads(undefined), false);
});

test("isSessionTransitioningStatus still marks startup and commit transitions", () => {
	assert.equal(isSessionTransitioningStatus(SessionStatus.PENDING), true);
	assert.equal(isSessionTransitioningStatus(SessionStatus.COMMITTING), true);
	assert.equal(isSessionTransitioningStatus(SessionStatus.STOPPED), false);
	assert.equal(isSessionTransitioningStatus(SessionStatus.COMMITTED), false);
});
