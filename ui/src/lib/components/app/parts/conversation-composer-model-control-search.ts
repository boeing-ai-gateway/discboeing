import type { ModelInfo } from "$lib/api-types";

export type DisplayModel = ModelInfo & {
	selectedIds: string[];
};

export type ModelProviderEntry = [string, DisplayModel[]];

export function getSearchableModelProvider(modelProvider: string | undefined) {
	return modelProvider || "Other";
}

export function getModelSearchText(model: DisplayModel) {
	return [
		model.name,
		model.description,
		getSearchableModelProvider(model.provider),
		model.id,
	]
		.join(" ")
		.toLowerCase();
}

export function filterModelProviderEntries(
	entries: ModelProviderEntry[],
	query: string,
) {
	const normalizedQuery = query.trim().toLowerCase();
	if (normalizedQuery.length === 0) {
		return entries;
	}

	const filteredEntries: ModelProviderEntry[] = [];
	for (const [provider, providerModels] of entries) {
		const matchingModels = providerModels.filter((model) =>
			getModelSearchText(model).includes(normalizedQuery),
		);

		if (matchingModels.length === 0) {
			continue;
		}

		filteredEntries.push([provider, matchingModels]);
	}

	return filteredEntries;
}
