import type { AsyncStatus } from "../resource/types";

/**
 * Identifies an entity inside the cache.
 */
export type EntityKey = string | number;

/**
 * Describes stale-while-refresh behavior for a resource.
 *
 * - `staleAfterMs` controls when cached data becomes eligible for background
 *   revalidation.
 * - `retry` is intentionally left open in this draft so the behavior can later
 *   align with the existing resource retry helpers.
 */
export type EntityStoreCachePolicy = {
	staleAfterMs?: number;
	retry?: {
		mode?: "none" | "background";
		initialDelayMs?: number;
		maxDelayMs?: number;
	};
};

/**
 * The live SWR-style state for a whole entity collection.
 *
 * Behavior:
 * - `list` returns the currently cached collection immediately.
 * - reading this state is expected to schedule background revalidation when the
 *   cached collection is missing or stale.
 * - `status` reflects the collection-level request state.
 * - `isRefreshing` is true when a background refresh is running while cached
 *   data remains available.
 * - `refresh()` forces a network refresh even when the cache is still fresh.
 * - `invalidate()` marks the collection stale without clearing cached data.
 */
export interface EntityListState<TItem> {
	readonly list: TItem[];
	readonly status: AsyncStatus;
	readonly error: string | null;
	readonly isRefreshing: boolean;
	readonly isStale: boolean;
	readonly fetchedAt: number | null;
	ensure(): Promise<TItem[]>;
	refresh(): Promise<TItem[]>;
	invalidate(): void;
}

/**
 * The live SWR-style state for one entity lookup.
 *
 * Behavior:
 * - `item` returns the currently cached entity immediately, or `null` when the
 *   entity has not been cached yet.
 * - reading this state is expected to schedule background revalidation for this
 *   specific entity when it is missing or stale.
 * - `status` reflects the request state for this entity lookup, not the whole
 *   collection.
 * - `isRefreshing` is true when a background refresh is running while a cached
 *   item is still available.
 * - `refresh()` forces a network refresh for this entity.
 * - `invalidate()` marks this entity stale without clearing cached data.
 */
export interface EntityItemState<TItem> {
	readonly item: TItem | null;
	readonly status: AsyncStatus;
	readonly error: string | null;
	readonly isRefreshing: boolean;
	readonly isStale: boolean;
	readonly fetchedAt: number | null;
	ensure(): Promise<TItem | null>;
	refresh(): Promise<TItem | null>;
	invalidate(): void;
}

/**
 * The base contract every entity store exposes.
 *
 * Behavior:
 * - `all()` returns a stable SWR-style view over the cached collection.
 * - `invalidateAll()` marks the whole collection stale without clearing the
 *   cached rows.
 * - `setList(items)` synchronously replaces the local collection cache.
 * - `mergeList(items)` synchronously merges the provided rows into the local
 *   collection cache.
 */
export interface EntityStoreBase<TItem> {
	all(): EntityListState<TItem>;
	invalidateAll(): void;
	setList(items: TItem[]): void;
	mergeList(items: TItem[]): void;
}

/**
 * Optional indexed capability.
 *
 * Behavior:
 * - `get(id)` returns a stable SWR-style view over one cached entity.
 * - `peek(id)` reads the cache only and never triggers network work.
 * - `upsert(item)` synchronously merges an entity into the local cache.
 * - `evict(id)` synchronously removes an entity from the local cache.
 * - `invalidate(id)` marks one cached entity stale without clearing it.
 */
export interface EntityStoreIndexedCapability<TItem, TId extends EntityKey> {
	get(id: TId): EntityItemState<TItem>;
	peek(id: TId): TItem | null;
	upsert(item: TItem): void;
	evict(id: TId): void;
	invalidate(id: TId): void;
}

/**
 * Configuration for collection reads.
 */
export type EntityListDefinition<TItem> = {
	load: () => Promise<TItem[]>;
	cache?: EntityStoreCachePolicy;
};

/**
 * Configuration for optional single-entity reads.
 */
export type EntityIndexedDefinition<TItem, TId extends EntityKey> = {
	getKey: (item: TItem) => TId;
	load?: (id: TId) => Promise<TItem>;
	isNotFoundError?: (error: unknown) => boolean;
	notFound?: "evict" | "ignore";
	cache?: EntityStoreCachePolicy;
};

/**
 * Configuration for create mutations.
 *
 * Behavior:
 * - `run(args)` performs the authoritative create request.
 * - `after` determines how the cache is updated after a successful create.
 * - `after: "merge"` means the returned entity is merged into the cache.
 * - `after: "refresh-list"` means the collection is force-refreshed.
 * - `after: "refresh-item"` means the returned entity key is force-refreshed.
 */
export type EntityCreateDefinition<
	TItem,
	TCreateArgs,
	TId extends EntityKey,
