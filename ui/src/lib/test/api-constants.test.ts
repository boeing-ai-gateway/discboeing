import assert from "node:assert/strict";
import test from "node:test";

import { CommitOperation, CommitStatus, SessionStatus } from "../api-constants";

test("api constants expose backend runtime values", () => {
	assert.equal(SessionStatus.READY, "ready");
	assert.equal(SessionStatus.STOPPED, "stopped");
	assert.equal(SessionStatus.REMOVED, "removed");
	assert.equal(CommitStatus.PENDING, "pending");
	assert.equal(CommitOperation.COMMIT, "commit");
});
