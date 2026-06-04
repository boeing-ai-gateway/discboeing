import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const source = readFileSync("ui/src/lib/app/app-runtime.svelte.ts", "utf8");
const commandSource = readFileSync(
	"ui/src/lib/context/commands/app-view.ts",
	"utf8",
);
const conversationComposerSource = readFileSync(
	"ui/src/lib/components/app/ConversationComposer.svelte",
	"utf8",
);
const conversationPaneSource = readFileSync(
	"ui/src/lib/components/app/ConversationPane.svelte",
	"utf8",
);
const dockPanelSource = readFileSync(
	"ui/src/lib/components/app/DockPanel.svelte",
	"utf8",
);

test("runtime projections mount the selected session before existing contexts", () => {
	assert.match(
		source,
		/ctx\.view\.app\.navigation\.mountedSessionIds = \[\s*ctx\.view\.app\.selection\.sessionId,\s*\.\.\.sessionContexts\.keys\(\),\s*ctx\.view\.app\.selection\.pendingSessionId,\s*\]/,
	);
});

test("runtime projects pending CQRS domains into root context", () => {
	assert.match(source, /function syncSessionViewProjection\(/);
	assert.match(source, /function syncSessionDomainDataProjection\(/);
	assert.match(
		source,
		/context\(\)\.data\.conversations\.byThreadId\[threadId\]/,
	);
	assert.match(source, /ctx\.data\.files\.bySessionId\[sessionId\]/);
	assert.match(source, /ctx\.data\.hooks\.bySessionId\[sessionId\]/);
	assert.match(source, /ctx\.data\.services\.bySessionId\[sessionId\]/);
	assert.match(source, /ctx\.data\.commands\.bySessionId\[sessionId\]/);
	assert.match(source, /ctx\.view\.sessions\[sessionId\]/);
});

test("app commands expose root entry points for pending domains", () => {
	for (const commandName of [
		"submitThread",
		"cancelThread",
		"setComposerDraft",
		"clearComposerDraft",
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
		"closeFile",
		"refreshFiles",
		"toggleFilesChangedOnly",
		"toggleFileDirectory",
		"expandFileTree",
		"collapseFileTree",
		"updateFileBuffer",
		"discardFileBuffer",
		"acceptFileConflict",
		"forceSaveFile",
		"getFileEditorModel",
		"setFileEditorModel",
		"getFileEditorViewState",
		"setFileEditorViewState",
		"saveFile",
		"renameFile",
		"removeFile",
		"refreshHooks",
		"rerunHook",
		"refreshServices",
		"startService",
		"stopService",
		"refreshAgentCommands",
		"runAgentCommand",
		"closeAgentCommandCredentialDialog",
		"confirmAgentCommandCredentialDialog",
		"selectAgentCommandCredentialOption",
		"setAgentCommandCredentialCreateName",
		"setAgentCommandCredentialCreateSecret",
		"setAgentCommandCredentialValidityPreset",
		"setAgentCommandCredentialValidityValue",
		"setAgentCommandCredentialValidityUnit",
		"launchAgentCommandCredentialOAuthWizard",
		"refreshAgentCommandCredentialDialogCredentials",
	]) {
		assert.match(
			commandSource,
			new RegExp(`export (?:async )?function ${commandName}\\b`),
		);
	}
});

test("app commands do not expose domain compatibility adapters", () => {
	assert.doesNotMatch(commandSource, /getSessionFilesDomain/);
	assert.doesNotMatch(commandSource, /SessionFilesDomain/);
	assert.doesNotMatch(commandSource, /getAgentCommandCredentialDialog/);
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
