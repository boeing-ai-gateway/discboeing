<script lang="ts">
	import ClockIcon from "@lucide/svelte/icons/clock";
	import XIcon from "@lucide/svelte/icons/x";
	import { onDestroy, onMount, tick } from "svelte";
	import { api } from "$lib/api-client";
	import { InputGroup, InputGroupAddon } from "$lib/components/ui/input-group";
	import { Button } from "$lib/components/ui/button";
	import ConversationComposerAttachmentButton from "$lib/components/app/parts/ConversationComposerAttachmentButton.svelte";
	import ConversationComposerAttachments from "$lib/components/app/parts/ConversationComposerAttachments.svelte";
	import ConversationComposerHooksControl from "$lib/components/app/parts/ConversationComposerHooksControl.svelte";
	import ConversationComposerModelControl from "$lib/components/app/parts/ConversationComposerModelControl.svelte";
	import ConversationComposerProvidersControl from "$lib/components/app/parts/ConversationComposerProvidersControl.svelte";
	import ConversationComposerReasoningControl from "$lib/components/app/parts/ConversationComposerReasoningControl.svelte";
	import ConversationComposerServiceTierControl from "$lib/components/app/parts/ConversationComposerServiceTierControl.svelte";
	import ConversationPromptQueuePanel from "$lib/components/app/parts/ConversationPromptQueuePanel.svelte";
	import ConversationComposerSessionSetupStatus from "$lib/components/app/ConversationComposerSessionSetupStatus.svelte";
	import ConversationComposerSubmitButton from "$lib/components/app/parts/ConversationComposerSubmitButton.svelte";
	import ConversationComposerTokenUsage from "$lib/components/app/parts/ConversationComposerTokenUsage.svelte";
	import ConversationPromptSchedulePicker from "$lib/components/app/parts/ConversationPromptSchedulePicker.svelte";
	import ConversationComposerTextarea from "$lib/components/app/parts/ConversationComposerTextarea.svelte";
	import ConversationCredentialsControl from "$lib/components/app/ConversationCredentialsControl.svelte";
	import ConversationHooksPanel from "$lib/components/app/ConversationHooksPanel.svelte";
	import ConversationWorkspaceSelector from "$lib/components/app/ConversationWorkspaceSelector.svelte";
	import {
		Popover,
		PopoverContent,
		PopoverTrigger,
	} from "$lib/components/ui/popover";
	import {
		moveComposerDraft,
		resolveComposerDraftStorageKey,
	} from "$lib/composer-draft-storage";
	import type {
		ComposerAttachment,
		ConversationComposerTextareaHandle,
		WorkspaceSelectionResult,
		WorkspaceSelectorHandle,
	} from "$lib/components/app/conversation-composer.types";
	import type {
		ModelInfo,
		SandboxProviderInstance,
		UpdateQueuedPromptRequest,
	} from "$lib/api-types";
	import type { ConversationComment } from "$lib/session/session-context.types";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";
	import {
		normalizeThreadComposerReasoning,
		normalizeThreadComposerServiceTier,
		parseComposerModelSelection,
		useThreadContext,
	} from "$lib/context/thread-context.svelte";
	import {
		buildUserMessageParts,
		createUserMessageAttachment,
		formatConversationComments,
	} from "$lib/session/domains/session-domain.helpers";

	type Props = {
		onContainerChange?: (element: HTMLDivElement | null) => void;
	};

	type FileMentionItem = {
		path: string;
		type: "file" | "directory";
	};

	let { onContainerChange }: Props = $props();

	const app = useAppContext();
	const models = app.models;
	const preferences = app.preferences;
	const ui = app.ui;
	const session = useSessionContext();
	const thread = useThreadContext();
	const sessionView = session.ui;
	const sessionHooks = session.hooks;
	const sessionCommands = session.commands;
	const sandboxProvidersUpdatedEvent = "discobot:sandbox-providers-updated";

	let attachmentFiles = $state<ComposerAttachment[]>([]);
	let composerContainer = $state<HTMLDivElement | null>(null);
	let composerTextareaRef = $state<ConversationComposerTextareaHandle | null>(
		null,
	);
	let sessionSetupRef = $state<WorkspaceSelectorHandle | null>(null);
	let pendingSubmitError = $state<string | null>(null);
	let sandboxProviders = $state<SandboxProviderInstance[]>([]);
	let sandboxDefaultProviderId = $state("");
	let sandboxProvidersError = $state<string | null>(null);
	let sandboxProviderMobileSelectOpen = $state(false);
	let sandboxProviderDesktopSelectOpen = $state(false);
	let schedulePopoverOpen = $state(false);
	let scheduledRunAfter = $state<string | null>(null);
	let pendingAutocompleteSessionCreation = $state<Promise<boolean> | null>(
		null,
	);
	let fileMentionQuery = $state("");
	let fileMentionOpen = $state(false);
	let fileMentionSuggestions = $state<FileMentionItem[]>([]);
	let fileMentionLoading = $state(false);
	let fileMentionRequestSequence = 0;

	function handleFileMentionQueryChange(query: string, open: boolean) {
		fileMentionQuery = query;
		fileMentionOpen = open;
	}

	$effect(() => {
		if (!fileMentionOpen || session.isPending) {
			fileMentionSuggestions = [];
			fileMentionLoading = false;
			return;
		}

		const requestId = fileMentionRequestSequence + 1;
		fileMentionRequestSequence = requestId;
		const currentQuery = fileMentionQuery;
		const controller = new AbortController();
		const timeout = window.setTimeout(
			async () => {
				fileMentionLoading = true;
				try {
					const response = await api.searchSessionFiles(
						session.sessionId,
						currentQuery,
						50,
						{ signal: controller.signal },
					);
					if (fileMentionRequestSequence !== requestId) {
						return;
					}
					fileMentionSuggestions = response.results.map((result) => ({
						path: result.path,
						type: result.type,
					}));
				} catch {
					if (
						controller.signal.aborted ||
						fileMentionRequestSequence !== requestId
					) {
						return;
					}
					fileMentionSuggestions = [];
				} finally {
					if (fileMentionRequestSequence === requestId) {
						fileMentionLoading = false;
					}
				}
			},
			currentQuery === "" ? 0 : 80,
		);

		return () => {
			window.clearTimeout(timeout);
			controller.abort();
		};
	});

	function findModelById(modelId: string | null): ModelInfo | null {
		if (!modelId) {
			return null;
		}
		return models.peek(modelId);
	}

	function normalizeReasoningForModel(
		model: ModelInfo | null,
		reasoning: string | undefined,
	): string | undefined {
		if (!model?.reasoning) {
			return undefined;
		}
		const normalizedReasoning = normalizeThreadComposerReasoning(reasoning);
		if (!normalizedReasoning) {
			return undefined;
		}
		if (normalizedReasoning === "default") {
			return "default";
		}
		const levels = model.reasoningLevels ?? [];
		if (levels.length === 0 || levels.includes(normalizedReasoning)) {
			return normalizedReasoning;
		}
		return undefined;
	}

	function getReasoningForModel(
		model: ModelInfo | null,
		preferredReasoning: string | undefined,
		fallbackReasoning: string | undefined,
	): string | undefined {
		return (
			normalizeReasoningForModel(model, preferredReasoning) ??
			normalizeReasoningForModel(model, fallbackReasoning)
		);
	}

	function getNextReasoningForModel(
		model: ModelInfo | null,
		preferredReasoning: string | undefined,
		fallbackReasoning: string | undefined,
	): string | undefined {
		if (!model?.reasoning) {
			return undefined;
		}
		return (
			getReasoningForModel(model, preferredReasoning, fallbackReasoning) ??
			"default"
		);
	}

	function isSameReasoningSelection(
		left: string | undefined,
		right: string | undefined,
	): boolean {
		return (left ?? "default") === (right ?? "default");
	}

	function normalizeServiceTierForModel(
		model: ModelInfo | null,
		serviceTier: string | undefined,
	): string | undefined {
		const normalizedTier = normalizeThreadComposerServiceTier(serviceTier);
		if (!normalizedTier) {
			return undefined;
		}
		const serviceTiers = model?.serviceTiers ?? [];
		return serviceTiers.some(
			(tier) => tier.toLowerCase() === normalizedTier.toLowerCase(),
		)
			? normalizedTier
			: undefined;
	}

	function getServiceTierForModel(
		model: ModelInfo | null,
		preferredServiceTier: string | null | undefined,
		fallbackServiceTier: string | undefined,
	): string | undefined {
		if (preferredServiceTier !== undefined) {
			return normalizeServiceTierForModel(
				model,
				preferredServiceTier ?? undefined,
			);
		}
		return normalizeServiceTierForModel(model, fallbackServiceTier);
	}

	function isSameServiceTierSelection(
		left: string | undefined,
		right: string | undefined,
	): boolean {
		return (left ?? "") === (right ?? "");
	}

	const effectiveModelId = $derived.by(
		() => thread.nextModelId ?? thread.modelId,
	);
	const selectedModelId = $derived.by(() =>
		thread.nextModelId !== undefined
			? (thread.nextModelId ?? preferences.defaultModel) || null
			: effectiveModelId,
	);
	const selectedModel = $derived.by(() => findModelById(selectedModelId));
	const effectiveReasoning = $derived.by(() =>
		getReasoningForModel(selectedModel, thread.nextReasoning, thread.reasoning),
	);
	const effectiveServiceTier = $derived.by(() =>
		getServiceTierForModel(
			selectedModel,
			thread.nextServiceTier,
			thread.serviceTier,
		),
	);
	const reasoningLevels = $derived.by(
		() => selectedModel?.reasoningLevels ?? [],
	);
	const serviceTiers = $derived.by(() => selectedModel?.serviceTiers ?? []);
	const hasAvailableModels = $derived.by(() => models.list.length > 0);
	const sessionSetupDisabled = $derived.by(
		() =>
			sessionView.pendingWorkspaceRequiresSourceInput &&
			!sessionView.pendingWorkspaceSourceIsValid,
	);
	const showPendingWorkspaceSelector = $derived.by(
		() => session.isPending && !thread.isStreaming,
	);
	const availableCommands = $derived.by(() =>
		session.isPending ? [] : sessionCommands.list,
	);
	const commandsLoading = $derived.by(
		() =>
			!session.isPending &&
			sessionCommands.fetchedAt === null &&
			sessionCommands.status !== "error",
	);
	const selectableSandboxProviders = $derived.by(() =>
		sandboxProviders.filter((provider) => provider.available),
	);
	const selectedSandboxProvider = $derived.by(() =>
		selectableSandboxProviders.find(
			(provider) =>
				provider.id ===
				(sessionView.pendingSandboxProviderId || sandboxDefaultProviderId),
		),
	);
	const selectedSandboxProviderTitle = $derived.by(() => {
		if (!selectedSandboxProvider) {
			return "Sandbox provider";
		}
		return sessionView.pendingSandboxProviderId
			? selectedSandboxProvider.name
			: `Default provider: ${selectedSandboxProvider.name}`;
	});
	const sandboxProviderSelectValue = $derived(
		sessionView.pendingSandboxProviderId || sandboxDefaultProviderId,
	);

	function handleSandboxProviderSelect(value: string) {
		sessionView.setPendingSandboxProviderId(
			value === sandboxDefaultProviderId ? "" : value,
		);
	}

	async function handleManageSandboxProvidersClick() {
		sandboxProviderMobileSelectOpen = false;
		sandboxProviderDesktopSelectOpen = false;
		await tick();
		ui.openSettings("providers");
	}

	function handleModelSelect(nextSelection: string | null) {
		const parsedSelection = parseComposerModelSelection(nextSelection);
		const nextModel = findModelById(
			(parsedSelection.modelId ?? preferences.defaultModel) || null,
		);
		const nextReasoning = getNextReasoningForModel(
			nextModel,
			thread.nextReasoning,
			thread.reasoning,
		);
		const nextServiceTier = getServiceTierForModel(
			nextModel,
			thread.nextServiceTier,
			thread.serviceTier,
		);

		if (parsedSelection.modelId === thread.modelId) {
			thread.setNextModelId(undefined);
			thread.setNextReasoning(
				isSameReasoningSelection(nextReasoning, thread.reasoning)
					? undefined
					: nextReasoning,
			);
			thread.setNextServiceTier(
				isSameServiceTierSelection(nextServiceTier, thread.serviceTier)
					? undefined
					: nextServiceTier,
			);
			return;
		}

		thread.setNextModelId(parsedSelection.modelId);
		thread.setNextReasoning(nextReasoning);
		thread.setNextServiceTier(nextServiceTier);
	}

	function handleReasoningSelect(nextReasoning: string | undefined) {
		if (nextReasoning === "default") {
			const modelDefaultReasoning = selectedModel?.defaultReasoning;
			if (effectiveReasoning === undefined) {
				thread.setNextReasoning(
					thread.nextModelId === undefined &&
						(thread.reasoning === undefined || thread.reasoning === "default")
						? undefined
						: "default",
				);
				return;
			}
			thread.setNextReasoning(modelDefaultReasoning ?? "default");
			return;
		}

		thread.setNextReasoning(
			thread.nextModelId === undefined &&
				isSameReasoningSelection(nextReasoning, thread.reasoning)
				? undefined
				: nextReasoning,
		);
	}

	function handleServiceTierSelect(nextServiceTier: string | undefined) {
		thread.setNextServiceTier(
			thread.nextModelId === undefined &&
				isSameServiceTierSelection(nextServiceTier, thread.serviceTier)
				? undefined
				: (nextServiceTier ?? null),
		);
	}

	const submitStatus = $derived.by(() => {
		if (session.isPending) return "ready" as const;
		if (thread.status === "loading") return "submitted" as const;
		if (thread.isStreaming) return "streaming" as const;
		if (thread.status === "error") return "error" as const;
		return "ready" as const;
	});
	const composerDisabledMessage = $derived.by(() => {
		if (!hasAvailableModels) {
			return "Please add a valid LLM provider credential";
		}
		if (thread.hasPendingQuestion) {
			return "Answer the agent's pending question before sending a new message.";
		}
		return null;
	});
	const composerDisabled = $derived.by(() => composerDisabledMessage !== null);

	function isGenerating() {
		return (
			!session.isPending && (thread.status === "loading" || thread.isStreaming)
		);
	}

	function inputEmpty() {
		return (
			sessionView.composerDraft.trim().length === 0 &&
			thread.pendingComments.length === 0
		);
	}

	function addFiles(files: File[] | FileList) {
		const incoming = Array.from(files);
		if (incoming.length === 0) {
			return;
		}

		attachmentFiles = attachmentFiles.concat(
			incoming.map((file) => ({
				id: `${Date.now()}-${Math.floor(Math.random() * 10_000)}`,
				file,
				filename: file.name,
				mediaType: file.type,
				url: URL.createObjectURL(file),
			})),
		);
	}

	function removeAttachment(id: string) {
		const target = attachmentFiles.find((item) => item.id === id);
		if (target?.url) {
			URL.revokeObjectURL(target.url);
		}
		attachmentFiles = attachmentFiles.filter((item) => item.id !== id);
	}

	function removeLastAttachment() {
		const lastAttachment = attachmentFiles.at(-1);
		if (lastAttachment) {
			removeAttachment(lastAttachment.id);
		}
	}

	function clearAttachments() {
		for (const file of attachmentFiles) {
			if (file.url) {
				URL.revokeObjectURL(file.url);
			}
		}
		attachmentFiles = [];
	}

	async function createMessageParts(text: string) {
		const attachments = await Promise.all(
			attachmentFiles.map(({ file }) => createUserMessageAttachment(file)),
		);
		return buildUserMessageParts(text, attachments);
	}

	function buildSubmitText(draft: string, comments: ConversationComment[]) {
		const text = draft.trim();
		const commentText = formatConversationComments(comments);
		return [text, commentText].filter(Boolean).join("\n\n");
	}

	async function focusComposerTextarea() {
		await tick();
		composerTextareaRef?.focus();
	}

	onMount(() => {
		void focusComposerTextarea();
		void loadSandboxProviders();
		const handleSandboxProvidersUpdated = () => {
			void loadSandboxProviders();
		};
		window.addEventListener(
			sandboxProvidersUpdatedEvent,
			handleSandboxProvidersUpdated,
		);
		return () => {
			window.removeEventListener(
				sandboxProvidersUpdatedEvent,
				handleSandboxProvidersUpdated,
			);
		};
	});

	$effect(() => {
		onContainerChange?.(composerContainer);
	});

	onDestroy(() => {
		onContainerChange?.(null);
	});

	async function getPendingWorkspaceSelection() {
		return (
			sessionSetupRef?.getWorkspaceSelection() ??
			Promise.resolve<WorkspaceSelectionResult>({
				ready: false,
				workspaceId: null,
				workspaceType: null,
				workspacePath: null,
			})
		);
	}

	async function getPendingSubmitOptions() {
		const workspaceSelection = await getPendingWorkspaceSelection();
		if (!workspaceSelection.ready) {
			return null;
		}

		return {
			...(sessionView.pendingSandboxProviderId
				? { providerId: sessionView.pendingSandboxProviderId }
				: {}),
			...(workspaceSelection.workspaceId
				? { workspaceId: workspaceSelection.workspaceId }
				: {}),
			...(workspaceSelection.workspaceType && workspaceSelection.workspacePath
				? {
						workspaceType: workspaceSelection.workspaceType,
						workspacePath: workspaceSelection.workspacePath,
					}
				: {}),
		};
	}

	async function loadSandboxProviders() {
		try {
			const response = await api.getSandboxProviders();
			sandboxProviders = response.providers;
			sandboxDefaultProviderId = response.default;
			sandboxProvidersError = null;
			if (
				sessionView.pendingSandboxProviderId &&
				!response.providers.some(
					(provider) =>
						provider.id === sessionView.pendingSandboxProviderId &&
						provider.available,
				)
			) {
				sessionView.setPendingSandboxProviderId("");
			}
		} catch (error) {
			sandboxProvidersError =
				error instanceof Error ? error.message : "Failed to load providers.";
		}
	}

	function movePendingDraftToThread(threadId: string, draft: string) {
		moveComposerDraft({
			fromStorageKey: resolveComposerDraftStorageKey({
				isPending: true,
				threadId: thread.threadId,
			}),
			toStorageKey: resolveComposerDraftStorageKey({
				isPending: false,
				threadId,
			}),
			value: draft,
		});
	}

	function clearCurrentDraft() {
		thread.clearComposerDraft();
	}

	function parseRunAfter(value?: string | null): Date | null {
		if (!value) {
			return null;
		}
		const parsed = new Date(value);
		return Number.isNaN(parsed.getTime()) ? null : parsed;
	}

	function isScheduledRunAfterPaused(value?: string | null): boolean {
		const parsed = parseRunAfter(value);
		if (!parsed) {
			return false;
		}
		return parsed.getTime() >= Date.now() + 25 * 365 * 24 * 60 * 60 * 1000;
	}

	const scheduledSubmitLabel = $derived.by(() => {
		if (!scheduledRunAfter) {
			return null;
		}
		if (isScheduledRunAfterPaused(scheduledRunAfter)) {
			return "Submit paused prompt";
		}
		const parsed = parseRunAfter(scheduledRunAfter);
		return parsed
			? `Submit scheduled prompt for ${parsed.toLocaleString()}`
			: null;
	});

	async function handleDeleteQueuedPrompt(queueId: string) {
		await thread.deleteQueuedPrompt(queueId);
	}

	async function createSessionForComposerAutocomplete(): Promise<boolean> {
		if (!session.isPending) {
			return true;
		}

		if (pendingAutocompleteSessionCreation) {
			return pendingAutocompleteSessionCreation;
		}

		const creation = submitComposer({
			forceEmptyPendingMessage: true,
			preserveDraft: true,
		});
		pendingAutocompleteSessionCreation = creation;
		return creation.finally(() => {
			if (pendingAutocompleteSessionCreation === creation) {
				pendingAutocompleteSessionCreation = null;
			}
		});
	}

	async function handleUpdateQueuedPrompt(
		queueId: string,
		payload: UpdateQueuedPromptRequest,
	) {
		await thread.updateQueuedPrompt(queueId, payload);
	}

	async function submitComposer({
		forceEmptyPendingMessage = false,
		preserveDraft = false,
	}: {
		forceEmptyPendingMessage?: boolean;
		preserveDraft?: boolean;
	} = {}) {
		if (composerDisabled && !forceEmptyPendingMessage) {
			return false;
		}
		const submitComments = forceEmptyPendingMessage
			? []
			: thread.pendingComments;
		const emptyWithoutAttachments =
			inputEmpty() && attachmentFiles.length === 0;
		if (isGenerating() && emptyWithoutAttachments) {
			await thread.cancel();
			composerTextareaRef?.closeMentionDropdown();
			composerTextareaRef?.closeSlashCommandDropdown();
			composerTextareaRef?.closePromptHistoryDropdown();
			return false;
		}
		if (!session.isPending && emptyWithoutAttachments) {
			return false;
		}

		pendingSubmitError = null;
		const wasPending = session.isPending;
		const currentDraft = sessionView.composerDraft;
		const nextMessageText = forceEmptyPendingMessage
			? ""
			: buildSubmitText(currentDraft, submitComments);
		const shouldAllowEmptyPendingMessage =
			wasPending &&
			(forceEmptyPendingMessage ||
				(attachmentFiles.length === 0 && nextMessageText.length === 0));
		const nextMessageParts = forceEmptyPendingMessage
			? []
			: shouldAllowEmptyPendingMessage
				? []
				: await createMessageParts(nextMessageText);
		const nextRunAfter =
			!forceEmptyPendingMessage && scheduledRunAfter
				? scheduledRunAfter
				: undefined;
		const pendingSubmitOptions = wasPending
			? await getPendingSubmitOptions()
			: null;
		if (wasPending && !pendingSubmitOptions) {
			return false;
		}

		if (!preserveDraft) {
			if (nextMessageText) {
				preferences.addPromptToHistory(nextMessageText);
			}
			clearCurrentDraft();
		}

		try {
			const result = await thread.submit({
				parts: nextMessageParts,
				allowEmptyPendingMessage: shouldAllowEmptyPendingMessage,
				...(nextRunAfter ? { runAfter: nextRunAfter } : {}),
				...pendingSubmitOptions,
			});
			if (wasPending && result) {
				app.sessions.openThread(result.sessionId, result.threadId);
				if (preserveDraft) {
					movePendingDraftToThread(result.threadId, currentDraft);
				}
				thread.clearNextComposerValues();
				sessionView.resetPendingWorkspaceSetup();
			}
			if (!preserveDraft) {
				thread.clearNextComposerValues();
				thread.clearPendingComments();
				scheduledRunAfter = null;
				schedulePopoverOpen = false;
				composerTextareaRef?.closeMentionDropdown();
				composerTextareaRef?.closeSlashCommandDropdown();
				composerTextareaRef?.closePromptHistoryDropdown();
				clearAttachments();
				await focusComposerTextarea();
			}
			return true;
		} catch (err) {
			if (wasPending) {
				pendingSubmitError =
					err instanceof Error ? err.message : "Failed to start chat";
			}
			await focusComposerTextarea();
			return false;
		}
	}

	async function handleComposerSubmit() {
		await submitComposer();
	}

	async function handleScheduledRunAfterSelect(runAfter: Date | null) {
		scheduledRunAfter = runAfter ? runAfter.toISOString() : null;
		schedulePopoverOpen = false;
	}

	async function loadTokenUsageDetails() {
		return api.getThreadTokenUsage(session.sessionId, thread.threadId);
	}
