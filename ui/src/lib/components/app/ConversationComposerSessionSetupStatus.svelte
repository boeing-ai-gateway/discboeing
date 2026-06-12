<script lang="ts">
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import { useContext } from "$lib/context";
	import {
		getPendingWorkspaceSourceIsValid,
		getPendingWorkspaceValidationMessage,
	} from "$lib/pending-workspace-helpers";

	type Props = {
		sessionId: string;
		threadId: string;
	};

	let { sessionId, threadId }: Props = $props();
	const context = useContext();
	const sessionRecord = $derived(context.data.sessions.byId[sessionId] ?? null);
	const sessionView = $derived(context.view.sessions[sessionId] ?? null);
	const pendingWorkspace = $derived(sessionView?.pendingWorkspace ?? null);
	const threadRecord = $derived(sessionRecord?.threads.byId[threadId] ?? null);
	const threadContent = $derived(threadRecord?.content ?? null);
	const isPending = $derived(
		context.view.selection.pendingSessionId === sessionId,
	);
	const sessionStatus = $derived.by(
		() => sessionRecord?.value?.sandboxStatus ?? null,
	);
	const sessionErrorMessage = $derived.by(
		() => sessionRecord?.value?.errorMessage?.trim() ?? "",
	);
	const sessionStatusMessage = $derived.by(
		() => sessionRecord?.value?.sandboxStatusMessage?.trim() ?? "",
	);
	const showSessionStatus = $derived.by(
		() => sessionStatus !== null && sessionStatus !== "ready",
	);
	const isThreadStarting = $derived.by(
		() =>
			threadContent?.isStreaming === true ||
			threadContent?.messages.some(
				(message) => message.provisional === true,
			) === true,
	);
	const pendingWorkspaceSourceIsValid = $derived.by(() =>
		getPendingWorkspaceSourceIsValid(pendingWorkspace),
	);
	const pendingWorkspaceValidationMessage = $derived.by(() =>
		getPendingWorkspaceValidationMessage(pendingWorkspace),
	);
	const pendingSessionStarted = $derived.by(
		() => isPending && isThreadStarting,
	);
</script>

{#if isPending || showSessionStatus}
	<div class="mb-2 px-1">
		{#if pendingSessionStarted && !sessionStatus}
			<div
				class="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground"
			>
				<Loader2Icon class="size-3.5 animate-spin" />
				<span>Creating session</span>
			</div>
		{:else if showSessionStatus && sessionStatus}
			<SessionStatus
				status={sessionStatus}
				class="text-sm"
				labelClass="font-medium"
			/>
			{#if sessionErrorMessage}
				<p
					class="mt-1 line-clamp-3 text-xs text-destructive"
					title={sessionErrorMessage}
				>
					{sessionErrorMessage}
				</p>
			{:else if sessionStatusMessage}
				<p
					class="mt-1 line-clamp-3 text-xs text-muted-foreground"
					title={sessionStatusMessage}
				>
					{sessionStatusMessage}
				</p>
			{/if}
		{:else}
			<p class="text-sm font-medium text-muted-foreground">
				Start a new session
			</p>
		{/if}
	</div>
	{#if isPending && context.data.workspaces.status.state === "loading"}
		<p class="mb-2 px-1 text-xs text-muted-foreground">Loading workspaces...</p>
	{/if}
	{#if isPending && pendingWorkspace?.setupMessage}
		<p
			class="mb-2 truncate px-1 text-xs text-destructive"
			title={pendingWorkspace.setupMessage}
		>
			{pendingWorkspace.setupMessage}
		</p>
	{/if}
	{#if isPending && pendingWorkspaceValidationMessage}
		<p
			class={`mb-2 truncate px-1 text-xs ${pendingWorkspaceSourceIsValid ? "text-muted-foreground" : "text-destructive"}`}
			title={pendingWorkspaceValidationMessage}
		>
			{pendingWorkspaceValidationMessage}
		</p>
	{/if}
	{#if isPending && pendingWorkspace?.validation?.authMessage}
		<p class="mb-2 px-1 text-xs text-muted-foreground">
			{pendingWorkspace.validation.authMessage}
		</p>
		{#if pendingWorkspace.validation.authRequired && pendingWorkspace.validation.authProvider === "github-git"}
			<button
				type="button"
				class="mb-2 px-1 text-xs text-primary underline underline-offset-2 hover:text-primary/80"
				onclick={() => {
					void context.commands.dialogs.openGitHubCredentialFlow();
				}}
			>
				Connect GitHub credential
			</button>
		{/if}
	{/if}
{/if}
