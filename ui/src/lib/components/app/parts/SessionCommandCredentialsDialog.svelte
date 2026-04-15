<script lang="ts">
	import { Button } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger,
	} from "$lib/components/ui/select";
	import {
		credentialBindingDescription,
		credentialDisplayName,
		findCredentialMatches,
		listAnyCredentials,
	} from "$lib/components/ai/tool-renderers/requestusercredential-helpers";
	import type { AgentCommandCredentialRequest } from "$lib/api-types";
	import type { SessionCommandCredentialDialogState } from "$lib/session/session-context.types";

	type Props = {
		dialog: SessionCommandCredentialDialogState;
	};

	let { dialog }: Props = $props();

	function commandLabel() {
		return (
			dialog.command?.discobot?.label?.trim() ||
			dialog.command?.name ||
			"Command"
		);
	}

	function approvedUses(request: AgentCommandCredentialRequest): string[] {
		return (request.approvedUses ?? [])
			.map((use) => use.description.trim())
			.filter((description) => description.length > 0);
	}
</script>

<Dialog.Root
	open={dialog.open}
	onOpenChange={(open) => {
		if (!open) {
			dialog.close();
		}
	}}
>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>{commandLabel()} needs credentials</Dialog.Title>
			<Dialog.Description>
				Choose which credentials should be pre-approved for this command.
			</Dialog.Description>
		</Dialog.Header>

		<div class="max-h-[min(32rem,70vh)] space-y-4 overflow-y-auto pr-1">
			{#each dialog.requests as request (request.envVar)}
				{@const matches = findCredentialMatches(
					request.envVar,
					dialog.projectCredentials,
					dialog.sessionAssignments,
				)}
				{@const anyCredentials = listAnyCredentials(
					dialog.projectCredentials,
					dialog.sessionAssignments,
				)}
				{@const selectedCredential = dialog.projectCredentials.find(
					(item) =>
						item.id === dialog.selectedCredentialIdsByEnvVar[request.envVar],
				)}
				<div class="space-y-3 rounded-lg border border-border p-4">
					<div class="space-y-1.5">
						<div class="flex items-center justify-between gap-3">
							<h3 class="font-medium text-sm">
								{request.name || request.envVar}
							</h3>
							<span class="font-mono text-muted-foreground text-xs">
								{request.envVar}
							</span>
						</div>
						<p class="text-muted-foreground text-sm">{request.justification}</p>
						{#if approvedUses(request).length > 0}
							<ul
								class="list-disc space-y-1 pl-5 text-muted-foreground text-sm"
							>
								{#each approvedUses(request) as futureUse}
									<li>{futureUse}</li>
								{/each}
							</ul>
						{/if}
					</div>

					<div class="space-y-2">
						<p
							class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
						>
							Suggested matches
						</p>
						{#if matches.length === 0}
							<p class="text-muted-foreground text-sm">
								No existing credential exposes <span class="font-mono"
									>{request.envVar}</span
								>.
							</p>
						{:else}
							<div class="space-y-2">
								{#each matches as match (match.credential.id)}
									<button
										type="button"
										class={`flex w-full items-start justify-between gap-3 rounded-md border p-3 text-left transition-colors ${dialog.selectedCredentialIdsByEnvVar[request.envVar] === match.credential.id ? "border-primary bg-primary/5" : "border-border hover:bg-muted/50"}`}
										onclick={() => {
											dialog.selectCredential(
												request.envVar,
												match.credential.id,
											);
										}}
									>
										<div class="min-w-0 flex-1">
											<p class="truncate font-medium text-sm">
												{credentialDisplayName(match.credential)}
											</p>
											<p class="truncate text-muted-foreground text-xs">
												{match.assigned
													? "Already assigned to this session"
													: "Exact env var match"}
											</p>
										</div>
										{#if dialog.selectedCredentialIdsByEnvVar[request.envVar] === match.credential.id}
											<span class="text-xs text-primary">Selected</span>
										{/if}
									</button>
								{/each}
							</div>
						{/if}
					</div>

					<div class="space-y-2">
						<p
							class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
						>
							Select any credential
						</p>
						{#if anyCredentials.length === 0}
							<p class="text-muted-foreground text-sm">
								No credentials are available.
							</p>
						{:else}
							<Select
								type="single"
								value={dialog.selectedCredentialIdsByEnvVar[request.envVar] ??
									""}
								onValueChange={(value) => {
									dialog.selectCredential(request.envVar, value);
								}}
							>
								<SelectTrigger class="w-full">
									{selectedCredential
										? credentialDisplayName(selectedCredential)
										: "Choose a credential"}
								</SelectTrigger>
								<SelectContent>
									{#each anyCredentials as match (match.credential.id)}
										<SelectItem value={match.credential.id}>
											{credentialDisplayName(match.credential)}
										</SelectItem>
									{/each}
								</SelectContent>
							</Select>
							{#if selectedCredential}
								<p class="text-muted-foreground text-xs">
									{credentialBindingDescription(
										request.envVar,
										selectedCredential,
									)}
								</p>
							{/if}
						{/if}
					</div>
				</div>
			{/each}
		</div>

		{#if dialog.error}
			<p class="text-destructive text-sm">{dialog.error}</p>
		{/if}

		<Dialog.Footer>
			<Button variant="ghost" onclick={dialog.close}>Cancel</Button>
			<Button onclick={() => void dialog.confirm()}>Continue</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
