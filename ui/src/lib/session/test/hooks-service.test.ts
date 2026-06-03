import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

import type { HooksStatusResponse } from "../../api-types";
import {
	getHookDisplayState,
	mergeHookOutput,
	toHooksStatus,
} from "../domains/session-domain.helpers";

const CONVERSATION_HOOKS_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../../components/app/ConversationHooksPanel.svelte",
);

const COMPOSER_HOOKS_CONTROL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../../components/app/parts/ConversationComposerHooksControl.svelte",
);

const CONVERSATION_COMPOSER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../../components/app/ConversationComposer.svelte",
);

const PROMPT_QUEUE_PANEL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../../components/app/parts/ConversationPromptQueuePanel.svelte",
);

const SESSION_HOOKS_DOMAIN = path.resolve(
	import.meta.dirname,
	"../domains/session-hooks.svelte.ts",
);

function readConversationHooksPanelSource() {
	return readFileSync(CONVERSATION_HOOKS_PANEL_COMPONENT, "utf-8");
}

function readComposerHooksControlSource() {
	return readFileSync(COMPOSER_HOOKS_CONTROL_COMPONENT, "utf-8");
}

function readConversationComposerSource() {
	return readFileSync(CONVERSATION_COMPOSER_COMPONENT, "utf-8");
}

function readPromptQueuePanelSource() {
	return readFileSync(PROMPT_QUEUE_PANEL_COMPONENT, "utf-8");
}

function readSessionHooksDomainSource() {
	return readFileSync(SESSION_HOOKS_DOMAIN, "utf-8");
}

const hooksStatusResponse: HooksStatusResponse = {
	hooks: {
		"hook-1": {
			hookId: "hook-1",
			hookName: "Prompt",
			type: "session",
			lastRunAt: "2026-03-11T00:00:00.000Z",
			lastResult: "success",
			lastExitCode: 0,
			outputPath: "/tmp/hook-1.log",
			runCount: 2,
			failCount: 0,
			consecutiveFailures: 0,
			executionPaused: false,
		},
		"hook-2": {
			hookId: "hook-2",
			hookName: "Formatter",
			type: "file",
			lastRunAt: "2026-03-11T00:00:01.000Z",
			lastResult: "failure",
			lastExitCode: 1,
			outputPath: "/tmp/hook-2.log",
			runCount: 3,
			failCount: 2,
			consecutiveFailures: 1,
			executionPaused: true,
		},
	},
	pendingHooks: ["hook-2"],
	lastEvaluatedAt: "2026-03-11T00:00:01.000Z",
	executionPaused: false,
};

test("toHooksStatus maps API hook response fields into session hook state", () => {
	const hooksStatus = toHooksStatus(hooksStatusResponse);

	assert.deepEqual(hooksStatus.pendingHookIds, ["hook-2"]);
	assert.deepEqual(
		hooksStatus.hooks.map((hook) => ({
			id: hook.hookId,
			type: hook.type,
			result: hook.lastResult,
			executionPaused: hook.executionPaused,
		})),
		[
			{
				id: "hook-1",
				type: "session",
				result: "success",
				executionPaused: false,
			},
			{ id: "hook-2", type: "file", result: "failure", executionPaused: true },
		],
	);
});

test("getHookDisplayState keeps failures visible when the hook is still pending", () => {
	const hooksStatus = toHooksStatus(hooksStatusResponse);
	const pendingHookIds = new Set(hooksStatus.pendingHookIds);

	assert.equal(
		getHookDisplayState(hooksStatus.hooks[1], pendingHookIds),
		"failure",
	);
	assert.equal(
		getHookDisplayState(hooksStatus.hooks[0], pendingHookIds),
		"success",
	);
});

test("conversation hooks panel uses the resolved display state for failure rendering", () => {
	const source = readConversationHooksPanelSource();

	assert.match(source, /\{@const displayState = hookDisplayState\(hook\)\}/);
	assert.match(source, /\{:else if displayState === "failure"\}/);
	assert.doesNotMatch(source, /\{:else if isHookPending\(hook\.hookId\)\}/);
});

test("conversation hook details renders idle as neutral instead of success", () => {
	const source = readConversationHooksPanelSource();

	assert.match(
		source,
		/<Dialog\.Title[\s\S]*\{:else if displayState === "success"\}[\s\S]*<CheckCircleIcon class="size-4 text-green-500" \/>[\s\S]*\{:else\}[\s\S]*<ClockIcon class="size-4 text-muted-foreground" \/>[\s\S]*<\/Dialog\.Title>/,
	);
});

