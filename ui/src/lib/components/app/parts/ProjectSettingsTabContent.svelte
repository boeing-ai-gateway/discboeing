<script lang="ts">
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import ServerCogIcon from "@lucide/svelte/icons/server-cog";
	import TerminalIcon from "@lucide/svelte/icons/terminal";
	import { ApiError, api } from "$lib/api-client";
	import type { ProjectInspectionInfo, ProjectResources } from "$lib/api-types";
	import { PROJECT_ID } from "$lib/api-config";
	import ProjectInspectionTerminalDialog from "$lib/components/app/parts/ProjectInspectionTerminalDialog.svelte";
	import { Button } from "$lib/components/ui/button";
	import {
		Card,
		CardAction,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle,
	} from "$lib/components/ui/card";
	import { Input } from "$lib/components/ui/input";
	import {
		Item,
		ItemActions,
		ItemContent,
		ItemDescription,
		ItemGroup,
		ItemSeparator,
		ItemTitle,
	} from "$lib/components/ui/item";
	import { Label } from "$lib/components/ui/label";

	type Props = {
		active: boolean;
		providerId?: string;
		providerName?: string;
		showResources?: boolean;
		showInspection?: boolean;
	};

	type LoadStatus = "idle" | "loading" | "ready" | "error" | "unsupported";

	let {
		active,
		providerId,
		providerName,
		showResources = true,
		showInspection = true,
	}: Props = $props();

	let resourcesStatus = $state<LoadStatus>("idle");
	let resources = $state<ProjectResources | null>(null);
	let resourcesError = $state<string | null>(null);
	let inspectionStatus = $state<LoadStatus>("idle");
	let inspection = $state<ProjectInspectionInfo | null>(null);
	let inspectionError = $state<string | null>(null);
	let memoryDraft = $state("");
	let diskDraft = $state("");
	let savePending = $state(false);
	let saveError = $state<string | null>(null);
	let saveSuccess = $state<string | null>(null);
	let inspectionDialogOpen = $state(false);
	const resourceDescription = $derived(
		providerName
			? `Adjust runtime resources for ${providerName}.`
			: "Adjust runtime resources for the current sandbox provider.",
	);
	const inspectionDescription = $derived(
		providerName
			? `Open the troubleshooting container for ${providerName}.`
			: "Open the troubleshooting container launched from the sandbox image.",
	);

	function hydrateDrafts(nextResources: ProjectResources) {
		memoryDraft = String(nextResources.vm.memoryMB / 1024);
		diskDraft = String(nextResources.vm.dataDiskGB);
	}

	async function loadResources(force = false) {
		if (resourcesStatus === "loading") {
			return;
		}
		if (
			!force &&
			(resourcesStatus === "ready" || resourcesStatus === "unsupported")
		) {
			return;
		}

		resourcesStatus = "loading";
		resourcesError = null;
		try {
			const nextResources = providerId
				? await api.getSandboxProviderResources(providerId)
				: await api.getProjectResources();
			resources = nextResources;
			hydrateDrafts(nextResources);
			resourcesStatus = "ready";
		} catch (error) {
			if (error instanceof ApiError && error.status === 501) {
				resourcesStatus = "unsupported";
				resourcesError = error.message;
				return;
			}
			resourcesStatus = "error";
			resourcesError =
				error instanceof Error ? error.message : "Failed to load resources.";
		}
	}

	async function loadInspection(force = false) {
		if (inspectionStatus === "loading") {
			return;
		}
		if (
			!force &&
			(inspectionStatus === "ready" || inspectionStatus === "unsupported")
		) {
			return;
		}

		inspectionStatus = "loading";
		inspectionError = null;
		try {
			inspection = providerId
				? await api.getSandboxProviderInspection(providerId)
				: await api.getProjectInspection();
			inspectionStatus = "ready";
		} catch (error) {
			if (error instanceof ApiError && error.status === 501) {
				inspectionStatus = "unsupported";
				inspectionError = error.message;
				return;
			}
			inspectionStatus = "error";
			inspectionError =
				error instanceof Error
					? error.message
					: "Failed to load inspection shell info.";
		}
	}

	async function refreshSystemSettings() {
		saveSuccess = null;
		if (showResources) {
			await loadResources(true);
		}
		if (showInspection) {
			await loadInspection(true);
		}
	}

	$effect(() => {
		if (!active) {
			return;
		}
		if (showResources) {
			void loadResources();
		}
		if (showInspection) {
			void loadInspection();
		}
	});

	const parsedMemory = $derived.by(() => Number.parseInt(memoryDraft, 10));
	const parsedDisk = $derived.by(() => Number.parseInt(diskDraft, 10));
	const parsedMemoryMB = $derived.by(() => parsedMemory * 1024);
	const hasValidMemoryDraft = $derived.by(
		() => Number.isInteger(parsedMemory) && parsedMemory > 0,
	);
	const hasValidDiskDraft = $derived.by(
		() => Number.isInteger(parsedDisk) && parsedDisk > 0,
	);
	const diskWouldDecrease = $derived.by(
		() =>
			resources !== null &&
			hasValidDiskDraft &&
			parsedDisk < resources.vm.dataDiskGB,
	);
	const isDirty = $derived.by(
		() =>
			resources !== null &&
			((hasValidMemoryDraft && parsedMemoryMB !== resources.vm.memoryMB) ||
				(hasValidDiskDraft && parsedDisk !== resources.vm.dataDiskGB)),
	);
	const canSave = $derived.by(
		() =>
			resourcesStatus === "ready" &&
			resources !== null &&
			hasValidMemoryDraft &&
			hasValidDiskDraft &&
			!diskWouldDecrease &&
			isDirty &&
			!savePending,
	);

	async function handleSave() {
		if (!canSave || resources === null) {
			return;
		}

		savePending = true;
		saveError = null;
		saveSuccess = null;
		try {
			const payload: { memoryMB?: number; dataDiskGB?: number } = {};
			if (parsedMemoryMB !== resources.vm.memoryMB) {
				payload.memoryMB = parsedMemoryMB;
			}
			if (parsedDisk !== resources.vm.dataDiskGB) {
				payload.dataDiskGB = parsedDisk;
			}

			const result = providerId
				? await api.updateSandboxProviderResources(providerId, payload)
				: await api.updateProjectResources(payload);
			resources = { provider: result.provider, vm: result.current };
			hydrateDrafts(resources);
			saveSuccess = result.restartRequired
				? "Saved. Restart the runtime by opening a new session to apply the change."
				: "Saved.";
		} catch (error) {
			saveError =
				error instanceof Error ? error.message : "Failed to update resources.";
		} finally {
			savePending = false;
		}
	}
