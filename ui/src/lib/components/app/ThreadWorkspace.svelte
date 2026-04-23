<script lang="ts">
	import ConversationComposerSessionSetupStatus from "$lib/components/app/ConversationComposerSessionSetupStatus.svelte";
	import ThreadWorkspaceHeader from "$lib/components/app/parts/ThreadWorkspaceHeader.svelte";
	import ThreadWorkspaceActive from "$lib/components/app/ThreadWorkspaceActive.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";

	type Props = {
		threadId: string;
		visible: boolean;
		mainClass: string;
		reserveSidebarSpace?: boolean;
		mode?: "full" | "conversation-only";
		sidebarOpen?: boolean;
	};

	const props: Props = $props();
	const session = useSessionContext();
	const hasSelectedThread = $derived.by(
		() => session.isPending || session.threads.selectedId !== null,
	);
	const sandboxReady = $derived.by(
		() => !session.isPending && session.current?.status === "ready",
	);
	const isLoadingThread = $derived.by(
		() => !session.isPending && !hasSelectedThread && !sandboxReady,
	);
	const showThreadSelectionPrompt = $derived.by(
		() => !hasSelectedThread && sandboxReady,
	);
</script>

<main class={props.mainClass}>
	{#if hasSelectedThread}
		<ThreadWorkspaceActive
			visible={props.visible}
			reserveSidebarSpace={props.reserveSidebarSpace}
			mode={props.mode}
		/>
	{:else}
		<ThreadWorkspaceHeader
			reserveSidebarSpace={props.reserveSidebarSpace ?? false}
			title={isLoadingThread ? "Loading thread" : "No thread selected"}
		/>
		{#if showThreadSelectionPrompt}
			<div class="flex min-h-0 min-w-0 flex-1 items-center justify-center p-6">
				<p class="text-sm text-muted-foreground">
					Select a thread to continue.
				</p>
			</div>
		{:else}
			<div class="flex min-h-0 min-w-0 flex-1 items-center justify-center p-6">
				<div class="w-full max-w-sm space-y-3">
					<ConversationComposerSessionSetupStatus />
					{#if isLoadingThread}
						<p class="px-1 text-sm text-muted-foreground">
							Loading the selected thread while the session starts.
						</p>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
</main>
