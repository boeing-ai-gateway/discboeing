<script lang="ts">
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { Label } from "$lib/components/ui/label";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import { Textarea } from "$lib/components/ui/textarea";
	import type {
		CredentialAuthType,
		SandboxProviderConfigField,
	} from "$lib/api-types";

	type CredentialOption = {
		id: string;
		name: string;
		provider: string;
	};

	type Props = {
		field: SandboxProviderConfigField;
		value: string;
		saving?: boolean;
		creatingCredential?: boolean;
		credentialOptions?: CredentialOption[];
		credentialProvider: string;
		credentialAuthType: CredentialAuthType;
		credentialDefaultName: string;
		newCredentialName: string;
		newCredentialSecret: string;
		onValueChange: (field: SandboxProviderConfigField, value: string) => void;
		onBeginCreateCredential: (field: SandboxProviderConfigField) => void;
		onCancelCreateCredential: () => void;
		onNewCredentialNameChange: (
			field: SandboxProviderConfigField,
			value: string,
		) => void;
		onNewCredentialSecretChange: (
			field: SandboxProviderConfigField,
			value: string,
		) => void;
		onCreateCredential: (field: SandboxProviderConfigField) => void;
	};

	let {
		field,
		value,
		saving = false,
		creatingCredential = false,
		credentialOptions = [],
		credentialProvider,
		credentialAuthType,
		credentialDefaultName,
		newCredentialName,
		newCredentialSecret,
		onValueChange,
		onBeginCreateCredential,
		onCancelCreateCredential,
		onNewCredentialNameChange,
		onNewCredentialSecretChange,
		onCreateCredential,
	}: Props = $props();

	function credentialDisplayName(credential: CredentialOption) {
		return credential.name.trim() || credential.provider;
	}

	function handleCredentialChange(event: Event) {
		const next = (event.currentTarget as HTMLSelectElement).value;
		if (next === "__create__") {
			onBeginCreateCredential(field);
			return;
		}
		onValueChange(field, next === "__none__" ? "" : next);
	}
</script>

<div
	class={field.type === "textarea"
		? "space-y-1.5 sm:col-span-2"
		: "space-y-1.5"}
>
	<div class="flex items-center justify-between gap-2">
		<Label for={`sandbox-provider-config-${field.key}`}>
			{field.label}{field.required ? " *" : ""}
		</Label>
		<span class="text-xs text-muted-foreground">
			{field.required ? "Required" : "Optional"}
		</span>
	</div>
	{#if field.type === "credential"}
		<NativeSelect
			id={`sandbox-provider-config-${field.key}`}
			value={value || "__none__"}
			disabled={saving}
			onchange={handleCredentialChange}
		>
			<option value="__none__">No credential</option>
			{#each credentialOptions as credential (credential.id)}
				<option value={credential.id}
					>{credentialDisplayName(credential)}</option
				>
			{/each}
			<option value="__create__">Create new API key...</option>
		</NativeSelect>
		{#if credentialOptions.length === 0 && !creatingCredential}
			<p class="text-xs text-muted-foreground">
				No active {credentialProvider}
				{credentialAuthType}
				credentials found.
			</p>
		{/if}
		{#if creatingCredential}
			<div class="space-y-2 rounded-md border border-border p-3">
				<div class="space-y-1.5">
					<Label for={`sandbox-provider-config-${field.key}-credential-name`}>
						Credential name
					</Label>
					<Input
						id={`sandbox-provider-config-${field.key}-credential-name`}
						value={newCredentialName || credentialDefaultName}
						disabled={saving}
						oninput={(event) => {
							onNewCredentialNameChange(
								field,
								(event.currentTarget as HTMLInputElement).value,
							);
						}}
					/>
				</div>
				<div class="space-y-1.5">
					<Label for={`sandbox-provider-config-${field.key}-credential-secret`}>
						API key
					</Label>
					<Input
						id={`sandbox-provider-config-${field.key}-credential-secret`}
						type="password"
						value={newCredentialSecret}
						disabled={saving}
						placeholder={`Paste ${credentialProvider} API key`}
						oninput={(event) => {
							onNewCredentialSecretChange(
								field,
								(event.currentTarget as HTMLInputElement).value,
							);
						}}
					/>
				</div>
				<div class="flex justify-end gap-2">
					<Button
						variant="ghost"
						size="xs"
						disabled={saving}
						onclick={onCancelCreateCredential}
					>
						Cancel
					</Button>
					<Button
						size="xs"
						disabled={saving || credentialAuthType !== "api_key"}
						onclick={() => onCreateCredential(field)}
					>
						Create credential
					</Button>
				</div>
			</div>
		{/if}
	{:else if field.type === "textarea"}
		<Textarea
			id={`sandbox-provider-config-${field.key}`}
			{value}
			disabled={saving}
			placeholder={field.placeholder}
			oninput={(event) => {
				onValueChange(
					field,
					(event.currentTarget as HTMLTextAreaElement).value,
				);
			}}
		/>
	{:else}
		<Input
			id={`sandbox-provider-config-${field.key}`}
			type={field.type === "number" ? "number" : "text"}
			{value}
			disabled={saving}
			placeholder={field.placeholder}
			oninput={(event) => {
				onValueChange(field, (event.currentTarget as HTMLInputElement).value);
			}}
		/>
	{/if}
	{#if field.description}
		<p class="text-xs text-muted-foreground">
			{field.description}
		</p>
	{/if}
</div>
