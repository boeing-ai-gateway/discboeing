export type RetryBackoffOptions = {
	initialDelayMs?: number;
	maxDelayMs?: number;
	factor?: number;
};

const DEFAULT_INITIAL_DELAY_MS = 500;
const DEFAULT_MAX_DELAY_MS = 10_000;
const DEFAULT_FACTOR = 2;

export async function retryUntilSuccess(
	work: () => Promise<void>,
	options: RetryBackoffOptions = {},
): Promise<void> {
	let delayMs = options.initialDelayMs ?? DEFAULT_INITIAL_DELAY_MS;
	const maxDelayMs = options.maxDelayMs ?? DEFAULT_MAX_DELAY_MS;
	const factor = options.factor ?? DEFAULT_FACTOR;

	for (;;) {
		try {
			await work();
			return;
		} catch {
			await sleep(delayMs);
			delayMs = Math.min(maxDelayMs, Math.ceil(delayMs * factor));
		}
	}
}

function sleep(delayMs: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, delayMs));
}
