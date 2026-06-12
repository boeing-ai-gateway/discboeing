import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { test } from "vitest";

const commandSource = readFileSync("src/lib/context/commands.ts", "utf8");
const conversationComposerSource = readFileSync(
	"src/lib/components/app/ConversationComposer.svelte",
	"utf8",
);
const conversationPaneSource = readFileSync(
	"src/lib/components/app/ConversationPane.svelte",
	"utf8",
);
const dockPanelSource = readFileSync(
	"src/lib/components/app/DockPanel.svelte",
	"utf8",
);
const layoutSource = readFileSync("src/routes/+layout.svelte", "utf8");
const projectsDomainSource = readFileSync(
	"src/lib/context/domains/projects.ts",
	"utf8",
);
const sessionsDomainSource = readFileSync(
	"src/lib/context/domains/sessions.ts",
	"utf8",
);

function getSvelteFiles(path: string): string[] {
	return readdirSync(path)
		.flatMap((entry) => {
			const filePath = `${path}/${entry}`;
			if (statSync(filePath).isDirectory()) {
				return getSvelteFiles(filePath);
			}
			return filePath.endsWith(".svelte") ? [filePath] : [];
		})
		.sort();
}

function getSourceFiles(path: string): string[] {
	return readdirSync(path)
		.flatMap((entry) => {
			const filePath = `${path}/${entry}`;
			if (statSync(filePath).isDirectory()) {
				return getSourceFiles(filePath);
			}
			return filePath.endsWith(".ts") || filePath.endsWith(".svelte")
				? [filePath]
				: [];
		})
		.sort();
}

function getSvelteEffectBlocks(source: string): string[] {
	const blocks: string[] = [];
	let offset = 0;
	while (offset < source.length) {
		const effectStart = nextEffectStart(source, offset);
		if (!effectStart) {
			break;
		}
		const bodyStart = source.indexOf("{", effectStart.index);
		if (bodyStart === -1) {
			break;
		}
		const bodyEnd = findMatchingBrace(source, bodyStart);
		if (bodyEnd === -1) {
			break;
		}
		blocks.push(source.slice(effectStart.index, bodyEnd + 1));
		offset = bodyEnd + 1;
	}
	return blocks;
}

function nextEffectStart(
	source: string,
	offset: number,
): { index: number } | null {
	const candidates = ["$effect(() => {", "$effect.pre(() => {"]
		.map((pattern) => source.indexOf(pattern, offset))
		.filter((index) => index !== -1);
	if (candidates.length === 0) {
		return null;
	}
	return { index: Math.min(...candidates) };
}

function findMatchingBrace(source: string, start: number): number {
	let depth = 0;
	for (let index = start; index < source.length; index += 1) {
		if (source[index] === "{") {
			depth += 1;
		} else if (source[index] === "}") {
			depth -= 1;
			if (depth === 0) {
				return index;
			}
		}
	}
	return -1;
}

test("legacy ng runtime directory has been removed", () => {
	assert.equal(existsSync("src/lib/context/runtime"), false);
});

test("ng commands expose root entry points for pending domains", () => {
	for (const commandName of [
		"submitThread",
		"cancelThread",
		"setComposerDraft",
		"clearComposerDraft",
		"movePendingComposerDraftToThread",
		"setThreadNextModelId",
		"setThreadNextReasoning",
		"setThreadNextServiceTier",
		"clearThreadNextComposerValues",
		"addThreadPendingComment",
		"removeThreadPendingComment",
		"clearThreadPendingComments",
		"deleteQueuedPrompt",
		"updateQueuedPrompt",
		"setConversationScrollTop",
		"openFile",
		"saveFile",
		"renameFile",
		"deleteFile",
		"rerunHook",
		"startService",
		"stopService",
		"bindServiceLocalhost",
		"unbindServiceLocalhost",
		"runAgentCommand",
		"closeCommandCredentialDialog",
		"confirmCommandCredentialDialog",
		"selectCommandCredentialOption",
		"setCommandCredentialCreateName",
		"setCommandCredentialCreateSecret",
		"setCommandCredentialValidityPreset",
		"setCommandCredentialValidityValue",
		"setCommandCredentialValidityUnit",
		"launchCommandCredentialOAuthWizard",
		"refreshCommandCredentialDialogCredentials",
	]) {
		assert.match(commandSource, new RegExp(`\\b${commandName}\\b`));
	}
});

test("ng commands do not expose legacy domain compatibility adapters", () => {
	assert.doesNotMatch(commandSource, /getSessionFilesDomain/);
	assert.doesNotMatch(commandSource, /SessionFilesDomain/);
	assert.doesNotMatch(commandSource, /getAgentCommandCredentialDialog/);
});

test("deleteSession waits for server status events instead of removing cache", () => {
	assert.match(sessionsDomainSource, /api\.deleteSession\(sessionId\)/);
	assert.doesNotMatch(
		sessionsDomainSource,
		/deleteSession[\s\S]*removeById\(context\.data\.sessions, sessionId\)/,
	);
});

test("removed session updates delete the cached session", () => {
	assert.match(
		projectsDomainSource,
		/session\.sandboxStatus === "removed"[\s\S]*removeById\(target\.sessions, session\.id\)/,
	);
});

test("route layout does not import ng runtime modules", () => {
	assert.doesNotMatch(
		layoutSource,
		/from\s+["']\$lib\/context\/runtime(?:\/|["'])/,
	);
	assert.doesNotMatch(
		layoutSource,
		/thread-selection|localStorage|readStorage/,
	);
	assert.match(layoutSource, /initializeApp/);
});

test("ui source does not import ng runtime modules", () => {
	for (const sourceFile of getSourceFiles("src/lib")) {
		assert.doesNotMatch(
			readFileSync(sourceFile, "utf8"),
			/from\s+["']\$lib\/context\/runtime(?:\/|["'])|import\s*\(["']\$lib\/context\/runtime(?:\/|["'])/,
			sourceFile,
		);
	}
});

test("svelte components do not access local storage directly", () => {
	for (const componentFile of getSvelteFiles("src/lib/components")) {
		const componentSource = readFileSync(componentFile, "utf8");
		assert.doesNotMatch(
			componentSource,
			/localStorage|readStorage|writeStorage|\$lib\/context\/stores/,
			componentFile,
		);
	}
});

test("svelte effects do not call ng commands", () => {
	for (const componentFile of getSvelteFiles("src/lib/components")) {
		for (const effectBlock of getSvelteEffectBlocks(
			readFileSync(componentFile, "utf8"),
		)) {
			assert.doesNotMatch(effectBlock, /(?:^|\W)commands\./, componentFile);
		}
	}
});

test("app components route composer and conversation writes through commands", () => {
	for (const componentSource of [
		conversationComposerSource,
		conversationPaneSource,
		dockPanelSource,
	]) {
		for (const forbiddenPattern of [
			/session\.ui\.setComposerDraft\(/,
			/sessionView\.setComposerDraft\(/,
			/thread\.clearComposerDraft\(/,
			/thread\.setNextModelId\(/,
			/thread\.setNextReasoning\(/,
			/thread\.setNextServiceTier\(/,
			/thread\.clearNextComposerValues\(/,
			/thread\.addPendingComment\(/,
			/thread\.removePendingComment\(/,
			/thread\.clearPendingComments\(/,
			/thread\.deleteQueuedPrompt\(/,
			/thread\.updateQueuedPrompt\(/,
			/session\.conversationScrollTopByThreadId/,
		]) {
			assert.doesNotMatch(componentSource, forbiddenPattern);
		}
	}
});
