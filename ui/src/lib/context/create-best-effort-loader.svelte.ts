import { SvelteMap } from "svelte/reactivity";

const BACKGROUND_RETRY_INITIAL_DELAY_MS = 1_000;
const BACKGROUND_RETRY_MAX_DELAY_MS = 30_000;

type CreateBestEffortLoaderArgs = {
	owner: string;
	isActive: () => boolean;
};

export function createBestEffortLoader(args: CreateBestEffortLoaderArgs) {
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
		if (retryTimers.has(key)) {
			return;
		}
		const attempt = (retryAttempts.get(key) ?? 0) + 1;
		retryAttempts.set(key, attempt);
		const delay = Math.min(
			BACKGROUND_RETRY_INITIAL_DELAY_MS * 2 ** (attempt - 1),
			BACKGROUND_RETRY_MAX_DELAY_MS,
		);
		const timer = setTimeout(() => {
			retryTimers.delete(key);
			void run(key, action);
		}, delay);
		retryTimers.set(key, timer);
	}

	async function run(key: string, action: () => Promise<void>) {
		if (!args.isActive()) {
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
