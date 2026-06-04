import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const CONVERSATION_COMPOSER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ConversationComposer.svelte",
);

function readConversationComposerSource() {
	return readFileSync(CONVERSATION_COMPOSER_COMPONENT, "utf-8");
}

test("pending composer submit opens the created thread", () => {
	const source = readConversationComposerSource();

	assert.match(source, /const wasPending = session\.isPending;/);
	assert.match(
		source,
		/const result = await submitThread\(session\.sessionId, thread\.threadId, \{/,
	);
	assert.match(source, /if \(wasPending && result\) \{/);
	assert.match(source, /openThread\(result\.sessionId, result\.threadId\);/);
	assert.doesNotMatch(source, /app\.sessions\.openThread/);
});
