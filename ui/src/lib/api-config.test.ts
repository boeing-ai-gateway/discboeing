import { readFileSync } from "node:fs";
import { test } from "node:test";
import assert from "node:assert/strict";

const source = readFileSync("ui/src/lib/api-config.ts", "utf8");

test("api config supports a Vite API root override", () => {
	assert.match(source, /VITE_DISCOBOT_API_ROOT/);
	assert.match(source, /new URL\(viteApiRoot, window\.location\.origin\)/);
});
