<script lang="ts">
	import TerminalIcon from "@lucide/svelte/icons/terminal";

	import type { AgentCommand } from "$lib/api-types";

	type Props = {
		sessionId: string | null;
		textareaRef: HTMLTextAreaElement | null;
		commands: AgentCommand[];
		commandsLoading: boolean;
		onDraftChange: (value: string) => void;
	};

	let {
		sessionId,
		textareaRef,
		commands,
		commandsLoading,
		onDraftChange,
	}: Props = $props();

	let open = $state(false);
	let query = $state("");
	let selectedIndex = $state(0);
	let dropdownRef = $state<HTMLDivElement | null>(null);

	const suggestions = $derived.by(() => {
		const normalizedQuery = query.trim().toLowerCase();
		return [...commands]
			.sort((left, right) => {
				const leftOrder = left.discobot?.order ?? 0;
				const rightOrder = right.discobot?.order ?? 0;
				if (leftOrder !== rightOrder) {
					return leftOrder - rightOrder;
				}
				return left.name.localeCompare(right.name);
			})
			.filter((command) => {
				if (normalizedQuery.length === 0) {
					return true;
				}
				const name = command.name.toLowerCase();
				const description = command.description.toLowerCase();
				return (
					name.includes(normalizedQuery) ||
					description.includes(normalizedQuery)
				);
			});
	});
	const selectedSuggestionIndex = $derived(
		suggestions.length === 0
			? 0
			: Math.min(selectedIndex, suggestions.length - 1),
	);
	const showEmpty = $derived.by(
		() => open && !!sessionId && !commandsLoading && suggestions.length === 0,
	);
	const shouldRender = $derived.by(
		() =>
			open &&
			!!sessionId &&
			(commandsLoading || suggestions.length > 0 || showEmpty),
	);

	function updateSlashCommandState(value: string, cursor: number) {
		const beforeCursor = value.slice(0, cursor);
		const match = beforeCursor.match(/^\/([^\s/]*)$/);
		if (!match) {
			open = false;
			return;
		}

		query = match[1] ?? "";
		open = true;
		selectedIndex = 0;
	}

	function selectCommand(command: AgentCommand) {
		const textarea = textareaRef;
		if (!textarea) {
			return;
		}

		textarea.setRangeText(`/${command.name} `, 0, textarea.value.length, "end");
		onDraftChange(textarea.value);
		open = false;
		textarea.focus();
	}

	export function handleInput(value: string, cursor: number) {
		updateSlashCommandState(value, cursor);
	}

	export function handleKeydown(event: KeyboardEvent) {
		if (!open) {
			return false;
		}

		if (suggestions.length > 0) {
			if (event.key === "ArrowDown") {
				event.preventDefault();
				selectedIndex = Math.min(
					selectedSuggestionIndex + 1,
					suggestions.length - 1,
				);
				return true;
			}
			if (event.key === "ArrowUp") {
				event.preventDefault();
				selectedIndex = Math.max(selectedSuggestionIndex - 1, 0);
				return true;
			}
			if (event.key === "Enter" || event.key === "Tab") {
				event.preventDefault();
				const selected = suggestions[selectedSuggestionIndex];
				if (selected) {
					selectCommand(selected);
				}
				return true;
			}
		}

		if (event.key === "Escape") {
			event.preventDefault();
			open = false;
			return true;
		}

		return false;
	}

	export function closeDropdown() {
		open = false;
	}

	$effect(() => {
		if (!open || !dropdownRef) {
			return;
		}

		const selectedItem = dropdownRef.querySelector(
			`[data-index="${selectedSuggestionIndex}"]`,
		);
		if (selectedItem && "scrollIntoView" in selectedItem) {
			(selectedItem as HTMLElement).scrollIntoView({ block: "nearest" });
		}
	});

	$effect(() => {
		if (!open) {
			return;
		}

		const handlePointerDown = (event: MouseEvent) => {
			const target = event.target as Node;
			if (dropdownRef?.contains(target) || textareaRef?.contains(target)) {
				return;
			}
			open = false;
		};

		document.addEventListener("mousedown", handlePointerDown);
		return () => {
			document.removeEventListener("mousedown", handlePointerDown);
		};
	});
</script>

{#if shouldRender}
	<div
		bind:this={dropdownRef}
		class="absolute bottom-full left-0 right-0 z-50 mb-1 flex max-h-64 flex-col overflow-hidden rounded-lg border border-border bg-popover shadow-lg"
	>
		<div
			class="sticky top-0 z-10 flex items-center gap-2 border-b border-border bg-popover px-3 py-2"
		>
			<TerminalIcon class="size-4 text-muted-foreground" />
			<span class="text-xs font-medium text-muted-foreground">Commands</span>
			<span class="ml-auto text-xs text-muted-foreground"
				>↑/↓ navigate · Tab to select</span
			>
		</div>

		{#if commandsLoading}
			<div class="px-3 py-3 text-xs text-muted-foreground">
				Loading commands...
			</div>
		{:else if showEmpty}
			<div class="px-3 py-3 text-xs text-muted-foreground">
				{query.length === 0
					? "No commands available"
					: `No commands match “${query}”`}
			</div>
		{:else}
			<div class="overflow-y-auto py-1">
				{#each suggestions as command, index (command.name)}
					<button
						type="button"
						data-index={index}
						class={`flex w-full items-start gap-2 px-3 py-2 text-left text-sm transition-colors ${index === selectedSuggestionIndex ? "bg-accent" : "hover:bg-accent"}`}
						onmousedown={(event) => {
							event.preventDefault();
						}}
						onclick={() => selectCommand(command)}
					>
						<TerminalIcon
							class="mt-0.5 size-3.5 shrink-0 text-muted-foreground"
						/>
						<div class="min-w-0 flex-1">
							<div class="truncate font-mono text-xs">/{command.name}</div>
							<div class="line-clamp-2 text-xs text-muted-foreground">
								{command.description}
							</div>
						</div>
					</button>
				{/each}
			</div>
		{/if}
	</div>
{/if}
