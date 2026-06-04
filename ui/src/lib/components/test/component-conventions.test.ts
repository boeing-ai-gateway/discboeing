/**
 * Enforces component folder conventions for the Svelte UI.
 *
 * Rules:
 *  - ui/**       Pure primitives. No global context imports ever.
 *  - ai/**       Self-contained compound components. No global context imports ever.
 *  - app/parts/  Pure sub-components of app/ context consumers. No global context imports.
 *
 * "Global context" means the three app-wide contexts:
 *   $lib/context/app-context.svelte
 *   $lib/context/session-context.svelte
 *   $lib/context/thread-context.svelte
 *
 * app/ root components (excluding parts/) are intentionally context consumers and are not checked.
 */

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { glob } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const COMPONENTS_ROOT = path.resolve(import.meta.dirname, "..");

// Matches unknown import from the three global context modules
const GLOBAL_CONTEXT_IMPORT =
	/from\s+["'][$]lib\/context\/(app|session|thread)-context(?:\.svelte)?["']/;
const OLD_CONTEXT_ENTRY_POINT =
	/\b(?:useAppContext|useSessionContext|useThreadContext|setAppContext|setSessionContext|setThreadContext|getAppContextIfPresent|getSessionContextIfPresent|getThreadContextIfPresent)\s*\(/;
const LEGACY_SESSION_DOMAIN_CALL =
	/\b(?:session\.(?:files|hooks|services|commands)\.|session\.submit\s*\(|session\.ensureThread\s*\(|thread\.(?:submit|cancel|refresh|connect|dispose)\s*\()/;
const LEGACY_COMMAND_CREDENTIAL_DIALOG_USAGE =
	/\b(?:getAgentCommandCredentialDialog|SessionCommandCredentialDialogState)\b/;

async function findSvelteFiles(dir: string): Promise<string[]> {
	const files: string[] = [];
	for await (const file of glob("**/*.svelte", { cwd: dir })) {
		files.push(path.join(dir, file));
	}
	return files.sort();
}

function importsGlobalContext(filePath: string): boolean {
	const content = readFileSync(filePath, "utf-8");
	return GLOBAL_CONTEXT_IMPORT.test(content);
}

function usesOldContextEntryPoint(filePath: string): boolean {
	const content = readFileSync(filePath, "utf-8");
	return (
		GLOBAL_CONTEXT_IMPORT.test(content) || OLD_CONTEXT_ENTRY_POINT.test(content)
	);
}

function callsLegacySessionDomain(filePath: string): boolean {
	return LEGACY_SESSION_DOMAIN_CALL.test(readFileSync(filePath, "utf-8"));
}

function usesLegacyCommandCredentialDialog(filePath: string): boolean {
	return LEGACY_COMMAND_CREDENTIAL_DIALOG_USAGE.test(
		readFileSync(filePath, "utf-8"),
	);
}

function relPath(filePath: string): string {
	return path.relative(COMPONENTS_ROOT, filePath);
}

async function assertNoGlobalContext(
	dir: string,
	label: string,
): Promise<void> {
	const files = await findSvelteFiles(dir);
	assert.ok(files.length > 0, `expected to find .svelte files in ${label}`);

	const violations = files.filter(importsGlobalContext).map(relPath);

	assert.deepEqual(
		violations,
		[],
		`${label} components must not import global context (app/session/thread).\nViolations:\n  ${violations.join("\n  ")}`,
	);
}

test("ui/ components are pure — no global context imports", async () => {
	await assertNoGlobalContext(path.join(COMPONENTS_ROOT, "ui"), "ui/");
});

test("ai/ components are self-contained — no global context imports", async () => {
	await assertNoGlobalContext(path.join(COMPONENTS_ROOT, "ai"), "ai/");
});

test("app/parts/ components are pure — no global context imports", async () => {
	await assertNoGlobalContext(
		path.join(COMPONENTS_ROOT, "app/parts"),
		"app/parts/",
	);
});

test("app/ root components do not call old context entry points", async () => {
	const files: string[] = [];
	for await (const file of glob("*.svelte", {
		cwd: path.join(COMPONENTS_ROOT, "app"),
	})) {
		files.push(path.join(COMPONENTS_ROOT, "app", file));
	}

	const violations = files.filter(usesOldContextEntryPoint).map(relPath);
	assert.deepEqual(
		violations,
		[],
		`app/ root components must not call old context entry points.\nViolations:\n  ${violations.join("\n  ")}`,
	);
});

test("app/ root components call root commands instead of legacy session domains", async () => {
	const files: string[] = [];
	for await (const file of glob("*.svelte", {
		cwd: path.join(COMPONENTS_ROOT, "app"),
	})) {
		files.push(path.join(COMPONENTS_ROOT, "app", file));
	}

	const violations = files.filter(callsLegacySessionDomain).map(relPath);
	assert.deepEqual(
		violations,
		[],
		`app/ root components must call root commands instead of session/thread domain methods.\nViolations:\n  ${violations.join("\n  ")}`,
	);
});

test("app components consume command credential dialog through root view and commands", async () => {
	const files: string[] = [];
	for await (const file of glob("app/**/*.svelte", { cwd: COMPONENTS_ROOT })) {
		files.push(path.join(COMPONENTS_ROOT, file));
	}

	const violations = files
		.filter(usesLegacyCommandCredentialDialog)
		.map(relPath);
	assert.deepEqual(
		violations,
		[],
		`app components must not use legacy command credential dialog adapters/types.\nViolations:\n  ${violations.join("\n  ")}`,
	);
});
