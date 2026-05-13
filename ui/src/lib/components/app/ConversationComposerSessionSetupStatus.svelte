<script lang="ts">
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";
	import { useThreadContext } from "$lib/context/thread-context.svelte";

	const app = useAppContext();
	const ui = app.ui;
	const workspaces = app.workspaces;
	const session = useSessionContext();
	const thread = useThreadContext();
	const sessionStatus = $derived.by(() => session.current?.status ?? null);
	const sessionErrorMessage = $derived.by(
		() => session.current?.errorMessage?.trim() ?? "",
	);
	const showSessionStatus = $derived.by(
		() => sessionStatus !== null && sessionStatus !== "ready",
	);
	const pendingSessionStarted = $derived.by(
		() => session.isPending && thread.isStreaming,
	);
</script>

{#if session.isPending || showSessionStatus}
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
			{/if}
		{:else}
			<p class="text-sm font-medium text-muted-foreground">
				Start a new session
			</p>
		{/if}
	</div>
	{#if session.isPending && workspaces.status === "loading"}
		<p class="mb-2 px-1 text-xs text-muted-foreground">Loading workspaces...</p>
	{/if}
	{#if session.isPending && session.ui.pendingWorkspaceSetupMessage}
		<p
			class="mb-2 truncate px-1 text-xs text-destructive"
			title={session.ui.pendingWorkspaceSetupMessage}
		>
			{session.ui.pendingWorkspaceSetupMessage}
		</p>
	{/if}
	{#if session.isPending && session.ui.pendingWorkspaceValidationMessage}
		<p
			class={`mb-2 truncate px-1 text-xs ${session.ui.pendingWorkspaceSourceIsValid ? "text-muted-foreground" : "text-destructive"}`}
			title={session.ui.pendingWorkspaceValidationMessage}
		>
			{session.ui.pendingWorkspaceValidationMessage}
		</p>
	{/if}
	{#if session.isPending && session.ui.pendingWorkspaceValidation?.authMessage}
		<p class="mb-2 px-1 text-xs text-muted-foreground">
			{session.ui.pendingWorkspaceValidation.authMessage}
		</p>
		{#if session.ui.pendingWorkspaceValidation.authRequired && session.ui.pendingWorkspaceValidation.authProvider === "github-git"}
			<button
				type="button"
				class="mb-2 px-1 text-xs text-primary underline underline-offset-2 hover:text-primary/80"
				onclick={ui.openGitHubCredentialFlow}
			>
				Connect GitHub credential
			</button>
		{/if}
	{/if}
{/if}
