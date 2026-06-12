export function normalizeThreadComposerReasoning(
	reasoning: string | null | undefined,
): string | undefined {
	return reasoning && reasoning.length > 0 ? reasoning : undefined;
}

export function normalizeThreadComposerServiceTier(
	serviceTier: string | null | undefined,
): string | undefined {
	const normalized = serviceTier?.trim().toLowerCase();
	if (!normalized) {
		return undefined;
	}
	return normalized === "fast" ? "priority" : normalized;
}

export function parseComposerModelSelection(
	modelId: string | null | undefined,
): { modelId: string | null } {
	return {
		modelId: modelId && modelId.length > 0 ? modelId : null,
	};
}

export function resolveThreadComposerSubmitValues({
	modelId,
	reasoning,
	serviceTier,
	nextModelId,
	nextReasoning,
	nextServiceTier,
}: {
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
	nextModelId: string | null | undefined;
	nextReasoning: string | undefined;
	nextServiceTier: string | null | undefined;
}): {
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
} {
	const resolvedModelId = nextModelId !== undefined ? nextModelId : modelId;
	return {
		modelId: resolvedModelId,
		reasoning: resolvedModelId
			? normalizeThreadComposerReasoning(nextReasoning ?? reasoning)
			: undefined,
		serviceTier: resolvedModelId
			? normalizeThreadComposerServiceTier(
					nextServiceTier !== undefined ? nextServiceTier : serviceTier,
				)
			: undefined,
	};
}
