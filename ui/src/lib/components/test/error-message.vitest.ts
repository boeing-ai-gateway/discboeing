import assert from "node:assert/strict";
import { test } from "vitest";
import { getErrorMessage } from "../../error-message";

test("getErrorMessage extracts useful messages", () => {
	assert.equal(getErrorMessage(null), null);
	assert.equal(getErrorMessage(""), null);
	assert.equal(getErrorMessage("boom"), "boom");
	assert.equal(getErrorMessage(new Error("failed")), "failed");
	assert.equal(getErrorMessage({ message: "bad request" }), "bad request");
	assert.equal(getErrorMessage({ error: "not found" }), "not found");
	assert.equal(getErrorMessage({ detail: "denied" }), "denied");
	assert.equal(getErrorMessage({ code: "E_FAIL" }), '{"code":"E_FAIL"}');
	assert.equal(getErrorMessage(Symbol("simple")), "An unknown error occurred.");
	assert.equal(getErrorMessage(404), "404");
});
