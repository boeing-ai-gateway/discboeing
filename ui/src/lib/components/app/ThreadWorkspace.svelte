<script lang="ts">
	import { onMount } from "svelte";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import {
		canLoadSessionThreads,
		isSessionTransitioningStatus,
	} from "$lib/api-constants";
	import ConversationComposerSessionSetupStatus from "$lib/components/app/ConversationComposerSessionSetupStatus.svelte";
	import SessionHeaderDropdown from "$lib/components/app/SessionHeaderDropdown.svelte";
	import ThreadWorkspaceHeader from "$lib/components/app/parts/ThreadWorkspaceHeader.svelte";
	import ThreadWorkspaceActive from "$lib/components/app/ThreadWorkspaceActive.svelte";
	import { useContext } from "$lib/context";
	import type { ResourceStatus } from "$lib/context/cache";
	import type { AsyncStatus } from "$lib/resource/types";

	type Props = {
		sessionId: string;
		threadId: string;
		visible: boolean;
		mainClass: string;
		sidebarOpen?: boolean;
		reserveSidebarSpace?: boolean;
		onPinSidebar?: () => void;
		mode?: "full" | "conversation-only";
	};

	let {
		sessionId,
		threadId,
		visible,
		mainClass,
		reserveSidebarSpace,
		onPinSidebar,
		mode,
	}: Props = $props();

	const context = useContext();
	const sessionRecord = $derived(context.data.sessions.byId[sessionId] ?? null);
	const currentSession = $derived(sessionRecord?.value ?? null);
	const isPendingSession = $derived(currentSession === null);
	const sessionThreads = $derived(sessionRecord?.threads ?? null);
	const threadRecord = $derived(sessionThreads?.byId[threadId] ?? null);
	const selectedThreadId = $derived.by(() => {
		if (context.view.selection.sessionId === sessionId) {
			return context.view.selection.threadId;
		}
		return (
			context.view.selection.requestedThreadIdBySessionId[sessionId] ?? null
		);
	});
	const selectedThread = $derived.by(() =>
		selectedThreadId
			? (sessionThreads?.byId[selectedThreadId]?.value ?? null)
			: null,
	);
	const canLoadThreadData = $derived.by(
		() =>
			!isPendingSession && canLoadSessionThreads(currentSession?.sandboxStatus),
	);
	const hasSelectedThread = $derived.by(
		() =>
			isPendingSession ||
			selectedThreadId !== null ||
			isSessionTransitioningStatus(currentSession?.sandboxStatus),
	);
	const hasConversationMessages = $derived.by(
		() => (threadRecord?.content.messages.length ?? 0) > 0,
	);
	const showActiveConversation = $derived.by(
		() => hasSelectedThread || hasConversationMessages,
	);
	const headerTitle = $derived.by(() => selectedThread?.name ?? "");
	const sessionTitle = $derived.by(
		() => currentSession?.displayName || currentSession?.name || "Sessions",
	);
	const isLoadingThread = $derived.by(
		() =>
			!showActiveConversation &&
			!isPendingSession &&
			(isSessionTransitioningStatus(currentSession?.sandboxStatus) ||
				getCacheStatus(sessionThreads?.status) === "loading"),
	);
	const showThreadSelectionPrompt = $derived.by(
		() => !isLoadingThread && !showActiveConversation && canLoadThreadData,
	);

	function getCacheStatus(
		status: ResourceStatus | null | undefined,
	): AsyncStatus {
		switch (status?.state) {
			case "loading":
			case "refreshing":
				return "loading";
			case "ready":
				return "ready";
			case "error":
				return "error";
			default:
				return "idle";
		}
	}

	onMount(() => {
		void context.commands.view.mountThreadView(sessionId, threadId);
	});
</script>

{#snippet sessionHeaderDropdown()}
	<SessionHeaderDropdown label={sessionTitle} {onPinSidebar} />
{/snippet}

<main class={mainClass}>
	{#if showActiveConversation}
		<ThreadWorkspaceActive
			{sessionId}
			{threadId}
			{visible}
			{reserveSidebarSpace}
			{onPinSidebar}
			{mode}
		/>
	{:else}
		<ThreadWorkspaceHeader
			reserveSidebarSpace={reserveSidebarSpace ?? false}
			title={headerTitle}
			state={selectedThread?.state}
			titleContent={sessionHeaderDropdown}
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
					<ConversationComposerSessionSetupStatus {sessionId} {threadId} />
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
