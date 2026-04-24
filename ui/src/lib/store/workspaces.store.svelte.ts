import { SvelteSet } from "svelte/reactivity";

import { api } from "$lib/api-client";
import type {
	CreateWorkspaceRequest,
	Workspace,
	WorkspaceValidationResult,
} from "$lib/api-types";
import type { AsyncStatus } from "$lib/shell-types";

import {
	createEntityStore,
	type CreateEntityStoreArgs,
} from "./create-entity-store.svelte";
import { RequestCoalescer } from "./request-coalescer";

const workspaceStoreResourceArgs = {
	owner: "WorkspaceStore",
	list: {
		load: async () => {
			const { workspaces } = await api.getWorkspaces();
			return workspaces;
		},
	},
	indexed: {
		getKey: (workspace: Workspace) => workspace.id,
		load: (id: string) => api.getWorkspace(id),
	},
	create: {
		run: (data: CreateWorkspaceRequest) => api.createWorkspace(data),
		after: "merge",
	},
	update: {
		run: (id: string, data: { path?: string; displayName?: string | null }) =>
			api.updateWorkspace(id, data),
		after: "merge",
	},
} satisfies CreateEntityStoreArgs<
	Workspace,
	string,
	CreateWorkspaceRequest,
	{ path?: string; displayName?: string | null }
>;

export class WorkspaceStore {
	#resource = createEntityStore<
		Workspace,
		string,
		typeof workspaceStoreResourceArgs
	>(workspaceStoreResourceArgs);
	#inflight = new SvelteSet<string>();
	#fetchOneRequests = new RequestCoalescer<string, Workspace | null>();

	get list(): Workspace[] {
		return this.#resource.all().list;
	}

	get status(): AsyncStatus {
		return this.#resource.all().status;
	}

	/** Returns the cached workspace without side effects. */
	peek(id: string): Workspace | null {
		return this.#resource.peek(id);
	}

	/** Returns the cached workspace and triggers a background fetchOne on cache miss. */
	ensure(id: string): Workspace | null {
		const cached = this.#resource.peek(id);
		if (cached === null && !this.#inflight.has(id)) {
			this.#inflight.add(id);
			void this.fetchOne(id).finally(() => this.#inflight.delete(id));
		}
		return cached;
	}

	async fetch(): Promise<void> {
		await this.#resource.all().ensure();
	}

	invalidate(): void {
		this.#resource.invalidateAll();
	}

	async fetchOne(id: string): Promise<void> {
		await this.#fetchOneRequests.run(id, async () => {
			try {
				const workspace = await api.getWorkspace(id);
				this.#resource.upsert(workspace);
				return workspace;
			} catch (error) {
				console.error("[WorkspaceStore] Failed to fetch workspace:", id, error);
				return null;
			}
		});
	}

	async validate(
		path: string,
		sourceType: "local" | "git",
	): Promise<WorkspaceValidationResult> {
		return api.validateWorkspace({ path, sourceType });
	}

	create(data: CreateWorkspaceRequest): Promise<Workspace> {
		return this.#resource.create(data);
	}

	update(
		id: string,
		data: { path?: string; displayName?: string | null },
	): Promise<Workspace> {
		return this.#resource.update(id, data);
	}

	async remove(id: string, deleteFiles = false): Promise<void> {
		await api.deleteWorkspace(id, deleteFiles);
		this.#resource.evict(id);
	}
}
