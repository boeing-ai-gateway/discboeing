/**
 * Enforces component folder conventions for the Svelte UI.
 *
 * Rules:
 *  - ui/**       Pure primitives. No global context imports ever.
 *  - ai/**       Self-contained compound components. No global context imports ever.
 *  - app/parts/  Pure sub-components of app/ context consumers. No global context imports.
 *
 * "Legacy global context" means the removed app/session/thread context entry
 * points that app components migrated away from.
 *
 * app/ root components (excluding parts/) are intentionally context consumers and are not checked.
 */

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { glob } from "node:fs/promises";
import path from "node:path";
import { test } from "vitest";

const COMPONENTS_ROOT = path.resolve(import.meta.dirname, "..");
const LIB_ROOT = path.resolve(COMPONENTS_ROOT, "..");
const CONTEXT_DOMAINS_ROOT = path.join(LIB_ROOT, "context/domains");

// Matches imports from the removed global context modules.
const LEGACY_GLOBAL_CONTEXT_IMPORT =
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
	return LEGACY_GLOBAL_CONTEXT_IMPORT.test(content);
}

function usesOldContextEntryPoint(filePath: string): boolean {
	const content = readFileSync(filePath, "utf-8");
	return (
		LEGACY_GLOBAL_CONTEXT_IMPORT.test(content) ||
		OLD_CONTEXT_ENTRY_POINT.test(content)
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

function usesSvelteReactivity(filePath: string): boolean {
	return /\$state|\$derived|\$effect|from\s+["']svelte(?:\/reactivity)?["']/.test(
		readFileSync(filePath, "utf-8"),
	);
}

function importsContextRuntime(filePath: string): boolean {
	return /from\s+["'][$]lib\/context\/runtime(?:\/|["'])/.test(
		readFileSync(filePath, "utf-8"),
	);
}

function importsContextDomain(filePath: string): boolean {
	return /from\s+["'][$]lib\/context\/domains\//.test(
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

test("context domains are plain TypeScript with no Svelte reactivity", async () => {
	const svelteDomainFiles: string[] = [];
	for await (const file of glob("**/*.svelte.ts", {
		cwd: CONTEXT_DOMAINS_ROOT,
	})) {
		svelteDomainFiles.push(path.join(CONTEXT_DOMAINS_ROOT, file));
	}

	const sourceFiles: string[] = [];
	for await (const file of glob("*.ts", { cwd: CONTEXT_DOMAINS_ROOT })) {
		sourceFiles.push(path.join(CONTEXT_DOMAINS_ROOT, file));
	}

	const reactiveFiles = sourceFiles.filter(usesSvelteReactivity);
	assert.deepEqual(
		[...svelteDomainFiles, ...reactiveFiles].map((filePath) =>
			path.relative(LIB_ROOT, filePath),
		),
		[],
		"context domains must not use .svelte.ts files or Svelte runes/reactivity.",
	);
});

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

test("Svelte components do not import context runtime modules", async () => {
	const files = await findSvelteFiles(COMPONENTS_ROOT);
	const violations = files.filter(importsContextRuntime).map(relPath);
	assert.deepEqual(
		violations,
		[],
		`Svelte components must not import context/runtime modules.\nViolations:\n  ${violations.join("\n  ")}`,
	);
});

test("components do not import context domain modules directly", async () => {
	const files = await findSvelteFiles(COMPONENTS_ROOT);
	const violations = files.filter(importsContextDomain).map(relPath);
	assert.deepEqual(
		violations,
		[],
		`components must not import context/domain modules directly.\nViolations:\n  ${violations.join("\n  ")}`,
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
