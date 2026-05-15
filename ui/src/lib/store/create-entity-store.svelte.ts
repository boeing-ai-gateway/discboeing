import { SvelteMap } from "svelte/reactivity";

import { createResource } from "../resource/create-resource.svelte";

import type {
	CreateEntityStoreArgs,
	EntityItemState,
	EntityKey,
	EntityListState,
	EntityStoreFromArgs,
} from "./create-entity-store.types";

function now() {
	return typeof performance !== "undefined" ? performance.now() : Date.now();
}

function mergeByKey<TItem, TId extends EntityKey>(
	items: TItem[],
	nextItems: TItem[],
	getKey: (item: TItem) => TId,
): TItem[] {
	const merged = [...items];
	for (const item of nextItems) {
		const idx = merged.findIndex((current) => getKey(current) === getKey(item));
		if (idx === -1) {
			merged.push(item);
		} else {
			merged[idx] = item;
		}
	}
	return merged;
}

export type { CreateEntityStoreArgs } from "./create-entity-store.types";
export type * from "./create-entity-store.types";

export function createEntityStore<
	TItem,
	TId extends EntityKey = never,
	TArgs extends CreateEntityStoreArgs<TItem, TId, never, never> =
		CreateEntityStoreArgs<TItem, TId, never, never>,
