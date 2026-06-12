import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";
import assert from "node:assert/strict";
import { filterModelProviderEntries } from "../app/parts/conversation-composer-model-control-search";
import type {
	DisplayModel,
	ModelProviderEntry,
} from "../app/parts/conversation-composer-model-control-search";

const source = readFileSync(
	path.resolve(
		import.meta.dirname,
		"../app/parts/ConversationComposerModelControl.svelte",
	),
	"utf8",
);

function buildModelEntry(overrides: Partial<DisplayModel>): DisplayModel {
	return {
		...overrides,
		id: overrides.id ?? "model-id",
		name: overrides.name ?? "Model",
		selectedIds: overrides.selectedIds ?? [overrides.id ?? "model-id"],
	} as DisplayModel;
}

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

test("conversation composer model control includes search input in dropdown", () => {
	assert.match(source, /type="search"/);
	assert.match(source, /placeholder="Search models"/);
	assert.match(source, /filterModelProviderEntries/);
});

test("conversation composer model control filters models by query and supports empty-provider alias", () => {
	const entries: ModelProviderEntry[] = [
		[
			"Other",
			[
				buildModelEntry({
					id: "openai-gpt",
					name: "GPT",
					provider: "openai",
					description: "fast model",
					selectedIds: ["openai-gpt"],
				}),
			],
		],
		[
			"Google",
			[
				buildModelEntry({
					id: "gemini",
					name: "Gemini",
					provider: "google",
					description: "search model",
					selectedIds: ["gemini"],
				}),
			],
		],
		[
			"Other",
			[
				buildModelEntry({
					id: "local",
					name: "Local model",
					description: "unlabeled provider",
					selectedIds: ["local"],
				}),
			],
		],
	];

	assert.equal(filterModelProviderEntries(entries, "gpt").length, 1);
	assert.equal(
		filterModelProviderEntries(entries, "search")[0]?.[1][0]?.id,
		"gemini",
	);
	assert.equal(
		filterModelProviderEntries(entries, "google")[0]?.[1][0]?.id,
		"gemini",
	);
	assert.equal(
		filterModelProviderEntries(entries, "openai-gpt")[0]?.[1][0]?.id,
		"openai-gpt",
	);
	assert.equal(
		filterModelProviderEntries(entries, "other")[0]?.[1][0]?.id,
		"local",
	);
	assert.equal(filterModelProviderEntries(entries, "nope").length, 0);
	assert.equal(filterModelProviderEntries(entries, "").length, 3);
});

test("conversation composer model control keeps duplicate selected ids visible", () => {
	assert.match(source, /selectedIds: \[model\.id\]/);
	assert.match(source, /mergeSelectedIds\(existing, model\);/);
	assert.match(source, /model\.selectedIds\.includes\(value\)/);
});