> = {
	run: (args: TCreateArgs) => Promise<TItem>;
	after?: "merge" | "refresh-list" | "refresh-item";
	getKey?: (item: TItem) => TId;
};

/**
 * Configuration for update mutations.
 *
 * Behavior:
 * - `run(id, args)` performs the authoritative update request.
 * - `after` controls how the cache is reconciled after success.
 */
export type EntityUpdateDefinition<
	TItem,
	TId extends EntityKey,
	TUpdateArgs,
> = {
	run: (id: TId, args: TUpdateArgs) => Promise<TItem>;
	after?: "merge" | "refresh-list" | "refresh-item";
};

/**
 * Configuration for delete mutations.
 *
 * Behavior:
 * - `run(id)` performs the authoritative delete request.
 * - `after` controls post-delete cache handling.
 * - `after: "evict"` removes the entity from the local cache.
 * - `after: "refresh-list"` forces the collection to refresh after delete.
 */
export type EntityRemoveDefinition<TId extends EntityKey> = {
	run: (id: TId) => Promise<void>;
	after?: "evict" | "refresh-list";
};

/**
 * Draft configuration for a full entity store.
 *
 * This intentionally separates collection reads, optional indexed access, and
 * mutations so the returned store type can expose only the capabilities that
 * are actually configured.
 */
export type CreateEntityStoreArgs<
	TItem,
	TId extends EntityKey = never,
	TCreateArgs = never,
	TUpdateArgs = never,
> = {
	owner: string;
	enabled?: () => boolean;
	list: EntityListDefinition<TItem>;
	indexed?: EntityIndexedDefinition<TItem, TId>;
	create?: EntityCreateDefinition<TItem, TCreateArgs, TId>;
	update?: EntityUpdateDefinition<TItem, TId, TUpdateArgs>;
	remove?: EntityRemoveDefinition<TId>;
};

/**
 * Create capability that only exists when `create` is configured.
 *
 * Behavior:
 * - runs the create mutation
 * - reconciles the cache according to the configured `after` policy
 * - returns the authoritative created entity
 */
export interface EntityStoreCreateCapability<TItem, TCreateArgs> {
	create(args: TCreateArgs): Promise<TItem>;
}

/**
 * Update capability that only exists when `update` is configured.
 *
 * Behavior:
 * - runs the update mutation for one entity
 * - reconciles the cache according to the configured `after` policy
 * - returns the authoritative updated entity
 */
export interface EntityStoreUpdateCapability<
	TItem,
	TId extends EntityKey,
	TUpdateArgs,
> {
	update(id: TId, args: TUpdateArgs): Promise<TItem>;
}

/**
 * Delete capability that only exists when `remove` is configured.
 *
 * Behavior:
 * - runs the delete mutation for one entity
 * - reconciles the cache according to the configured `after` policy
 */
export interface EntityStoreRemoveCapability<TId extends EntityKey> {
	remove(id: TId): Promise<void>;
}

export type EmptyCapability = Record<never, never>;

export type EntityStoreFromArgs<
	TItem,
	TId extends EntityKey,
	TArgs extends CreateEntityStoreArgs<TItem, TId, never, never>,
> = EntityStoreBase<TItem> &
	(TArgs extends { indexed: EntityIndexedDefinition<TItem, TId> }
		? EntityStoreIndexedCapability<TItem, TId>
		: EmptyCapability) &
	(TArgs extends {
		create: EntityCreateDefinition<TItem, infer TCreateArgs, TId>;
	}
		? EntityStoreCreateCapability<TItem, TCreateArgs>
		: EmptyCapability) &
	(TArgs extends {
		update: EntityUpdateDefinition<TItem, TId, infer TUpdateArgs>;
	}
		? EntityStoreUpdateCapability<TItem, TId, TUpdateArgs>
		: EmptyCapability) &
	(TArgs extends { remove: EntityRemoveDefinition<TId> }
		? EntityStoreRemoveCapability<TId>
		: EmptyCapability);

/**
 * Draft factory signature for a SWR-style entity store.
 *
 * Intended behavior:
 * - the returned store always supports collection reads through `all()`
 * - indexed access only exists when `indexed` is configured
 * - the returned store shape depends on which mutation definitions are passed
 * - collection and item access are stale-while-refresh by default
 * - cached data remains readable while background refresh is in flight
 * - explicit mutation methods only exist when configured
 *
 * This declaration mirrors the runtime implementation in
 * `create-entity-store.svelte.ts`.
 */
export declare function createEntityStore<
	TItem,
	TId extends EntityKey = never,
	TArgs extends CreateEntityStoreArgs<TItem, TId, never, never> =
		CreateEntityStoreArgs<TItem, TId, never, never>,
>(args: TArgs): EntityStoreFromArgs<TItem, TId, TArgs>;
