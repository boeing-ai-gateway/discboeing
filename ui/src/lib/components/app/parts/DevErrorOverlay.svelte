<script lang="ts">
	import { browser, dev } from "$app/environment";
	import { onMount } from "svelte";

	type DevError = {
		id: number;
		title: string;
		message: string;
		stack?: string;
	};

	const MAX_ERRORS = 20;

	let errors = $state<DevError[]>([]);
	let nextId = 1;

	function stringify(value: unknown) {
		if (value instanceof Error) {
			return {
				message: value.message,
				stack: value.stack,
			};
		}

		if (typeof value === "string") {
			return { message: value };
		}

		try {
			return { message: JSON.stringify(value, null, 2) };
		} catch {
			return { message: String(value) };
		}
	}

	function addError(title: string, value: unknown, stack?: string) {
		const details = stringify(value);
		errors = [
			...errors,
			{
				id: nextId,
				title,
				message: details.message,
				stack: stack ?? details.stack,
			},
		].slice(-MAX_ERRORS);
		nextId += 1;
	}

	function addConsoleError(args: unknown[]) {
		const parts = args.map((arg) => stringify(arg));
		addError(
			"Console error",
			parts.map((part) => part.message).join("\n"),
			parts.find((part) => part.stack)?.stack,
		);
	}

	function dismiss(id: number) {
		errors = errors.filter((error) => error.id !== id);
	}

	function clear() {
		errors = [];
	}

	onMount(() => {
		if (!dev || !browser) {
			return;
		}

		const originalConsoleError = console.error;
		const handleError = (event: ErrorEvent) => {
			addError(
				"Uncaught error",
				event.error ?? event.message,
				event.error?.stack,
			);
		};
		const handleUnhandledRejection = (event: PromiseRejectionEvent) => {
			addError("Unhandled promise rejection", event.reason);
		};

		console.error = (...args: unknown[]) => {
			originalConsoleError(...args);
			addConsoleError(args);
		};
		window.addEventListener("error", handleError);
		window.addEventListener("unhandledrejection", handleUnhandledRejection);

		return () => {
			console.error = originalConsoleError;
			window.removeEventListener("error", handleError);
			window.removeEventListener(
				"unhandledrejection",
				handleUnhandledRejection,
			);
		};
	});
</script>

{#if dev && errors.length > 0}
	<div
		class="fixed right-4 bottom-4 z-[99999] flex max-h-[80vh] w-[min(40rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-lg border border-red-500/70 bg-red-950 text-red-50 shadow-2xl"
	>
		<div
			class="flex items-center justify-between gap-4 border-b border-red-500/40 px-4 py-3"
		>
			<div>
				<p class="text-sm font-semibold">Development errors</p>
				<p class="text-xs text-red-100/80">
					Showing the latest {MAX_ERRORS} console and global errors.
				</p>
			</div>
			<button
				type="button"
				class="rounded border border-red-300/40 px-2 py-1 text-xs font-medium hover:bg-red-900"
				onclick={clear}
			>
				Clear
			</button>
		</div>

		<div class="overflow-auto p-3">
			{#each errors as error (error.id)}
				<div
					class="mb-3 rounded-md border border-red-500/40 bg-red-900/50 p-3 last:mb-0"
				>
					<div class="mb-2 flex items-start justify-between gap-3">
						<div>
							<p
								class="text-xs font-semibold tracking-wide text-red-100 uppercase"
							>
								{error.title}
							</p>
							<pre
								class="mt-1 whitespace-pre-wrap break-words font-mono text-xs">{error.message}</pre>
						</div>
						<button
							type="button"
							class="shrink-0 rounded border border-red-300/30 px-2 py-1 text-xs hover:bg-red-900"
							onclick={() => dismiss(error.id)}
						>
							Dismiss
						</button>
					</div>

					{#if error.stack}
						<pre
							class="mt-2 whitespace-pre-wrap break-words border-t border-red-500/30 pt-2 font-mono text-[11px] text-red-100/80">{error.stack}</pre>
					{/if}
				</div>
			{/each}
		</div>
	</div>
{/if}
