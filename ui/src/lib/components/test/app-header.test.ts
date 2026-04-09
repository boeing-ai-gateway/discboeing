import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const APP_HEADER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/AppHeader.svelte",
);

function readAppHeaderSource() {
	return readFileSync(APP_HEADER_COMPONENT, "utf-8");
}

test("app header preserves the toolbar grid slot even when the session toolbar is hidden", () => {
	const source = readAppHeaderSource();

	assert.match(
		source,
		/<div class="relative z-20 min-w-0 px-2">\s*\{#if showSessionToolbar\}[\s\S]*<SessionToolbarStack \/>[\s\S]*\{\/if\}\s*<\/div>/,
	);
});

test("app header keeps window controls in a dedicated rightmost grid column", () => {
	const source = readAppHeaderSource();

	assert.match(source, /grid-cols-\[auto_minmax\(0,1fr\)_auto_auto\]/);
	assert.match(source, /class="tauri-drag-region relative z-\[60\] grid h-10/);
	assert.match(
		source,
		/class="relative z-20 flex h-full min-w-0 items-stretch justify-self-end pr-0"[\s\S]*<RightWindowControls \/>/,
	);
	assert.ok(source.includes("<SessionToolbarStack />"));
	assert.ok(source.includes("<span>New Session</span>"));
});