</script>

<div class="space-y-4">
	{#if showResources}
		<Card class="gap-4 py-4">
			<CardHeader class="gap-1 border-b pb-4">
				<CardTitle class="flex items-center gap-2 text-sm">
					<ServerCogIcon class="size-4" />
					Resources
				</CardTitle>
				<CardDescription>
					{resourceDescription}
				</CardDescription>
				<CardAction>
					<Button variant="ghost" size="xs" onclick={refreshSystemSettings}>
						<RefreshCwIcon class="size-3.5" />
						Refresh
					</Button>
				</CardAction>
			</CardHeader>
			<CardContent class="space-y-3">
				{#if resourcesStatus === "loading" && resources === null}
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<Loader2Icon class="size-4 animate-spin" />
						Loading resources…
					</div>
				{:else if resourcesStatus === "unsupported"}
					<div
						class="rounded-md border border-border bg-muted/40 p-3 text-sm text-muted-foreground"
					>
						{resourcesError ??
							"Resource controls are not available for the current provider."}
					</div>
				{:else if resourcesStatus === "error"}
					<div
						class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
					>
						{resourcesError ?? "Failed to load resources."}
					</div>
				{:else if resources}
					<ItemGroup class="rounded-md border border-border">
						<Item size="sm">
							<ItemContent>
								<ItemTitle>Provider</ItemTitle>
								<ItemDescription>
									{resources.provider} · {resources.vm.cpuCount} CPU{resources
										.vm.cpuCount === 1
										? ""
										: "s"}
								</ItemDescription>
							</ItemContent>
						</Item>
						<ItemSeparator />
						<Item size="sm">
							<ItemContent>
								<ItemTitle>Memory</ItemTitle>
								<ItemDescription>
									Whole GiB only (1 GiB = 1024 MB). Changes apply after the next
									runtime restart.
								</ItemDescription>
							</ItemContent>
							<ItemActions class="ml-auto w-40 justify-end">
								<Label for="project-memory" class="sr-only">Memory (GiB)</Label>
								<Input
									id="project-memory"
									type="number"
									min="1"
									step="1"
									value={memoryDraft}
									oninput={(event) => {
										memoryDraft = (event.currentTarget as HTMLInputElement)
											.value;
										saveError = null;
										saveSuccess = null;
									}}
									class="text-right"
								/>
							</ItemActions>
						</Item>
						<ItemSeparator />
						<Item size="sm">
							<ItemContent>
								<ItemTitle>Data disk</ItemTitle>
								<ItemDescription>
									Increase-only. Existing data is preserved.
								</ItemDescription>
							</ItemContent>
							<ItemActions class="ml-auto w-40 justify-end">
								<Label for="project-disk" class="sr-only">Data disk (GB)</Label>
								<Input
									id="project-disk"
									type="number"
									min={String(resources.vm.dataDiskGB)}
									step="1"
									value={diskDraft}
									oninput={(event) => {
										diskDraft = (event.currentTarget as HTMLInputElement).value;
										saveError = null;
										saveSuccess = null;
									}}
									class="text-right"
								/>
							</ItemActions>
						</Item>
					</ItemGroup>

					{#if !hasValidMemoryDraft || !hasValidDiskDraft}
						<p class="text-xs text-destructive">
							Memory must be a positive whole number of GiB. Disk must be a
							positive whole number.
						</p>
					{:else if diskWouldDecrease}
						<p class="text-xs text-destructive">
							The data disk can only grow from {resources.vm.dataDiskGB} GB.
						</p>
					{:else}
						<p class="text-xs text-muted-foreground">
							Both memory and disk changes require the runtime to restart before
							they take effect.
						</p>
					{/if}

					{#if saveError}
						<p class="text-xs text-destructive">{saveError}</p>
					{/if}
					{#if saveSuccess}
						<p class="text-xs text-muted-foreground">{saveSuccess}</p>
					{/if}

					<div class="flex justify-end">
						<Button onclick={handleSave} disabled={!canSave} class="min-w-28">
							{#if savePending}
								<Loader2Icon class="size-4 animate-spin" />
								Saving…
							{:else}
								Save changes
							{/if}
						</Button>
					</div>
				{/if}
			</CardContent>
		</Card>
	{/if}

	{#if showInspection}
		<Card class="gap-4 py-4">
			<CardHeader class="gap-1 border-b pb-4">
				<CardTitle class="flex items-center gap-2 text-sm">
					<TerminalIcon class="size-4" />
					Inspection shell
				</CardTitle>
				<CardDescription>
					{inspectionDescription}
				</CardDescription>
			</CardHeader>
			<CardContent class="space-y-3">
				{#if inspectionStatus === "loading" && inspection === null}
					<div class="flex items-center gap-2 text-sm text-muted-foreground">
						<Loader2Icon class="size-4 animate-spin" />
						Loading inspection access…
					</div>
				{:else if inspectionStatus === "unsupported"}
					<div
						class="rounded-md border border-border bg-muted/40 p-3 text-sm text-muted-foreground"
					>
						{inspectionError ??
							"Inspection shell access is not available for the current provider."}
					</div>
				{:else if inspectionStatus === "error"}
					<div
						class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
					>
						{inspectionError ?? "Failed to load inspection shell access."}
					</div>
				{:else if inspection}
					<p class="text-xs text-muted-foreground">
						Use this shell for low-level troubleshooting. Access is limited to
						admins and owners.
					</p>

					<div class="flex justify-end">
						<Button
							variant="outline"
							onclick={() => {
								inspectionDialogOpen = true;
							}}
							disabled={!inspection.available}
						>
							Open shell
						</Button>
					</div>
				{/if}
			</CardContent>
		</Card>
	{/if}
</div>

{#if showInspection}
	<ProjectInspectionTerminalDialog
		open={inspectionDialogOpen}
		onOpenChange={(nextOpen) => {
			inspectionDialogOpen = nextOpen;
		}}
		projectId={PROJECT_ID}
		{providerId}
		title={providerName
			? `${providerName} inspection shell`
			: "Inspection shell"}
		description={providerName
			? `Troubleshooting shell for ${providerName}.`
			: "Troubleshooting shell for the inspection container."}
	/>
{/if}
