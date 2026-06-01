import { readFileSync } from "node:fs";
import test from "node:test";
import assert from "node:assert/strict";

const source = readFileSync(
	new URL("../app/SettingsDialog.svelte", import.meta.url),
	"utf8",
);
const uiStateSource = readFileSync(
	new URL("../../store/ui-state.store.svelte.ts", import.meta.url),
	"utf8",
);

test("settings dialog uses one as the disabled recent-list option", () => {
	assert.match(
		uiStateSource,
		/export const DEFAULT_RECENT_THREADS_VISIBLE_LIMIT = 1;/,
	);
	assert.match(
		uiStateSource,
		/export const RECENT_THREADS_VISIBLE_LIMIT_PRESETS = \[1, 4, 8, 12\] as const;/,
	);
	assert.match(source, /\{limit === 1 \? "Off" : limit\}/);
});

test("settings dialog groups default models by provider with optgroups", () => {
	assert.match(
		source,
		/const modelProviderEntries = \$derived\.by\(\(\) => \{/,
	);
	assert.match(source, /<optgroup label=\{provider\}>/);
	assert.match(
		source,
		/\{#each modelProviderEntries as \[provider, providerModels\] \(provider\)\}/,
	);
});

test("settings dialog preserves the selected default model when dedupe would hide it", () => {
	assert.match(source, /const selectedDefaultModel = \$derived\.by\(\(\) =>/);
	assert.match(
		source,
		/models\.list\.find\(\(model\) => model\.id === preferences\.defaultModel\)/,
	);
	assert.match(
		source,
		/selectedDefaultModel &&[\s\S]*!dedupedModels\.some\(\(model\) => model\.id === selectedDefaultModel\.id\)/,
	);
	assert.match(
		source,
		/grouped\[provider\] = \[selectedDefaultModel, \.\.\.grouped\[provider\]\];/,
	);
});

test("settings dialog only shows updates when the runtime supports them", () => {
	assert.match(
		source,
		/const showUpdateTab = \$derived\(environment\.supportsAppUpdates\);/,
	);
	assert.doesNotMatch(source, /environment\.runtime === "tauri"/);
});

test("settings dialog exposes a toggle for the editor button", () => {
	assert.match(source, /<ItemTitle>Show editor button<\/ItemTitle>/);
	assert.match(source, /id="settings-show-editor-button"/);
	assert.match(source, /checked=\{preferences\.showEditorButton\}/);
	assert.match(
		source,
		/onCheckedChange=\{\(checked\) => \{\s*preferences\.setShowEditorButton\(checked === true\);/m,
	);
});
