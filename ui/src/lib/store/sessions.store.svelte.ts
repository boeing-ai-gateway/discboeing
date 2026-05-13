import { generateId } from "ai";
import { SvelteSet } from "svelte/reactivity";

import { api, ApiError } from "$lib/api-client";
import type { Session, UpdateSessionRequest } from "$lib/api-types";
import type { AsyncStatus } from "$lib/resource/types";

import {
	createEntityStore,
	type CreateEntityStoreArgs,
} from "./create-entity-store.svelte";
import { RequestCoalescer } from "./request-coalescer";

export type CreateSessionData = {
	workspaceId?: string;
	model?: string;
	reasoning?: string;
};

const sessionStoreResourceArgs = {
	owner: "SessionStore",
	list: {
		load: async () => {
			const { sessions } = await api.getSessions();
			return sessions;
		},
	},
	indexed: {
		getKey: (session: Session) => session.id,
		load: (id: string) => api.getSession(id),
		isNotFoundError: (error: unknown) =>
			error instanceof ApiError && error.status === 404,
		notFound: "evict",
	},
	create: {
		run: async (data: CreateSessionData) => {
			const { id } = await api.createSession({ id: generateId(), ...data });
			return api.getSession(id);
		},
		after: "merge",
	},
	update: {
		run: async (id: string, data: UpdateSessionRequest) => {
			await api.updateSession(id, data);
			return api.getSession(id);
		},
		after: "merge",
	},
	remove: {
		run: (id: string) => api.deleteSession(id),
		after: "evict",
	},
} satisfies CreateEntityStoreArgs<
	Session,
	string,
	CreateSessionData,
	UpdateSessionRequest
>;

export class SessionStore {
	#resource = createEntityStore<
		Session,
		string,
		typeof sessionStoreResourceArgs
	>(sessionStoreResourceArgs);
	#fetchOneRequests = new RequestCoalescer<string, Session | null>();
	#backgroundFetches = new SvelteSet<string>();

	get list(): Session[] {
		return this.#resource.all().list;
	}

	get status(): AsyncStatus {
		return this.#resource.all().status;
	}

	/** Returns the cached session without side effects. */
	peek(id: string): Session | null {
		return this.#resource.peek(id);
	}

	/** Returns the cached session and triggers a background fetchOne on cache miss. */
	ensure(id: string): Session | null {
		const cached = this.#resource.peek(id);
		if (cached === null && !this.#backgroundFetches.has(id)) {
			this.#backgroundFetches.add(id);
			void this.fetchOne(id).finally(() => this.#backgroundFetches.delete(id));
		}
		return cached;
	}

	/** Removes an item from the local cache without an API call (e.g. server-pushed removal). */
	evict(id: string): void {
		this.#resource.evict(id);
	}

	upsert(session: Session): void {
		this.#resource.upsert(session);
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
				const session = await api.getSession(id);
				this.#resource.upsert(session);
				return session;
			} catch (error) {
				if (error instanceof ApiError && error.status === 404) {
					this.#resource.evict(id);
					return null;
				}
				console.error("[SessionStore] Failed to fetch session:", id, error);
				return null;
			}
		});
	}

	create(data: CreateSessionData = {}): Promise<Session> {
		return this.#resource.create(data);
	}

	update(id: string, data: UpdateSessionRequest): Promise<Session> {
		return this.#resource.update(id, data);
	}

	async stop(id: string): Promise<Session> {
		const session = await api.stopSession(id);
		this.#resource.upsert(session);
		return session;
	}

	remove(id: string): Promise<void> {
		return this.#resource.remove(id);
	}
}
