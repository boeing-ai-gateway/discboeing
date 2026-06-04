<script lang="ts">
	import { tick } from "svelte";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";

	type Props = {
		open: boolean;
		query: string;
		activeMatch: number;
		matchCount: number;
		onQueryChange: (query: string) => void;
		onNext: () => void;
		onPrevious: () => void;
		onClose: () => void;
	};

	let {
		open,
		query,
		activeMatch,
		matchCount,
		onQueryChange,
		onNext,
		onPrevious,
		onClose,
	}: Props = $props();

	let inputElement = $state<HTMLInputElement | null>(null);
	const matchStatus = $derived.by(() => {
		if (!query) {
			return "";
		}
		if (matchCount === 0) {
			return "No results";
		}
		return `${activeMatch} of ${matchCount}`;
	});

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === "Escape") {
			event.preventDefault();
			onClose();
			return;
		}
		if (event.key !== "Enter") {
			return;
		}

		event.preventDefault();
		if (event.shiftKey) {
			onPrevious();
			return;
		}
		onNext();
	}

	$effect(() => {
		if (!open) {
			return;
		}

		void tick().then(() => {
			inputElement?.focus();
			inputElement?.select();
		});
	});
</script>

{#if open}
	<div
		class="fixed right-4 top-4 z-50 flex items-center gap-1 rounded-lg border border-border bg-background p-2 shadow-lg"
		role="search"
		aria-label="Find in page"
	>
		<Input
			bind:ref={inputElement}
			value={query}
			type="search"
			placeholder="Find in page"
			aria-label="Find in page"
			class="h-8 w-56"
			oninput={(event) => onQueryChange(event.currentTarget.value)}
			onkeydown={handleKeydown}
		/>
		<div class="min-w-16 px-2 text-right text-xs text-muted-foreground">
			{matchStatus}
		</div>
		<Button
			variant="ghost"
			size="icon-xs"
			disabled={!query}
			aria-label="Previous match"
			onclick={onPrevious}
		>
			↑
		</Button>
		<Button
			variant="ghost"
			size="icon-xs"
			disabled={!query}
			aria-label="Next match"
			onclick={onNext}
		>
			↓
		</Button>
		<Button
			variant="ghost"
			size="icon-xs"
			aria-label="Close find"
			onclick={onClose}
		>
			×
		</Button>
	</div>
{/if}
