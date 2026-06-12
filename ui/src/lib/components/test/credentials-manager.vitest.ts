import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { test } from "vitest";

const COMPONENT = resolve(
	import.meta.dirname,
	"../app/CredentialsManager.svelte",
);

function readSource() {
	return readFileSync(COMPONENT, "utf-8");
}

test("credentials manager guards dialog effects without tracking handled state", () => {
	const source = readSource();

	assert.match(source, /import \{ onMount, untrack \} from "svelte";/);
	assert.match(
		source,
		/untrack\(\(\) => handledCredentialFlowIntent\) === flowIntent/,
	);
	assert.match(
		source,
		/untrack\(\(\) => handledCredentialsDialogTargetId\) === targetId/,
	);
	assert.doesNotMatch(source, /handledCredentialFlowIntent === flowIntent/);
	assert.doesNotMatch(source, /handledCredentialsDialogTargetId === targetId/);
});
