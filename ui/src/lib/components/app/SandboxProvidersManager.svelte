<script lang="ts">
	import { onMount } from "svelte";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import PencilIcon from "@lucide/svelte/icons/pencil";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import ServerCogIcon from "@lucide/svelte/icons/server-cog";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import * as simpleIcons from "simple-icons";
	import { api } from "$lib/api-client";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { Label } from "$lib/components/ui/label";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import { Switch } from "$lib/components/ui/switch";
	import {
		Item,
		ItemMedia,
		ItemActions,
		ItemContent,
		ItemDescription,
		ItemGroup,
		ItemSeparator,
		ItemTitle,
	} from "$lib/components/ui/item";
	import ProviderIcon from "$lib/components/app/parts/ProviderIcon.svelte";
	import SandboxProviderConfigFieldControl from "$lib/components/app/parts/SandboxProviderConfigField.svelte";
	import ProjectSettingsTabContent from "$lib/components/app/parts/ProjectSettingsTabContent.svelte";
	import type {
		CredentialAuthType,
		SandboxProviderConfigField,
		SandboxProviderInstance,
		SandboxProviderType,
	} from "$lib/api-types";
	import { refreshCredentials } from "$lib/context/commands/workspace";
	import { useContext } from "$lib/context/context.svelte";

	type SimpleIconData = {
		title: string;
		slug: string;
		hex: string;
		source: string;
		svg: string;
		path: string;
	};

	const context = useContext();
	const sandboxProvidersUpdatedEvent = "discobot:sandbox-providers-updated";

	let providerTypes = $state<SandboxProviderType[]>([]);
	let providers = $state<SandboxProviderInstance[]>([]);
	let defaultProviderId = $state("");
	let projectDefaultProviderId = $state("");
	let loading = $state(false);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let driverPickerOpen = $state(false);
	let formOpen = $state(false);
	let runtimeProvider = $state<SandboxProviderInstance | null>(null);
	let editingId = $state<string | null>(null);
	let type = $state("");
	let name = $state("");
	let icon = $state("");
	let configValues = $state<Record<string, string>>({});
	let creatingCredentialField = $state<string | null>(null);
	let newCredentialNames = $state<Record<string, string>>({});
	let newCredentialSecrets = $state<Record<string, string>>({});

	const availableTypes = $derived.by(() =>
		providerTypes.filter((providerType) => providerType.available),
	);
	const selectedType = $derived.by(
		() =>
			providerTypes.find((providerType) => providerType.id === type) ?? null,
	);
	const selectedConfigFields = $derived(selectedType?.configFields ?? []);
	const basicConfigFields = $derived(
		selectedConfigFields.filter((field) => !field.advanced),
	);
	const advancedConfigFields = $derived(
		selectedConfigFields.filter((field) => field.advanced),
	);
	const formTitle = $derived(
		editingId
			? "Edit provider instance"
			: selectedType
				? `Add ${selectedType.name} provider`
				: "Add provider instance",
	);
	const iconPreviewName = $derived(
		name.trim() || selectedType?.name || "Provider",
	);
	const simpleIconOptions = Object.values(simpleIcons)
		.filter(
			(value): value is SimpleIconData =>
				typeof value === "object" &&
				value !== null &&
				"slug" in value &&
				"title" in value &&
				"path" in value,
		)
		.map((icon) => ({ value: `simple:${icon.slug}`, label: icon.title }))
		.sort((a, b) => a.label.localeCompare(b.label));
	let advancedConfigOpen = $state(false);

	function getConfigValue(
		config: Record<string, unknown> | undefined,
		key: string,
	): string {
		const value = config?.[key];
		return typeof value === "string" ? value : "";
	}

	function driverName(providerType: string): string {
		return (
			providerTypes.find((item) => item.id === providerType)?.name ??
			providerType
		);
	}

	function providerName(provider: SandboxProviderInstance | null): string {
		if (!provider) {
			return "Provider";
		}
		return provider.name.trim() || driverName(provider.type);
	}

	function providerDisplayName(providerId: string): string {
		const provider = providers.find((item) => item.id === providerId);
		return provider ? providerName(provider) : providerId;
	}

	function resetForm() {
		editingId = null;
		type = "";
		name = "";
		icon = "";
		configValues = {};
		creatingCredentialField = null;
		newCredentialNames = {};
		newCredentialSecrets = {};
		advancedConfigOpen = false;
	}

	function closeForm() {
		resetForm();
		formOpen = false;
		driverPickerOpen = false;
		error = null;
	}

	function openProviderControls(provider: SandboxProviderInstance) {
		runtimeProvider = provider;
		formOpen = false;
		driverPickerOpen = false;
		error = null;
	}

	function closeProviderControls() {
		runtimeProvider = null;
		error = null;
	}

	function addProvider() {
		resetForm();
		runtimeProvider = null;
		formOpen = false;
		driverPickerOpen = true;
		error = null;
	}

	function selectDriver(providerType: SandboxProviderType) {
		resetForm();
		type = providerType.id;
		runtimeProvider = null;
		driverPickerOpen = false;
		formOpen = true;
		error = null;
	}

	async function refresh() {
		loading = true;
		error = null;
		try {
			const [typesResponse, providersResponse] = await Promise.all([
				api.getSandboxProviderTypes(),
				api.getSandboxProviders(),
				refreshCredentials(),
			]);
			providerTypes = typesResponse.providerTypes;
			providers = providersResponse.providers;
			defaultProviderId = providersResponse.default;
			projectDefaultProviderId = providersResponse.projectDefault ?? "";
		} catch (err) {
			error = err instanceof Error ? err.message : "Failed to load providers";
		} finally {
			loading = false;
		}
	}

	function notifySandboxProvidersUpdated() {
		window.dispatchEvent(new CustomEvent(sandboxProvidersUpdatedEvent));
	}

	function editProvider(provider: SandboxProviderInstance) {
		runtimeProvider = null;
		driverPickerOpen = false;
		editingId = provider.id;
		type = provider.type;
		name = provider.name;
		icon = getConfigValue(provider.config, "icon");
		configValues = Object.fromEntries(
			Object.entries(provider.config ?? {})
				.filter(
					([key, value]) =>
						key !== "icon" && value !== undefined && value !== null,
				)
				.map(([key, value]) => [key, String(value)]),
		);
		advancedConfigOpen = false;
		formOpen = true;
		error = null;
	}

	function buildConfig(): Record<string, unknown> | undefined {
		const config: Record<string, unknown> = {};
		for (const field of selectedConfigFields) {
			const value = (configValues[field.key] ?? "").trim();
			if (!value) {
				continue;
			}
			config[field.key] = field.type === "number" ? Number(value) : value;
		}
		return Object.keys(config).length > 0 ? config : undefined;
	}

	function configFieldValue(field: SandboxProviderConfigField): string {
		return configValues[field.key] ?? "";
	}

	function setConfigFieldValue(
		field: SandboxProviderConfigField,
		value: string,
	) {
		configValues = {
			...configValues,
			[field.key]: value,
		};
	}

	function credentialProvider(field: SandboxProviderConfigField): string {
		return field.credentialProvider?.trim() || type;
	}

	function credentialAuthType(
		field: SandboxProviderConfigField,
	): CredentialAuthType {
		return field.credentialAuthType ?? "api_key";
	}

	function credentialOptions(field: SandboxProviderConfigField) {
		const provider = credentialProvider(field);
		const authType = credentialAuthType(field);
		return context.data.credentials.items.filter(
			(credential) =>
				credential.provider === provider &&
				credential.authType === authType &&
				credential.isConfigured &&
				!credential.inactive,
		);
	}

	function credentialDefaultName(field: SandboxProviderConfigField): string {
		const driver = selectedType?.name || credentialProvider(field);
		return `${driver} API key`;
	}

	function beginCreateCredential(field: SandboxProviderConfigField) {
		creatingCredentialField = field.key;
		if (!newCredentialNames[field.key]) {
			newCredentialNames = {
				...newCredentialNames,
				[field.key]: credentialDefaultName(field),
			};
		}
	}

	function cancelCreateCredential() {
		creatingCredentialField = null;
	}

	function handleConfigFieldValueChange(
		field: SandboxProviderConfigField,
		value: string,
	) {
		if (creatingCredentialField === field.key) {
			creatingCredentialField = null;
		}
		setConfigFieldValue(field, value);
	}

	function setNewCredentialName(
		field: SandboxProviderConfigField,
		value: string,
	) {
		newCredentialNames = {
			...newCredentialNames,
			[field.key]: value,
		};
	}

	function setNewCredentialSecret(
		field: SandboxProviderConfigField,
		value: string,
	) {
		newCredentialSecrets = {
			...newCredentialSecrets,
			[field.key]: value,
		};
	}

	async function createCredentialForField(field: SandboxProviderConfigField) {
		const provider = credentialProvider(field);
		const authType = credentialAuthType(field);
		const name = (
			newCredentialNames[field.key] || credentialDefaultName(field)
		).trim();
		const apiKey = (newCredentialSecrets[field.key] ?? "").trim();
		if (authType !== "api_key") {
			error = "Only API key credentials can be created inline.";
			return;
		}
		if (!apiKey) {
			error = "API key is required.";
			return;
		}

		saving = true;
		error = null;
		try {
			const credential = await api.createCredential({
				provider,
				name,
				authType,
				apiKey,
			});
			await refreshCredentials();
			setConfigFieldValue(field, credential.id);
			creatingCredentialField = null;
			newCredentialSecrets = {
				...newCredentialSecrets,
				[field.key]: "",
			};
		} catch (err) {
			error =
				err instanceof Error ? err.message : "Failed to create credential";
		} finally {
			saving = false;
		}
	}

	async function saveProvider() {
		if (!type) {
			error = "Provider driver is required.";
			return;
		}
		const missingField = selectedConfigFields.find(
			(field) => field.required && !(configValues[field.key] ?? "").trim(),
		);
		if (missingField) {
			error = `${missingField.label} is required.`;
			return;
		}

		saving = true;
		error = null;
		try {
			const payload = {
				type,
				name: name.trim(),
				icon: icon.trim() || undefined,
				config: buildConfig(),
			};
			if (editingId) {
				await api.updateSandboxProvider(editingId, payload);
			} else {
				await api.createSandboxProvider(payload);
			}
			formOpen = false;
			resetForm();
			await refresh();
			notifySandboxProvidersUpdated();
		} catch (err) {
			error = err instanceof Error ? err.message : "Failed to save provider";
		} finally {
			saving = false;
		}
	}

	async function deleteProvider(provider: SandboxProviderInstance) {
		if (!confirm(`Delete sandbox provider "${providerName(provider)}"?`)) {
			return;
		}

		saving = true;
		error = null;
		try {
			await api.deleteSandboxProvider(provider.id);
			if (editingId === provider.id) {
				resetForm();
			}
			await refresh();
			notifySandboxProvidersUpdated();
		} catch (err) {
			error = err instanceof Error ? err.message : "Failed to delete provider";
		} finally {
			saving = false;
		}
	}

	async function setProviderDisabled(
		provider: SandboxProviderInstance,
		disabled: boolean,
	) {
		saving = true;
		error = null;
		try {
			await api.updateSandboxProvider(provider.id, { disabled });
			await refresh();
			notifySandboxProvidersUpdated();
		} catch (err) {
			error = err instanceof Error ? err.message : "Failed to update provider";
		} finally {
			saving = false;
		}
	}

	async function setDefaultProvider(providerId: string) {
		saving = true;
		error = null;
		try {
			const response = await api.updateDefaultSandboxProvider(providerId);
			defaultProviderId = response.default;
			projectDefaultProviderId = response.projectDefault;
			await refresh();
			notifySandboxProvidersUpdated();
		} catch (err) {
			error =
				err instanceof Error
					? err.message
					: "Failed to update default provider";
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		void refresh();
	});
