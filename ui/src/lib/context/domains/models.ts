import { api } from "$lib/api-client";
import type { ModelInfo } from "$lib/api-types";
import type { CollectionCache } from "$lib/context/cache";
import {
	createCollectionCache,
	createErrorStatus,
	createLoadingStatus,
	createReadyStatus,
	upsertById,
} from "$lib/context/cache";
import type { Context } from "$lib/context/context.types";

export type ModelsState = CollectionCache<ModelInfo>;

export function createModelsState(): ModelsState {
	return createCollectionCache<ModelInfo>();
}

export async function loadModelsIntoCache(context: Context): Promise<void> {
	context.data.models.status = createLoadingStatus();

	try {
		const response = await api.getProjectModels();
		const state = createModelsState();
		for (const model of response.models) {
			upsertById(state, model.id, model);
		}
		state.status = createReadyStatus();
		context.data.models = state;
	} catch (error) {
		context.data.models.status = createErrorStatus(error);
		throw error;
	}
}
