<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import FolderIcon from "@lucide/svelte/icons/folder";
	import FolderOpenIcon from "@lucide/svelte/icons/folder-open";
	import GitCommitIcon from "@lucide/svelte/icons/git-commit";
	import PackageIcon from "@lucide/svelte/icons/package";
	import { onDestroy, onMount, tick, untrack } from "svelte";
	import { api } from "$lib/api-client";
	import type { Workspace, WorkspaceValidationResult } from "$lib/api-types";
	import type { WorkspaceSelectionResult } from "$lib/components/app/conversation-composer.types";
	import GithubIcon from "$lib/components/ui/icons/GithubIcon.svelte";
	import { isDesktopShell, pickDirectory } from "$lib/shell";
	import { InputGroupButton } from "$lib/components/ui/input-group";
	import { Input } from "$lib/components/ui/input";
	import { NativeSelect } from "$lib/components/ui/native-select";
	import { useContext } from "$lib/context";
	import {
		createCollectionCache,
		createReadyStatus,
		upsertById,
	} from "$lib/context/cache";
	import { getPendingWorkspaceRequiresSourceInput } from "$lib/pending-workspace-helpers";

	let {
		sessionId,
		fullWidth = false,
	}: { sessionId: string; fullWidth?: boolean } = $props();

	const context = useContext();
	const mountedSessionId = untrack(() => sessionId);
	void context.commands.view.mountSessionView(mountedSessionId);
	const sessionView = $derived(context.view.sessions[mountedSessionId]!);
	const pendingWorkspace = $derived(sessionView.pendingWorkspace);

	let showWorkspaceSuggestions = $state(false);
	let selectedWorkspaceSuggestionIndex = $state(-1);
	let hasUserSelectedWorkspace = $state(false);
	let hasInitializedSelection = $state(false);
	let workspaceSelectRef = $state<HTMLSelectElement | null>(null);
	let workspaceSourceInputRef = $state<HTMLInputElement | null>(null);
	let shouldFocusWorkspaceSourceInput = $state(false);

	let workspaceValidationDebounce: ReturnType<typeof setTimeout> | null = null;
	let workspaceValidationRequestId = 0;
	let workspaceSuggestionsCloseTimeout: ReturnType<typeof setTimeout> | null =
		null;
	let destroyed = false;

	const availableWorkspaces = $derived.by(() =>
		context.data.workspaces.allIds
			.map((workspaceId) => context.data.workspaces.byId[workspaceId] ?? null)
			.filter((workspace): workspace is Workspace => workspace !== null),
	);
	const loadingWorkspaces = $derived.by(
		() => context.data.workspaces.status.state === "loading",
	);
	const requiresSourceInput = $derived.by(() =>
		getPendingWorkspaceRequiresSourceInput(pendingWorkspace),
	);
	const selectedExistingWorkspace = $derived.by(() => {
		if (!pendingWorkspace.option.startsWith("existing:")) {
			return null;
		}

		const selectedWorkspaceId = pendingWorkspace.option.slice(
			"existing:".length,
		);
		return (
			availableWorkspaces.find(
				(workspace) => workspace.id === selectedWorkspaceId,
			) ?? null
		);
	});
	const existingWorkspaceIsGithub = $derived.by(
		() =>
			selectedExistingWorkspace !== null &&
			isGithubWorkspace(selectedExistingWorkspace),
	);
	const workspaceSourceType = $derived.by(() =>
		pendingWorkspace.option === "git-repo" ? "git" : "local",
	);
	const workspaceSuggestions = $derived.by(
		() => pendingWorkspace.validation?.suggestions ?? [],
	);
	const showLocalDirectoryPicker = $derived.by(
		() => isDesktopShell() && workspaceSourceType === "local",
	);

	function setPendingWorkspaceOption(value: string) {
		pendingWorkspace.option = value;
	}

	function setPendingWorkspaceBranch(value: string) {
		pendingWorkspace.branch = value;
	}

	function setPendingWorkspaceSourceInput(value: string) {
		pendingWorkspace.sourceInput = value;
	}

	function setPendingWorkspaceValidation(
		value: WorkspaceValidationResult | null,
	) {
		pendingWorkspace.validation = value;
	}

	function setPendingWorkspaceValidating(value: boolean) {
		pendingWorkspace.validating = value;
	}

	function setPendingWorkspaceSetupMessage(value: string | null) {
		pendingWorkspace.setupMessage = value;
	}

	function resetPendingWorkspaceSetup() {
		pendingWorkspace.option = "new-workspace";
		pendingWorkspace.branch = "";
		pendingWorkspace.sourceInput = "";
		pendingWorkspace.validation = null;
		pendingWorkspace.validating = false;
		pendingWorkspace.setupMessage = null;
		pendingWorkspace.sandboxProviderId = "";
	}

	function shortenHomePath(path: string): string {
		const homeMatch = path.match(/^(\/home\/[^/]+|\/Users\/[^/]+)(\/.*)?$/);
		if (homeMatch) {
			const rest = homeMatch[2] || "";
			return `~${rest}`;
		}
		return path;
	}

	function getWorkspaceOptionLabel(workspace: Workspace): string {
		const displayName = workspace.displayName?.trim();
		if (displayName) {
			return displayName;
		}
		if (workspace.sourceType === "managed") {
			return "Unnamed Workspace";
		}
		return shortenHomePath(workspace.path);
	}

	function isGithubWorkspace(workspace: Workspace): boolean {
		if (workspace.sourceType !== "git") {
			return false;
		}

		const value =
			`${workspace.path} ${workspace.displayName || ""}`.toLowerCase();
		return value.includes("github.com") || value.includes("github");
	}

	async function loadWorkspacesIntoCache() {
		const response = await api.getWorkspaces();
		const state = createCollectionCache<Workspace>();
		for (const workspace of response.workspaces) {
			upsertById(state, workspace.id, workspace);
		}
		state.status = createReadyStatus();
		context.data.workspaces = state;
	}

	function isGithubRepoInput(value: string): boolean {
		const trimmed = value.trim().toLowerCase();
		if (trimmed.length === 0) {
			return false;
		}

		if (trimmed.startsWith("github.com/") || trimmed.includes("github.com")) {
			return true;
		}

		return /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})\/[A-Za-z0-9._-]+$/.test(trimmed);
	}

	function clearWorkspaceValidationDebounce() {
		if (!workspaceValidationDebounce) {
			return;
		}

		clearTimeout(workspaceValidationDebounce);
		workspaceValidationDebounce = null;
	}

	function clearWorkspaceSuggestionsCloseTimeout() {
		if (!workspaceSuggestionsCloseTimeout) {
			return;
		}

		clearTimeout(workspaceSuggestionsCloseTimeout);
		workspaceSuggestionsCloseTimeout = null;
	}

	function cancelWorkspaceValidation() {
		clearWorkspaceValidationDebounce();
		workspaceValidationRequestId += 1;
	}

	function focusWorkspaceSourceInput(): boolean {
		const input = workspaceSourceInputRef;
		if (!input || input.getClientRects().length === 0 || input.disabled) {
			return false;
		}

		input.focus({ preventScroll: true });
		return document.activeElement === input;
	}

	function requestWorkspaceSourceInputFocus() {
		shouldFocusWorkspaceSourceInput = true;
	}

	function resetToWorkspaceDropdown() {
		showWorkspaceSuggestions = false;
		selectedWorkspaceSuggestionIndex = -1;
		setPendingWorkspaceOption("new-workspace");
		setPendingWorkspaceBranch("");
		setPendingWorkspaceSourceInput("");
		setPendingWorkspaceSetupMessage(null);
		setPendingWorkspaceValidation(null);
		setPendingWorkspaceValidating(false);
		cancelWorkspaceValidation();
		clearWorkspaceSuggestionsCloseTimeout();
	}

	function openWorkspaceDropdown() {
		const select = workspaceSelectRef as
			| (HTMLSelectElement & { showPicker?: () => void })
			| null;
		if (!select || select.disabled) {
			return;
		}

		select.focus();
		if (typeof select.showPicker === "function") {
			try {
				select.showPicker();
				return;
			} catch {
				// Fall back to a click when showPicker is unavailable.
			}
		}

		select.click();
	}

	async function handleWorkspaceIconClick() {
		resetToWorkspaceDropdown();
		await tick();
		openWorkspaceDropdown();
	}

	function handleWorkspaceOptionChange(nextOption: string) {
		hasUserSelectedWorkspace = true;
		setPendingWorkspaceOption(nextOption);
		setPendingWorkspaceBranch("");
		setPendingWorkspaceSetupMessage(null);
		setPendingWorkspaceValidation(null);
		setPendingWorkspaceValidating(false);
		cancelWorkspaceValidation();
		clearWorkspaceSuggestionsCloseTimeout();
		showWorkspaceSuggestions = false;
		selectedWorkspaceSuggestionIndex = -1;

		if (nextOption === "local-directory" || nextOption === "git-repo") {
			setPendingWorkspaceSourceInput("");
			requestWorkspaceSourceInputFocus();
			return;
		}

		setPendingWorkspaceSourceInput("");
	}

	function resetWorkspaceValidationState(clearSuggestions = false) {
		cancelWorkspaceValidation();
		setPendingWorkspaceValidating(false);
		setPendingWorkspaceValidation(null);
		if (clearSuggestions) {
			showWorkspaceSuggestions = false;
		}
		selectedWorkspaceSuggestionIndex = -1;
	}

	function scheduleWorkspaceValidation() {
		if (!requiresSourceInput) {
			resetWorkspaceValidationState(true);
			return;
		}

		const currentInput = pendingWorkspace.sourceInput;
		if (currentInput.trim().length === 0) {
			resetWorkspaceValidationState();
			return;
		}

		const currentSourceType = workspaceSourceType;
		clearWorkspaceValidationDebounce();
		setPendingWorkspaceValidating(true);
		const requestId = workspaceValidationRequestId + 1;
		workspaceValidationRequestId = requestId;

		workspaceValidationDebounce = setTimeout(async () => {
			try {
				const result = await api.validateWorkspace({
					path: currentInput,
					sourceType: currentSourceType,
				});

				if (destroyed || workspaceValidationRequestId !== requestId) {
					return;
				}

				setPendingWorkspaceValidation(result);
			} catch (error) {
				if (destroyed || workspaceValidationRequestId !== requestId) {
					return;
				}

				setPendingWorkspaceValidation({
					path: currentInput,
					sourceType: currentSourceType,
					valid: false,
					classification: "invalid",
					error:
						error instanceof Error
							? error.message
							: "Failed to validate workspace.",
					suggestions: [],
				});
			} finally {
				if (!destroyed && workspaceValidationRequestId === requestId) {
					setPendingWorkspaceValidating(false);
				}
			}
		}, 250);
	}

	function handleWorkspaceSourceInputChange(value: string) {
		setPendingWorkspaceSourceInput(value);
		setPendingWorkspaceSetupMessage(null);
		showWorkspaceSuggestions = true;
		selectedWorkspaceSuggestionIndex = -1;
		scheduleWorkspaceValidation();
	}

	function handleWorkspaceSourceFocus() {
		clearWorkspaceSuggestionsCloseTimeout();
		showWorkspaceSuggestions = true;
	}

	async function handleLocalDirectoryPickerClick() {
		try {
			const selectedDirectory = await pickDirectory();
			if (!selectedDirectory) {
				requestWorkspaceSourceInputFocus();
				return;
			}

			handleWorkspaceSourceInputChange(selectedDirectory);
			requestWorkspaceSourceInputFocus();
		} catch (error) {
			setPendingWorkspaceSetupMessage(
				error instanceof Error
					? error.message
					: `Failed to open the directory picker: ${String(error)}`,
			);
			requestWorkspaceSourceInputFocus();
		}
	}

	function handleWorkspaceSourceBlur() {
		clearWorkspaceSuggestionsCloseTimeout();
		workspaceSuggestionsCloseTimeout = setTimeout(() => {
			showWorkspaceSuggestions = false;
			selectedWorkspaceSuggestionIndex = -1;
			workspaceSuggestionsCloseTimeout = null;
		}, 120);
	}

	function applyWorkspaceSuggestion(suggestionValue: string) {
		setPendingWorkspaceSourceInput(suggestionValue);
		setPendingWorkspaceSetupMessage(null);
		showWorkspaceSuggestions = false;
		selectedWorkspaceSuggestionIndex = -1;
		scheduleWorkspaceValidation();
		focusWorkspaceSourceInput();
	}

	function acceptWorkspaceSuggestion(preferFirst: boolean): boolean {
		if (!showWorkspaceSuggestions || workspaceSuggestions.length === 0) {
			return false;
		}

		let suggestionIndex = selectedWorkspaceSuggestionIndex;
		if (suggestionIndex < 0 && preferFirst) {
			suggestionIndex = 0;
		}

		if (suggestionIndex < 0 || suggestionIndex >= workspaceSuggestions.length) {
			return false;
		}

		applyWorkspaceSuggestion(workspaceSuggestions[suggestionIndex].value);
		return true;
	}

	function handleSourceKeydown(event: KeyboardEvent) {
		if (event.key === "ArrowDown") {
			if (!showWorkspaceSuggestions || workspaceSuggestions.length === 0) {
				return;
			}
			event.preventDefault();
			selectedWorkspaceSuggestionIndex = Math.min(
				selectedWorkspaceSuggestionIndex + 1,
				workspaceSuggestions.length - 1,
			);
			return;
		}

		if (event.key === "ArrowUp") {
			if (!showWorkspaceSuggestions || workspaceSuggestions.length === 0) {
				return;
			}
			event.preventDefault();
			selectedWorkspaceSuggestionIndex = Math.max(
				selectedWorkspaceSuggestionIndex - 1,
				-1,
			);
			return;
		}

		if (event.key === "Escape") {
			event.preventDefault();
			resetToWorkspaceDropdown();
			return;
		}

		if (event.key === "Enter") {
			if (acceptWorkspaceSuggestion(true)) {
				event.preventDefault();
			}
			showWorkspaceSuggestions = false;
			selectedWorkspaceSuggestionIndex = -1;
			return;
		}

		if (event.key === "Tab") {
			if (acceptWorkspaceSuggestion(true)) {
				event.preventDefault();
			}
		}
	}

	export function resetForNewSession() {
		hasUserSelectedWorkspace = false;
		hasInitializedSelection = false;
		resetPendingWorkspaceSetup();
		cancelWorkspaceValidation();
		clearWorkspaceSuggestionsCloseTimeout();
		showWorkspaceSuggestions = false;
		selectedWorkspaceSuggestionIndex = -1;
	}

	export async function getWorkspaceSelection(): Promise<WorkspaceSelectionResult> {
		if (pendingWorkspace.option.startsWith("existing:")) {
			const workspaceId = pendingWorkspace.option.slice("existing:".length);
			if (
				!availableWorkspaces.some((workspace) => workspace.id === workspaceId)
			) {
				setPendingWorkspaceSetupMessage("Select an existing workspace.");
				return {
					ready: false,
					workspaceId: null,
					workspaceType: null,
					workspacePath: null,
				};
			}

			setPendingWorkspaceSetupMessage(null);
			return {
				ready: true,
				workspaceId,
				workspaceType: null,
				workspacePath: null,
			};
		}

		if (!requiresSourceInput) {
			setPendingWorkspaceSetupMessage(null);
			return {
				ready: true,
				workspaceId: null,
				workspaceType: null,
				workspacePath: null,
			};
		}

		if (pendingWorkspace.validating) {
			setPendingWorkspaceSetupMessage("Validating workspace...");
			return {
				ready: false,
				workspaceId: null,
				workspaceType: null,
				workspacePath: null,
			};
		}

		if (
			!pendingWorkspace.validation ||
			pendingWorkspace.validation.sourceType !== workspaceSourceType
		) {
			setPendingWorkspaceSetupMessage(
				workspaceSourceType === "git"
					? "Enter a Git repository URL."
					: "Enter a local directory path.",
			);
			return {
				ready: false,
				workspaceId: null,
				workspaceType: null,
				workspacePath: null,
			};
		}

		if (!pendingWorkspace.validation.valid) {
			setPendingWorkspaceSetupMessage(
				pendingWorkspace.validation.error ||
					(workspaceSourceType === "git"
						? "Enter a valid Git repository URL."
						: "Enter a valid local directory path."),
			);
			return {
				ready: false,
				workspaceId: null,
				workspaceType: null,
				workspacePath: null,
			};
		}

		const normalizedPath = pendingWorkspace.validation.path.trim();
		if (normalizedPath.length === 0) {
			setPendingWorkspaceSetupMessage(
				workspaceSourceType === "git"
					? "Enter a Git repository URL."
					: "Enter a local directory path.",
			);
			return {
				ready: false,
				workspaceId: null,
				workspaceType: null,
				workspacePath: null,
			};
		}

		setPendingWorkspaceSetupMessage(null);
		return {
			ready: true,
			workspaceId: null,
			workspaceType: workspaceSourceType,
			workspacePath: normalizedPath,
		};
	}

	function syncPendingWorkspaceSelection() {
		const workspacesList = availableWorkspaces;
		if (workspacesList.length === 0) {
			if (pendingWorkspace.option.startsWith("existing:")) {
				setPendingWorkspaceOption("new-workspace");
			}
			return false;
		}

		const preferredWorkspace =
			workspacesList.find((workspace) => workspace.status === "ready") ||
			workspacesList[0];
		if (!preferredWorkspace) {
			return false;
		}

		if (pendingWorkspace.option.startsWith("existing:")) {
			const selectedWorkspaceId = pendingWorkspace.option.slice(
				"existing:".length,
			);
			if (
				!workspacesList.some(
					(workspace) => workspace.id === selectedWorkspaceId,
				) &&
				!hasUserSelectedWorkspace
			) {
				setPendingWorkspaceOption(`existing:${preferredWorkspace.id}`);
				return true;
			}
			return false;
		}

		if (
			!hasUserSelectedWorkspace &&
			!hasInitializedSelection &&
			pendingWorkspace.option === "new-workspace"
		) {
			hasInitializedSelection = true;
			setPendingWorkspaceOption(`existing:${preferredWorkspace.id}`);
			return true;
		}
		return false;
	}

	onMount(() => {
		void (async () => {
			if (context.data.workspaces.status.state === "idle") {
				await loadWorkspacesIntoCache();
				if (destroyed) {
					return;
				}
			}
			syncPendingWorkspaceSelection();
			scheduleWorkspaceValidation();
			if (requiresSourceInput) {
				requestWorkspaceSourceInputFocus();
			}
		})();
	});

	$effect(() => {
		if (!requiresSourceInput || !shouldFocusWorkspaceSourceInput) {
			return;
		}

		let cancelled = false;
		let attemptCount = 0;

		const tryFocus = () => {
			if (cancelled) {
				return;
			}

			attemptCount += 1;
			if (focusWorkspaceSourceInput() || attemptCount >= 4) {
				shouldFocusWorkspaceSourceInput = false;
				return;
			}

			requestAnimationFrame(() => {
				window.setTimeout(tryFocus, 0);
			});
		};

		void tick().then(() => {
			requestAnimationFrame(() => {
				window.setTimeout(tryFocus, 0);
			});
		});

		return () => {
			cancelled = true;
		};
	});

	onDestroy(() => {
		destroyed = true;
		cancelWorkspaceValidation();
		clearWorkspaceSuggestionsCloseTimeout();
	});
