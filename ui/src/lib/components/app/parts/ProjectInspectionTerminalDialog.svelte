<script lang="ts">
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import TerminalIcon from "@lucide/svelte/icons/terminal";
	import { createProjectInspectionTerminal } from "$lib/components/app/project-inspection-terminal.svelte";
	import * as Dialog from "$lib/components/ui/dialog";
	import { Button } from "$lib/components/ui/button";

	type Props = {
		open: boolean;
		onOpenChange: (open: boolean) => void;
		projectId: string;
		providerId?: string;
		title?: string;
		description?: string;
	};

	let {
		open,
		onOpenChange,
		projectId,
		providerId,
		title = "Inspection shell",
		description = "Open a troubleshooting shell in the inspection container.",
	}: Props = $props();

	let terminalHost = $state<HTMLDivElement | null>(null);
	const terminal = createProjectInspectionTerminal({
		open: () => open,
		projectId: () => projectId,
		providerId: () => providerId,
		terminalHost: () => terminalHost,
	});
</script>

<Dialog.Root {open} {onOpenChange}>
	<Dialog.Content
		class="fixed inset-x-0 bottom-0 top-10 z-50 flex h-auto w-screen max-w-none translate-x-0 translate-y-0 flex-col overflow-hidden rounded-none border-0 p-0 shadow-none sm:max-w-none"
	>
		<div
			class="flex items-center justify-between border-b border-border px-5 py-4"
		>
			<div class="min-w-0">
				<Dialog.Title class="flex items-center gap-2 text-sm">
					<TerminalIcon class="size-4" />
					<span class="truncate">{title}</span>
				</Dialog.Title>
				<Dialog.Description class="mt-1 text-xs">
					{description}
				</Dialog.Description>
			</div>
			<div class="flex items-center gap-2 pr-8 text-xs text-muted-foreground">
				<div class={`size-2 rounded-full ${terminal.statusClass}`}></div>
				<span class="capitalize">{terminal.connectionStatus}</span>
			</div>
		</div>

		<div class="relative min-h-0 flex-1 overflow-hidden p-4">
			{#if terminal.connectionStatus !== "connected"}
				<div
					class="absolute inset-4 z-10 flex items-center justify-center rounded-md bg-background/80 backdrop-blur-sm"
				>
					<div class="flex flex-col items-center gap-3 text-center">
						<span class="text-xs text-muted-foreground">
							{terminal.overlayMessage}
						</span>
						{#if terminal.connectionStatus !== "connecting"}
							<Button
								variant="outline"
								size="xs"
								onclick={terminal.reconnectTerminal}
								class="gap-2"
							>
								<RotateCcwIcon class="size-3.5" />
								Reconnect
							</Button>
						{/if}
					</div>
				</div>
			{/if}

			{#if open}
				<div
					bind:this={terminalHost}
					class="h-full w-full cursor-text overflow-hidden rounded-md border border-border bg-terminal-bg p-3 outline-none [caret-color:transparent]"
				></div>
			{/if}
		</div>
	</Dialog.Content>
</Dialog.Root>