</script>

<div bind:this={composerContainer} class="shrink-0 bg-background p-0 md:p-3">
	<div
		class={`w-full ${preferences.chatWidthMode === "constrained" ? "md:mx-auto md:max-w-3xl" : ""}`}
	>
		{#if !session.isPending}
			<ConversationPromptQueuePanel
				entries={thread.promptQueue}
				onDelete={handleDeleteQueuedPrompt}
				onUpdate={handleUpdateQueuedPrompt}
			/>

			<ConversationHooksPanel
				expanded={sessionView.hooksExpanded}
				hooksStatus={sessionHooks.status}
				outputById={sessionHooks.outputById}
				onRerunHook={(hookId) => sessionHooks.rerun(hookId)}
				onSetExecutionPaused={(paused) => {
					void sessionHooks.setExecutionPaused(paused);
					sessionView.hooksExpanded = false;
				}}
				onSetHookExecutionPaused={(hookId, paused) => {
					void sessionHooks.setHookExecutionPaused(hookId, paused);
				}}
			/>
		{/if}

		{#if session.isPending || session.current?.sandboxStatus !== "ready"}
			<ConversationComposerSessionSetupStatus />
			{#if showPendingWorkspaceSelector}
				<div class="mb-2 flex w-full flex-col gap-2 px-1 md:hidden">
					<ConversationWorkspaceSelector
						bind:this={sessionSetupRef}
						fullWidth={true}
					/>
					{#if selectableSandboxProviders.length > 0}
						<div class="space-y-1">
							<ConversationComposerProvidersControl
								id="pending-sandbox-provider-mobile"
								bind:open={sandboxProviderMobileSelectOpen}
								value={sandboxProviderSelectValue}
								providers={selectableSandboxProviders}
								defaultProviderId={sandboxDefaultProviderId}
								selectedProvider={selectedSandboxProvider}
								selectedProviderTitle={selectedSandboxProviderTitle}
								labelClass="text-xs text-muted-foreground"
								triggerClass="h-9 px-3"
								contentClass=""
								onSelect={handleSandboxProviderSelect}
								onManageClick={handleManageSandboxProvidersClick}
							/>
						</div>
					{/if}
				</div>
			{/if}
		{/if}

		{#if pendingSubmitError}
			<div class="mb-2 text-sm text-destructive">{pendingSubmitError}</div>
		{/if}
		{#if thread.pendingComments.length > 0}
			<div
				class="mb-2 rounded-xl border border-amber-300/70 bg-amber-50 p-3 text-sm shadow-sm dark:border-amber-400/30 dark:bg-amber-950/20"
			>
				<div class="mb-2 font-medium text-foreground">
					{thread.pendingComments.length}
					{thread.pendingComments.length === 1 ? "comment" : "comments"} ready to
					submit
				</div>
				<div class="space-y-2">
					{#each thread.pendingComments as comment (comment.id)}
						<div
							class="rounded-lg border border-border/70 bg-background/80 p-2 text-xs"
						>
							<div class="flex items-start gap-2">
								<div class="min-w-0 flex-1 space-y-1">
									<div
										class="line-clamp-2 border-muted-foreground/30 border-l-2 pl-2 text-muted-foreground italic"
									>
										{comment.snippet}
									</div>
									<div class="whitespace-pre-wrap text-foreground">
										{comment.comment}
									</div>
								</div>
								<Button
									aria-label="Remove comment"
									class="size-6 shrink-0"
									onclick={() => thread.removePendingComment(comment.id)}
									size="icon-xs"
									type="button"
									variant="ghost"
								>
									<XIcon class="size-3.5" />
								</Button>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}
		{#if session.isPending && sandboxProvidersError}
			<div class="mb-2 text-sm text-destructive">{sandboxProvidersError}</div>
		{/if}
		{#if composerDisabledMessage}
			<div
				class="mb-2 flex flex-wrap items-center gap-2 text-sm text-muted-foreground"
			>
				<span>{composerDisabledMessage}</span>
				{#if !hasAvailableModels}
					<Button
						variant="link"
						size="xs"
						class="h-auto px-0"
						onclick={ui.openCredentialsDialog}
					>
						Open credentials
					</Button>
				{/if}
			</div>
		{/if}

		<div class="relative">
			<form
				onsubmit={(event) => {
					event.preventDefault();
					void submitComposer();
				}}
			>
				<InputGroup class="rounded-t-md rounded-b-none md:rounded-md">
					<ConversationComposerAttachments
						files={attachmentFiles}
						onRemove={removeAttachment}
					/>

					<ConversationComposerTextarea
						bind:this={composerTextareaRef}
						draft={sessionView.composerDraft}
						disabled={composerDisabled}
						onDraftChange={(v) => sessionView.setComposerDraft(v)}
						sessionId={session.isPending ? null : session.sessionId}
						commands={availableCommands}
						{commandsLoading}
						{fileMentionSuggestions}
						{fileMentionLoading}
						promptHistory={preferences.promptHistory}
						pinnedPrompts={preferences.pinnedPrompts}
						attachmentCount={attachmentFiles.length}
						onAddFiles={addFiles}
						onRemoveLastAttachment={removeLastAttachment}
						onPinPrompt={(prompt) => preferences.pinPrompt(prompt)}
						onUnpinPrompt={(prompt) => preferences.unpinPrompt(prompt)}
						onRemovePromptFromHistory={(prompt) =>
							preferences.removePromptFromHistory(prompt)}
						isPromptPinned={(prompt) => preferences.isPromptPinned(prompt)}
						onFileMentionQueryChange={handleFileMentionQueryChange}
						onRequestAutocompleteSession={createSessionForComposerAutocomplete}
						onSubmit={handleComposerSubmit}
					/>

					<InputGroupAddon align="block-end" class="justify-between gap-1">
						<div
							class="desktop-no-drag flex min-w-0 flex-1 flex-wrap items-center gap-1"
						>
							<ConversationComposerAttachmentButton
								onFilesAdd={addFiles}
								disabled={composerDisabled}
							/>
							{#if !session.isPending}
								<ConversationCredentialsControl />
							{/if}
							<div class="flex min-w-0 items-center gap-0">
								<ConversationComposerModelControl
									value={thread.nextModelId !== undefined
										? thread.nextModelId
										: thread.modelId}
									onSelect={handleModelSelect}
									models={models.list}
								/>
								{#if selectedModel?.reasoning}
									<ConversationComposerReasoningControl
										value={effectiveReasoning}
										defaultValue={selectedModel.defaultReasoning}
										levels={reasoningLevels}
										onSelect={handleReasoningSelect}
									/>
								{/if}
								{#if serviceTiers.length > 0}
									<ConversationComposerServiceTierControl
										value={effectiveServiceTier}
										tiers={serviceTiers}
										onSelect={handleServiceTierSelect}
									/>
								{/if}
								<ConversationComposerTokenUsage
									usage={thread.thread?.tokenUsage}
									onLoadDetails={loadTokenUsageDetails}
								/>
							</div>
						</div>

						<div
							class="desktop-no-drag ml-auto flex items-center justify-end gap-2"
						>
							{#if showPendingWorkspaceSelector}
								<div class="hidden items-center gap-2 md:flex">
									{#if selectableSandboxProviders.length > 0}
										<ConversationComposerProvidersControl
											id="pending-sandbox-provider"
											bind:open={sandboxProviderDesktopSelectOpen}
											value={sandboxProviderSelectValue}
											providers={selectableSandboxProviders}
											defaultProviderId={sandboxDefaultProviderId}
											selectedProvider={selectedSandboxProvider}
											selectedProviderTitle={selectedSandboxProviderTitle}
											onSelect={handleSandboxProviderSelect}
											onManageClick={handleManageSandboxProvidersClick}
										/>
									{/if}
									<ConversationWorkspaceSelector bind:this={sessionSetupRef} />
								</div>
							{:else if !session.isPending}
								<ConversationComposerHooksControl
									bind:expanded={sessionView.hooksExpanded}
									hooksStatus={sessionHooks.status}
								/>
							{/if}
							<Popover bind:open={schedulePopoverOpen}>
								<PopoverTrigger>
									<Button
										variant={scheduledRunAfter ? "default" : "ghost"}
										size="icon-sm"
										title={scheduledSubmitLabel ?? "Schedule prompt"}
										aria-label={scheduledSubmitLabel ?? "Schedule prompt"}
										disabled={composerDisabled ||
											(session.isPending ? sessionSetupDisabled : false)}
									>
										<ClockIcon class="size-4" />
									</Button>
								</PopoverTrigger>
								<PopoverContent align="end" class="w-72 p-3">
									<ConversationPromptSchedulePicker
										currentRunAfter={scheduledRunAfter ?? undefined}
										onSelect={handleScheduledRunAfterSelect}
									/>
								</PopoverContent>
							</Popover>
							<ConversationComposerSubmitButton
								status={submitStatus}
								inputEmpty={inputEmpty()}
								isPending={session.isPending}
								disabled={composerDisabled ||
									(session.isPending ? sessionSetupDisabled : false)}
								onPress={handleComposerSubmit}
							/>
						</div></InputGroupAddon
					>
				</InputGroup>
			</form>
		</div>
	</div>
</div>
