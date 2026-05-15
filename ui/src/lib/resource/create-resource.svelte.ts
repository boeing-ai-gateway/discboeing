import type { AsyncStatus } from "../resource/types";

const RETRY_INITIAL_DELAY_MS = 1_000;
const RETRY_MAX_DELAY_MS = 30_000;

function toErrorMessage(error: unknown): string {
	return error instanceof Error ? error.message : "Failed to load resource";
}

function now() {
	return typeof performance !== "undefined" ? performance.now() : Date.now();
}

export type RetryPolicy = {
	mode?: "none" | "background";
	initialDelayMs?: number;
	maxDelayMs?: number;
};

type CreateResourceArgs<TData> = {
	owner: string;
	enabled?: () => boolean;
	load: () => Promise<TData>;
	createEmptyValue: () => TData;
	staleAfterMs?: number;
	retry?: RetryPolicy;
};

export type ResourceMutationOptions = {
	markFresh?: boolean;
	freshAt?: number;
	clearError?: boolean;
	setReady?: boolean;
};

export type ResourceState<TData> = {
	data: TData;
	status: AsyncStatus;
	error: string | null;
	isRefreshing: boolean;
	isStale: boolean;
	fetchedAt: number | null;
	peek: () => TData;
	ensure: () => Promise<TData>;
	refresh: () => Promise<void>;
	invalidate: () => void;
	reset: () => void;
	setData: (nextData: TData, options?: ResourceMutationOptions) => void;
	update: (
		updater: (currentData: TData) => TData,
		options?: ResourceMutationOptions,
	) => void;
};