</script>

<div class="flex items-center gap-1.5 {fullWidth ? 'flex-1' : ''}">
	{#if requiresSourceInput}
		<button
			type="button"
			class="-m-1 inline-flex items-center justify-center rounded-sm p-1 text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
			aria-label="Return to workspace dropdown"
			title="Return to workspace dropdown"
			onclick={() => {
				void handleWorkspaceIconClick();
			}}
		>
			{#if workspaceSourceType === "local"}
				<FolderIcon class="size-4" />
			{:else if isGithubRepoInput(pendingWorkspace.sourceInput)}
				<GithubIcon class="size-4" />
			{:else}
				<GitCommitIcon class="size-4" />
			{/if}
		</button>
	{:else if pendingWorkspace.option === "local-directory"}
		<FolderIcon class="size-4 text-muted-foreground" />
	{:else if pendingWorkspace.option === "git-repo"}
		<GithubIcon class="size-4 text-muted-foreground" />
	{:else if pendingWorkspace.option.startsWith("existing:")}
		{#if selectedExistingWorkspace?.sourceType === "managed"}
			<PackageIcon class="size-4 text-muted-foreground" />
		{:else if selectedExistingWorkspace?.sourceType === "local"}
			<FolderIcon class="size-4 text-muted-foreground" />
		{:else if existingWorkspaceIsGithub}
			<GithubIcon class="size-4 text-muted-foreground" />
		{:else}
			<GitCommitIcon class="size-4 text-muted-foreground" />
		{/if}
	{:else}
		<FolderIcon class="size-4 text-muted-foreground" />
	{/if}

	{#if requiresSourceInput}
		<div class="relative {fullWidth ? 'flex-1' : ''}">
			<div class="flex items-center gap-1.5">
				<Input
					id="session-setup-source-inline"
					aria-label={workspaceSourceType === "local"
						? "Local directory path"
						: "Git repository URL"}
					aria-autocomplete="list"
					aria-controls="workspace-source-suggestions"
					aria-expanded={showWorkspaceSuggestions &&
						workspaceSuggestions.length > 0}
					aria-activedescendant={selectedWorkspaceSuggestionIndex >= 0
						? `workspace-source-suggestion-${selectedWorkspaceSuggestionIndex}`
						: undefined}
					role="combobox"
					bind:ref={workspaceSourceInputRef}
					class="h-8 {fullWidth
						? 'w-full'
						: 'w-[320px]'} min-w-0 flex-1 text-xs"
					value={pendingWorkspace.sourceInput}
					placeholder={workspaceSourceType === "local"
						? "~/projects/my-app"
						: "https://github.com/org/repo or org/repo"}
					onfocus={handleWorkspaceSourceFocus}
					onblur={handleWorkspaceSourceBlur}
					oninput={(event) => {
						handleWorkspaceSourceInputChange(
							(event.currentTarget as HTMLInputElement).value,
						);
					}}
					onkeydown={handleSourceKeydown}
				/>

				{#if showLocalDirectoryPicker}
					<InputGroupButton
						type="button"
						size="icon-sm"
						variant="ghost"
						aria-label="Choose local directory"
						title="Choose local directory"
						onclick={() => {
							void handleLocalDirectoryPickerClick();
						}}
					>
						<FolderOpenIcon class="size-4" />
					</InputGroupButton>
				{/if}
			</div>

			{#if showWorkspaceSuggestions && workspaceSuggestions.length > 0}
				<div
					id="workspace-source-suggestions"
					role="listbox"
					aria-label="Workspace suggestions"
					class="absolute right-0 top-full z-50 mt-1 max-h-56 {fullWidth
						? 'w-full'
						: 'w-[320px]'} overflow-y-auto rounded-md border border-border bg-popover shadow-lg"
				>
					{#each workspaceSuggestions as suggestion, index (suggestion.value)}
						<button
							id={`workspace-source-suggestion-${index}`}
							type="button"
							role="option"
							aria-selected={index === selectedWorkspaceSuggestionIndex}
							class={`flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-xs hover:bg-accent ${index === selectedWorkspaceSuggestionIndex ? "bg-accent" : ""} ${suggestion.valid ? "" : "opacity-70"}`}
							onmouseenter={() => {
								selectedWorkspaceSuggestionIndex = index;
							}}
							onmousedown={(event) => {
								event.preventDefault();
								applyWorkspaceSuggestion(suggestion.value);
							}}
						>
							<span class="truncate font-mono">{suggestion.value}</span>
							{#if suggestion.valid}
								<CheckIcon class="size-3.5 text-primary" />
							{/if}
						</button>
					{/each}
				</div>
			{/if}
		</div>
	{:else}
		<NativeSelect
			id="session-setup-workspace-inline"
			aria-label="Workspace"
			bind:ref={workspaceSelectRef}
			class="h-8 {fullWidth ? 'w-full' : 'w-[320px]'} min-w-0 text-xs"
			value={pendingWorkspace.option}
			disabled={loadingWorkspaces}
			onchange={(event) => {
				handleWorkspaceOptionChange(
					(event.currentTarget as HTMLSelectElement).value,
				);
			}}
		>
			{#if availableWorkspaces.length > 0}
				<optgroup label="Existing workspaces">
					{#each availableWorkspaces as workspace (workspace.id)}
						<option value={`existing:${workspace.id}`}>
							{getWorkspaceOptionLabel(workspace)}
						</option>
					{/each}
				</optgroup>
			{/if}
			<optgroup label="Create new">
				<option value="new-workspace">Create New Workspace</option>
				<option value="local-directory">Local Directory</option>
				<option value="git-repo">GitHub Repo</option>
			</optgroup>
		</NativeSelect>
	{/if}
</div>
