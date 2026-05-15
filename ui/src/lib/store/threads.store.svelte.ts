import { SvelteSet } from "svelte/reactivity";

import { api } from "$lib/api-client";
import type {
	CreateThreadRequest,
	Thread,
	UpdateThreadRequest,
} from "$lib/api-types";
import type { AsyncStatus } from "$lib/resource/types";

import {
	createEntityStore,
	type CreateEntityStoreArgs,
} from "./create-entity-store.svelte";
import { RequestCoalescer } from "./request-coalescer";

type ThreadStoreArgs = {
	sessionId: string;
	enabled?: () => boolean;
};

export class ThreadStore {
	#sessionId: string;
	#resource;
	#fetchOneRequests = new RequestCoalescer<string, Thread | null>();
	#backgroundFetches = new SvelteSet<string>();

	constructor(args: ThreadStoreArgs) {
		this.#sessionId = args.sessionId;
		const resourceArgs = {
			owner: `ThreadStore:${args.sessionId}`,
			enabled: args.enabled,
			list: {
				load: async () => {
					const { threads } = await api.getThreads(args.sessionId);
					return threads;
				},
			},
			indexed: {
				getKey: (thread: Thread) => thread.id,
				load: (id: string) => api.getThread(args.sessionId, id),
			},
			create: {
				run: async (data: CreateThreadRequest) => {
					const created = await api.createThread(args.sessionId, data);
					return api.getThread(args.sessionId, created.id);
				},
				after: "merge",
			},
			update: {
				run: async (id: string, data: UpdateThreadRequest) => {
					await api.updateThread(args.sessionId, id, data);
					return api.getThread(args.sessionId, id);
				},
				after: "merge",
			},
			remove: {
				run: async (id: string) => {
					await api.deleteThread(args.sessionId, id);
				},
				after: "evict",
			},
		} satisfies CreateEntityStoreArgs<
			Thread,
			string,
			CreateThreadRequest,
			UpdateThreadRequest
		>;
		this.#resource = createEntityStore<Thread, string, typeof resourceArgs>(
			resourceArgs,
		);
	}

	get list(): Thread[] {
		return this.#resource.all().list;
	}

	get status(): AsyncStatus {
		return this.#resource.all().status;
	}

	/** Returns the cached thread, or null. Use fetchOne to populate on a cache miss. */
	get(id: string): Thread | null {
		return this.#resource.peek(id);
	}

	ensure(id: string): Thread | null {
		const cached = this.#resource.peek(id);
		if (cached === null && !this.#backgroundFetches.has(id)) {
			this.#backgroundFetches.add(id);
			void this.fetchOne(id).finally(() => this.#backgroundFetches.delete(id));
		}
		return cached;
	}

	reset(): void {
		this.#resource.setList([]);
		this.#resource.invalidateAll();
	}

	async fetch(): Promise<void> {
		await this.#resource.all().refresh();
	}

	async fetchOne(threadId: string): Promise<void> {
		await this.#fetchOneRequests.run(threadId, async () => {
			try {
				const thread = await api.getThread(this.#sessionId, threadId);
				this.#resource.upsert(thread);
				return thread;
			} catch (error) {
				console.error("[ThreadStore] Failed to fetch thread:", threadId, error);
				return null;
			}
		});
	}

	upsert(thread: Thread): void {
		this.#resource.upsert(thread);
	}

	create(data: CreateThreadRequest): Promise<Thread> {
		return this.#resource.create(data);
	}

	update(threadId: string, data: UpdateThreadRequest): Promise<Thread> {
		return this.#resource.update(threadId, data);
	}

	remove(threadId: string): Promise<void> {
		return this.#resource.remove(threadId);
	}
}
