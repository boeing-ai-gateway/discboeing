export type ResourceStatusState =
	| "idle"
	| "loading"
	| "ready"
	| "refreshing"
	| "missing"
	| "error";

export type ResourceStatus = {
	state: ResourceStatusState;
	error?: string;
	lastLoadedAt?: number;
	refreshingSince?: number;
};

export type CollectionCache<T> = {
	byId: Record<string, T>;
	allIds: string[];
	status: ResourceStatus;
};

export function createIdleStatus(): ResourceStatus {
	return { state: "idle" };
}

export function createLoadingStatus(): ResourceStatus {
	return { state: "loading" };
}

export function createReadyStatus(now = Date.now()): ResourceStatus {
	return { state: "ready", lastLoadedAt: now };
}

export function createRefreshingStatus(now = Date.now()): ResourceStatus {
	return { state: "refreshing", refreshingSince: now };
}

export function createMissingStatus(now = Date.now()): ResourceStatus {
	return { state: "missing", lastLoadedAt: now };
}

export function createErrorStatus(error: unknown): ResourceStatus {
	return {
		state: "error",
		error: error instanceof Error ? error.message : String(error),
	};
}

export function createCollectionCache<T>(): CollectionCache<T> {
	return {
		byId: {},
		allIds: [],
		status: createIdleStatus(),
	};
}

export function upsertById<T>(
	cache: CollectionCache<T>,
	id: string,
	value: T,
): void {
	cache.byId[id] = value;
	if (!cache.allIds.includes(id)) {
		cache.allIds.push(id);
	}
}

export function removeById<T>(cache: CollectionCache<T>, id: string): void {
	delete cache.byId[id];
	cache.allIds = cache.allIds.filter((itemId) => itemId !== id);
}
