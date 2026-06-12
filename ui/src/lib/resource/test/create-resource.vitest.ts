import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const RESOURCE_FILE = path.resolve(
	path.dirname(import.meta.filename),
	"../create-resource.svelte.ts",
);

function readResourceSource() {
	return readFileSync(RESOURCE_FILE, "utf-8");
}

test("createResource avoids component-scoped effects during module/store initialization", () => {
	const source = readResourceSource();

	assert.doesNotMatch(source, /\$effect\s*\(/);
	assert.match(source, /let wasEnabled = args\.enabled\(\);/);
	assert.match(
		source,
		/function isEnabled\(\) \{[\s\S]*if \(!enabled && wasEnabled\) \{[\s\S]*reset\(\);/,
	);
});
