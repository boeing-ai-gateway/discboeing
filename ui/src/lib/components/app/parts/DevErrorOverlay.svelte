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
	let copiedId = $state<number | null>(null);
	let copyResetTimeout: ReturnType<typeof setTimeout> | undefined;

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

	function formatError(error: DevError) {
		return [error.title, error.message, error.stack]
			.filter(Boolean)
			.join("\n\n");
	}

	async function copyError(error: DevError) {
		const text = formatError(error);

		try {
			if (navigator.clipboard?.writeText) {
				await navigator.clipboard.writeText(text);
			} else {
				const textarea = document.createElement("textarea");
				textarea.value = text;
				textarea.style.position = "fixed";
				textarea.style.opacity = "0";
				document.body.append(textarea);
				textarea.select();
				document.execCommand("copy");
				textarea.remove();
			}

			copiedId = error.id;
			if (copyResetTimeout) {
				clearTimeout(copyResetTimeout);
			}
			copyResetTimeout = setTimeout(() => {
				copiedId = null;
			}, 1500);
		} catch {
			copiedId = null;
		}
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
			if (copyResetTimeout) {
				clearTimeout(copyResetTimeout);
			}
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
		class="fixed right-4 bottom-4 z-[99999] flex max-h-[80vh] w-[min(40rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-lg border border-destructive/70 bg-background text-foreground shadow-2xl"
	>
		<div
			class="flex items-center justify-between gap-4 border-b border-destructive/30 bg-destructive/10 px-4 py-3"
		>
			<div>
				<p class="text-sm font-semibold text-destructive">Development errors</p>
				<p class="text-xs text-muted-foreground">
					Showing the latest {MAX_ERRORS} console and global errors.
				</p>
			</div>
			<button
				type="button"
				class="rounded border border-border px-2 py-1 text-xs font-medium hover:bg-destructive/10"
				onclick={clear}
			>
				Clear
			</button>
		</div>

		<div class="overflow-auto p-3">
			{#each errors as error (error.id)}
				<div
					class="mb-3 rounded-md border border-destructive/30 bg-destructive/5 p-3 last:mb-0"
				>
					<div class="mb-2 flex items-start justify-between gap-3">
						<div>
							<p
								class="text-xs font-semibold tracking-wide text-destructive uppercase"
							>
								{error.title}
							</p>
							<pre
								class="mt-1 whitespace-pre-wrap break-words font-mono text-xs">{error.message}</pre>
						</div>
						<div class="flex shrink-0 gap-2">
							<button
								type="button"
								class="rounded border border-border px-2 py-1 text-xs hover:bg-destructive/10"
								onclick={() => copyError(error)}
							>
								{copiedId === error.id ? "Copied" : "Copy"}
							</button>
							<button
								type="button"
								class="rounded border border-border px-2 py-1 text-xs hover:bg-destructive/10"
								onclick={() => dismiss(error.id)}
							>
								Dismiss
							</button>
						</div>
					</div>

					{#if error.stack}
						<pre
							class="mt-2 whitespace-pre-wrap break-words border-t border-destructive/20 pt-2 font-mono text-[11px] text-muted-foreground">{error.stack}</pre>
					{/if}
				</div>
			{/each}
		</div>
	</div>
{/if}
