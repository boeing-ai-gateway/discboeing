<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import ClockIcon from "@lucide/svelte/icons/clock";
	import CodeIcon from "@lucide/svelte/icons/code";
	import FilesIcon from "@lucide/svelte/icons/files";
	import GitBranchIcon from "@lucide/svelte/icons/git-branch";
	import GitCompareIcon from "@lucide/svelte/icons/git-compare";
	import GitCommitIcon from "@lucide/svelte/icons/git-commit";
	import GlobeIcon from "@lucide/svelte/icons/globe";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import MonitorIcon from "@lucide/svelte/icons/monitor";
	import PlusIcon from "@lucide/svelte/icons/plus";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import SquareIcon from "@lucide/svelte/icons/square";
	import SquareTerminalIcon from "@lucide/svelte/icons/square-terminal";
	import TerminalIcon from "@lucide/svelte/icons/terminal";
	import type { Component } from "svelte";
	import { untrack } from "svelte";
	import { SvelteSet } from "svelte/reactivity";
	import {
		AlertDialog,
		AlertDialogAction,
		AlertDialogCancel,
		AlertDialogContent,
		AlertDialogDescription,
		AlertDialogFooter,
		AlertDialogHeader,
		AlertDialogTitle,
	} from "$lib/components/ui/alert-dialog";
	import {
		DropdownMenu,
		DropdownMenuContent,
		DropdownMenuGroup,
		DropdownMenuItem,
		DropdownMenuLabel,
		DropdownMenuSeparator,
		DropdownMenuSub,
		DropdownMenuSubContent,
		DropdownMenuSubTrigger,
		DropdownMenuTrigger,
	} from "$lib/components/ui/dropdown-menu";
	import {
		Dialog,
		DialogContent,
		DialogDescription,
		DialogFooter,
		DialogHeader,
		DialogTitle,
	} from "$lib/components/ui/dialog";
	import {
		Popover,
		PopoverContent,
		PopoverTrigger,
	} from "$lib/components/ui/popover";
	import { Button } from "$lib/components/ui/button";
	import { SplitDropdownButton } from "$lib/components/ui/split-dropdown-button";
	import { Textarea } from "$lib/components/ui/textarea";
	import SessionCommandCredentialsDialog from "$lib/components/app/parts/SessionCommandCredentialsDialog.svelte";
	import type { SessionCommandCredentialsDialogActions } from "$lib/components/app/parts/SessionCommandCredentialsDialog.svelte";
	import { getSSHPort } from "$lib/api-config";
	import type { AgentCommand } from "$lib/api-types";
	import { openUrl } from "$lib/shell";
	import type { IdeOption, JetBrainsIdeOption } from "$lib/app/ide-options";
	import {
		DESKTOP_SERVICE_ID,
		VSCODE_SERVICE_ID,
	} from "$lib/session/service-ids";
	import {
		closeAgentCommandCredentialDialog,
		confirmAgentCommandCredentialDialog,
		ensureSessionState,
		launchAgentCommandCredentialOAuthWizard,
		openFile,
		refreshAgentCommandCredentialDialogCredentials,
		runAgentCommand,
		selectAgentCommandCredentialOption,
		setAgentCommandCredentialCreateName,
		setAgentCommandCredentialCreateSecret,
		setAgentCommandCredentialValidityPreset,
		setAgentCommandCredentialValidityUnit,
		setAgentCommandCredentialValidityValue,
		setPreferredIde,
		startService as startSessionService,
		stopService as stopSessionService,
		submitThread,
	} from "$lib/context/commands/app-view";
	import { useContext } from "$lib/context/context.svelte";
	import type { ServiceItem } from "$lib/context/context.types";
	import { buildUserMessageParts } from "$lib/session/domains/session-domain.helpers";

	type Props = {
		sessionId: string;
	};

	type LucideIcon = Component<{ class?: string }>;
	type LucideIconModule = { default: LucideIcon };

	let { sessionId }: Props = $props();
	const context = useContext();
	const appEnvironment = context.view.app.environment;
	const lucideIconModules = import.meta.glob<LucideIconModule>(
		"../../../../node_modules/@lucide/svelte/dist/icons/*.js",
	);
	const staticCommandIcons: Record<string, LucideIcon> = {
		"git-branch": GitBranchIcon,
		"git-commit": GitCommitIcon,
	};
	const attemptedCommandIcons = new SvelteSet<string>();
	const preferences = $derived(context.view.app.preferences);
	const session = ensureSessionState(untrack(() => sessionId));
	const sessionView = session.ui;
	const serviceData = $derived(context.data.services.bySessionId[sessionId]);
	const fileData = $derived(context.data.files.bySessionId[sessionId]);
	const commandData = $derived(context.data.commands.bySessionId[sessionId]);
	const commandView = $derived(context.view.sessions[sessionId]?.commands);
	const commandCredentialDialog = $derived(
		commandView?.credentialDialog ?? null,
	);
	const commandCredentialDialogActions: SessionCommandCredentialsDialogActions =
		{
			close: () => closeAgentCommandCredentialDialog(session.sessionId),
			confirm: () => confirmAgentCommandCredentialDialog(session.sessionId),
			selectOption: (envVar, value) =>
				selectAgentCommandCredentialOption(session.sessionId, envVar, value),
			setCreateCredentialName: (envVar, value) =>
				setAgentCommandCredentialCreateName(session.sessionId, envVar, value),
			setCreateCredentialSecret: (envVar, value) =>
				setAgentCommandCredentialCreateSecret(session.sessionId, envVar, value),
			setValidityPreset: (envVar, value) =>
				setAgentCommandCredentialValidityPreset(
					session.sessionId,
					envVar,
					value,
				),
			setValidityValue: (envVar, value) =>
				setAgentCommandCredentialValidityValue(
					session.sessionId,
					envVar,
					value,
				),
			setValidityUnit: (envVar, value) =>
				setAgentCommandCredentialValidityUnit(session.sessionId, envVar, value),
			launchOAuthWizard: (envVar) =>
				launchAgentCommandCredentialOAuthWizard(session.sessionId, envVar),
			refreshCredentials: () =>
				refreshAgentCommandCredentialDialogCredentials(session.sessionId),
		};
	let loadedCommandIcons = $state<Record<string, LucideIcon>>({});
	let servicesPopoverOpen = $state(false);
	let mutatingServiceIds = $state<Record<string, boolean>>({});
	let addServiceDialogOpen = $state(false);
	let requestedServiceDescription = $state("");
	let submittingAddServicePrompt = $state(false);
	let learnMoreDialogOpen = $state(false);
	let submittingLearnMorePrompt = $state(false);
	const toolbarButtonTextClass = "text-sidebar-foreground/70";
	const sessionServices = $derived.by(() =>
		(serviceData?.items ?? []).filter(
			(service) =>
				service.id !== DESKTOP_SERVICE_ID && service.id !== VSCODE_SERVICE_ID,
		),
	);
	const hasRunningServices = $derived.by(() =>
		sessionServices.some((service) => service.status === "running"),
	);
	const vscodeAvailable = $derived.by(() =>
		(serviceData?.items ?? []).some(
			(service) => service.id === VSCODE_SERVICE_ID,
		),
	);

	function isJetBrainsIdeOption(
		option: IdeOption,
	): option is JetBrainsIdeOption {
		return option.family === "jetbrains";
	}

	function preferredIdeOption() {
		return (
			preferences.ideOptions.find(
				(option) => option.id === preferences.preferredIde,
			) ??
			preferences.ideOptions[0] ??
			null
		);
	}

	const WORKSPACE_PATH = "/home/discobot/workspace";

	const standardIdeOptions = $derived.by(() =>
		preferences.ideOptions.filter((option) => option.family === "standard"),
	);

	const jetbrainsIdeOptions = $derived.by(() =>
		preferences.ideOptions.filter(isJetBrainsIdeOption),
	);

	function getSSHHost() {
		if (typeof window === "undefined") {
			return "localhost";
		}

		const { hostname } = window.location;
		if (hostname === "127.0.0.1" || hostname === "::1") {
			return "localhost";
		}

		return hostname;
	}

	function buildJetBrainsIdeUrl(option: JetBrainsIdeOption, sessionId: string) {
		const host = getSSHHost();
		const port = getSSHPort();
		const params = new URLSearchParams({
			h: host,
			u: sessionId,
			p: String(port),
			launchIde: "true",
			ideHint: option.productCode,
			projectHint: WORKSPACE_PATH,
		});
		return `jetbrains://gateway/ssh/environment?${params.toString()}`;
	}

	function buildIdeUrl(option: IdeOption, sessionId: string) {
		const host = getSSHHost();
		const port = getSSHPort();
		if (option.family === "jetbrains") {
			return buildJetBrainsIdeUrl(option, sessionId);
		}
		if (option.id === "zed") {
			return `zed://ssh/${sessionId}@${host}:${port}${WORKSPACE_PATH}`;
		}
		if (option.id === "cursor") {
			return `cursor://vscode-remote/ssh-remote+${sessionId}@${host}:${port}${WORKSPACE_PATH}`;
		}
		return `vscode://vscode-remote/ssh-remote+${sessionId}@${host}:${port}${WORKSPACE_PATH}`;
	}

	const selectedIdeOption = $derived.by(() => preferredIdeOption());

	const ideLaunchUrl = $derived.by(() => {
		const activeSessionId = session.current?.id;
		if (!activeSessionId || !selectedIdeOption) {
			return null;
		}
		return buildIdeUrl(selectedIdeOption, activeSessionId);
	});

	const preferredIdeActionDisabled = $derived.by(() => ideLaunchUrl === null);

	async function openPreferredIde() {
		if (!ideLaunchUrl) {
			return;
		}

		await openUrl(ideLaunchUrl);
	}

	function toggleTerminal() {
		if (sessionView.activeView.kind === "terminal") {
			sessionView.openChat();
			return;
		}

		sessionView.openTerminal();
	}

	function toggleDesktop() {
		if (sessionView.activeView.kind === "desktop") {
			sessionView.openChat();
			return;
		}

		sessionView.openDesktop();
	}

	function toggleVSCode() {
		if (sessionView.activeView.kind === "vscode") {
			sessionView.openChat();
			return;
		}

		sessionView.openVSCode();
	}

	function toggleFiles() {
		if (sessionView.activeView.kind === "file") {
			sessionView.openChat();
			return;
		}

		void openFile(session.sessionId);
	}

	function toggleDiffReview() {
		if (sessionView.activeView.kind === "diff-review") {
			sessionView.openChat();
			return;
		}

		sessionView.openDiffReview();
	}

	function hasServiceWebPage(service: ServiceItem): boolean {
		return (
			typeof service.http === "number" || typeof service.https === "number"
		);
	}

	function hasServiceLogs(service: ServiceItem): boolean {
		return !service.passive;
	}

	function serviceIsMutating(service: ServiceItem): boolean {
		return mutatingServiceIds[service.id] === true;
	}

	function serviceTransitioning(service: ServiceItem): boolean {
		return service.status === "starting" || service.status === "stopping";
	}

	function canStartService(service: ServiceItem): boolean {
		return !service.passive && service.status === "stopped";
	}

	function canStopService(service: ServiceItem): boolean {
		return !service.passive && service.status === "running";
	}

	function canRestartService(service: ServiceItem): boolean {
		return !service.passive && service.status === "running";
	}

	function serviceStatusLabel(service: ServiceItem): string {
		if (service.passive) {
			return "External";
		}
		if (service.status === "starting") {
			return "Starting";
		}
		if (service.status === "stopping") {
			return "Stopping";
		}
		if (service.status === "running") {
			return "Running";
		}
		if (service.exitCode !== undefined && service.exitCode !== 0) {
			return `Stopped (${service.exitCode})`;
		}
		return "Stopped";
	}

	function serviceStatusClass(service: ServiceItem): string {
		if (service.passive || service.status === "running") {
			return "bg-green-500";
		}
		if (service.status === "starting" || service.status === "stopping") {
			return "bg-yellow-500";
		}
		if (service.exitCode !== undefined && service.exitCode !== 0) {
			return "bg-red-500";
		}
		return "bg-muted-foreground/40";
	}

	function openServicePanel(
		service: ServiceItem,
		viewMode: "preview" | "logs",
	) {
		if (viewMode === "logs" && !hasServiceLogs(service)) {
			return;
		}
		sessionView.openService(service.id, viewMode);
		servicesPopoverOpen = false;
	}

	function setServiceMutating(serviceId: string, mutating: boolean) {
		if (mutating) {
			mutatingServiceIds = { ...mutatingServiceIds, [serviceId]: true };
			return;
		}
		const next = { ...mutatingServiceIds };
		delete next[serviceId];
		mutatingServiceIds = next;
	}

	async function startService(service: ServiceItem) {
		if (!canStartService(service) || serviceIsMutating(service)) {
			return;
		}
		setServiceMutating(service.id, true);
		try {
			await startSessionService(sessionId, service.id);
		} finally {
			setServiceMutating(service.id, false);
		}
	}

	async function stopService(service: ServiceItem) {
		if (!canStopService(service) || serviceIsMutating(service)) {
			return;
		}
		setServiceMutating(service.id, true);
		try {
			await stopSessionService(sessionId, service.id);
		} finally {
			setServiceMutating(service.id, false);
		}
	}

	async function restartService(service: ServiceItem) {
		if (!canRestartService(service) || serviceIsMutating(service)) {
			return;
		}
		setServiceMutating(service.id, true);
		try {
			await stopSessionService(sessionId, service.id);
			await startSessionService(sessionId, service.id);
		} finally {
			setServiceMutating(service.id, false);
		}
	}

	function openAddServiceDialog() {
		requestedServiceDescription = "";
		addServiceDialogOpen = true;
		servicesPopoverOpen = false;
	}

	async function submitAddServicePrompt() {
		const description = requestedServiceDescription.trim();
		if (submittingAddServicePrompt || description.length === 0) {
			return;
		}

		submittingAddServicePrompt = true;
		try {
			await submitThread(
				session.sessionId,
				session.threads.selectedId ?? session.sessionId,
				{
					parts: buildUserMessageParts(
						`Create a Discobot service for this workspace based on the user's description below.

User description:
${description}

Please inspect the project first, then add or update the appropriate service file under .discobot/services. Use the documented Discobot service conventions:
- executable services have a shebang, executable bit, front matter, and a script body
- passive services declare an externally-managed HTTP/HTTPS port and have no body
- http/https fields enable web previews and proxy URLs
- non-passive service output is available in Discobot logs

When you are done, respond with:
- what service was created or changed
- the exact file path
- the full contents of the service file
- how to start, restart, or stop it from Discobot when applicable
- what web page, preview URL/path, or logs will be available`,
					),
				},
			);
			requestedServiceDescription = "";
			addServiceDialogOpen = false;
		} catch (error) {
			console.error("Failed to ask agent to create service:", error);
		} finally {
			submittingAddServicePrompt = false;
		}
	}

	async function submitLearnMorePrompt() {
		if (submittingLearnMorePrompt) {
			return;
		}

		submittingLearnMorePrompt = true;
		try {
			await submitThread(
				session.sessionId,
				session.threads.selectedId ?? session.sessionId,
				{
					parts: buildUserMessageParts(
						"Please explain Discobot services and hooks and how they could be used with the current application. Include concrete examples of services or hooks that would accelerate this project's development lifecycle, and mention what files would need to be added under `.discobot/services` or `.discobot/hooks`.",
					),
				},
			);
			learnMoreDialogOpen = false;
		} catch (error) {
			console.error("Failed to ask agent about Discobot services:", error);
		} finally {
			submittingLearnMorePrompt = false;
		}
	}

	const diffStats = $derived.by(() => {
		const stats = fileData?.diffStats ?? {
			additions: 0,
			deletions: 0,
			filesChanged: 0,
		};
		const additions = stats.additions;
		const filesChanged = stats.filesChanged;
		const deletions = stats.deletions;
		return { additions, deletions, filesChanged };
	});

	const uiCommands = $derived.by(() => commandData?.visibleItems ?? []);
	const primaryCommand = $derived.by(() => uiCommands[0] ?? null);
	const secondaryCommands = $derived.by(() => uiCommands.slice(1));
	const commandGroups = $derived.by(() => {
		const groups: Array<{
			key: string;
			label: string | null;
			commands: AgentCommand[];
		}> = [];
		for (const command of uiCommands) {
			const label = command.discobot?.group?.trim() || null;
			const key = label ?? "__ungrouped__";
			const existing = groups.find((group) => group.key === key);
			if (existing) {
				existing.commands.push(command);
				continue;
			}
			groups.push({ key, label, commands: [command] });
		}
		return groups;
	});
	const groupedSecondaryCommands = $derived.by(() =>
		commandGroups
			.map((group) => ({
				...group,
				commands: group.commands.filter(
					(command) => command !== primaryCommand,
				),
			}))
			.filter((group) => group.commands.length > 0),
	);

	$effect(() => {
		for (const iconName of uiCommands
			.map((command) => normalizeLucideIconName(command.discobot?.icon))
			.filter((iconName): iconName is string => iconName !== null)) {
			if (attemptedCommandIcons.has(iconName)) {
				continue;
			}
			attemptedCommandIcons.add(iconName);
			if (staticCommandIcons[iconName]) {
				continue;
			}
			const loader =
				lucideIconModules[
					`../../../../node_modules/@lucide/svelte/dist/icons/${iconName}.js`
				];
			if (!loader) {
				continue;
			}
			void loader().then((module) => {
				loadedCommandIcons = {
					...loadedCommandIcons,
					[iconName]: module.default,
				};
			});
		}
	});
	const operationState = $derived.by(() => {
		const isPending = session.current?.commitStatus === "pending";
		const activeCommandName = normalizeActiveCommandName(
			session.threads.selected?.activeCommand,
		);
		const showBusy = activeCommandName !== null || isPending;
		const activeCommand =
			uiCommands.find((command) => command.name === activeCommandName) ?? null;
		const primaryLabel =
			primaryCommand?.discobot?.label || primaryCommand?.name || "Run";
		return {
			activeCommandName,
			showSplitButton: secondaryCommands.length > 0,
			showPending: isPending,
			showBusy,
			buttonLabel: isPending
				? "Pending..."
				: activeCommand
					? activeCommand.discobot?.activeLabel?.trim() ||
						`${activeCommand.discobot?.label || activeCommand.name}...`
					: activeCommandName
						? `${activeCommandName}...`
						: primaryLabel,
		};
	});
	const operationDisabled = $derived.by(
		() =>
			!session.current ||
			!primaryCommand ||
			operationState.showBusy ||
			(commandData?.isSubmitting ?? false) ||
			(commandView?.credentialDialog.open ?? false),
	);

	function commandLabel(command: AgentCommand): string {
		return command.discobot?.label?.trim() || command.name;
	}

	function normalizeLucideIconName(
		name: string | null | undefined,
	): string | null {
		const trimmed = name?.trim() ?? "";
		if (trimmed.length === 0) {
			return null;
		}
		return trimmed
			.replace(/([a-z0-9])([A-Z])/g, "$1-$2")
			.replace(/[\s_]+/g, "-")
			.toLowerCase();
	}

	function commandIcon(command: AgentCommand): LucideIcon | null {
		const iconName = normalizeLucideIconName(command.discobot?.icon);
		if (!iconName) {
			return null;
		}
		return staticCommandIcons[iconName] ?? loadedCommandIcons[iconName] ?? null;
	}

	function normalizeActiveCommandName(
		name: string | null | undefined,
	): string | null {
		const trimmed = name?.trim() ?? "";
		return trimmed.length > 0 ? trimmed : null;
	}

	function commandBusy(command: AgentCommand) {
		return operationState.activeCommandName === command.name;
	}

	function handlePrimaryCommand() {
		if (!primaryCommand) {
			return;
		}
		void runAgentCommand(session.sessionId, primaryCommand).catch((error) => {
			console.error(`Failed to start ${primaryCommand.name}:`, error);
		});
	}

	function handleCommand(command: AgentCommand) {
		void runAgentCommand(session.sessionId, command).catch((error) => {
			console.error(`Failed to start ${command.name}:`, error);
		});
	}