>(args: TArgs): EntityStoreFromArgs<TItem, TId, TArgs> {
	const enabled = args.enabled ?? (() => true);
	const getIndexedKey = args.indexed?.getKey as
		| ((item: TItem) => TId)
		| undefined;
	const itemResources = new SvelteMap<
		TId,
		ReturnType<typeof createResource<TItem | null>>
	>();
	const itemStates = new SvelteMap<TId, EntityItemState<TItem>>();
	const tombstones = new SvelteMap<TId, number>();

	function findInList(id: TId, items = listResource.peek()): TItem | null {
		if (!getIndexedKey) {
			return null;
		}
		return items.find((item) => getIndexedKey(item) === id) ?? null;
	}

	function getNewerItemSnapshot(
		id: TId,
		freshAt?: number,
	): TItem | null | undefined {
		if (freshAt === undefined) {
			return undefined;
		}
		const resource = itemResources.get(id);
		if (
			resource &&
			resource.fetchedAt !== null &&
			resource.fetchedAt > freshAt
		) {
			return resource.peek();
		}
		return undefined;
	}

	function mergeWithNewerItemResources(
		items: TItem[],
		freshAt?: number,
	): TItem[] {
		if (!getIndexedKey || freshAt === undefined) {
			return getIndexedKey
				? items.filter((item) => !tombstones.has(getIndexedKey(item)))
				: items;
		}
		let merged = [...items];
		for (const [id, deletedAt] of tombstones) {
			if (deletedAt > freshAt) {
				merged = merged.filter((item) => getIndexedKey(item) !== id);
			} else if (merged.some((item) => getIndexedKey(item) === id)) {
				tombstones.delete(id);
			}
		}
		for (const [id] of itemResources) {
			const newerItem = getNewerItemSnapshot(id, freshAt);
			if (newerItem === undefined) {
				continue;
			}
			if (newerItem === null) {
				merged = merged.filter((item) => getIndexedKey(item) !== id);
				continue;
			}
			const idx = merged.findIndex((item) => getIndexedKey(item) === id);
			if (idx === -1) {
				merged.push(newerItem);
			} else {
				merged[idx] = newerItem;
			}
		}
		return merged;
	}

	function syncItemResourcesFromList(
		items: TItem[],
		options: {
			markFresh?: boolean;
			clearMissing?: boolean;
			freshAt?: number;
		} = {},
	) {
		if (!getIndexedKey) {
			return;
		}
		for (const [id, resource] of itemResources) {
			const newerItem = getNewerItemSnapshot(id, options.freshAt);
			if (newerItem !== undefined) {
				continue;
			}
			const nextItem = items.find((item) => getIndexedKey(item) === id) ?? null;
			if (nextItem !== null || options.clearMissing) {
				resource.setData(nextItem, {
					markFresh: options.markFresh,
					freshAt: options.freshAt,
				});
			}
		}
	}

	const listResource = createResource<TItem[]>({
		owner: `${args.owner}:list`,
		enabled,
		load: async () => {
			const startedAt = now();
			const items = await args.list.load();
			const mergedItems = mergeWithNewerItemResources(items, startedAt);
			syncItemResourcesFromList(mergedItems, {
				markFresh: true,
				clearMissing: true,
				freshAt: startedAt,
			});
			return mergedItems;
		},
		createEmptyValue: () => [],
		staleAfterMs: args.list.cache?.staleAfterMs,
		retry: args.list.cache?.retry,
	});

	function setListInternal(
		items: TItem[],
		options: {
			markFresh?: boolean;
			clearMissing?: boolean;
			freshAt?: number;
		} = {},
	) {
		const mergedItems = mergeWithNewerItemResources(items, options.freshAt);
		listResource.setData(mergedItems, {
			markFresh: options.markFresh,
			freshAt: options.freshAt,
		});
		syncItemResourcesFromList(mergedItems, options);
	}

	function mergeListInternal(
		items: TItem[],
		options: { markFresh?: boolean; freshAt?: number } = {},
	) {
		if (getIndexedKey) {
			for (const item of items) {
				const id = getIndexedKey(item);
				const deletedAt = tombstones.get(id);
				if (deletedAt === undefined) {
					continue;
				}
				if (options.freshAt === undefined || options.freshAt >= deletedAt) {
					tombstones.delete(id);
				}
			}
			listResource.update(
				(current) =>
					mergeWithNewerItemResources(
						mergeByKey(current, items, getIndexedKey),
						options.freshAt,
					),
				{ markFresh: options.markFresh, freshAt: options.freshAt },
			);
			for (const item of items) {
				const id = getIndexedKey(item);
				itemResources.get(id)?.setData(item, {
					markFresh: options.markFresh,
					freshAt: options.freshAt,
				});
			}
			return;
		}

		listResource.update(
			(current) =>
				mergeWithNewerItemResources([...current, ...items], options.freshAt),
			{
				markFresh: options.markFresh,
				freshAt: options.freshAt,
			},
		);
	}

	function evictInternal(
		id: TId,
		options: { markFresh?: boolean; freshAt?: number } = {},
	) {
		tombstones.set(id, options.freshAt ?? now());
		if (getIndexedKey) {
			listResource.update(
				(items) =>
					mergeWithNewerItemResources(
						items.filter((item) => getIndexedKey(item) !== id),
						options.freshAt,
					),
				{ markFresh: options.markFresh, freshAt: options.freshAt },
			);
		}
		itemResources.get(id)?.setData(null, {
			markFresh: options.markFresh,
			freshAt: options.freshAt,
		});
	}

	function requireIndexedKey(message: string): (item: TItem) => TId {
		if (!getIndexedKey) {
			throw new Error(message);
		}
		return getIndexedKey;
	}

	function createItemResource(id: TId) {
		const resource = createResource<TItem | null>({
			owner: `${args.owner}:item:${String(id)}`,
			enabled,
			load: async () => {
				const startedAt = now();
				if (!args.indexed?.load) {
					const items = await listResource.ensure();
					return findInList(id, items);
				}

				try {
					const item = await args.indexed.load(id);
					mergeListInternal([item], { markFresh: true, freshAt: startedAt });
					return item;
				} catch (error) {
					if (args.indexed?.isNotFoundError?.(error)) {
						if (args.indexed.notFound === "evict") {
							evictInternal(id, { markFresh: true, freshAt: startedAt });
						} else {
							resource.setData(null, {
								markFresh: true,
								freshAt: startedAt,
							});
						}
						return null;
					}
					throw error;
				}
			},
			createEmptyValue: () => null,
			staleAfterMs: args.indexed?.cache?.staleAfterMs,
			retry: args.indexed?.cache?.retry,
		});

		if (listResource.fetchedAt !== null) {
			const cached = findInList(id);
			if (cached !== null) {
				resource.setData(cached, { markFresh: true });
			}
		} else {
			const cached = findInList(id);
			if (cached !== null) {
				resource.setData(cached);
			}
		}

		itemResources.set(id, resource);
		return resource;
	}

	function getItemResource(id: TId) {
		return itemResources.get(id) ?? createItemResource(id);
	}

	async function refreshIndexedItem(id: TId) {
		const resource = itemResources.get(id);
		if (resource) {
			return resource.refresh();
		}
		const startedAt = now();
		if (!args.indexed?.load) {
			const items = await listResource.ensure();
			return findInList(id, items);
		}
		try {
			const item = await args.indexed.load(id);
			mergeListInternal([item], { markFresh: true, freshAt: startedAt });
			return item;
		} catch (error) {
			if (args.indexed?.isNotFoundError?.(error)) {
				if (args.indexed.notFound === "evict") {
					evictInternal(id, { markFresh: true, freshAt: startedAt });
				}
				return null;
			}
			throw error;
		}
	}

	const allState: EntityListState<TItem> = {
		get list() {
			return listResource.data;
		},
		get status() {
			return listResource.status;
		},
		get error() {
			return listResource.error;
		},
		get isRefreshing() {
			return listResource.isRefreshing;
		},
		get isStale() {
			return listResource.isStale;
		},
		get fetchedAt() {
			return listResource.fetchedAt;
		},
		ensure: () => listResource.ensure(),
		refresh: () => listResource.refresh(),
		invalidate: () => listResource.invalidate(),
	};

	const store: Record<string, unknown> = {
		all: () => allState,
		invalidateAll: () => {
			listResource.invalidate();
			for (const resource of itemResources.values()) {
				resource.invalidate();
			}
		},
		setList: (items: TItem[]) => setListInternal(items, { clearMissing: true }),
		mergeList: (items: TItem[]) => mergeListInternal(items),
	};

	if (args.indexed) {
		Object.assign(store, {
			get(id: TId) {
				const existingState = itemStates.get(id);
				if (existingState) {
					return existingState;
				}

				// get(id) is the intentional item-state materialization point. It
				// creates the rune-backed per-item resource, so callers should create
				// item state from an effect/rune context instead of plain imperative
				// mutation paths.
				const resource = getItemResource(id);
				const state: EntityItemState<TItem> = {
					get item() {
						return resource.data;
					},
					get status() {
						return resource.status;
					},
					get error() {
						return resource.error;
					},
					get isRefreshing() {
						return resource.isRefreshing;
					},
					get isStale() {
						return resource.isStale;
					},
					get fetchedAt() {
						return resource.fetchedAt;
					},
					ensure: () => resource.ensure(),
					refresh: () => resource.refresh(),
					invalidate: () => resource.invalidate(),
				};
				itemStates.set(id, state);
				return state;
			},
			peek(id: TId) {
				if (itemResources.has(id)) {
					return itemResources.get(id)!.peek();
				}
				return findInList(id);
			},
			upsert(item: TItem) {
				mergeListInternal([item]);
			},
			evict(id: TId) {
				evictInternal(id);
			},
			invalidate(id: TId) {
				itemResources.get(id)?.invalidate();
			},
		});
	}

	if (args.create) {
		Object.assign(store, {
			async create(createArgs: unknown) {
				const startedAt = now();
				const item = await args.create!.run(createArgs as never);
				switch (args.create!.after ?? "merge") {
					case "merge": {
						const getKey =
							(args.create!.getKey as ((item: TItem) => TId) | undefined) ??
							requireIndexedKey(
								`[${args.owner}] create.after "merge" requires indexed.getKey or create.getKey`,
							);
						mergeListInternal([item], { markFresh: true, freshAt: startedAt });
						itemResources.get(getKey(item))?.setData(item, {
							markFresh: true,
							freshAt: startedAt,
						});
						break;
					}
					case "refresh-list":
						await listResource.refresh();
						break;
					case "refresh-item": {
						const getKey =
							(args.create!.getKey as ((item: TItem) => TId) | undefined) ??
							requireIndexedKey(
								`[${args.owner}] create.after "refresh-item" requires indexed.getKey or create.getKey`,
							);
						await refreshIndexedItem(getKey(item));
						break;
					}
				}
				return item;
			},
		});
	}

	if (args.update) {
		Object.assign(store, {
			async update(id: TId, updateArgs: unknown) {
				const startedAt = now();
				const item = await args.update!.run(id, updateArgs as never);
				switch (args.update!.after ?? "merge") {
					case "merge":
						mergeListInternal([item], { markFresh: true, freshAt: startedAt });
						itemResources.get(id)?.setData(item, {
							markFresh: true,
							freshAt: startedAt,
						});
						break;
					case "refresh-list":
						await listResource.refresh();
						break;
					case "refresh-item":
						await refreshIndexedItem(id);
						break;
				}
				return item;
			},
		});
	}

	if (args.remove) {
		Object.assign(store, {
			async remove(id: TId) {
				const startedAt = now();
				await args.remove!.run(id);
				switch (args.remove!.after ?? "evict") {
					case "evict":
						evictInternal(id, { markFresh: true, freshAt: startedAt });
						break;
					case "refresh-list":
						await listResource.refresh();
						break;
				}
			},
		});
	}

	return store as EntityStoreFromArgs<TItem, TId, TArgs>;
}
