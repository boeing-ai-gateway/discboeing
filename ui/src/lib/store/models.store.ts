import { api } from "$lib/api-client";
import type { ModelInfo } from "$lib/api-types";
import type { AsyncStatus } from "$lib/resource/types";

import {
	createEntityStore,
	type CreateEntityStoreArgs,
} from "./create-entity-store.svelte";

const modelStoreResourceArgs = {
	owner: "ModelStore",
	list: {
		load: async () => {
			const { models } = await api.getProjectModels();
			return models;
		},
	},
	indexed: {
		getKey: (model: ModelInfo) => model.id,
	},
} satisfies CreateEntityStoreArgs<ModelInfo, string>;

export class ModelStore {
	#resource = createEntityStore<
		ModelInfo,
		string,
		typeof modelStoreResourceArgs
	>(modelStoreResourceArgs);

	get list(): ModelInfo[] {
		return this.#resource.all().list;
	}

	get status(): AsyncStatus {
		return this.#resource.all().status;
	}

	/** Returns the cached model without side effects. */
	peek(id: string): ModelInfo | null {
		return this.#resource.peek(id);
	}

	/** Returns the cached model and triggers a background fetch of the full list on cache miss. */
	ensure(id: string): ModelInfo | null {
		return this.#resource.peek(id);
	}

	async fetch(): Promise<void> {
		await this.#resource.all().ensure();
	}

	invalidate(): void {
		this.#resource.invalidateAll();
	}
}
