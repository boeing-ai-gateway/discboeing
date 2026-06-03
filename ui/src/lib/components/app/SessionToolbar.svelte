<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import ClockIcon from "@lucide/svelte/icons/clock";
	import CodeIcon from "@lucide/svelte/icons/code";
	import FilesIcon from "@lucide/svelte/icons/files";
	import GitBranchIcon from "@lucide/svelte/icons/git-branch";
	import GitCompareIcon from "@lucide/svelte/icons/git-compare";
	import GitCommitIcon from "@lucide/svelte/icons/git-commit";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import MonitorIcon from "@lucide/svelte/icons/monitor";
	import ServerIcon from "@lucide/svelte/icons/server";
	import SquareTerminalIcon from "@lucide/svelte/icons/square-terminal";
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
	import { Button } from "$lib/components/ui/button";
	import { SplitDropdownButton } from "$lib/components/ui/split-dropdown-button";
	import SessionCommandCredentialsDialog from "$lib/components/app/parts/SessionCommandCredentialsDialog.svelte";
	import { getSSHPort } from "$lib/api-config";
	import type { AgentCommand } from "$lib/api-types";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { setSessionContext } from "$lib/context/session-context.svelte";
	import { IsMobile } from "$lib/hooks/is-mobile.svelte.js";
	import { openUrl } from "$lib/shell";
	import type { IdeOption, JetBrainsIdeOption } from "$lib/app/ide-options";
	import {
		DESKTOP_SERVICE_ID,
		VSCODE_SERVICE_ID,
	} from "$lib/session/service-ids";

	type Props = {
		sessionId: string;
	};

	type LucideIcon = Component<{ class?: string }>;
	type LucideIconModule = { default: LucideIcon };

	let { sessionId }: Props = $props();
	const app = useAppContext();
	const isMobile = new IsMobile(1024);
	const lucideIconModules = import.meta.glob<LucideIconModule>(
		"../../../../node_modules/@lucide/svelte/dist/icons/*.js",
	);
	const staticCommandIcons: Record<string, LucideIcon> = {
		"git-branch": GitBranchIcon,
		"git-commit": GitCommitIcon,
	};
	const attemptedCommandIcons = new SvelteSet<string>();
	const preferences = app.preferences;
	const session = app.ensureSession(untrack(() => sessionId));
	setSessionContext(session);
	const sessionView = session.ui;
	let loadedCommandIcons = $state<Record<string, LucideIcon>>({});
	let learnMoreDialogOpen = $state(false);
	let submittingLearnMorePrompt = $state(false);
	const toolbarButtonTextClass = "text-sidebar-foreground/70";
	const sessionServices = $derived.by(() =>
		session.services.list.filter(
			(service) =>
				service.id !== DESKTOP_SERVICE_ID && service.id !== VSCODE_SERVICE_ID,
		),
	);
	const vscodeAvailable = $derived.by(() =>
		session.services.list.some((service) => service.id === VSCODE_SERVICE_ID),
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

		void session.files.open();
	}

	function toggleDiffReview() {
		if (sessionView.activeView.kind === "diff-review") {
			sessionView.openChat();
			return;
		}

		sessionView.openDiffReview();
	}

	function toggleServices() {
		if (sessionView.activeView.kind === "services") {
			sessionView.openChat();
			return;
		}

		if (sessionServices.length === 0) {
			learnMoreDialogOpen = true;
			return;
		}

		sessionView.openServices();
	}

	async function submitLearnMorePrompt() {
		if (submittingLearnMorePrompt) {
			return;
		}

		submittingLearnMorePrompt = true;
		try {
			await session.submit(
				"Please explain Discobot services and hooks and how they could be used with the current application. Include concrete examples of services or hooks that would accelerate this project's development lifecycle, and mention what files would need to be added under `.discobot/services` or `.discobot/hooks`.",
			);
			learnMoreDialogOpen = false;
		} catch (error) {
			console.error("Failed to ask agent about Discobot services:", error);
		} finally {
			submittingLearnMorePrompt = false;
		}
	}

	const diffStats = $derived.by(() => {
		const stats = session.files.diffStats;
		const additions = stats.additions;
		const filesChanged = stats.filesChanged;
		const deletions = stats.deletions;
		return { additions, deletions, filesChanged };
	});

	const uiCommands = $derived.by(() => session.commands.uiVisible);
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
			session.commands.isSubmitting ||
			session.commands.credentialDialog.open,
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
		void session.commands.run(primaryCommand).catch((error) => {
			console.error(`Failed to start ${primaryCommand.name}:`, error);
		});
	}

	function handleCommand(command: AgentCommand) {
		void session.commands.run(command).catch((error) => {
			console.error(`Failed to start ${command.name}:`, error);
		});
	}
</script>

<div
	class="flex h-full w-full min-w-0 items-center justify-end gap-2 bg-background px-2"
	data-desktop-drag-region
>
	{#if !isMobile.current}
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
			<Button
				variant={sessionView.activeView.kind === "services"
					? "secondary"
					: "ghost"}
				size="xs"
				onclick={toggleServices}
				class={toolbarButtonTextClass}
				aria-label="Services"
				title="Services"
			>
				<ServerIcon class="size-3.5" />
				{#if !preferences.topBarIconOnly}
					Services
				{/if}
			</Button>
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

	{#if !isMobile.current}
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
					onclick={() => preferences.setPreferredIde(option.id)}
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
							onclick={() => preferences.setPreferredIde(option.id)}
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

	<SessionCommandCredentialsDialog dialog={session.commands.credentialDialog} />
</div>

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