export function createResource<TData>(
	args: CreateResourceArgs<TData>,
): ResourceState<TData> {
	const enabled = args.enabled ?? (() => true);
	let data = $state<TData>(args.createEmptyValue());
	let status = $state<AsyncStatus>("idle");
	let error = $state<string | null>(null);
	let isRefreshing = $state(false);
	let fetchedAt = $state<number | null>(null);
	let staleAt = $state<number | null>(null);
	let invalidatedAt = $state<number | null>(null);
	let loadingPromise = $state<Promise<TData> | null>(null);
	let queuedPromise = $state<Promise<TData> | null>(null);
	let queuedForce = false;
	let resolveQueued: ((value: TData | PromiseLike<TData>) => void) | null =
		null;
	let rejectQueued: ((reason?: unknown) => void) | null = null;
	let ensureScheduled = false;
	let retryTimer = $state<ReturnType<typeof setTimeout> | null>(null);
	let retryAttempt = $state(0);

	function syncStaleAt(nextFetchedAt: number | null) {
		staleAt =
			nextFetchedAt !== null && args.staleAfterMs !== undefined
				? nextFetchedAt + args.staleAfterMs
				: null;
	}

	function applyData(nextData: TData, options: ResourceMutationOptions = {}) {
		if (
			options.markFresh &&
			options.freshAt !== undefined &&
			fetchedAt !== null &&
			options.freshAt < fetchedAt
		) {
			return;
		}
		data = nextData;
		if (options.setReady ?? true) {
			status = "ready";
		}
		if (options.clearError ?? true) {
			error = null;
		}
		if (options.markFresh) {
			const freshAt = options.freshAt ?? now();
			fetchedAt = freshAt;
			invalidatedAt =
				invalidatedAt !== null && invalidatedAt > freshAt
					? invalidatedAt
					: null;
			syncStaleAt(fetchedAt);
		}
	}

	function clearRetry(resetAttempts = true) {
		if (retryTimer !== null) {
			clearTimeout(retryTimer);
			retryTimer = null;
		}
		if (resetAttempts) {
			retryAttempt = 0;
		}
	}

	function scheduleRetry(force: boolean) {
		if (
			args.retry?.mode !== "background" ||
			retryTimer !== null ||
			!enabled()
		) {
			return;
		}
		retryAttempt += 1;
		const baseDelay = args.retry?.initialDelayMs ?? RETRY_INITIAL_DELAY_MS;
		const maxDelay = args.retry?.maxDelayMs ?? RETRY_MAX_DELAY_MS;
		const delay = Math.min(baseDelay * 2 ** (retryAttempt - 1), maxDelay);
		retryTimer = setTimeout(() => {
			retryTimer = null;
			void ensure(force).catch(() => {});
		}, delay);
	}

	function scheduleEnsure(force = false) {
		if (
			ensureScheduled ||
			!enabled() ||
			loadingPromise !== null ||
			(!force && !isStale())
		) {
			return;
		}
		ensureScheduled = true;
		queueMicrotask(() => {
			ensureScheduled = false;
			void ensure(force).catch(() => {});
		});
	}

	function reset() {
		clearRetry();
		ensureScheduled = false;
		queuedPromise = null;
		queuedForce = false;
		resolveQueued = null;
		rejectQueued = null;
		data = args.createEmptyValue();
		status = "idle";
		error = null;
		isRefreshing = false;
		fetchedAt = null;
		staleAt = null;
		invalidatedAt = null;
		loadingPromise = null;
	}

	function settleDisabledRequest() {
		if (fetchedAt === null) {
			status = "idle";
			error = null;
		}
		return data;
	}

	function isStale() {
		if (!enabled()) {
			return false;
		}
		if (fetchedAt === null) {
			return true;
		}
		if (invalidatedAt !== null && invalidatedAt > fetchedAt) {
			return true;
		}
		if (staleAt !== null && Date.now() >= staleAt) {
			return true;
		}
		return false;
	}

	function invalidate() {
		if (!enabled()) {
			return;
		}
		invalidatedAt = now();
	}

	async function ensure(force = false): Promise<TData> {
		if (!enabled()) {
			return settleDisabledRequest();
		}
		if (!force && !isStale()) {
			return data;
		}
		if (loadingPromise) {
			if (!force) {
				return loadingPromise;
			}
			queuedForce = true;
			if (queuedPromise === null) {
				queuedPromise = new Promise<TData>((resolve, reject) => {
					resolveQueued = resolve;
					rejectQueued = reject;
				});
			}
			return queuedPromise;
		}

		clearRetry(false);

		const hasData = fetchedAt !== null;
		const startedAt = now();
		if (hasData) {
			isRefreshing = true;
		} else {
			status = "loading";
		}

		const promise = args
			.load()
			.then((nextData) => {
				if (!enabled()) {
					return settleDisabledRequest();
				}
				applyData(nextData, { markFresh: true, freshAt: startedAt });
				clearRetry();
				return nextData;
			})
			.catch((nextError) => {
				if (!enabled()) {
					return settleDisabledRequest();
				}
				error = toErrorMessage(nextError);
				if (!hasData) {
					status = "error";
				}
				console.warn(`[${args.owner}] Failed to load resource`, nextError);
				scheduleRetry(force);
				throw nextError;
			})
			.finally(() => {
				isRefreshing = false;
				loadingPromise = null;
				if (queuedForce) {
					const nextResolve = resolveQueued;
					const nextReject = rejectQueued;
					queuedForce = false;
					queuedPromise = null;
					resolveQueued = null;
					rejectQueued = null;
					void ensure(true)
						.then(nextResolve ?? (() => {}))
						.catch(nextReject ?? (() => {}));
				}
			});

		loadingPromise = promise;
		return promise;
	}

	return {
		get data() {
			if (!enabled()) {
				return data;
			}
			scheduleEnsure(false);
			return data;
		},
		get status() {
			return status;
		},
		get error() {
			return error;
		},
		get isRefreshing() {
			return isRefreshing;
		},
		get isStale() {
			return isStale();
		},
		get fetchedAt() {
			return fetchedAt;
		},
		peek: () => data,
		ensure: () => ensure(false),
		refresh: async () => {
			await ensure(true).catch(() => undefined);
		},
		invalidate,
		reset,
		setData: applyData,
		update: (updater, options) => {
			applyData(updater(data), options);
		},
	};
}
