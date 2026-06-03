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
