<script lang="ts">
	import ClockIcon from "@lucide/svelte/icons/clock";
	import Maximize2Icon from "@lucide/svelte/icons/maximize-2";
	import Minimize2Icon from "@lucide/svelte/icons/minimize-2";
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
	import type {
		ComposerAttachment,
		ConversationComposerTextareaHandle,
		WorkspaceSelectionResult,
		WorkspaceSelectorHandle,
	} from "$lib/components/app/conversation-composer.types";
	import type {
		ChatMessage,
		AgentCommand,
		ModelInfo,
		SandboxProviderInstance,
		Thread,
		UpdateQueuedPromptRequest,
	} from "$lib/api-types";
	import type { ConversationComment } from "$lib/context/context.types";
	import {
		normalizeThreadComposerReasoning,
		normalizeThreadComposerServiceTier,
		parseComposerModelSelection,
	} from "$lib/thread-composer-helpers";
	import {
		buildUserMessageParts,
		createUserMessageAttachment,
		formatConversationComments,
		getPendingQuestionApprovalId,
	} from "$lib/conversation-helpers";
	import { useContext } from "$lib/context";
	import {
		canLoadSessionThreads,
		isSessionTransitioningStatus,
	} from "$lib/api-constants";
	import { isThreadSnapshotRunning } from "$lib/session-status-helpers";
	import {
		getPendingWorkspaceRequiresSourceInput,
		getPendingWorkspaceSourceIsValid,
	} from "$lib/pending-workspace-helpers";

	type Props = {
		sessionId: string;
		threadId: string;
		onContainerChange?: (element: HTMLDivElement | null) => void;
	};

	type FileMentionItem = {
		path: string;
		type: "file" | "directory";
	};

	let { sessionId, threadId, onContainerChange }: Props = $props();

	const context = useContext();
	const models = $derived(context.data.models);
	const preferences = $derived(context.view.app.preferences);
	const sessionRecord = $derived(context.data.sessions.byId[sessionId] ?? null);
	const session = $derived(sessionRecord?.value ?? null);
	const sessionView = $derived(context.view.sessions[sessionId] ?? null);
	const threadRecord = $derived(sessionRecord?.threads.byId[threadId] ?? null);
	const thread = $derived(threadRecord?.value ?? null);
	const threadContent = $derived(threadRecord?.content ?? null);
	const threadView = $derived(sessionView?.threads[threadId] ?? null);
	const isPending = $derived(
		context.view.selection.pendingSessionId === sessionId &&
			context.view.selection.sessionId !== sessionId,
	);
	const composerDraft = $derived.by(() => sessionView?.composer.draft ?? "");
	const sessionHooks = $derived(sessionRecord?.hooks ?? null);
	const sessionCommands = $derived(sessionRecord?.commands ?? null);
	const modelItems = $derived.by(() =>
		models.allIds
			.map((id) => models.byId[id])
			.filter((model): model is ModelInfo => Boolean(model)),
	);
	const hooksStatus = $derived.by(() => ({
		hooks: (sessionHooks?.allIds ?? [])
			.map((id) => sessionHooks?.byId[id])
			.filter((hook) => Boolean(hook)),
		pendingHookIds: sessionHooks?.pendingHookIds ?? [],
		executionPaused: sessionHooks?.executionPaused ?? false,
	}));
	const hookOutputById = $derived.by(() => sessionHooks?.outputsById ?? {});
	const sandboxProvidersUpdatedEvent = "discobot:sandbox-providers-updated";

	let attachmentFiles = $state<ComposerAttachment[]>([]);
	let composerContainer = $state<HTMLDivElement | null>(null);
	let hooksPanelContainer = $state<HTMLDivElement | null>(null);
	let hooksControlContainer = $state<HTMLDivElement | null>(null);
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
	let composerExpanded = $state(false);

	function handleFileMentionQueryChange(query: string, open: boolean) {
		fileMentionQuery = query;
		fileMentionOpen = open;
	}

	$effect(() => {
		if (!fileMentionOpen || isPending) {
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
						sessionId,
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
		return models.byId[modelId] ?? null;
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

	type ThreadContentStatus = "idle" | "loading" | "ready" | "error";

	function getThreadContentStatus(
		status: NonNullable<typeof threadContent>["status"] | undefined,
	): ThreadContentStatus {
		switch (status?.state) {
			case "loading":
			case "refreshing":
				return "loading";
			case "ready":
				return "ready";
			case "error":
				return "error";
			default:
				return "idle";
		}
	}

	function getThreadComposerValues(
		currentThread: Thread | null,
		defaultModel: string | null,
		defaultReasoning?: string | null,
		defaultServiceTier?: string | null,
	): {
		modelId: string | null;
		reasoning: string | undefined;
		serviceTier: string | undefined;
	} {
		const modelId = currentThread?.model ?? defaultModel;
		if (!modelId) {
			return {
				modelId,
				reasoning: undefined,
				serviceTier: undefined,
			};
		}

		const useDefaultComposerPreferences = currentThread?.model === undefined;
		return {
			modelId,
			reasoning: useDefaultComposerPreferences
				? (normalizeThreadComposerReasoning(currentThread?.reasoning) ??
					normalizeThreadComposerReasoning(defaultReasoning))
				: normalizeThreadComposerReasoning(currentThread?.reasoning),
			serviceTier: useDefaultComposerPreferences
				? (normalizeThreadComposerServiceTier(currentThread?.serviceTier) ??
					normalizeThreadComposerServiceTier(defaultServiceTier))
				: normalizeThreadComposerServiceTier(currentThread?.serviceTier),
		};
	}

	function isSessionThreadStatusRunningForThread(
		threadStatus:
			| NonNullable<typeof session>["threadStatus"]
			| null
			| undefined,
		currentThreadId: string,
	): boolean {
		return (
			threadStatus?.status === "running" &&
			threadStatus.threadId === currentThreadId
		);
	}

	function hasPendingQuestion(
		messages: ChatMessage[],
		pendingQuestionId: string | null | undefined,
		answeredApprovalIds: Record<string, { approved: boolean; reason?: string }>,
	): boolean {
		const pendingApprovalId = getPendingQuestionApprovalId(
			messages,
			answeredApprovalIds,
		);
		if (pendingApprovalId) {
			return true;
		}
		return !!pendingQuestionId && !answeredApprovalIds[pendingQuestionId];
	}

	const threadComposerState = $derived.by(() => {
		const composerValues = getThreadComposerValues(
			thread,
			preferences.defaultModel,
			preferences.defaultReasoning,
			preferences.defaultServiceTier,
		);
		const messages = threadContent?.messages ?? [];
		return {
			thread,
			modelId: composerValues.modelId,
			reasoning: composerValues.reasoning,
			serviceTier: composerValues.serviceTier,
			nextModelId: threadView?.composer.nextModelId,
			nextReasoning: threadView?.composer.nextReasoning,
			nextServiceTier: threadView?.composer.nextServiceTier,
			promptQueue: thread?.promptQueue ?? [],
			status: getThreadContentStatus(threadContent?.status),
			isStreaming:
				isThreadSnapshotRunning(thread) ||
				threadContent?.isStreaming === true ||
				isSessionThreadStatusRunningForThread(
					session?.threadStatus,
					threadId,
				) ||
				(!canLoadSessionThreads(session?.sandboxStatus) &&
					(isSessionTransitioningStatus(session?.sandboxStatus) ||
						messages.some((message) => message.provisional === true))),
			hasPendingQuestion: hasPendingQuestion(
				messages,
				threadContent?.pendingQuestionId,
				threadContent?.answeredApprovalIds ?? {},
			),
			pendingComments: threadView?.composer.pendingComments ?? [],
		};
	});

	const hooksExpanded = $derived(sessionView?.hooks.expanded ?? false);
	const hasAttachedComposerPanel = $derived(
		!isPending &&
			(threadComposerState.promptQueue.length > 0 ||
				(hooksExpanded && hooksStatus.hooks.length > 0)),
	);
	const pendingWorkspaceRequiresSourceInput = $derived.by(() =>
		getPendingWorkspaceRequiresSourceInput(sessionView?.pendingWorkspace),
	);
	const pendingWorkspaceSourceIsValid = $derived.by(() =>
		getPendingWorkspaceSourceIsValid(sessionView?.pendingWorkspace),
	);
	const pendingSandboxProviderId = $derived(
		sessionView?.pendingWorkspace.sandboxProviderId ?? "",
	);
	function setHooksExpanded(expanded: boolean) {
		void context.commands.view.setSessionHooksExpanded(sessionId, expanded);
	}
	function setPendingSandboxProviderId(providerId: string) {
		void context.commands.view.setPendingWorkspaceSandboxProviderId(
			sessionId,
			providerId,
		);
	}
	function resetPendingWorkspaceSetup() {
		void context.commands.view.resetPendingWorkspaceSetup(sessionId);
	}

	const effectiveModelId = $derived.by(
		() => threadComposerState.nextModelId ?? threadComposerState.modelId,
	);
	const selectedModelId = $derived.by(() =>
		threadComposerState.nextModelId !== undefined
			? (threadComposerState.nextModelId ?? preferences.defaultModel) || null
			: effectiveModelId,
	);
	const selectedModel = $derived.by(() => findModelById(selectedModelId));
	const effectiveReasoning = $derived.by(() =>
		getReasoningForModel(
			selectedModel,
			threadComposerState.nextReasoning,
			threadComposerState.reasoning,
		),
	);
	const effectiveServiceTier = $derived.by(() =>
		getServiceTierForModel(
			selectedModel,
			threadComposerState.nextServiceTier,
			threadComposerState.serviceTier,
		),
	);
	const reasoningLevels = $derived.by(
		() => selectedModel?.reasoningLevels ?? [],
	);
	const serviceTiers = $derived.by(() => selectedModel?.serviceTiers ?? []);
	const hasAvailableModels = $derived.by(() => modelItems.length > 0);
	const sessionSetupDisabled = $derived.by(
		() => pendingWorkspaceRequiresSourceInput && !pendingWorkspaceSourceIsValid,
	);
	const showPendingWorkspaceSelector = $derived.by(
		() => isPending && !threadComposerState.isStreaming,
	);
	const availableCommands = $derived.by(() =>
		isPending
			? []
			: (sessionCommands?.allIds ?? [])
					.map((id) => sessionCommands?.byId[id])
					.filter((command): command is AgentCommand => Boolean(command)),
	);
	const commandsLoading = $derived.by(
		() =>
			!isPending &&
			(sessionCommands?.status.state ?? "idle") !== "ready" &&
			(sessionCommands?.status.state ?? "idle") !== "error",
	);
	const selectableSandboxProviders = $derived.by(() =>
		sandboxProviders.filter((provider) => provider.available),
	);
	const selectedSandboxProvider = $derived.by(() =>
		selectableSandboxProviders.find(
			(provider) =>
				provider.id === (pendingSandboxProviderId || sandboxDefaultProviderId),
		),
	);
	const selectedSandboxProviderTitle = $derived.by(() => {
		if (!selectedSandboxProvider) {
			return "Sandbox provider";
		}
		return pendingSandboxProviderId
			? selectedSandboxProvider.name
			: `Default provider: ${selectedSandboxProvider.name}`;
	});
	const sandboxProviderSelectValue = $derived(
		pendingSandboxProviderId || sandboxDefaultProviderId,
	);

	function handleSandboxProviderSelect(value: string) {
		setPendingSandboxProviderId(
			value === sandboxDefaultProviderId ? "" : value,
		);
	}

	async function handleManageSandboxProvidersClick() {
		sandboxProviderMobileSelectOpen = false;
		sandboxProviderDesktopSelectOpen = false;
		await tick();
		void context.commands.dialogs.openSettingsDialog("providers");
	}

	function handleModelSelect(nextSelection: string | null) {
		const parsedSelection = parseComposerModelSelection(nextSelection);
		const nextModel = findModelById(
			(parsedSelection.modelId ?? preferences.defaultModel) || null,
		);
		const nextReasoning = getNextReasoningForModel(
			nextModel,
			threadComposerState.nextReasoning,
			threadComposerState.reasoning,
		);
		const nextServiceTier = getServiceTierForModel(
			nextModel,
			threadComposerState.nextServiceTier,
			threadComposerState.serviceTier,
		);

		if (parsedSelection.modelId === threadComposerState.modelId) {
			void context.commands.threadComposer.setThreadNextModelId(
				sessionId,
				threadId,
				undefined,
			);
			void context.commands.threadComposer.setThreadNextReasoning(
				sessionId,
				threadId,
				isSameReasoningSelection(nextReasoning, threadComposerState.reasoning)
					? undefined
					: nextReasoning,
			);
			void context.commands.threadComposer.setThreadNextServiceTier(
				sessionId,
				threadId,
				isSameServiceTierSelection(
					nextServiceTier,
					threadComposerState.serviceTier,
				)
					? undefined
					: nextServiceTier,
			);
			return;
		}

		void context.commands.threadComposer.setThreadNextModelId(
			sessionId,
			threadId,
			parsedSelection.modelId,
		);
		void context.commands.threadComposer.setThreadNextReasoning(
			sessionId,
			threadId,
			nextReasoning,
		);
		void context.commands.threadComposer.setThreadNextServiceTier(
			sessionId,
			threadId,
			nextServiceTier,
		);
	}

	function handleReasoningSelect(nextReasoning: string | undefined) {
		if (nextReasoning === "default") {
			const modelDefaultReasoning = selectedModel?.defaultReasoning;
			if (effectiveReasoning === undefined) {
				void context.commands.threadComposer.setThreadNextReasoning(
					sessionId,
					threadId,
					threadComposerState.nextModelId === undefined &&
						(threadComposerState.reasoning === undefined ||
							threadComposerState.reasoning === "default")
						? undefined
						: "default",
				);
				return;
			}
			void context.commands.threadComposer.setThreadNextReasoning(
				sessionId,
				threadId,
				modelDefaultReasoning ?? "default",
			);
			return;
		}

		void context.commands.threadComposer.setThreadNextReasoning(
			sessionId,
			threadId,
			threadComposerState.nextModelId === undefined &&
				isSameReasoningSelection(nextReasoning, threadComposerState.reasoning)
				? undefined
				: nextReasoning,
		);
	}

	function handleServiceTierSelect(nextServiceTier: string | undefined) {
		void context.commands.threadComposer.setThreadNextServiceTier(
			sessionId,
			threadId,
			threadComposerState.nextModelId === undefined &&
				isSameServiceTierSelection(
					nextServiceTier,
					threadComposerState.serviceTier,
				)
				? undefined
				: (nextServiceTier ?? null),
		);
	}

	const submitStatus = $derived.by(() => {
		if (isPending) return "ready" as const;
		if (threadComposerState.status === "loading") return "submitted" as const;
		if (threadComposerState.isStreaming) return "streaming" as const;
		if (threadComposerState.status === "error") return "error" as const;
		return "ready" as const;
	});
	const composerDisabledMessage = $derived.by(() => {
		if (!hasAvailableModels) {
			return "Please add a valid LLM provider credential";
		}
		if (threadComposerState.hasPendingQuestion) {
			return "Answer the agent's pending question before sending a new message.";
		}
		return null;
	});
	const composerDisabled = $derived.by(() => composerDisabledMessage !== null);

	function isGenerating() {
		return (
			!isPending &&
			(threadComposerState.status === "loading" ||
				threadComposerState.isStreaming)
		);
	}

	function inputEmpty() {
		return (
			composerDraft.trim().length === 0 &&
			threadComposerState.pendingComments.length === 0
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

	function handleDocumentPointerDown(event: PointerEvent) {
		if (!hooksExpanded) {
			return;
		}

		const target = event.target;
		if (!(target instanceof Node)) {
			return;
		}

		if (
			hooksPanelContainer?.contains(target) ||
			hooksControlContainer?.contains(target)
		) {
			return;
		}

		setHooksExpanded(false);
	}

	async function toggleComposerExpanded() {
		composerExpanded = !composerExpanded;
		await focusComposerTextarea();
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
		document.addEventListener("pointerdown", handleDocumentPointerDown, true);
		return () => {
			window.removeEventListener(
				sandboxProvidersUpdatedEvent,
				handleSandboxProvidersUpdated,
			);
			document.removeEventListener(
				"pointerdown",
				handleDocumentPointerDown,
				true,
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
			...(pendingSandboxProviderId
				? { providerId: pendingSandboxProviderId }
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
				pendingSandboxProviderId &&
				!response.providers.some(
					(provider) =>
						provider.id === pendingSandboxProviderId && provider.available,
				)
			) {
				setPendingSandboxProviderId("");
			}
		} catch (error) {
			sandboxProvidersError =
				error instanceof Error ? error.message : "Failed to load providers.";
		}
	}

	function movePendingDraftToThread(nextThreadId: string, draft: string) {
		void context.commands.threadComposer.movePendingComposerDraftToThread(
			threadId,
			nextThreadId,
			draft,
		);
	}

	function clearCurrentDraft() {
		void context.commands.threadComposer.clearComposerDraft(
			sessionId,
			threadId,
		);
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
		await context.commands.threadComposer.deleteQueuedPrompt(
			sessionId,
			threadId,
			queueId,
		);
	}

	async function createSessionForComposerAutocomplete(): Promise<boolean> {
		if (!isPending) {
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
		await context.commands.threadComposer.updateQueuedPrompt(
			sessionId,
			threadId,
			queueId,
			payload,
		);
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
			: threadComposerState.pendingComments;
		const emptyWithoutAttachments =
			inputEmpty() && attachmentFiles.length === 0;
		if (isGenerating() && emptyWithoutAttachments) {
			await context.commands.threadComposer.cancelThread(sessionId, threadId);
			composerTextareaRef?.closeMentionDropdown();
			composerTextareaRef?.closeSlashCommandDropdown();
			composerTextareaRef?.closePromptHistoryDropdown();
			return false;
		}
		if (!isPending && emptyWithoutAttachments) {
			return false;
		}

		pendingSubmitError = null;
		const wasPending = isPending;
		const currentDraft = composerDraft;
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
				void context.commands.preferences.addPromptToHistory(nextMessageText);
			}
			clearCurrentDraft();
		}

		try {
			const result = await context.commands.threadComposer.submitThread(
				sessionId,
				threadId,
				{
					parts: nextMessageParts,
					allowEmptyPendingMessage: shouldAllowEmptyPendingMessage,
					...(nextRunAfter ? { runAfter: nextRunAfter } : {}),
					...pendingSubmitOptions,
				},
			);
			if (wasPending && result) {
				await context.commands.navigation.completePendingSession(
					sessionId,
					result.sessionId,
				);
				if (preserveDraft) {
					movePendingDraftToThread(result.threadId, currentDraft);
				}
				void context.commands.threadComposer.clearThreadNextComposerValues(
					sessionId,
					threadId,
				);
				resetPendingWorkspaceSetup();
			}
			if (!preserveDraft) {
				void context.commands.threadComposer.clearThreadNextComposerValues(
					sessionId,
					threadId,
				);
				void context.commands.threadComposer.clearThreadPendingComments(
					sessionId,
					threadId,
				);
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
		return api.getThreadTokenUsage(sessionId, threadId);
	}
</script>

<div
	bind:this={composerContainer}
	class={composerExpanded
		? "fixed inset-0 z-50 flex min-h-0 flex-col bg-background p-3 md:p-6"
		: "shrink-0 bg-background p-0 md:p-3"}
>
	<div
		class={`w-full ${composerExpanded ? "flex min-h-0 flex-1 flex-col" : preferences.chatWidthMode === "constrained" ? "md:mx-auto md:max-w-3xl" : ""}`}
	>
		{#if !isPending}
			<ConversationPromptQueuePanel
				entries={threadComposerState.promptQueue}
				onDelete={handleDeleteQueuedPrompt}
				onUpdate={handleUpdateQueuedPrompt}
			/>

			<div bind:this={hooksPanelContainer}>
				<ConversationHooksPanel
					{sessionId}
					{threadId}
					expanded={hooksExpanded}
					{hooksStatus}
					outputById={hookOutputById}
					onRerunHook={(hookId) =>
						void context.commands.hooks.rerunHook(sessionId, hookId)}
					onSetExecutionPaused={(paused) => {
						void context.commands.hooks.pauseHooks(sessionId, paused);
						setHooksExpanded(false);
					}}
					onSetHookExecutionPaused={(hookId, paused) => {
						void context.commands.hooks.pauseHook(sessionId, hookId, paused);
					}}
				/>
			</div>
		{/if}

		{#if isPending || session?.sandboxStatus !== "ready"}
			<ConversationComposerSessionSetupStatus {sessionId} {threadId} />
			{#if showPendingWorkspaceSelector}
				<div class="mb-2 flex w-full flex-col gap-2 px-1 md:hidden">
					<ConversationWorkspaceSelector
						{sessionId}
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
		{#if threadComposerState.pendingComments.length > 0}
			<div
				class="mb-2 rounded-xl border border-amber-300/70 bg-amber-50 p-3 text-sm shadow-sm dark:border-amber-400/30 dark:bg-amber-950/20"
			>
				<div class="mb-2 font-medium text-foreground">
					{threadComposerState.pendingComments.length}
					{threadComposerState.pendingComments.length === 1
						? "comment"
						: "comments"} ready to submit
				</div>
				<div class="space-y-2">
					{#each threadComposerState.pendingComments as comment (comment.id)}
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
									onclick={() =>
										void context.commands.threadComposer.removeThreadPendingComment(
											sessionId,
											threadId,
											comment.id,
										)}
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
		{#if isPending && sandboxProvidersError}
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
						onclick={() =>
							void context.commands.dialogs.openCredentialsDialog()}
					>
						Open credentials
					</Button>
				{/if}
			</div>
		{/if}

		<div
			class={composerExpanded
				? "relative flex min-h-0 flex-1 flex-col"
				: "relative"}
		>
			<form
				class={composerExpanded ? "flex min-h-0 flex-1 flex-col" : undefined}
				onsubmit={(event) => {
					event.preventDefault();
					void submitComposer();
				}}
			>
				<InputGroup
					class={`${composerExpanded ? "min-h-0 flex-1 items-stretch has-[[data-slot=input-group-control]:focus-visible]:border-input has-[[data-slot=input-group-control]:focus-visible]:ring-0" : ""} ${
						hasAttachedComposerPanel
							? "rounded-t-none rounded-b-md"
							: "rounded-t-md rounded-b-none md:rounded-md"
					} group/composer`}
				>
					<Button
						type="button"
						variant="ghost"
						size="icon-sm"
						class={`desktop-no-drag absolute top-2 right-2 z-10 text-muted-foreground hover:text-foreground ${
							composerExpanded
								? ""
								: "transition-opacity [@media(hover:hover)]:opacity-0 [@media(hover:hover)]:group-hover/composer:opacity-100 focus-visible:opacity-100"
						}`}
						aria-label={composerExpanded
							? "Restore composer"
							: "Maximize composer"}
						title={composerExpanded ? "Restore composer" : "Maximize composer"}
						onclick={toggleComposerExpanded}
					>
						{#if composerExpanded}
							<Minimize2Icon class="size-4" />
						{:else}
							<Maximize2Icon class="size-4" />
						{/if}
					</Button>
					<ConversationComposerAttachments
						files={attachmentFiles}
						onRemove={removeAttachment}
					/>

					<ConversationComposerTextarea
						bind:this={composerTextareaRef}
						draft={composerDraft}
						disabled={composerDisabled}
						expanded={composerExpanded}
						onDraftChange={(v) =>
							void context.commands.threadComposer.setComposerDraft(
								sessionId,
								v,
							)}
						sessionId={isPending ? null : sessionId}
						commands={availableCommands}
						{commandsLoading}
						{fileMentionSuggestions}
						{fileMentionLoading}
						promptHistory={preferences.promptHistory}
						pinnedPrompts={preferences.pinnedPrompts}
						attachmentCount={attachmentFiles.length}
						onAddFiles={addFiles}
						onRemoveLastAttachment={removeLastAttachment}
						onPinPrompt={(prompt) =>
							void context.commands.preferences.pinPrompt(prompt)}
						onUnpinPrompt={(prompt) =>
							void context.commands.preferences.unpinPrompt(prompt)}
						onRemovePromptFromHistory={(prompt) =>
							void context.commands.preferences.removePromptFromHistory(prompt)}
						isPromptPinned={(prompt) =>
							preferences.pinnedPrompts.includes(prompt)}
						onFileMentionQueryChange={handleFileMentionQueryChange}
						onRequestAutocompleteSession={createSessionForComposerAutocomplete}
						onSubmit={handleComposerSubmit}
					/>

					{#if !composerExpanded}
						<InputGroupAddon align="block-end" class="justify-between gap-1">
							<div
								class="desktop-no-drag flex min-w-0 flex-1 flex-wrap items-center gap-1"
							>
								<ConversationComposerAttachmentButton
									onFilesAdd={addFiles}
									disabled={composerDisabled}
								/>
								{#if !isPending}
									<ConversationCredentialsControl {sessionId} />
								{/if}
								<div class="flex min-w-0 items-center gap-0">
									<ConversationComposerModelControl
										value={threadComposerState.nextModelId !== undefined
											? threadComposerState.nextModelId
											: threadComposerState.modelId}
										onSelect={handleModelSelect}
										models={modelItems}
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
										usage={threadComposerState.thread?.tokenUsage}
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
										<ConversationWorkspaceSelector
											{sessionId}
											bind:this={sessionSetupRef}
										/>
									</div>
								{:else if !isPending}
									<div bind:this={hooksControlContainer}>
										<ConversationComposerHooksControl
											expanded={hooksExpanded}
											onExpandedChange={setHooksExpanded}
											{hooksStatus}
											threadPhase={thread?.phase ?? ""}
										/>
									</div>
								{/if}
								<Popover bind:open={schedulePopoverOpen}>
									<PopoverTrigger>
										<Button
											variant={scheduledRunAfter ? "default" : "ghost"}
											size="icon-sm"
											title={scheduledSubmitLabel ?? "Schedule prompt"}
											aria-label={scheduledSubmitLabel ?? "Schedule prompt"}
											disabled={composerDisabled ||
												(isPending ? sessionSetupDisabled : false)}
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
									{isPending}
									disabled={composerDisabled ||
										(isPending ? sessionSetupDisabled : false)}
									onPress={handleComposerSubmit}
								/>
							</div>
						</InputGroupAddon>
					{/if}
				</InputGroup>
			</form>
		</div>
	</div>
</div>
