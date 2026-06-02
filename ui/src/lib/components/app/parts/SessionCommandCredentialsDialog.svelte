<script lang="ts">
	import { Button } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";
	import { Input } from "$lib/components/ui/input";
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger,
	} from "$lib/components/ui/select";
	import {
		CUSTOM_CREDENTIAL_OPTION,
		type CredentialValidityPreset,
		type CredentialValidityUnit,
		credentialBindingDescription,
		credentialDisplayName,
		findPreferredCredentialId,
		formatApprovedUses,
		listAnyCredentials,
		listOAuthCredentialOptions,
		parseOAuthCredentialOption,
	} from "$lib/components/ai/tool-renderers/requestusercredential-helpers";
	import type { AgentCommandCredentialRequest } from "$lib/api-types";
	import type { SessionCommandCredentialDialogState } from "$lib/session/session-context.types";

	type Props = {
		dialog: SessionCommandCredentialDialogState;
	};

	let { dialog }: Props = $props();

	const validityUnits: Array<{
		value: CredentialValidityUnit;
		label: string;
	}> = [
		{ value: "hours", label: "Hours" },
		{ value: "days", label: "Days" },
		{ value: "weeks", label: "Weeks" },
		{ value: "never", label: "Never expires" },
	];
	const validityPresets: Array<{
		value: CredentialValidityPreset;
		label: string;
	}> = [
		{ value: "15_minutes", label: "15 minutes" },
		{ value: "1_hour", label: "1 hour" },
		{ value: "1_day", label: "1 day" },
		{ value: "1_week", label: "1 week" },
		{ value: "custom", label: "Custom" },
	];

	function commandLabel() {
		return (
			dialog.command?.discobot?.label?.trim() ||
			dialog.command?.name ||
			"Command"
		);
	}

	function selectedLabel(
		request: AgentCommandCredentialRequest,
		selectedOption: string,
	) {
		const value = selectedOption;
		if (value === CUSTOM_CREDENTIAL_OPTION) {
			return "Custom credential";
		}
		const oauthType = parseOAuthCredentialOption(value, dialog.credentialTypes);
		if (oauthType) {
			return `New ${oauthType.name} OAuth`;
		}
		const credential = dialog.projectCredentials.find(
			(item) => item.id === value,
		);
		return credential
			? credentialDisplayName(credential)
			: "Choose a credential";
	}

	$effect(() => {
		if (!dialog.open || typeof window === "undefined") {
			return;
		}
		const handleCredentialsChanged = () => {
			void dialog.refreshAvailableCredentials();
		};
		window.addEventListener(
			"discobot:credentials-changed",
			handleCredentialsChanged,
		);
		return () => {
			window.removeEventListener(
				"discobot:credentials-changed",
				handleCredentialsChanged,
			);
		};
	});
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
			<Dialog.Title>Approve credential access for {commandLabel()}</Dialog.Title
			>
			<Dialog.Description>
				Review why the command needs access, choose which credential to use,
				then approve or deny the request.
			</Dialog.Description>
		</Dialog.Header>

		<div class="max-h-[min(36rem,70vh)] space-y-4 overflow-y-auto pr-1">
			{#each dialog.requests as request (request.envVar)}
				{@const preferredId =
					findPreferredCredentialId(
						request.envVar,
						dialog.projectCredentials,
						dialog.sessionAssignments,
					) || dialog.selectedOptionByEnvVar[request.envVar]}
				{@const selectedOption =
					dialog.selectedOptionByEnvVar[request.envVar] ?? preferredId ?? ""}
				{@const selectedCredential = dialog.projectCredentials.find(
					(item) => item.id === selectedOption,
				)}
				{@const selectedOAuthType = parseOAuthCredentialOption(
					selectedOption,
					dialog.credentialTypes,
				)}
				{@const oauthOptions = listOAuthCredentialOptions(
					request.envVar,
					dialog.credentialTypes,
					dialog.projectCredentials,
				)}
				{@const uses = formatApprovedUses({
					envVar: request.envVar,
					name: request.name,
					justification: request.justification,
					approvedUses: request.approvedUses ?? [],
				})}
				<div class="space-y-4 rounded-lg border border-border bg-card p-4">
					<div class="space-y-3">
						<div class="space-y-1">
							<h3 class="font-semibold text-sm">
								{request.name || request.envVar}
							</h3>
							<p class="font-mono text-xs text-muted-foreground">
								{request.envVar}
							</p>
						</div>

						<div class="space-y-2 rounded-md bg-muted/40 p-3">
							<p
								class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
							>
								Why access is needed
							</p>
							<p class="text-sm">{request.justification}</p>
						</div>

						{#if uses.length > 0}
							<div class="space-y-2 rounded-md bg-muted/40 p-3">
								<p
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
								>
									Allowed uses
								</p>
								<ul class="list-disc space-y-1 pl-5 text-sm">
									{#each uses as use, __key0 (__key0)}
										<li>{use}</li>
									{/each}
								</ul>
							</div>
						{/if}
					</div>

					<div class="space-y-2">
						<p
							class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
						>
							Credential to use
						</p>
						<Select
							type="single"
							value={selectedOption}
							onValueChange={(value) => {
								dialog.selectOption(request.envVar, value);
							}}
						>
							<SelectTrigger class="w-full"
								>{selectedLabel(request, selectedOption)}</SelectTrigger
							>
							<SelectContent>
								{#each listAnyCredentials(dialog.projectCredentials, dialog.sessionAssignments) as match (match.credential.id)}
									<SelectItem value={match.credential.id}>
										{credentialDisplayName(match.credential)}
									</SelectItem>
								{/each}
								{#each oauthOptions as option (option.value)}
									<SelectItem value={option.value}>{option.label}</SelectItem>
								{/each}
								<SelectItem value={CUSTOM_CREDENTIAL_OPTION}
									>Custom credential</SelectItem
								>
							</SelectContent>
						</Select>
						{#if selectedCredential}
							<p class="text-muted-foreground text-xs">
								{credentialBindingDescription(
									request.envVar,
									selectedCredential,
								)}
							</p>
						{:else if selectedOAuthType}
							<div
								class="space-y-2 rounded-md border border-dashed border-border p-3"
							>
								<p
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
								>
									OAuth sign-in
								</p>
								<p class="text-sm">
									Launch the {selectedOAuthType.name} OAuth wizard, then come back
									here and select the connected credential.
								</p>
								<div class="flex flex-wrap gap-2">
									<Button
										variant="outline"
										size="sm"
										onclick={() =>
											void dialog.launchOAuthWizard(request.envVar)}
									>
										Launch wizard
									</Button>
									<Button
										variant="ghost"
										size="sm"
										onclick={() => void dialog.refreshAvailableCredentials()}
									>
										Refresh credentials
									</Button>
								</div>
							</div>
						{/if}
					</div>

					<div class="space-y-2">
						<p
							class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
						>
							How long it is valid
						</p>
						<Select
							type="single"
							value={dialog.validityPresetByEnvVar[request.envVar] ?? "1_hour"}
							onValueChange={(value) => {
								dialog.setValidityPreset(
									request.envVar,
									value as CredentialValidityPreset,
								);
							}}
						>
							<SelectTrigger class="w-full">
								{validityPresets.find(
									(preset) =>
										preset.value ===
										(dialog.validityPresetByEnvVar[request.envVar] ?? "1_hour"),
								)?.label ?? "1 hour"}
							</SelectTrigger>
							<SelectContent>
								{#each validityPresets as preset (preset.value)}
									<SelectItem value={preset.value}>{preset.label}</SelectItem>
								{/each}
							</SelectContent>
						</Select>
					</div>

					{#if (dialog.validityPresetByEnvVar[request.envVar] ?? "1_hour") === "custom"}
						<div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_10rem]">
							<div class="space-y-2">
								<label
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
									for={`validity-${request.envVar}`}
								>
									Custom duration
								</label>
								<Input
									id={`validity-${request.envVar}`}
									type="number"
									min="1"
									disabled={(dialog.validityUnitByEnvVar[request.envVar] ??
										"hours") === "never"}
									value={dialog.validityValueByEnvVar[request.envVar] ?? "1"}
									oninput={(event) => {
										dialog.setValidityValue(
											request.envVar,
											(event.currentTarget as HTMLInputElement).value,
										);
									}}
								/>
							</div>
							<div class="space-y-2">
								<p
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
								>
									Unit
								</p>
								<Select
									type="single"
									value={dialog.validityUnitByEnvVar[request.envVar] ?? "hours"}
									onValueChange={(value) => {
										dialog.setValidityUnit(
											request.envVar,
											value as CredentialValidityUnit,
										);
									}}
								>
									<SelectTrigger class="w-full">
										{validityUnits.find(
											(unit) =>
												unit.value ===
												(dialog.validityUnitByEnvVar[request.envVar] ??
													"hours"),
										)?.label ?? "Hours"}
									</SelectTrigger>
									<SelectContent>
										{#each validityUnits as unit (unit.value)}
											<SelectItem value={unit.value}>{unit.label}</SelectItem>
										{/each}
									</SelectContent>
								</Select>
							</div>
						</div>
					{/if}

					{#if selectedOption === CUSTOM_CREDENTIAL_OPTION}
						<div
							class="space-y-3 rounded-md border border-dashed border-border p-3"
						>
							<p
								class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
							>
								Enter new credential
							</p>
							<div class="space-y-2">
								<label
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
									for={`credential-name-${request.envVar}`}
								>
									Credential name
								</label>
								<Input
									id={`credential-name-${request.envVar}`}
									value={dialog.createCredentialNamesByEnvVar[request.envVar] ??
										""}
									placeholder="Credential name"
									oninput={(event) => {
										dialog.setCreateCredentialName(
											request.envVar,
											(event.currentTarget as HTMLInputElement).value,
										);
									}}
								/>
							</div>
							<div class="space-y-2">
								<label
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
									for={`credential-secret-${request.envVar}`}
								>
									Credential secret
								</label>
								<Input
									id={`credential-secret-${request.envVar}`}
									type="password"
									value={dialog.createCredentialSecretsByEnvVar[
										request.envVar
									] ?? ""}
									placeholder={`Enter ${request.envVar}`}
									class="font-mono"
									oninput={(event) => {
										dialog.setCreateCredentialSecret(
											request.envVar,
											(event.currentTarget as HTMLInputElement).value,
										);
									}}
								/>
							</div>
						</div>
					{/if}
				</div>
			{/each}
		</div>

		{#if dialog.error}
			<p class="text-destructive text-sm">{dialog.error}</p>
		{/if}

		<Dialog.Footer>
			<Button variant="outline" onclick={dialog.close}>Deny</Button>
			<Button onclick={() => void dialog.confirm()}>Approve</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
