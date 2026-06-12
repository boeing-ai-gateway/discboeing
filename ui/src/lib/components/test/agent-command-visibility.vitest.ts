import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "vitest";

test("session toolbar filters visible agent commands", () => {
	const source = readFileSync(
		"src/lib/components/app/SessionToolbar.svelte",
		"utf8",
	);

	assert.match(
		source,
		/from\s+["']\$lib\/agent-command-helpers["']/,
		"SessionToolbar.svelte must import the shared agent command visibility helper",
	);
	assert.match(
		source,
		/\.filter\(isUiAgentCommand\)/,
		"SessionToolbar.svelte must filter commands with isUiAgentCommand",
	);
});

test("composer leaves slash commands unfiltered", () => {
	const source = readFileSync(
		"src/lib/components/app/ConversationComposer.svelte",
		"utf8",
	);

	assert.doesNotMatch(source, /\$lib\/agent-command-helpers/);
	assert.doesNotMatch(source, /\.filter\(isUiAgentCommand\)/);
});
