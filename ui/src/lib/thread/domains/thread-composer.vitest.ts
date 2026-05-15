import { expect, test } from "vitest";

import {
	getThreadComposerValues,
	normalizeThreadComposerReasoning,
	resolveThreadComposerSubmitValues,
} from "./thread-composer.svelte";

test("getThreadComposerValues restores thread model and reasoning", () => {
	expect(
		getThreadComposerValues(
			{
				id: "thread-1",
				name: "Main",
				model: "openai/gpt-5",
				reasoning: "high",
				serviceTier: "priority",
			},
			"anthropic/claude-sonnet-4-6",
		),
	).toEqual({
		modelId: "openai/gpt-5",
		reasoning: "high",
		serviceTier: "priority",
	});
});

test("getThreadComposerValues falls back to the default model", () => {
	expect(getThreadComposerValues(null, "anthropic/claude-sonnet-4-6")).toEqual({
		modelId: "anthropic/claude-sonnet-4-6",
		reasoning: undefined,
		serviceTier: undefined,
	});
});

test("normalizeThreadComposerReasoning keeps explicit levels and drops empty values", () => {
	expect(normalizeThreadComposerReasoning("default")).toBe("default");
	expect(normalizeThreadComposerReasoning("medium")).toBe("medium");
	expect(normalizeThreadComposerReasoning("")).toBe(undefined);
	expect(normalizeThreadComposerReasoning(undefined)).toBe(undefined);
});

test("resolveThreadComposerSubmitValues falls back to current values when next values are unset", () => {
	expect(
		resolveThreadComposerSubmitValues({
			modelId: "openai/gpt-5",
			reasoning: "high",
			serviceTier: "priority",
			nextModelId: undefined,
			nextReasoning: undefined,
			nextServiceTier: undefined,
		}),
	).toEqual({
		modelId: "openai/gpt-5",
		reasoning: "high",
		serviceTier: "priority",
	});
});

test("resolveThreadComposerSubmitValues clears reasoning when using the default model", () => {
	expect(
		resolveThreadComposerSubmitValues({
			modelId: "openai/gpt-5",
			reasoning: "high",
			serviceTier: "priority",
			nextModelId: null,
			nextReasoning: "default",
			nextServiceTier: "priority",
		}),
	).toEqual({
		modelId: null,
		reasoning: undefined,
		serviceTier: undefined,
	});
});

test("resolveThreadComposerSubmitValues prefers staged next values", () => {
	expect(
		resolveThreadComposerSubmitValues({
			modelId: "anthropic/claude-sonnet-4-6",
			reasoning: "auto",
			serviceTier: undefined,
			nextModelId: "openai/gpt-5",
			nextReasoning: "high",
			nextServiceTier: "priority",
		}),
	).toEqual({
		modelId: "openai/gpt-5",
		reasoning: "high",
		serviceTier: "priority",
	});
});

test("resolveThreadComposerSubmitValues allows staged standard service tier", () => {
	expect(
		resolveThreadComposerSubmitValues({
			modelId: "codex/gpt-5.5",
			reasoning: "default",
			serviceTier: "priority",
			nextModelId: undefined,
			nextReasoning: undefined,
			nextServiceTier: null,
		}),
	).toEqual({
		modelId: "codex/gpt-5.5",
		reasoning: "default",
		serviceTier: undefined,
	});
});
