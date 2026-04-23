import { SvelteMap } from "svelte/reactivity";

import type { AsyncStatus } from "$lib/shell-types";

const RETRY_INITIAL_DELAY_MS = 1_000;
const RETRY_MAX_DELAY_MS = 30_000;

function toErrorMessage(error: unknown): string {
	return error instanceof Error ? error.message : "Failed to load resource";
}

export type RetryPolicy = {
	mode?: "none" | "background";
	initialDelayMs?: number;
	maxDelayMs?: number;
};

type CreateRetrySchedulerArgs = {
	owner: string;
	enabled: () => boolean;
	retry?: RetryPolicy;
};

type CreateResourceArgs<TData> = {
	owner: string;
	enabled: () => boolean;
	load: () => Promise<TData>;
	createEmptyValue: () => TData;
	staleAfterMs?: number;
	retry?: RetryPolicy;
};

export type ResourceState<TData> = {
	data: TData;
	status: AsyncStatus;
	error: string | null;
	isRefreshing: boolean;
	isStale: boolean;
	fetchedAt: number | null;
	ensure: () => Promise<TData>;
	refresh: () => Promise<TData>;
	invalidate: () => void;
	reset: () => void;
};

export function createRetryScheduler(args: CreateRetrySchedulerArgs) {
	const retryTimers = new SvelteMap<string, ReturnType<typeof setTimeout>>();
	const retryAttempts = new SvelteMap<string, number>();

	function clear(key: string) {
		const existingTimer = retryTimers.get(key);
		if (existingTimer !== undefined) {
			clearTimeout(existingTimer);
			retryTimers.delete(key);
		}
		retryAttempts.delete(key);
	}

	function dispose() {
		for (const timer of retryTimers.values()) {
			clearTimeout(timer);
		}
		retryTimers.clear();
		retryAttempts.clear();
	}

	function schedule(key: string, action: () => Promise<void>) {
		if (
			args.retry?.mode !== "background" ||
			retryTimers.has(key) ||
			!args.enabled()
		) {
			return;
		}
		const attempt = (retryAttempts.get(key) ?? 0) + 1;
		retryAttempts.set(key, attempt);
		const baseDelay = args.retry?.initialDelayMs ?? RETRY_INITIAL_DELAY_MS;
		const maxDelay = args.retry?.maxDelayMs ?? RETRY_MAX_DELAY_MS;
		const delay = Math.min(baseDelay * 2 ** (attempt - 1), maxDelay);
		const timer = setTimeout(() => {
			retryTimers.delete(key);
			void run(key, action);
		}, delay);
		retryTimers.set(key, timer);
	}

	async function run(key: string, action: () => Promise<void>) {
		if (!args.enabled()) {
			clear(key);
			return;
		}
		try {
			await action();
			clear(key);
		} catch (error) {
			console.warn(`[${args.owner}] Failed to load ${key}`, error);
			schedule(key, action);
		}
	}

	return {
		run,
		clear,
		dispose,
	};
}

export function createResource<TData>(
	args: CreateResourceArgs<TData>,
): ResourceState<TData> {
	let data = $state<TData>(args.createEmptyValue());
	let status = $state<AsyncStatus>("idle");
	let error = $state<string | null>(null);
	let isRefreshing = $state(false);
	let fetchedAt = $state<number | null>(null);
	let staleAt = $state<number | null>(null);
	let invalidatedAt = $state<number | null>(null);
	let loadingPromise = $state<Promise<TData> | null>(null);
	let ensureScheduled = false;
	let retryTimer = $state<ReturnType<typeof setTimeout> | null>(null);
	let retryAttempt = $state(0);

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
			!args.enabled()
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
			!args.enabled() ||
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
		data = args.createEmptyValue();
		status = "idle";
		error = null;
		isRefreshing = false;
		fetchedAt = null;
		staleAt = null;
		invalidatedAt = null;
		loadingPromise = null;
	}

	function isStale() {
		if (!args.enabled()) {
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
		if (!args.enabled()) {
			return;
		}
		invalidatedAt = Date.now();
	}

	async function ensure(force = false): Promise<TData> {
		if (!args.enabled()) {
			reset();
			return data;
		}
		if (!force && !isStale()) {
			return data;
		}
		if (loadingPromise) {
			return loadingPromise;
		}

		clearRetry(false);

		const hasData = fetchedAt !== null;
		if (hasData) {
			isRefreshing = true;
		} else {
			status = "loading";
		}

		const promise = args
			.load()
			.then((nextData) => {
				data = nextData;
				status = "ready";
				error = null;
				fetchedAt = Date.now();
				staleAt =
					args.staleAfterMs !== undefined
						? fetchedAt + args.staleAfterMs
						: null;
				invalidatedAt = null;
				clearRetry();
				return nextData;
			})
			.catch((nextError) => {
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
			});

		loadingPromise = promise;
		return promise;
	}

	$effect(() => {
		const enabled = args.enabled();
		if (!enabled) {
			reset();
		}
	});

	return {
		get data() {
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
		ensure: () => ensure(false),
		refresh: () => ensure(true),
		invalidate,
		reset,
	};
}
