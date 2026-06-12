import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

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

const REQUEST_COMMIT_PULL_RENDERER_FILE = path.resolve(
	import.meta.dirname,
	"../ai/tool-renderers/RequestCommitPullToolRenderer.svelte",
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

function readRequestCommitPullRendererSource() {
	return readFileSync(REQUEST_COMMIT_PULL_RENDERER_FILE, "utf-8");
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
	assert.match(
		source,
		/toolPart\.state === "approval-requested" && !isApprovalAnswered/,
	);
	assert.match(
		source,
		/toolPart\.toolName === "RequestUserCredential" && isPendingApproval/,
	);
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

test("credential denial form is not reset for the same pending request", () => {
	const source = readRequestUserCredentialRendererSource();

	assert.match(
		source,
		/let activeCredentialRequestKey = \$state<string \| null>\(null\);/,
	);
	assert.match(source, /function preparePendingCredentialRequest/);
	assert.match(source, /if \(nextKey !== activeCredentialRequestKey\) \{/);
	assert.doesNotMatch(
		source,
		/approvalError = null;\s*showRejectionForm = false;\s*isSubmittingApproval = false;\s*isSubmittingRejection = false;/,
	);
});

test("credential renderer uses answered approval metadata", () => {
	const source = readRequestUserCredentialRendererSource();

	assert.match(source, /approvalResponse,/);
	assert.match(source, /approvalResponse\?\.approved === false/);
	assert.match(source, /approvalResponse\.reason \?\? ""/);
	assert.match(source, /isPendingApproval \|\| isApprovalAnswered/);
	assert.match(
		source,
		/approvalStatus === "answered"[\s\S]*approvalResponse\?\.approved === false[\s\S]*Credential request rejected/,
	);
});

test("commit pull renderer uses answered approval metadata", () => {
	const source = readRequestCommitPullRendererSource();

	assert.match(source, /approvalResponse,/);
	assert.match(source, /approvalResponse\?\.approved !== false/);
	assert.match(source, /approvalResponse\?\.approved === true/);
	assert.match(source, /approvalResponse\?\.approved === false/);
	assert.match(source, /approvalResponse\.reason \?\? ""/);
	assert.match(source, /disabled=\{isPendingApproval\}/);
});