</script>

<div class="space-y-4">
	{#if error}
		<div
			class="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
		>
			{error}
		</div>
	{/if}

	{#if driverPickerOpen}
		<div class="rounded-md border border-border p-3">
			<div class="mb-3 flex items-center justify-between gap-3">
				<div>
					<p class="text-sm font-medium">Choose a sandbox driver</p>
					<p class="text-xs text-muted-foreground">
						Pick the driver to use for this provider instance.
					</p>
				</div>
				<Button variant="ghost" size="xs" onclick={closeForm} disabled={saving}>
					Cancel
				</Button>
			</div>

			<ItemGroup class="rounded-md border border-border">
				{#if availableTypes.length === 0}
					<Item size="sm">
						<ItemContent>
							<ItemTitle>No sandbox drivers available</ItemTitle>
							<ItemDescription>
								Enable a sandbox driver before adding a provider instance.
							</ItemDescription>
						</ItemContent>
					</Item>
				{:else}
					{#each availableTypes as providerType, index (providerType.id)}
						{#if index > 0}<ItemSeparator />{/if}
						<Item size="sm" class="p-0">
							{#snippet child({ props })}
								<button
									{...props}
									type="button"
									class={`${props.class} w-full cursor-pointer gap-3 p-4 text-left hover:bg-accent/50`}
									disabled={saving}
									onclick={() => selectDriver(providerType)}
								>
									<ItemMedia
										class="h-10 w-10 rounded-md border border-border bg-muted/50"
									>
										<ProviderIcon
											icon={providerType.icon}
											name={providerType.name}
											class="size-8 border-0 bg-transparent"
										/>
									</ItemMedia>
									<ItemContent>
										<ItemTitle>{providerType.name}</ItemTitle>
										<ItemDescription>
											Driver: {providerType.id}{providerType.description
												? ` · ${providerType.description}`
												: ""}
										</ItemDescription>
									</ItemContent>
								</button>
							{/snippet}
						</Item>
					{/each}
				{/if}
			</ItemGroup>
		</div>
	{:else if formOpen}
		<div class="rounded-md border border-border p-3">
			<div class="mb-3 flex items-center justify-between gap-3">
				<div>
					<p class="text-sm font-medium">{formTitle}</p>
					<p class="text-xs text-muted-foreground">
						Configure an active provider instance. Credential fields store
						references, not raw secrets.
					</p>
				</div>
				<div class="flex items-center gap-2">
					{#if !editingId}
						<Button
							variant="ghost"
							size="xs"
							onclick={() => {
								formOpen = false;
								driverPickerOpen = true;
							}}
							disabled={saving}
						>
							Back
						</Button>
					{/if}
					<Button
						variant="ghost"
						size="xs"
						onclick={closeForm}
						disabled={saving}
					>
						Cancel
					</Button>
				</div>
			</div>

			{#if selectedType}
				<div
					class="mb-3 flex items-center gap-3 rounded-md border border-border bg-muted/20 p-3"
				>
					<ProviderIcon
						icon={selectedType.icon}
						name={selectedType.name}
						class="size-10 shrink-0 p-1"
					/>
					<div class="min-w-0">
						<p class="truncate text-sm font-medium">{selectedType.name}</p>
						<p class="text-xs text-muted-foreground">
							Driver: {selectedType.id}{selectedType.description
								? ` · ${selectedType.description}`
								: ""}
						</p>
					</div>
				</div>
			{/if}

			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-1.5 sm:col-span-2">
					<Label for="sandbox-provider-name">Name</Label>
					<Input
						id="sandbox-provider-name"
						value={name}
						disabled={saving}
						placeholder={selectedType
							? selectedType.name
							: "Defaults to the driver name"}
						oninput={(event) => {
							name = (event.currentTarget as HTMLInputElement).value;
						}}
					/>
					<p class="text-xs text-muted-foreground">
						Optional. If empty, the driver name is shown.
					</p>
				</div>
				{#each basicConfigFields as field (field.key)}
					<SandboxProviderConfigFieldControl
						{field}
						value={configFieldValue(field)}
						{saving}
						creatingCredential={creatingCredentialField === field.key}
						credentialOptions={credentialOptions(field)}
						credentialProvider={credentialProvider(field)}
						credentialAuthType={credentialAuthType(field)}
						credentialDefaultName={credentialDefaultName(field)}
						newCredentialName={newCredentialNames[field.key] ?? ""}
						newCredentialSecret={newCredentialSecrets[field.key] ?? ""}
						onValueChange={handleConfigFieldValueChange}
						onBeginCreateCredential={beginCreateCredential}
						onCancelCreateCredential={cancelCreateCredential}
						onNewCredentialNameChange={setNewCredentialName}
						onNewCredentialSecretChange={setNewCredentialSecret}
						onCreateCredential={(field) => void createCredentialForField(field)}
					/>
				{/each}
			</div>
			{#if type}
				<div class="mt-3">
					<button
						type="button"
						class="flex w-full items-center justify-between gap-3 py-1 text-left text-sm font-medium"
						aria-expanded={advancedConfigOpen}
						onclick={() => {
							advancedConfigOpen = !advancedConfigOpen;
						}}
					>
						<span>Advanced configuration</span>
						<ChevronDownIcon
							class={`size-4 text-muted-foreground transition-transform ${advancedConfigOpen ? "rotate-180" : ""}`}
						/>
					</button>
					{#if advancedConfigOpen}
						<div class="mt-3 grid gap-3 sm:grid-cols-2">
							<div class="space-y-1.5 sm:col-span-2">
								<Label for="sandbox-provider-icon">Icon</Label>
								<div class="flex items-center gap-3">
									<ProviderIcon
										{icon}
										name={iconPreviewName}
										class="size-10 shrink-0 p-1"
									/>
									<Input
										id="sandbox-provider-icon"
										value={icon}
										disabled={saving}
										list="sandbox-provider-simple-icons"
										placeholder="simple:docker, <svg>, data URL, or image URL"
										oninput={(event) => {
											icon = (event.currentTarget as HTMLInputElement).value;
										}}
									/>
									<datalist id="sandbox-provider-simple-icons">
										{#each simpleIconOptions as option (option.value)}
											<option value={option.value} label={option.label}
											></option>
										{/each}
									</datalist>
								</div>
								<p class="text-xs text-muted-foreground">
									Preview updates as you type. Supports Simple Icons, inline
									SVG, data URL, or image URL values.
								</p>
							</div>
							{#each advancedConfigFields as field (field.key)}
								<SandboxProviderConfigFieldControl
									{field}
									value={configFieldValue(field)}
									{saving}
									creatingCredential={creatingCredentialField === field.key}
									credentialOptions={credentialOptions(field)}
									credentialProvider={credentialProvider(field)}
									credentialAuthType={credentialAuthType(field)}
									credentialDefaultName={credentialDefaultName(field)}
									newCredentialName={newCredentialNames[field.key] ?? ""}
									newCredentialSecret={newCredentialSecrets[field.key] ?? ""}
									onValueChange={handleConfigFieldValueChange}
									onBeginCreateCredential={beginCreateCredential}
									onCancelCreateCredential={cancelCreateCredential}
									onNewCredentialNameChange={setNewCredentialName}
									onNewCredentialSecretChange={setNewCredentialSecret}
									onCreateCredential={(field) =>
										void createCredentialForField(field)}
								/>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
			<div class="mt-3 flex justify-end gap-2">
				<Button
					variant="outline"
					size="sm"
					onclick={closeForm}
					disabled={saving}
				>
					Cancel
				</Button>
				<Button
					size="sm"
					onclick={() => void saveProvider()}
					disabled={saving || loading}
				>
					{saving
						? "Saving..."
						: editingId
							? "Save provider instance"
							: "Add provider instance"}
				</Button>
			</div>
		</div>
	{:else if runtimeProvider}
		<div class="space-y-3">
			<div class="flex items-center justify-between gap-3">
				<div class="flex min-w-0 items-center gap-3">
					<ProviderIcon
						icon={runtimeProvider.icon}
						name={providerName(runtimeProvider)}
						class="size-8"
					/>
					<div class="min-w-0">
						<p class="truncate text-sm font-medium">
							{providerName(runtimeProvider)} controls
						</p>
						<p class="text-xs text-muted-foreground">
							Provider-scoped resources and inspection shell access.
						</p>
					</div>
				</div>
				<Button variant="ghost" size="xs" onclick={closeProviderControls}>
					Back
				</Button>
			</div>
			{#key runtimeProvider.id}
				<ProjectSettingsTabContent
					active={true}
					providerId={runtimeProvider.id}
					providerName={providerName(runtimeProvider)}
					showResources={runtimeProvider.capabilities.resources}
					showInspection={runtimeProvider.capabilities.inspection}
				/>
			{/key}
		</div>
	{:else}
		<div class="space-y-2">
			<div class="flex items-start justify-between gap-3">
				<div>
					<p class="text-sm font-medium">Active provider instances</p>
					<p class="text-xs text-muted-foreground">
						{#if projectDefaultProviderId}
							Project default: {providerDisplayName(projectDefaultProviderId)}
						{:else if defaultProviderId}
							Effective default: {providerDisplayName(defaultProviderId)}
						{:else}
							No default provider is available
						{/if}
					</p>
				</div>
				<div class="flex items-center gap-2">
					<div class="flex items-center gap-2">
						<Label
							for="sandbox-provider-default"
							class="text-xs text-muted-foreground"
						>
							Default
						</Label>
						<NativeSelect
							id="sandbox-provider-default"
							value={defaultProviderId || "__none__"}
							disabled={loading || saving || providers.length === 0}
							onchange={(event) => {
								const next = (event.currentTarget as HTMLSelectElement).value;
								if (next !== "__none__" && next !== projectDefaultProviderId) {
									void setDefaultProvider(next);
								}
							}}
						>
							{#if !defaultProviderId}
								<option value="__none__">No default</option>
							{/if}
							{#each providers as provider (provider.id)}
								<option
									value={provider.id}
									disabled={provider.disabled || !provider.available}
								>
									{providerName(provider)}
								</option>
							{/each}
						</NativeSelect>
					</div>
					<Button
						variant="ghost"
						size="icon-sm"
						onclick={() => void refresh()}
						disabled={loading}
						aria-label="Refresh provider instances"
						title="Refresh provider instances"
					>
						<RefreshCwIcon class={loading ? "size-4 animate-spin" : "size-4"} />
					</Button>
					<Button
						variant="default"
						size="xs"
						onclick={addProvider}
						disabled={loading || saving || availableTypes.length === 0}
					>
						Add provider instance
					</Button>
				</div>
			</div>

			<ItemGroup class="rounded-md border border-border">
				{#if providers.length === 0}
					<Item size="sm">
						<ItemContent>
							<ItemTitle>No sandbox providers found</ItemTitle>
							<ItemDescription>
								Add a provider instance or use the platform default when
								creating a session.
							</ItemDescription>
						</ItemContent>
					</Item>
				{:else}
					{#each providers as provider, index (provider.id)}
						{#if index > 0}<ItemSeparator />{/if}
						<Item size="sm">
							<ItemMedia
								class="h-10 w-10 rounded-md border border-border bg-muted/50"
							>
								<ProviderIcon
									icon={provider.icon}
									name={providerName(provider)}
									class="size-8 border-0 bg-transparent"
								/>
							</ItemMedia>
							<ItemContent>
								<ItemTitle>{providerName(provider)}</ItemTitle>
								<ItemDescription>
									Driver: {provider.type}{provider.id === defaultProviderId
										? " · default"
										: ""}{provider.builtIn
										? " · built-in"
										: ""}{provider.disabled
										? " · disabled"
										: provider.available
											? ""
											: " · unavailable"}
								</ItemDescription>
							</ItemContent>
							<ItemActions class="ml-auto gap-2">
								{#if (provider.capabilities.resources || provider.capabilities.inspection) && provider.available && !provider.disabled}
									<Button
										variant="ghost"
										size="icon-sm"
										disabled={saving}
										aria-label={`Open ${providerName(provider)} controls`}
										title="Open resources and inspection controls"
										onclick={() => openProviderControls(provider)}
									>
										<ServerCogIcon class="size-4" />
									</Button>
								{/if}
								{#if provider.builtIn}
									<Button
										variant="ghost"
										size="icon-sm"
										disabled={saving}
										aria-label={`Edit ${providerName(provider)}`}
										title="Edit provider"
										onclick={() => editProvider(provider)}
									>
										<PencilIcon class="size-4" />
									</Button>
									<Switch
										checked={!provider.disabled}
										disabled={saving}
										aria-label={provider.disabled
											? `Enable ${providerName(provider)}`
											: `Disable ${providerName(provider)}`}
										title={`${provider.disabled ? "Enable" : "Disable"} ${providerName(provider)}`}
										onCheckedChange={(checked) =>
											void setProviderDisabled(provider, checked !== true)}
									/>
								{:else}
									<Button
										variant="ghost"
										size="icon-sm"
										disabled={saving}
										aria-label={`Edit ${providerName(provider)}`}
										title="Edit provider"
										onclick={() => editProvider(provider)}
									>
										<PencilIcon class="size-4" />
									</Button>
									{#if provider.disabled}
										<Button
											variant="ghost"
											size="icon-sm"
											disabled={saving}
											class="text-destructive hover:text-destructive"
											aria-label={`Delete ${providerName(provider)}`}
											title="Delete provider"
											onclick={() => void deleteProvider(provider)}
										>
											<Trash2Icon class="size-4" />
										</Button>
									{/if}
									<Switch
										checked={!provider.disabled}
										disabled={saving}
										aria-label={provider.disabled
											? `Enable ${providerName(provider)}`
											: `Disable ${providerName(provider)}`}
										title={`${provider.disabled ? "Enable" : "Disable"} ${providerName(provider)}`}
										onCheckedChange={(checked) =>
											void setProviderDisabled(provider, checked !== true)}
									/>
								{/if}
							</ItemActions>
						</Item>
					{/each}
				{/if}
			</ItemGroup>
		</div>
	{/if}
</div>