test("conversation hooks panel lists review hooks first with the phase toggle", () => {
	const source = readConversationHooksPanelSource();

	assert.match(
		source,
		/const reviewHooks = \$derived\.by\(\(\) =>[\s\S]*hook\.phase === "review"/,
	);
	assert.match(
		source,
		/const draftHooks = \$derived\.by\(\(\) =>[\s\S]*hook\.phase !== "review"/,
	);
	assert.match(
		source,
		/Review hooks[\s\S]*variant=\{selectedThreadPhase === "review" \? "outline" : "default"\}[\s\S]*"Set to Draft"[\s\S]*"Ready for Review"[\s\S]*\{#each reviewHooks as hook[\s\S]*\{#if draftHooks\.length > 0\}[\s\S]*Change hooks[\s\S]*\{#each draftHooks as hook/,
	);
	assert.doesNotMatch(source, /border-blue-600|bg-blue-600|text-white/);
});

test("composer panels attach to the composer block below them", () => {
	const hooksPanelSource = readConversationHooksPanelSource();
	const promptQueueSource = readPromptQueuePanelSource();
	const composerSource = readConversationComposerSource();

	for (const source of [hooksPanelSource, promptQueueSource]) {
		assert.match(source, /-mb-px/);
		assert.match(source, /rounded-t-md rounded-b-none/);
		assert.match(source, /border border-b-0 border-border/);
		assert.match(source, /group-hover:bg-muted\/50/);
		assert.doesNotMatch(source, /mb-2 rounded-lg border border-border/);
	}
	assert.match(
		hooksPanelSource,
		/max-h-96 overflow-x-hidden overflow-y-auto px-1 pt-1 pb-3/,
	);
	assert.match(composerSource, /const hasAttachedComposerPanel = \$derived/);
	assert.match(composerSource, /hasAttachedComposerPanel[\s\S]*rounded-t-none/);
});

test("queued prompts can be reordered by dragging", () => {
	const source = readPromptQueuePanelSource();

	assert.match(source, /import Sortable from "sortablejs"/);
	assert.match(source, /bind:this=\{promptListElement\}/);
	assert.match(source, /\$effect\(\(\) => \{[\s\S]*Sortable\.create\(element/);
	assert.equal(
		source.indexOf("$effect(() => {"),
		source.lastIndexOf("$effect(() => {"),
	);
	assert.ok(
		source.indexOf("$effect(() => {") <
			source.indexOf("Sortable.create(element"),
	);
	assert.match(source, /draggable: "\.queued-prompt-row"/);
	assert.match(source, /handle: "\.queued-prompt-drag-handle"/);
	assert.match(source, /void movePrompt\(movedEntry, event\.newIndex\)/);
	assert.match(source, /<span[\s\S]*queued-prompt-drag-handle/);
	assert.match(source, /aria-hidden="true"/);
	assert.doesNotMatch(source, /aria-label="Drag queued prompt to reorder"/);
});

test("composer hooks control resolves failures through shared hook display state", () => {
	const source = readComposerHooksControlSource();

	assert.match(source, /import \{ getHookDisplayState \} from/);
	assert.match(
		source,
		/hooks\.map\(\(hook\) => getHookDisplayState\(hook, pendingHookSet\)\)/,
	);
	assert.match(
		source,
		/hookDisplayStates\.some\(\(state\) => state === "failure"\)/,
	);
});

test("session hooks domain polls while hooks are pending or running", () => {
	const source = readSessionHooksDomainSource();

	assert.match(source, /HOOK_STATUS_POLL_MS/);
	assert.match(source, /status\.pendingHookIds\.length === 0/);
	assert.match(source, /hook\.lastResult === "running"/);
	assert.match(source, /void refresh\(\)/);
});

test("mergeHookOutput replaces the latest output for the given hook", () => {
	assert.deepEqual(
		mergeHookOutput(
			{
				"hook-1": {
					output: "previous output",
					sizeBytes: 15,
					displayedBytes: 15,
					tooLarge: false,
				},
			},
			"hook-1",
			{
				output: "latest output",
				sizeBytes: 13,
				displayedBytes: 13,
				tooLarge: false,
			},
		),
		{
			"hook-1": {
				output: "latest output",
				sizeBytes: 13,
				displayedBytes: 13,
				tooLarge: false,
			},
		},
	);
});

test("mergeHookOutput preserves large-output metadata when inline output is suppressed", () => {
	assert.deepEqual(
		mergeHookOutput({}, "hook-2", {
			output: "tail output",
			sizeBytes: 250000,
			displayedBytes: 200 * 1024,
			tooLarge: true,
		}),
		{
			"hook-2": {
				output: "tail output",
				sizeBytes: 250000,
				displayedBytes: 200 * 1024,
				tooLarge: true,
			},
		},
	);
});
