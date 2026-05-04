import test from "node:test";
import assert from "node:assert/strict";

import {
	normalizeTauriVersion,
	validateExplicitTauriVersion,
} from "./build-app.mjs";

test("normalizeTauriVersion keeps stable semver versions", () => {
	assert.equal(normalizeTauriVersion("1.2.3"), "1.2.3");
});

test("normalizeTauriVersion converts prerelease labels to numeric MSI-safe values", () => {
	assert.equal(normalizeTauriVersion("0.0.0-dev"), "0.0.0-0");
	assert.equal(normalizeTauriVersion("0.1.0-alpha12"), "0.1.0-12");
	assert.equal(normalizeTauriVersion("0.1.0-beta.4"), "0.1.0-4");
});

test("normalizeTauriVersion clamps numeric prerelease values above the MSI limit", () => {
	assert.equal(normalizeTauriVersion("1.2.3-99999"), "1.2.3-65535");
});

test("validateExplicitTauriVersion accepts explicit MSI-safe versions", () => {
	assert.equal(validateExplicitTauriVersion("1.2.3"), "1.2.3");
	assert.equal(validateExplicitTauriVersion("1.2.3-42"), "1.2.3-42");
});

test("validateExplicitTauriVersion rejects unsupported overrides", () => {
	assert.throws(() => validateExplicitTauriVersion("1.2.3-dev"));
	assert.throws(() => validateExplicitTauriVersion("1.2.3-70000"));
});
