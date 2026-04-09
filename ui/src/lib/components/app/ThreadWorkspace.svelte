<script lang="ts">
	import ConversationComposerSessionSetupStatus from "$lib/components/app/ConversationComposerSessionSetupStatus.svelte";
	import ThreadWorkspaceHeader from "$lib/components/app/parts/ThreadWorkspaceHeader.svelte";
	import ThreadWorkspaceActive from "$lib/components/app/ThreadWorkspaceActive.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";

	type Props = {
		threadId: string;
		visible: boolean;
		mainClass: string;
		showSidebarToggle?: boolean;
		reserveSidebarSpace?: boolean;
		onToggleSidebar?: () => void;
		mode?: "full" | "conversation-only";
		sidebarOpen?: boolean;
	};

	const props: Props = $props();
	const noop = () => {};
	const session = useSessionContext();
	const hasSelectedThread = $derived.by(
		() => session.isPending || session.threads.selectedId !== null,
	);
	const sandboxReady = $derived.by(
		() => !session.isPending && session.current?.status === "ready",
	);
	const showThreadSelectionPrompt = $derived.by(
		() => !hasSelectedThread && sandboxReady,
	);
</script>

<main class={props.mainClass}>
	{#if hasSelectedThread}
		<ThreadWorkspaceActive
			visible={props.visible}
			showSidebarToggle={props.showSidebarToggle}
			reserveSidebarSpace={props.reserveSidebarSpace}
			onToggleSidebar={props.onToggleSidebar}
			mode={props.mode}
		/>
	{:else}
		<ThreadWorkspaceHeader
			showSidebarToggle={props.showSidebarToggle ?? false}
			reserveSidebarSpace={props.reserveSidebarSpace ?? false}
			onToggleSidebar={props.onToggleSidebar ?? noop}
			title="No thread selected"
		/>
		{#if showThreadSelectionPrompt}
			<div class="flex min-h-0 min-w-0 flex-1 items-center justify-center p-6">
				<p class="text-sm text-muted-foreground">
					Select a thread to continue.
				</p>
			</div>
		{:else}
			<div class="flex min-h-0 min-w-0 flex-1 items-center justify-center p-6">
				<div class="w-full max-w-sm">
					<ConversationComposerSessionSetupStatus />
				</div>
			</div>
		{/if}
	{/if}
</main>
