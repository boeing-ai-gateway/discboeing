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

const REQUEST_USER_CREDENTIAL_RENDERER_FILE = path.resolve(
	import.meta.dirname,
	"../ai/tool-renderers/RequestUserCredentialToolRenderer.svelte",
);

function readRegistrySource() {
	return readFileSync(REGISTRY_FILE, "utf-8");
}

function readOptimizedRendererSource() {
	return readFileSync(OPTIMIZED_RENDERER_FILE, "utf-8");
}

function readRequestUserCredentialRendererSource() {
	return readFileSync(REQUEST_USER_CREDENTIAL_RENDERER_FILE, "utf-8");
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

test("powershell uses the optimized bash renderer", () => {
	const source = readRegistrySource();

	assert.match(source, /PowerShell: BashToolRenderer,/);
	assert.match(source, /case "PowerShell": \{/);
});

test("optimized tool renderer auto-expands pending credential requests", () => {
	const source = readOptimizedRendererSource();

	assert.match(source, /toolPart\.toolName === "RequestUserCredential"/);
	assert.match(source, /toolPart\.state === "approval-requested"/);
});

test("sudo credential requests use approval-only UI", () => {
	const source = readRequestUserCredentialRendererSource();

	assert.match(source, /const SUDO_TOKEN_ENV_VAR = "DISCOBOT_SUDO_TOKEN";/);
	assert.match(source, /Approve sudo access/);
	assert.match(source, /No credential value is needed/);
	assert.match(source, /Internal sudo approval token/);
	assert.match(
		source,
		/envVars: \[[\s\S]*key: SUDO_TOKEN_ENV_VAR, value: generateSudoApprovalToken\(\)/,
	);
	assert.match(source, /\{#if !isSudoCredentialRequest\(request\)\}/);
});
