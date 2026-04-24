import { api } from "$lib/api-client";
import type { StartupTask } from "$lib/api-types";
import type { AsyncStatus } from "$lib/shell-types";

import {
	createEntityStore,
	type CreateEntityStoreArgs,
} from "./create-entity-store.svelte";

const startupTaskStoreResourceArgs = {
	owner: "StartupTaskStore",
	list: {
		load: async () => {
			const status = await api.getSystemStatus();
			return status.startupTasks ?? [];
		},
	},
	indexed: {
		getKey: (task: StartupTask) => task.id,
	},
} satisfies CreateEntityStoreArgs<StartupTask, string>;

export class StartupTaskStore {
	#resource = createEntityStore<
		StartupTask,
		string,
		typeof startupTaskStoreResourceArgs
	>(startupTaskStoreResourceArgs);

	get list(): StartupTask[] {
		return this.#resource.all().list;
	}

	get status(): AsyncStatus {
		return this.#resource.all().status;
	}

	/** Returns the cached task without side effects. */
	peek(id: string): StartupTask | null {
		return this.#resource.peek(id);
	}

	/** Returns the cached task and triggers a background fetch of the full list on cache miss. */
	ensure(id: string): StartupTask | null {
		return this.#resource.peek(id);
	}

	async fetch(): Promise<void> {
		await this.#resource.all().ensure();
	}

	invalidate(): void {
		this.#resource.invalidateAll();
	}
}
