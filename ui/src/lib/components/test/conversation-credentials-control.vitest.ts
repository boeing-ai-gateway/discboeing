import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import assert from "node:assert/strict";
import { test } from "vitest";

const COMPONENT = resolve(
	import.meta.dirname,
	"../app/ConversationCredentialsControl.svelte",
);

function readSource() {
	return readFileSync(COMPONENT, "utf-8");
}

test("conversation credentials control uses context and commands", () => {
	const source = readSource();

	assert.match(source, /import \{ useContext \} from "\$lib\/context";/);
	assert.match(source, /const context = useContext\(\);/);
	assert.match(
		source,
		/context\.commands\.sessionCredentials\.refreshSessionCredentials\(\s*targetSessionId,\s*\)/,
	);
	assert.match(
		source,
		/context\.commands\.sessionCredentials\.setSessionCredentialAssignments\(/,
	);
	assert.match(source, /context\.commands\.dialogs\.openCredentialsDialog/);
	assert.doesNotMatch(source, /\$lib\/context\/runtime/);
	assert.doesNotMatch(source, /SessionRuntimeState/);
	assert.doesNotMatch(source, /\$lib\/ng/);
	assert.doesNotMatch(source, /api\.getSessionCredentials/);
	assert.doesNotMatch(source, /api\.setSessionCredentials/);
});
