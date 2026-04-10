import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const REGISTRY_FILE = path.resolve(
	import.meta.dirname,
	"../ai/tool-renderers/registry.ts",
);

const OPTIMIZED_RENDERER_FILE = path.resolve(
	import.meta.dirname,
	"../ai/tool-renderers/OptimizedToolRenderer.svelte",
);

function readRegistrySource() {
	return readFileSync(REGISTRY_FILE, "utf-8");
}

function readOptimizedRendererSource() {
	return readFileSync(OPTIMIZED_RENDERER_FILE, "utf-8");
}

test("request user credential uses the optimized tool renderer", () => {
	const source = readRegistrySource();

	assert.match(
		source,
		/import RequestUserCredentialToolRenderer from "\.\/RequestUserCredentialToolRenderer\.svelte";/,
	);
	assert.match(
		source,
		/RequestUserCredential: RequestUserCredentialToolRenderer,/,
	);
});

test("optimized tool renderer auto-expands pending credential requests", () => {
	const source = readOptimizedRendererSource();

	assert.match(source, /toolPart\.toolName === "RequestUserCredential"/);
	assert.match(source, /toolPart\.state === "approval-requested"/);
});
