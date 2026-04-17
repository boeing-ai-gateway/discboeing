import { readFileSync } from "node:fs";
import test from "node:test";
import assert from "node:assert/strict";

const source = readFileSync(
	new URL(
		"../app/parts/ConversationComposerModelControl.svelte",
		import.meta.url,
	),
	"utf8",
);

test("conversation composer model control dedupes models by provider and cleaned name", () => {
	assert.match(
		source,
		/const dedupeKey = `\$\{model\.provider \|\| "Other"\}::\$\{cleanName\}`;/,
	);
	assert.doesNotMatch(
		source,
		/const modelByName: Record<string, ModelInfo> = \{\}/,
	);
});
