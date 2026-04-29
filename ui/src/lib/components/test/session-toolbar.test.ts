import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const SESSION_TOOLBAR_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/SessionToolbar.svelte",
);

function readSessionToolbarSource() {
	return readFileSync(SESSION_TOOLBAR_COMPONENT, "utf-8");
}

test("session toolbar reserves a dedicated editor pane and hides its service from the generic list", () => {
	const source = readSessionToolbarSource();

	assert.match(
		source,
		/service\.id !== DESKTOP_SERVICE_ID && service\.id !== VSCODE_SERVICE_ID/,
	);
	assert.match(source, /const vscodeAvailable = \$derived\.by\(\(\) =>/);
	assert.match(source, /function toggleVSCode\(\)/);
	assert.match(source, /if \(sessionView\.activeView\.kind === "vscode"\) \{/);
	assert.match(source, /sessionView\.openVSCode\(\);/);
	assert.match(source, /disabled=\{!vscodeAvailable\}/);
	assert.match(source, />\s*Editor\s*<\/Button>/);
});
