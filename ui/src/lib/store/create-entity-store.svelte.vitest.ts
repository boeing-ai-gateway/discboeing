import { flushSync } from "svelte";
import { describe, expect, it } from "vitest";

import { createEntityStore } from "./create-entity-store.svelte";

type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T | PromiseLike<T>) => void;
	reject: (reason?: unknown) => void;
};

type Item = {
	id: string;
	name: string;
};

type TestItemState = {
	item: Item | null;
	refresh: () => Promise<Item | null>;
	isStale: boolean;
};

type TestStore = {
	all: () => {
		ensure: () => Promise<Item[]>;
		refresh: () => Promise<Item[]>;
		list: Item[];
		isStale: boolean;
	};
	get: (id: string) => TestItemState;
	peek: (id: string) => Item | null;
	invalidate: (id: string) => void;
	invalidateAll: () => void;
	create?: (args: { id: string; name: string }) => Promise<Item>;
	update?: (id: string, args: { name: string }) => Promise<Item>;
	remove?: (id: string) => Promise<void>;
};

function createDeferred<T>(): Deferred<T> {
	let resolve!: Deferred<T>["resolve"];
	let reject!: Deferred<T>["reject"];
	const promise = new Promise<T>((nextResolve, nextReject) => {
		resolve = nextResolve;
		reject = nextReject;
	});
	return { promise, resolve, reject };
}

async function flushUpdates() {
	await Promise.resolve();
	flushSync();
	await Promise.resolve();
}

function createStore(factory: () => unknown): {
	store: TestStore;
	cleanup: () => void;
} {
	let store: TestStore | null = null;
	const cleanup = $effect.root(() => {
		store = factory() as TestStore;
		return () => {};
	});
	if (store === null) {
		throw new Error("Expected entity store to initialize inside effect root");
	}
	return { store: store as TestStore, cleanup };
}

function createItemState(
	store: TestStore,
	id: string,
): { item: TestItemState; cleanup: () => void } {
	let item: TestItemState | null = null;
	const cleanup = $effect.root(() => {
		item = store.get(id);
		return () => {};
	});
	if (item === null) {
		throw new Error("Expected item state to initialize inside effect root");
	}
	return { item: item as TestItemState, cleanup };
}