</script>

<div
	class="flex h-full w-full min-w-0 items-center justify-end gap-2 bg-background px-2"
	data-desktop-drag-region
>
	{#if !appEnvironment.isMobile}
		<div
			class="desktop-no-drag inline-flex items-center overflow-hidden rounded-md bg-background p-0.5 shadow-xs"
		>
			<Button
				variant={sessionView.activeView.kind === "terminal"
					? "secondary"
					: "ghost"}
				size="xs"
				onclick={toggleTerminal}
				class={toolbarButtonTextClass}
				aria-label="Terminal"
				title="Terminal"
			>
				<SquareTerminalIcon class="size-3.5" />
				{#if !preferences.topBarIconOnly}
					Terminal
				{/if}
			</Button>
			<Button
				variant={sessionView.activeView.kind === "desktop"
					? "secondary"
					: "ghost"}
				size="xs"
				onclick={toggleDesktop}
				class={toolbarButtonTextClass}
				aria-label="Desktop"
				title="Desktop"
			>
				<MonitorIcon class="size-3.5" />
				{#if !preferences.topBarIconOnly}
					Desktop
				{/if}
			</Button>
			<Button
				variant={sessionView.activeView.kind === "vscode"
					? "secondary"
					: "ghost"}
				size="xs"
				onclick={toggleVSCode}
				disabled={!vscodeAvailable}
				class={toolbarButtonTextClass}
				aria-label="Editor"
				title="Editor"
			>
				<CodeIcon class="size-3.5" />
				{#if !preferences.topBarIconOnly}
					Editor
				{/if}
			</Button>
			<Button
				variant={sessionView.activeView.kind === "file" ? "secondary" : "ghost"}
				size="xs"
				onclick={toggleFiles}
				class={toolbarButtonTextClass}
				aria-label="Files"
				title="Files"
			>
				<FilesIcon class="size-3.5" />
				{#if !preferences.topBarIconOnly}
					Files
				{/if}
			</Button>
			{#if diffStats.filesChanged > 0}
				<Button
					variant={sessionView.activeView.kind === "diff-review"
						? "secondary"
						: "ghost"}
					size="xs"
					onclick={toggleDiffReview}
					class={`gap-1 ${toolbarButtonTextClass}`}
					aria-label={`Diff review: ${diffStats.additions} additions, ${diffStats.deletions} deletions, ${diffStats.filesChanged} files changed`}
					title="Diff review"
				>
					<GitCompareIcon class="size-3.5" />
					<span class="text-diff-add-line">+{diffStats.additions}</span>
					<span class="text-diff-remove-line">-{diffStats.deletions}</span>
					<span class="text-muted-foreground">{diffStats.filesChanged}</span>
				</Button>
			{/if}
			<Popover bind:open={servicesPopoverOpen}>
				<PopoverTrigger>
					{#snippet child({ props })}
						<Button
							{...props}
							variant={sessionView.activeView.kind === "services"
								? "secondary"
								: "ghost"}
							size="xs"
							class={`gap-1.5 ${toolbarButtonTextClass}`}
							aria-label="Run services"
							title="Run services"
						>
							<svg
								viewBox="0 0 16 16"
								aria-hidden="true"
								class={`size-3.5 fill-current ${hasRunningServices ? "text-green-500" : ""}`}
							>
								<path d="M4.5 3.25v9.5L12 8 4.5 3.25Z" />
							</svg>
							{#if !preferences.topBarIconOnly}
								Run
							{/if}
							<ChevronDownIcon class="size-3" />
						</Button>
					{/snippet}
				</PopoverTrigger>
				<PopoverContent align="end" class="w-[30rem] p-0">
					<div class="flex items-start gap-3 border-b px-3 py-2">
						<div class="min-w-0 flex-1">
							<p class="text-xs font-medium">Services</p>
							<p class="text-[11px] text-muted-foreground">
								Manage session services and open previews or logs.
							</p>
						</div>
						<Button
							variant="ghost"
							size="icon-xs"
							aria-label="Add service"
							title="Add service"
							onclick={openAddServiceDialog}
						>
							<PlusIcon class="size-3.5" />
						</Button>
					</div>
					{#if sessionServices.length === 0}
						<div class="p-3 text-sm text-muted-foreground">
							No services are available for this session.
						</div>
						<div class="border-t p-2">
							<Button
								variant="outline"
								size="xs"
								class="w-full"
								onclick={() => {
									learnMoreDialogOpen = true;
									servicesPopoverOpen = false;
								}}
							>
								Learn about services
							</Button>
						</div>
					{:else}
						<div class="max-h-80 overflow-y-auto p-1">
							{#each sessionServices as service (service.id)}
								<div
									class="flex min-h-10 items-center gap-2 rounded-md px-2 py-1.5 hover:bg-accent/60"
								>
									<div class="min-w-0 flex-1">
										<div class="flex min-w-0 items-center gap-2">
											<span class="truncate text-sm font-medium">
												{service.label}
											</span>
											{#if !service.passive}
												<span
													class="shrink-0 rounded border px-1.5 py-0.5 text-[10px] text-muted-foreground"
													title={serviceStatusLabel(service)}
												>
													<span
														class={`mr-1 inline-block size-1.5 rounded-full ${serviceStatusClass(service)}`}
													></span>
													{serviceStatusLabel(service)}
												</span>
											{/if}
										</div>
										{#if service.description}
											<p class="truncate text-xs text-muted-foreground">
												{service.description}
											</p>
										{/if}
									</div>
									<div class="flex shrink-0 items-center gap-0.5">
										<Button
											variant="ghost"
											size="icon-xs"
											disabled={!hasServiceWebPage(service)}
											aria-label={`Open ${service.label} web page`}
											title={hasServiceWebPage(service)
												? "Open web page"
												: "No web page"}
											onclick={() => openServicePanel(service, "preview")}
										>
											<GlobeIcon class="size-3.5" />
										</Button>
										{#if !service.passive}
											<Button
												variant="ghost"
												size="icon-xs"
												aria-label={`Open ${service.label} logs`}
												title="Open logs"
												onclick={() => openServicePanel(service, "logs")}
											>
												<TerminalIcon class="size-3.5" />
											</Button>
											{#if canStartService(service)}
												<Button
													variant="ghost"
													size="icon-xs"
													disabled={serviceIsMutating(service) ||
														serviceTransitioning(service)}
													aria-label={`Start ${service.label}`}
													title="Start"
													onclick={() => void startService(service)}
												>
													{#if serviceIsMutating(service) || serviceTransitioning(service)}
														<Loader2Icon class="size-3.5 animate-spin" />
													{:else}
														<svg
															viewBox="0 0 16 16"
															aria-hidden="true"
															class="size-3.5 fill-current"
														>
															<path d="M4.5 3.25v9.5L12 8 4.5 3.25Z" />
														</svg>
													{/if}
												</Button>
											{/if}
											{#if canRestartService(service)}
												<Button
													variant="ghost"
													size="icon-xs"
													disabled={serviceIsMutating(service) ||
														serviceTransitioning(service)}
													aria-label={`Restart ${service.label}`}
													title="Restart"
													onclick={() => void restartService(service)}
												>
													<RefreshCwIcon
														class={`size-3.5 ${serviceIsMutating(service) ? "animate-spin" : ""}`}
													/>
												</Button>
											{/if}
											{#if canStopService(service)}
												<Button
													variant="ghost"
													size="icon-xs"
													disabled={serviceIsMutating(service) ||
														serviceTransitioning(service)}
													aria-label={`Stop ${service.label}`}
													title="Stop"
													onclick={() => void stopService(service)}
												>
													{#if serviceIsMutating(service)}
														<Loader2Icon class="size-3.5 animate-spin" />
													{:else}
														<SquareIcon class="size-3.5 fill-current" />
													{/if}
												</Button>
											{/if}
										{/if}
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</PopoverContent>
			</Popover>
		</div>
	{:else if diffStats.filesChanged > 0}
		<div
			class="desktop-no-drag inline-flex items-center overflow-hidden rounded-md bg-background p-0.5 shadow-xs"
		>
			<Button
				variant={sessionView.activeView.kind === "diff-review"
					? "secondary"
					: "ghost"}
				size="xs"
				onclick={toggleDiffReview}
				class={`gap-1 ${toolbarButtonTextClass}`}
				aria-label={`Diff review: ${diffStats.additions} additions, ${diffStats.deletions} deletions, ${diffStats.filesChanged} files changed`}
				title="Diff review"
			>
				<GitCompareIcon class="size-3.5" />
				<span class="text-diff-add-line">+{diffStats.additions}</span>
				<span class="text-diff-remove-line">-{diffStats.deletions}</span>
				<span class="text-muted-foreground">{diffStats.filesChanged}</span>
			</Button>
		</div>
	{/if}

	{#if session.current && primaryCommand}
		{#if operationState.showSplitButton}
			<DropdownMenu>
				<div
					class="desktop-no-drag group inline-flex items-center overflow-hidden rounded-md bg-background p-0.5 text-sidebar-foreground/70 shadow-xs"
				>
					<Button
						variant="outline"
						size="xs"
						onclick={handlePrimaryCommand}
						disabled={operationDisabled}
						class="gap-1.5 rounded-l-[calc(var(--radius)-1px)] rounded-r-none border-0 bg-transparent shadow-none group-hover:bg-accent group-hover:text-accent-foreground dark:bg-transparent dark:group-hover:bg-accent/50"
						title={commandLabel(primaryCommand)}
						aria-label={operationState.buttonLabel}
					>
						{#if operationState.showPending}
							<ClockIcon class="size-3.5" />
						{:else if operationState.showBusy}
							<Loader2Icon class="size-3.5 animate-spin" />
						{:else}
							{@const PrimaryIcon = commandIcon(primaryCommand)}
							{#if PrimaryIcon}
								<PrimaryIcon class="size-3.5" />
							{:else}
								<GitCommitIcon class="size-3.5" />
							{/if}
						{/if}
						{#if !preferences.topBarIconOnly}
							{operationState.buttonLabel}
						{/if}
					</Button>
					<DropdownMenuTrigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="outline"
								size="xs"
								disabled={operationDisabled}
								class="rounded-r-[calc(var(--radius)-1px)] rounded-l-none border-0 bg-transparent px-2 shadow-none group-hover:bg-accent group-hover:text-accent-foreground dark:bg-transparent dark:group-hover:bg-accent/50"
								aria-label="More actions"
								title="More actions"
							>
								<ChevronDownIcon class="size-3.5" />
							</Button>
						{/snippet}
					</DropdownMenuTrigger>
				</div>
				<DropdownMenuContent align="end" sideOffset={8} class="min-w-[8rem]">
					{#each groupedSecondaryCommands as group, index (index)}
						{#if index > 0}
							<DropdownMenuSeparator />
						{/if}
						<DropdownMenuGroup>
							{#if group.label}
								<DropdownMenuLabel
									class="text-xs uppercase tracking-[0.16em] text-muted-foreground"
								>
									{group.label}
								</DropdownMenuLabel>
							{/if}
							{#each group.commands as command, __key1 (__key1)}
								<DropdownMenuItem
									onclick={() => handleCommand(command)}
									class="gap-2"
								>
									{#if commandBusy(command)}
										<Loader2Icon class="size-3.5 animate-spin" />
									{:else}
										{@const Icon = commandIcon(command)}
										{#if Icon}
											<Icon class="size-3.5" />
										{/if}
									{/if}
									{commandLabel(command)}
								</DropdownMenuItem>
							{/each}
						</DropdownMenuGroup>
					{/each}
				</DropdownMenuContent>
			</DropdownMenu>
		{:else}
			<Button
				variant="outline"
				size="xs"
				onclick={handlePrimaryCommand}
				disabled={operationDisabled}
				class="desktop-no-drag gap-1.5 border-0 text-sidebar-foreground/70 shadow-none"
				title={commandLabel(primaryCommand)}
				aria-label={operationState.buttonLabel}
			>
				{#if operationState.showPending}
					<ClockIcon class="size-3.5" />
				{:else if operationState.showBusy}
					<Loader2Icon class="size-3.5 animate-spin" />
				{:else}
					{@const PrimaryIcon = commandIcon(primaryCommand)}
					{#if PrimaryIcon}
						<PrimaryIcon class="size-3.5" />
					{:else}
						<GitCommitIcon class="size-3.5" />
					{/if}
				{/if}
				{#if !preferences.topBarIconOnly}
					{operationState.buttonLabel}
				{/if}
			</Button>
		{/if}
	{/if}

	{#if !appEnvironment.isMobile}
		<SplitDropdownButton
			class="desktop-no-drag text-sidebar-foreground/70"
			label={`Open ${selectedIdeOption?.label ?? "Cursor"}`}
			menuAriaLabel="Select preferred IDE"
			iconOnly={preferences.topBarIconOnly}
			onclick={openPreferredIde}
			primaryDisabled={preferredIdeActionDisabled}
			variant="ghost"
			size="xs"
			contentClass="min-w-[11rem]"
		>
			{#snippet icon()}
				{#if selectedIdeOption?.icon}
					<svg
						class="size-3.5"
						viewBox="0 0 24 24"
						aria-hidden="true"
						fill="currentColor"
					>
						<path d={selectedIdeOption.icon.path} />
					</svg>
				{:else}
					<CodeIcon class="size-3.5" />
				{/if}
			{/snippet}
			<DropdownMenuLabel
				class="text-xs uppercase tracking-[0.16em] text-muted-foreground"
			>
				Preferred IDE
			</DropdownMenuLabel>
			{#each standardIdeOptions as option, __key2 (__key2)}
				<DropdownMenuItem
					onclick={() => setPreferredIde(option.id)}
					class="justify-between gap-3"
				>
					<span class="flex items-center gap-2">
						<svg
							class="size-3.5"
							viewBox="0 0 24 24"
							aria-hidden="true"
							fill="currentColor"
						>
							<path d={option.icon.path} />
						</svg>
						<span>{option.label}</span>
					</span>
					{#if preferences.preferredIde === option.id}
						<CheckIcon class="size-3.5" aria-label="Selected" />
					{/if}
				</DropdownMenuItem>
			{/each}
			<DropdownMenuSub>
				<DropdownMenuSubTrigger class="gap-3">
					<span class="flex items-center gap-2">
						<svg
							class="size-3.5"
							viewBox="0 0 24 24"
							aria-hidden="true"
							fill="currentColor"
						>
							<path d={jetbrainsIdeOptions[0]?.icon.path} />
						</svg>
						<span>JetBrains</span>
					</span>
					{#if selectedIdeOption?.family === "jetbrains"}
						<CheckIcon class="size-3.5" aria-label="Selected" />
					{/if}
				</DropdownMenuSubTrigger>
				<DropdownMenuSubContent class="min-w-[13rem]">
					{#each jetbrainsIdeOptions as option, __key3 (__key3)}
						<DropdownMenuItem
							onclick={() => setPreferredIde(option.id)}
							class="justify-between gap-3"
						>
							<span class="flex items-center gap-2">
								<svg
									class="size-3.5"
									viewBox="0 0 24 24"
									aria-hidden="true"
									fill="currentColor"
								>
									<path d={option.icon.path} />
								</svg>
								<span>{option.label}</span>
							</span>
							{#if preferences.preferredIde === option.id}
								<CheckIcon class="size-3.5" aria-label="Selected" />
							{/if}
						</DropdownMenuItem>
					{/each}
				</DropdownMenuSubContent>
			</DropdownMenuSub>
		</SplitDropdownButton>
	{/if}

	{#if commandCredentialDialog}
		<SessionCommandCredentialsDialog
			dialog={commandCredentialDialog}
			actions={commandCredentialDialogActions}
		/>
	{/if}
</div>

<Dialog bind:open={addServiceDialogOpen}>
	<DialogContent class="sm:max-w-lg">
		<DialogHeader class="space-y-2">
			<DialogTitle>Add a Discobot service</DialogTitle>
			<DialogDescription class="space-y-2">
				<span class="block">
					Services are workspace automation files in
					<code>.discobot/services</code>. They can start dev servers,
					background workers, databases, or declare an externally managed
					service.
				</span>
				<span class="block">
					Services with HTTP/HTTPS ports get web previews, proxy URLs, and logs
					in Discobot.
				</span>
			</DialogDescription>
		</DialogHeader>
		<div class="space-y-2">
			<label for="add-service-description" class="text-sm font-medium">
				What service should Discobot start?
			</label>
			<Textarea
				id="add-service-description"
				bind:value={requestedServiceDescription}
				disabled={submittingAddServicePrompt}
				class="min-h-28"
				placeholder="For example: Start the Vite dev server for the UI on port 3100 and expose it as a preview."
			/>
			<p class="text-muted-foreground text-xs">
				The agent will inspect the project, create the service file, and report
				the service name, file path, full contents, web page, and logs.
			</p>
		</div>
		<DialogFooter>
			<Button
				variant="outline"
				disabled={submittingAddServicePrompt}
				onclick={() => {
					addServiceDialogOpen = false;
				}}
			>
				Cancel
			</Button>
			<Button
				disabled={submittingAddServicePrompt ||
					requestedServiceDescription.trim().length === 0}
				onclick={() => void submitAddServicePrompt()}
			>
				{submittingAddServicePrompt ? "Asking..." : "Ask agent to create"}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>

<AlertDialog bind:open={learnMoreDialogOpen}>
	<AlertDialogContent>
		<AlertDialogHeader>
			<AlertDialogTitle>Learn about services and hooks?</AlertDialogTitle>
			<AlertDialogDescription>
				This project does not have Discobot services configured yet. Would you
				like the agent to explain how services and hooks could help run,
				preview, and automate this application?
			</AlertDialogDescription>
		</AlertDialogHeader>
		<AlertDialogFooter>
			<AlertDialogCancel disabled={submittingLearnMorePrompt}>
				Cancel
			</AlertDialogCancel>
			<AlertDialogAction
				disabled={submittingLearnMorePrompt}
				onclick={(event) => {
					event.preventDefault();
					void submitLearnMorePrompt();
				}}
			>
				{submittingLearnMorePrompt ? "Asking..." : "Yes, ask agent"}
			</AlertDialogAction>
		</AlertDialogFooter>
	</AlertDialogContent>
</AlertDialog>
