<script lang="ts">
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import { untrack } from "svelte";
	import {
		canLoadSessionThreads,
		isSessionTransitioningStatus,
	} from "$lib/api-constants";
	import ConversationComposerSessionSetupStatus from "$lib/components/app/ConversationComposerSessionSetupStatus.svelte";
	import ThreadWorkspaceHeader from "$lib/components/app/parts/ThreadWorkspaceHeader.svelte";
	import ThreadWorkspaceActive from "$lib/components/app/ThreadWorkspaceActive.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";
	import { setThreadContext } from "$lib/context/thread-context.svelte";

	type Props = {
		threadId: string;
		visible: boolean;
		mainClass: string;
		reserveSidebarSpace?: boolean;
		mode?: "full" | "conversation-only";
		sidebarOpen?: boolean;
	};

	let {
		threadId,
		visible,
		mainClass,
		reserveSidebarSpace,
		mode,
		sidebarOpen,
	}: Props = $props();
	const session = useSessionContext();
	const thread = session.ensureThread(untrack(() => threadId));
	setThreadContext(thread);
	const canLoadThreadData = $derived.by(
		() => !session.isPending && canLoadSessionThreads(session.current?.status),
	);
	const hasSelectedThread = $derived.by(
		() =>
			session.isPending ||
			(canLoadThreadData && session.threads.selectedId !== null),
	);
	const hasConversationMessages = $derived.by(() => thread.messages.length > 0);
	const showActiveConversation = $derived.by(
		() => hasSelectedThread || hasConversationMessages,
	);
	const isLoadingThread = $derived.by(
		() =>
			!showActiveConversation &&
			!session.isPending &&
			(isSessionTransitioningStatus(session.current?.status) ||
				session.threads.status === "loading"),
	);
	const showThreadSelectionPrompt = $derived.by(
		() => !isLoadingThread && !showActiveConversation && canLoadThreadData,
	);
</script>

<main class={mainClass}>
	{#if showActiveConversation}
		<ThreadWorkspaceActive {visible} {reserveSidebarSpace} {mode} />
	{:else}
		<ThreadWorkspaceHeader
			reserveSidebarSpace={reserveSidebarSpace ?? false}
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
						<div
							class="flex items-center gap-2 px-1 text-sm text-muted-foreground"
						>
							<Loader2Icon class="size-4 animate-spin" />
							<p>Loading the selected thread while the session starts.</p>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
</main>