describe("createEntityStore concurrency", () => {
	it("preserves a newer item refresh when an older list refresh resolves later", async () => {
		const listRefresh = createDeferred<Item[]>();
		const itemRefresh = createDeferred<Item>();
		let listCalls = 0;
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => {
						listCalls += 1;
						if (listCalls === 1) {
							return [{ id: "item-1", name: "Initial" }];
						}
						return listRefresh.promise;
					},
				},
				indexed: {
					getKey: (item) => item.id,
					load: async () => itemRefresh.promise,
				},
			}),
		);

		try {
			await store.all().ensure();
			const { item, cleanup: cleanupItem } = createItemState(store, "item-1");

			try {
				const listPromise = store.all().refresh();
				const itemPromise = item.refresh();
				await flushUpdates();

				itemRefresh.resolve({ id: "item-1", name: "Newer item" });
				await itemPromise;
				await flushUpdates();

				listRefresh.resolve([{ id: "item-1", name: "Older list" }]);
				await listPromise;
				await flushUpdates();

				expect(item.item).toEqual({ id: "item-1", name: "Newer item" });
				expect(store.peek("item-1")).toEqual({
					id: "item-1",
					name: "Newer item",
				});
				expect(store.all().list).toEqual([
					{ id: "item-1", name: "Newer item" },
				]);
			} finally {
				cleanupItem();
			}
		} finally {
			cleanup();
		}
	});

	it("keeps list and item state stale when invalidateAll happens during an item refresh", async () => {
		const itemRefresh = createDeferred<Item>();
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => [{ id: "item-1", name: "Initial" }],
				},
				indexed: {
					getKey: (item) => item.id,
					load: async () => itemRefresh.promise,
				},
			}),
		);

		try {
			await store.all().ensure();
			const { item, cleanup: cleanupItem } = createItemState(store, "item-1");
			try {
				const itemPromise = item.refresh();
				await flushUpdates();

				store.invalidateAll();
				expect(store.all().isStale).toBe(true);
				expect(item.isStale).toBe(true);

				itemRefresh.resolve({ id: "item-1", name: "Updated" });
				await itemPromise;
				await flushUpdates();

				expect(store.all().list).toEqual([{ id: "item-1", name: "Updated" }]);
				expect(item.item).toEqual({ id: "item-1", name: "Updated" });
				expect(store.all().isStale).toBe(true);
				expect(item.isStale).toBe(true);
			} finally {
				cleanupItem();
			}
		} finally {
			cleanup();
		}
	});

	it("keeps an item stale when invalidate(id) happens during a list refresh", async () => {
		const listRefresh = createDeferred<Item[]>();
		let listCalls = 0;
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => {
						listCalls += 1;
						if (listCalls === 1) {
							return [{ id: "item-1", name: "Initial" }];
						}
						return listRefresh.promise;
					},
				},
				indexed: {
					getKey: (item) => item.id,
				},
			}),
		);

		try {
			await store.all().ensure();
			const { item, cleanup: cleanupItem } = createItemState(store, "item-1");
			try {
				const listPromise = store.all().refresh();
				await flushUpdates();

				store.invalidate("item-1");
				expect(item.isStale).toBe(true);

				listRefresh.resolve([{ id: "item-1", name: "From list refresh" }]);
				await listPromise;
				await flushUpdates();

				expect(store.all().isStale).toBe(false);
				expect(store.all().list).toEqual([
					{ id: "item-1", name: "From list refresh" },
				]);
				expect(item.item).toEqual({ id: "item-1", name: "From list refresh" });
				expect(item.isStale).toBe(true);
			} finally {
				cleanupItem();
			}
		} finally {
			cleanup();
		}
	});

	it("allows invalidate(id) before any item state exists without creating a rune-backed item resource", async () => {
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => [{ id: "item-1", name: "Initial" }],
				},
				indexed: {
					getKey: (item) => item.id,
				},
			}),
		);

		try {
			await store.all().ensure();
			expect(() => store.invalidate("item-1")).not.toThrow();
			await flushUpdates();
			expect(store.peek("item-1")).toEqual({ id: "item-1", name: "Initial" });
		} finally {
			cleanup();
		}
	});

	it("preserves a create-after-merge result when an older list refresh resolves later", async () => {
		const listRefresh = createDeferred<Item[]>();
		let listCalls = 0;
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => {
						listCalls += 1;
						if (listCalls === 1) {
							return [];
						}
						return listRefresh.promise;
					},
				},
				indexed: {
					getKey: (item: Item) => item.id,
				},
				create: {
					run: async ({ id, name }: { id: string; name: string }) => ({
						id,
						name,
					}),
				},
			}),
		);

		try {
			await store.all().ensure();
			const listPromise = store.all().refresh();
			await flushUpdates();

			const created = await store.create?.({ id: "item-3", name: "Created" });
			await flushUpdates();

			listRefresh.resolve([]);
			await listPromise;
			await flushUpdates();

			expect(created).toEqual({ id: "item-3", name: "Created" });
			expect(store.peek("item-3")).toEqual({ id: "item-3", name: "Created" });
			expect(store.all().list).toEqual([{ id: "item-3", name: "Created" }]);
		} finally {
			cleanup();
		}
	});

	it("preserves an update-after-merge result when an older list refresh resolves later", async () => {
		const listRefresh = createDeferred<Item[]>();
		let listCalls = 0;
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => {
						listCalls += 1;
						if (listCalls === 1) {
							return [{ id: "item-1", name: "Initial" }];
						}
						return listRefresh.promise;
					},
				},
				indexed: {
					getKey: (item: Item) => item.id,
				},
				update: {
					run: async (id: string, { name }: { name: string }) => ({ id, name }),
				},
			}),
		);

		try {
			await store.all().ensure();
			const listPromise = store.all().refresh();
			await flushUpdates();

			const updated = await store.update?.("item-1", { name: "Updated" });
			await flushUpdates();

			listRefresh.resolve([{ id: "item-1", name: "Older list" }]);
			await listPromise;
			await flushUpdates();

			expect(updated).toEqual({ id: "item-1", name: "Updated" });
			expect(store.peek("item-1")).toEqual({ id: "item-1", name: "Updated" });
			expect(store.all().list).toEqual([{ id: "item-1", name: "Updated" }]);
		} finally {
			cleanup();
		}
	});

	it("preserves a remove-after-evict result when an older list refresh resolves later", async () => {
		const listRefresh = createDeferred<Item[]>();
		let listCalls = 0;
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => {
						listCalls += 1;
						if (listCalls === 1) {
							return [{ id: "item-1", name: "Initial" }];
						}
						return listRefresh.promise;
					},
				},
				indexed: {
					getKey: (item) => item.id,
				},
				remove: {
					run: async () => {},
				},
			}),
		);

		try {
			await store.all().ensure();
			const listPromise = store.all().refresh();
			await flushUpdates();

			await store.remove?.("item-1");
			await flushUpdates();

			listRefresh.resolve([{ id: "item-1", name: "Older list" }]);
			await listPromise;
			await flushUpdates();

			expect(store.peek("item-1")).toBe(null);
			expect(store.all().list).toEqual([]);
		} finally {
			cleanup();
		}
	});

	it("keeps a remove result when a stale item refresh resolves after delete", async () => {
		const itemRefresh = createDeferred<Item>();
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => [{ id: "item-1", name: "Initial" }],
				},
				indexed: {
					getKey: (item) => item.id,
					load: async () => itemRefresh.promise,
				},
				remove: {
					run: async () => {},
				},
			}),
		);

		try {
			await store.all().ensure();
			const { item, cleanup: cleanupItem } = createItemState(store, "item-1");
			try {
				const itemPromise = item.refresh();
				await flushUpdates();

				await store.remove?.("item-1");
				await flushUpdates();

				itemRefresh.resolve({ id: "item-1", name: "Late stale item" });
				await itemPromise;
				await flushUpdates();

				expect(store.peek("item-1")).toBe(null);
				expect(store.all().list).toEqual([]);
				expect(item.item).toBe(null);
			} finally {
				cleanupItem();
			}
		} finally {
			cleanup();
		}
	});

	it("runs update-after-refresh-item without requiring a pre-existing item state and keeps the authoritative refreshed item", async () => {
		const refreshedItem = createDeferred<Item>();
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => [{ id: "item-1", name: "Initial" }],
				},
				indexed: {
					getKey: (item: Item) => item.id,
					load: async () => refreshedItem.promise,
				},
				update: {
					run: async (id: string, { name }: { name: string }) => ({ id, name }),
					after: "refresh-item",
				},
			}),
		);

		try {
			await store.all().ensure();
			const updatePromise = store.update?.("item-1", { name: "Mutated" });
			await flushUpdates();

			refreshedItem.resolve({ id: "item-1", name: "Authoritative" });
			const updated = await updatePromise;
			await flushUpdates();

			expect(updated).toEqual({ id: "item-1", name: "Mutated" });
			expect(store.peek("item-1")).toEqual({
				id: "item-1",
				name: "Authoritative",
			});
			expect(store.all().list).toEqual([
				{ id: "item-1", name: "Authoritative" },
			]);
		} finally {
			cleanup();
		}
	});

	it("runs create-after-refresh-item without requiring a pre-existing item state", async () => {
		const refreshedItem = createDeferred<Item>();
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => [],
				},
				indexed: {
					getKey: (item: Item) => item.id,
					load: async () => refreshedItem.promise,
				},
				create: {
					run: async ({ id, name }: { id: string; name: string }) => ({
						id,
						name,
					}),
					after: "refresh-item",
				},
			}),
		);

		try {
			await store.all().ensure();
			const createPromise = store.create?.({ id: "item-2", name: "Created" });
			await flushUpdates();

			refreshedItem.resolve({ id: "item-2", name: "Authoritative create" });
			const created = await createPromise;
			await flushUpdates();

			expect(created).toEqual({ id: "item-2", name: "Created" });
			expect(store.peek("item-2")).toEqual({
				id: "item-2",
				name: "Authoritative create",
			});
			expect(store.all().list).toEqual([
				{ id: "item-2", name: "Authoritative create" },
			]);
		} finally {
			cleanup();
		}
	});

	it("keeps a remove-after-refresh-list result when an older list refresh resolves first", async () => {
		const olderRefresh = createDeferred<Item[]>();
		const removeRefresh = createDeferred<Item[]>();
		let listCalls = 0;
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => {
						listCalls += 1;
						if (listCalls === 1) {
							return [{ id: "item-1", name: "Initial" }];
						}
						if (listCalls === 2) {
							return olderRefresh.promise;
						}
						return removeRefresh.promise;
					},
				},
				indexed: {
					getKey: (item: Item) => item.id,
				},
				remove: {
					run: async () => {},
					after: "refresh-list",
				},
			}),
		);

		try {
			await store.all().ensure();
			const staleListPromise = store.all().refresh();
			const removePromise = store.remove?.("item-1");
			await flushUpdates();

			olderRefresh.resolve([{ id: "item-1", name: "Older list" }]);
			await staleListPromise;
			await flushUpdates();

			removeRefresh.resolve([]);
			await removePromise;
			await flushUpdates();

			expect(store.peek("item-1")).toBe(null);
			expect(store.all().list).toEqual([]);
		} finally {
			cleanup();
		}
	});

	it("keeps not-found eviction when an older list refresh resolves later", async () => {
		const listRefresh = createDeferred<Item[]>();
		const notFound = new Error("missing");
		let listCalls = 0;
		const { store, cleanup } = createStore(() =>
			createEntityStore<Item, string>({
				owner: "EntityStoreRaceTest",
				list: {
					load: async () => {
						listCalls += 1;
						if (listCalls === 1) {
							return [{ id: "item-1", name: "Initial" }];
						}
						return listRefresh.promise;
					},
				},
				indexed: {
					getKey: (item: Item) => item.id,
					load: async () => {
						throw notFound;
					},
					isNotFoundError: (error) => error === notFound,
					notFound: "evict",
				},
			}),
		);

		try {
			await store.all().ensure();
			const { item, cleanup: cleanupItem } = createItemState(store, "item-1");
			try {
				const listPromise = store.all().refresh();
				const itemPromise = item.refresh();
				await flushUpdates();

				await itemPromise;
				await flushUpdates();

				listRefresh.resolve([{ id: "item-1", name: "Older list" }]);
				await listPromise;
				await flushUpdates();

				expect(item.item).toBe(null);
				expect(store.peek("item-1")).toBe(null);
				expect(store.all().list).toEqual([]);
			} finally {
				cleanupItem();
			}
		} finally {
			cleanup();
		}
	});
});
