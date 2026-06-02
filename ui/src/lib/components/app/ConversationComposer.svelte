<script lang="ts">
	import ClockIcon from "@lucide/svelte/icons/clock";
	import Maximize2Icon from "@lucide/svelte/icons/maximize-2";
	import Minimize2Icon from "@lucide/svelte/icons/minimize-2";
	import SettingsIcon from "@lucide/svelte/icons/settings";
	import XIcon from "@lucide/svelte/icons/x";
	import { onDestroy, onMount, tick } from "svelte";
	import { api } from "$lib/api-client";
	import { InputGroup, InputGroupAddon } from "$lib/components/ui/input-group";
	import { Button } from "$lib/components/ui/button";
	import { Label } from "$lib/components/ui/label";
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectSeparator,
		SelectTrigger,
	} from "$lib/components/ui/select";
	import ConversationComposerAttachmentButton from "$lib/components/app/parts/ConversationComposerAttachmentButton.svelte";
	import ConversationComposerAttachments from "$lib/components/app/parts/ConversationComposerAttachments.svelte";
	import ConversationComposerHooksControl from "$lib/components/app/parts/ConversationComposerHooksControl.svelte";
	import ConversationComposerModelControl from "$lib/components/app/parts/ConversationComposerModelControl.svelte";
	import ConversationComposerReasoningControl from "$lib/components/app/parts/ConversationComposerReasoningControl.svelte";
	import ConversationComposerServiceTierControl from "$lib/components/app/parts/ConversationComposerServiceTierControl.svelte";
	import ConversationPromptQueuePanel from "$lib/components/app/parts/ConversationPromptQueuePanel.svelte";
	import ConversationComposerSessionSetupStatus from "$lib/components/app/ConversationComposerSessionSetupStatus.svelte";
	import ConversationComposerSubmitButton from "$lib/components/app/parts/ConversationComposerSubmitButton.svelte";
	import ConversationPromptSchedulePicker from "$lib/components/app/parts/ConversationPromptSchedulePicker.svelte";
	import ProviderIcon from "$lib/components/app/parts/ProviderIcon.svelte";
	import ConversationComposerTextarea from "$lib/components/app/parts/ConversationComposerTextarea.svelte";
	import ConversationCredentialsControl from "$lib/components/app/ConversationCredentialsControl.svelte";
	import ConversationHooksPanel from "$lib/components/app/ConversationHooksPanel.svelte";
	import ConversationWorkspaceSelector from "$lib/components/app/ConversationWorkspaceSelector.svelte";
	import {
		Popover,
		PopoverContent,
		PopoverTrigger,
	} from "$lib/components/ui/popover";
	import * as Dialog from "$lib/components/ui/dialog";
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
		TokenPrices,
		TokenUsage,
		ThreadTokenUsageDetails,
		TokenUsageInfo,
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

	type TokenUsageLaneKey =
		| "inputNoCache"
		| "cacheRead"
		| "cacheWrite"
		| "outputText"
		| "outputReasoning";

	type TokenUsageLane = {
		key: TokenUsageLaneKey;
		label: string;
		description: string;
		color: string;
		total: number;
		maxStepValue: number;
	};

	type TokenUsageChartStep = {
		id: string;
		turnId: string;
		turnIndex: number;
		turnLabel: string;
		stepIndex: number;
		stepLabel: string;
		toolCalls: string[];
		usage?: TokenUsage;
		values: Record<TokenUsageLaneKey, number>;
		total: number;
	};

	type TokenUsageChartTurn = {
		id: string;
		label: string;
		subtitle: string;
		stepCount: number;
		values: Record<TokenUsageLaneKey, number>;
		total: number;
	};

	type TokenUsageChart = {
		lanes: TokenUsageLane[];
		turns: TokenUsageChartTurn[];
		steps: TokenUsageChartStep[];
		maxLaneTotal: number;
	};

	type TokenUsageChartColumn = {
		id: string;
		turnId: string;
		type: "turn" | "step";
		turnLabel: string;
		label: string;
		toolCalls: string[];
		values: Record<TokenUsageLaneKey, number>;
		total: number;
	};

	type TokenUsageChartRenderTurn = TokenUsageChartTurn & {
		columnCount: number;
		expanded: boolean;
	};

	type TokenUsageChartRender = {
		lanes: TokenUsageLane[];
		turns: TokenUsageChartRenderTurn[];
		columns: TokenUsageChartColumn[];
		maxLaneTotal: number;
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
	let tokenUsageDetailsOpen = $state(false);
	let tokenUsageDetailsLoading = $state(false);
	let tokenUsageDetailsError = $state<string | null>(null);
	let tokenUsageDetails = $state<ThreadTokenUsageDetails | null>(null);
	let tokenUsageDetailsThreadId = $state<string | null>(null);
	let tokenUsageDetailsFullscreen = $state(false);
	let expandedTokenUsageTurns = $state<Record<string, boolean>>({});

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

	function usageTotal(usage: TokenUsage | undefined): number {
		return (usage?.inputTokens?.total ?? 0) + (usage?.outputTokens?.total ?? 0);
	}

	function formatRounded(value: number): string {
		return Number.isInteger(value) ? value.toString() : value.toFixed(1);
	}

	function formatTokenCount(value: number): string {
		if (value >= 1_000_000) {
			return `${formatRounded(value / 1_000_000)}M`;
		}
		if (value >= 1_000) {
			return `${formatRounded(value / 1_000)}K`;
		}
		return `${value}`;
	}

	function formatFullTokenCount(value: number): string {
		return new Intl.NumberFormat().format(value);
	}

	function formatPercent(value: number): string {
		return value < 1 ? "<1%" : `${Math.round(value)}%`;
	}

	function formatTokenPrice(value: number | undefined): string | null {
		if (!value) {
			return null;
		}
		return `$${value.toFixed(value >= 1 ? 2 : 4)} / 1M`;
	}

	function estimateUsageCost(
		usage: TokenUsage | undefined,
		prices: TokenPrices | undefined,
	) {
		const inputPrice = prices?.input ?? 0;
		const outputPrice = prices?.output ?? 0;
		if (inputPrice <= 0 && outputPrice <= 0) {
			return null;
		}
		return (
			((usage?.inputTokens?.total ?? 0) * inputPrice +
				(usage?.outputTokens?.total ?? 0) * outputPrice) /
			1_000_000
		);
	}

	function formatEstimatedCost(value: number | null): string | null {
		if (value === null) {
			return null;
		}
		if (value > 0 && value < 0.0001) {
			return "<$0.0001";
		}
		return `$${value.toFixed(value < 0.01 ? 4 : 2)}`;
	}

	function tokenUsageRows(usage: TokenUsage | undefined) {
		return [
			{
				label: "Input",
				value: usage?.inputTokens?.total ?? 0,
			},
			{
				label: "No cache",
				value: usage?.inputTokens?.noCache ?? 0,
			},
			{
				label: "Cache read",
				value: usage?.inputTokens?.cacheRead ?? 0,
			},
			{
				label: "Cache write",
				value: usage?.inputTokens?.cacheWrite ?? 0,
			},
			{
				label: "Output",
				value: usage?.outputTokens?.total ?? 0,
			},
			{
				label: "Text",
				value: usage?.outputTokens?.text ?? 0,
			},
			{
				label: "Reasoning",
				value: usage?.outputTokens?.reasoning ?? 0,
			},
			{
				label: "Total",
				value: usageTotal(usage),
			},
		];
	}

	function visibleTokenUsageRows(
		lastTurnRows: ReturnType<typeof tokenUsageRows>,
		totalRows: ReturnType<typeof tokenUsageRows>,
	) {
		return lastTurnRows
			.map((row, index) => ({
				label: row.label,
				lastValue: row.value,
				totalValue: totalRows[index]?.value ?? 0,
			}))
			.filter((row) => row.lastValue > 0 || row.totalValue > 0);
	}

	function buildTokenUsageSummary(usage: TokenUsageInfo | undefined) {
		const totalTokens = usageTotal(usage?.total);
		const lastTurnTokens = usageTotal(usage?.lastTurn);
		const contextTokens = usage?.lastStep?.inputTokens?.total ?? 0;
		const modelMaxTokens = usage?.modelMaxTokens ?? 0;
		const contextPercent =
			contextTokens > 0 && modelMaxTokens > 0
				? (contextTokens / modelMaxTokens) * 100
				: 0;

		if (totalTokens <= 0 && lastTurnTokens <= 0 && contextTokens <= 0) {
			return null;
		}

		const contextText =
			contextTokens > 0 && modelMaxTokens > 0
				? `${formatTokenCount(contextTokens)} / ${formatTokenCount(modelMaxTokens)} (${formatPercent((contextTokens / modelMaxTokens) * 100)})`
				: null;
		const inputPriceText = formatTokenPrice(usage?.prices?.input);
		const outputPriceText = formatTokenPrice(usage?.prices?.output);
		const lastTurnCost = formatEstimatedCost(
			estimateUsageCost(usage?.lastTurn, usage?.prices),
		);
		const totalCost = formatEstimatedCost(
			estimateUsageCost(usage?.total, usage?.prices),
		);
		const labelParts = [
			contextText ? `Context: ${contextText}` : null,
			`Last turn: ${formatFullTokenCount(lastTurnTokens)} tokens`,
			`Thread total: ${formatFullTokenCount(totalTokens)} tokens`,
			totalCost ? `Estimated total cost: ${totalCost}` : null,
		].filter((part): part is string => part !== null);

		return {
			contextText,
			primaryText:
				contextPercent > 0
					? formatPercent(contextPercent)
					: formatTokenCount(lastTurnTokens),
			modelMaxTokensText:
				modelMaxTokens > 0 ? formatFullTokenCount(modelMaxTokens) : null,
			maxOutputTokensText:
				(usage?.maxOutputTokens ?? 0) > 0
					? formatFullTokenCount(usage?.maxOutputTokens ?? 0)
					: null,
			lastTurnText: formatTokenCount(lastTurnTokens),
			totalText: formatTokenCount(totalTokens),
			rows: visibleTokenUsageRows(
				tokenUsageRows(usage?.lastTurn),
				tokenUsageRows(usage?.total),
			),
			inputPriceText,
			outputPriceText,
			lastTurnCost,
			totalCost,
			ariaLabel: labelParts.join(", "),
		};
	}

	function visibleUsageRows(usage: TokenUsage | undefined) {
		return tokenUsageRows(usage).filter((row) => row.value > 0);
	}

	const tokenUsageLaneDefinitions: Array<
		Omit<TokenUsageLane, "total" | "maxStepValue">
	> = [
		{
			key: "inputNoCache",
			label: "Input",
			description: "Prompt tokens that were not served from cache",
			color: "#3b82f6",
		},
		{
			key: "cacheRead",
			label: "Cache read",
			description: "Input tokens read from provider cache",
			color: "#10b981",
		},
		{
			key: "cacheWrite",
			label: "Cache write",
			description: "Input tokens written to provider cache",
			color: "#f59e0b",
		},
		{
			key: "outputText",
			label: "Output text",
			description: "Visible assistant output tokens",
			color: "#8b5cf6",
		},
		{
			key: "outputReasoning",
			label: "Reasoning",
			description: "Reasoning output tokens",
			color: "#d946ef",
		},
	];

	function tokenLaneValues(
		usage: TokenUsage | undefined,
	): Record<TokenUsageLaneKey, number> {
		return {
			inputNoCache: usage?.inputTokens?.noCache ?? 0,
			cacheRead: usage?.inputTokens?.cacheRead ?? 0,
			cacheWrite: usage?.inputTokens?.cacheWrite ?? 0,
			outputText: usage?.outputTokens?.text ?? 0,
			outputReasoning: usage?.outputTokens?.reasoning ?? 0,
		};
	}

	function tokenUsageChartGridColumns(stepCount: number): string {
		return `8.5rem repeat(${stepCount}, minmax(4.75rem, 1fr)) 6rem`;
	}

	function tokenUsageBarPercent(value: number, maxValue: number): number {
		if (value <= 0 || maxValue <= 0) {
			return 0;
		}
		return Math.max(4, (value / maxValue) * 100);
	}

	function tokenUsageCellTitle(
		lane: TokenUsageLane,
		column: TokenUsageChartColumn,
	): string {
		const parts = [
			`${column.turnLabel} · ${column.label}`,
			`${lane.label}: ${formatFullTokenCount(column.values[lane.key])} tokens`,
			`${column.type === "step" ? "Step" : "Turn"} total: ${formatFullTokenCount(column.total)} tokens`,
		];
		if (column.toolCalls.length > 0) {
			parts.push(`Tool calls: ${column.toolCalls.join(", ")}`);
		}
		return parts.join("\n");
	}

	function buildTokenUsageChartRender(
		chart: TokenUsageChart | null,
		expandedTurns: Record<string, boolean>,
	): TokenUsageChartRender | null {
		if (!chart) {
			return null;
		}

		const columns: TokenUsageChartColumn[] = [];
		const turns = chart.turns.map((usageTurn) => {
			const turnSteps = chart.steps.filter(
				(step) => step.turnId === usageTurn.id,
			);
			const expanded =
				Boolean(expandedTurns[usageTurn.id]) && turnSteps.length > 1;

			if (expanded) {
				for (const step of turnSteps) {
					columns.push({
						id: step.id,
						turnId: usageTurn.id,
						type: "step",
						turnLabel: step.turnLabel,
						label: step.stepLabel,
						toolCalls: step.toolCalls,
						values: step.values,
						total: step.total,
					});
				}
			} else {
				columns.push({
					id: usageTurn.id,
					turnId: usageTurn.id,
					type: "turn",
					turnLabel: usageTurn.label,
					label: "Turn total",
					toolCalls: [],
					values: usageTurn.values,
					total: usageTurn.total,
				});
			}

			return {
				...usageTurn,
				columnCount: expanded ? turnSteps.length : 1,
				expanded,
			};
		});

		const lanes = chart.lanes.map((lane) => ({
			...lane,
			maxStepValue: Math.max(
				0,
				...columns.map((column) => column.values[lane.key]),
			),
		}));

		return {
			lanes,
			turns,
			columns,
			maxLaneTotal: chart.maxLaneTotal,
		};
	}

	function buildTokenUsageChart(
		details: ThreadTokenUsageDetails | null,
	): TokenUsageChart | null {
		if (!details) {
			return null;
		}

		const turns: TokenUsageChartTurn[] = [];
		const steps: TokenUsageChartStep[] = [];
		for (const [turnIndex, usageTurn] of (details.turns ?? []).entries()) {
			const turnSteps = usageTurn.steps ?? [];
			if (turnSteps.length === 0) {
				continue;
			}
			const label = `Turn ${turnIndex + 1}`;
			turns.push({
				id: usageTurn.id,
				label,
				subtitle: tokenUsageTurnSubtitle(
					usageTurn.model,
					usageTurn.reasoning,
					usageTurn.serviceTier,
				),
				stepCount: turnSteps.length,
				values: tokenLaneValues(usageTurn.usage),
				total: usageTotal(usageTurn.usage),
			});
			for (const usageStep of turnSteps) {
				steps.push({
					id: `${usageTurn.id}:${usageStep.index}`,
					turnId: usageTurn.id,
					turnIndex,
					turnLabel: label,
					stepIndex: usageStep.index,
					stepLabel: `Step ${usageStep.index + 1}`,
					toolCalls: (usageStep.toolCalls ?? []).map(
						(call) => call.name || call.id,
					),
					usage: usageStep.usage,
					values: tokenLaneValues(usageStep.usage),
					total: usageTotal(usageStep.usage),
				});
			}
		}

		if (steps.length === 0) {
			return null;
		}

		const summaryValues = tokenLaneValues(details.summary?.total);
		const lanes = tokenUsageLaneDefinitions
			.map((definition) => {
				const stepTotal = steps.reduce(
					(total, step) => total + step.values[definition.key],
					0,
				);
				return {
					...definition,
					total: summaryValues[definition.key] || stepTotal,
					maxStepValue: Math.max(
						...steps.map((step) => step.values[definition.key]),
					),
				};
			})
			.filter((lane) => lane.total > 0 || lane.maxStepValue > 0);

		return {
			lanes,
			turns,
			steps,
			maxLaneTotal: Math.max(...lanes.map((lane) => lane.total)),
		};
	}

	function formatUsageDate(value: string | undefined): string | null {
		if (!value) {
			return null;
		}
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) {
			return value;
		}
		return new Intl.DateTimeFormat(undefined, {
			month: "short",
			day: "numeric",
			hour: "numeric",
			minute: "2-digit",
		}).format(date);
	}

	function tokenUsageTurnSubtitle(
		model: string | undefined,
		reasoning: string | undefined,
		serviceTier: string | undefined,
	): string {
		return [model, reasoning ? `reasoning ${reasoning}` : null, serviceTier]
			.filter((part): part is string => Boolean(part))
			.join(" · ");
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
	const tokenUsageSummary = $derived.by(() =>
		buildTokenUsageSummary(thread.thread?.tokenUsage),
	);
	const tokenUsageChart = $derived.by(() =>
		buildTokenUsageChart(tokenUsageDetails),
	);
	const visibleTokenUsageChart = $derived.by(() =>
		buildTokenUsageChartRender(tokenUsageChart, expandedTokenUsageTurns),
	);
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

	function toggleTokenUsageTurn(turnId: string) {
		expandedTokenUsageTurns = {
			...expandedTokenUsageTurns,
			[turnId]: !expandedTokenUsageTurns[turnId],
		};
	}

	async function openTokenUsageDetails() {
		tokenUsageDetailsOpen = true;
		const currentThreadID = thread.threadId;
		if (
			tokenUsageDetailsLoading &&
			tokenUsageDetailsThreadId === currentThreadID
		) {
			return;
		}

		tokenUsageDetailsLoading = true;
		tokenUsageDetailsError = null;
		tokenUsageDetailsThreadId = currentThreadID;
		try {
			tokenUsageDetails = await api.getThreadTokenUsage(
				session.sessionId,
				currentThreadID,
			);
			expandedTokenUsageTurns = {};
		} catch (err) {
			tokenUsageDetails = null;
			tokenUsageDetailsError =
				err instanceof Error ? err.message : "Failed to load token details";
		} finally {
			tokenUsageDetailsLoading = false;
		}
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
							<Label
								for="pending-sandbox-provider-mobile"
								class="text-xs text-muted-foreground">Sandbox provider</Label
							>
							<Select
								type="single"
								bind:open={sandboxProviderMobileSelectOpen}
								value={sandboxProviderSelectValue}
								onValueChange={handleSandboxProviderSelect}
							>
								<SelectTrigger
									id="pending-sandbox-provider-mobile"
									size="sm"
									class="h-9 px-3"
									title={selectedSandboxProviderTitle}
								>
									<ProviderIcon
										icon={selectedSandboxProvider?.icon}
										name={selectedSandboxProvider?.name ?? "Sandbox provider"}
										class="pointer-events-none size-4 border-0 bg-transparent"
									/>
								</SelectTrigger>
								<SelectContent>
									{#each selectableSandboxProviders as provider (provider.id)}
										<SelectItem value={provider.id} label={provider.name}>
											<ProviderIcon
												icon={provider.icon}
												name={provider.name}
												class="size-4"
											/>
											<span>{provider.name}</span>
											{#if provider.id === sandboxDefaultProviderId}
												<span
													class="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground"
												>
													default
												</span>
											{/if}
										</SelectItem>
									{/each}
									<SelectSeparator />
									<button
										type="button"
										class="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-hidden"
										onclick={handleManageSandboxProvidersClick}
									>
										<SettingsIcon class="size-4" />
										<span>Manage</span>
									</button>
								</SelectContent>
							</Select>
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
						attachmentCount={attachmentFiles.length}
						onAddFiles={addFiles}
						onRemoveLastAttachment={removeLastAttachment}
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
									<span class="shrink-0 text-xs text-muted-foreground/60"
										>·</span
									>
									<ConversationComposerReasoningControl
										value={effectiveReasoning}
										defaultValue={selectedModel.defaultReasoning}
										levels={reasoningLevels}
										onSelect={handleReasoningSelect}
									/>
								{/if}
								{#if serviceTiers.length > 0}
									<span class="shrink-0 text-xs text-muted-foreground/60"
										>·</span
									>
									<ConversationComposerServiceTierControl
										value={effectiveServiceTier}
										tiers={serviceTiers}
										onSelect={handleServiceTierSelect}
									/>
								{/if}
							</div>
							{#if tokenUsageSummary}
								<div class="flex items-center gap-1">
									<div class="group relative">
										<button
											type="button"
											class="flex h-6 appearance-none items-center gap-1.5 rounded-md border-0 bg-transparent px-2 text-xs font-normal text-muted-foreground"
											aria-label={tokenUsageSummary.ariaLabel}
										>
											<span>{tokenUsageSummary.primaryText}</span>
											<span class="text-border">/</span>
											<span>{tokenUsageSummary.totalText}</span>
										</button>
										<div
											class="absolute bottom-full left-0 z-50 hidden w-max max-w-[calc(100vw-2rem)] pb-2 group-hover:block group-focus-within:block"
										>
											<div
												class="rounded-lg border border-border bg-popover p-3 text-xs text-popover-foreground shadow-lg"
											>
												<div
													class="grid grid-cols-[auto_auto_auto] gap-x-4 gap-y-1.5"
												>
													<div></div>
													<div class="font-medium text-foreground">Last</div>
													<div class="font-medium text-foreground">Total</div>
													{#each tokenUsageSummary.rows as row (row.label)}
														<div class="text-muted-foreground">{row.label}</div>
														<div class="text-right tabular-nums">
															{formatFullTokenCount(row.lastValue)}
														</div>
														<div class="text-right tabular-nums">
															{formatFullTokenCount(row.totalValue)}
														</div>
													{/each}
													{#if tokenUsageSummary.lastTurnCost && tokenUsageSummary.totalCost}
														<div class="text-muted-foreground">Est. cost</div>
														<div class="text-right tabular-nums">
															{tokenUsageSummary.lastTurnCost}
														</div>
														<div class="text-right tabular-nums">
															{tokenUsageSummary.totalCost}
														</div>
													{/if}
												</div>
												{#if tokenUsageSummary.contextText || tokenUsageSummary.modelMaxTokensText}
													<div
														class="mt-2 space-y-1 border-t border-border pt-2 text-muted-foreground"
													>
														{#if tokenUsageSummary.contextText}
															<div>Context {tokenUsageSummary.contextText}</div>
														{/if}
														{#if tokenUsageSummary.modelMaxTokensText}
															<div>
																Model max {tokenUsageSummary.modelMaxTokensText} tokens
															</div>
														{/if}
														{#if tokenUsageSummary.maxOutputTokensText}
															<div>
																Max output {tokenUsageSummary.maxOutputTokensText}
																tokens
															</div>
														{/if}
														{#if tokenUsageSummary.inputPriceText || tokenUsageSummary.outputPriceText}
															<div>
																Prices:
																{#if tokenUsageSummary.inputPriceText}
																	input {tokenUsageSummary.inputPriceText}
																{/if}
																{#if tokenUsageSummary.inputPriceText && tokenUsageSummary.outputPriceText}
																	/
																{/if}
																{#if tokenUsageSummary.outputPriceText}
																	output {tokenUsageSummary.outputPriceText}
																{/if}
															</div>
														{/if}
													</div>
												{/if}
												<div class="mt-3 border-t border-border pt-3">
													<Button
														type="button"
														variant="outline"
														size="xs"
														class="h-7 w-full text-xs"
														onclick={() => void openTokenUsageDetails()}
													>
														Details
													</Button>
												</div>
											</div>
										</div>
									</div>
								</div>
							{/if}
						</div>

						<div class="desktop-no-drag flex items-center justify-end gap-2">
							{#if showPendingWorkspaceSelector}
								<div class="hidden items-center gap-2 md:flex">
									{#if selectableSandboxProviders.length > 0}
										<Label for="pending-sandbox-provider" class="sr-only"
											>Sandbox provider</Label
										>
										<Select
											type="single"
											bind:open={sandboxProviderDesktopSelectOpen}
											value={sandboxProviderSelectValue}
											onValueChange={handleSandboxProviderSelect}
										>
											<SelectTrigger
												id="pending-sandbox-provider"
												size="sm"
												class="h-8 px-2 text-xs"
												title={selectedSandboxProviderTitle}
											>
												<ProviderIcon
													icon={selectedSandboxProvider?.icon}
													name={selectedSandboxProvider?.name ??
														"Sandbox provider"}
													class="pointer-events-none size-4 border-0 bg-transparent"
												/>
											</SelectTrigger>
											<SelectContent class="min-w-44">
												{#each selectableSandboxProviders as provider (provider.id)}
													<SelectItem value={provider.id} label={provider.name}>
														<ProviderIcon
															icon={provider.icon}
															name={provider.name}
															class="size-4"
														/>
														<span>{provider.name}</span>
														{#if provider.id === sandboxDefaultProviderId}
															<span
																class="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground"
															>
																default
															</span>
														{/if}
													</SelectItem>
												{/each}
												<SelectSeparator />
												<button
													type="button"
													class="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-hidden"
													onclick={handleManageSandboxProvidersClick}
												>
													<SettingsIcon class="size-4" />
													<span>Manage</span>
												</button>
											</SelectContent>
										</Select>
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
						</div>
					</InputGroupAddon>
				</InputGroup>
			</form>
			<Dialog.Root bind:open={tokenUsageDetailsOpen}>
				<Dialog.Content
					class={`overflow-hidden ${tokenUsageDetailsFullscreen ? "h-[calc(100vh-2rem)] max-h-[calc(100vh-2rem)] sm:max-w-[calc(100vw-2rem)]" : "max-h-[85vh] sm:max-w-3xl"}`}
				>
					<Button
						type="button"
						variant="ghost"
						size="icon-sm"
						class="absolute top-4 right-12"
						aria-label={tokenUsageDetailsFullscreen
							? "Exit fullscreen token details"
							: "Expand token details fullscreen"}
						title={tokenUsageDetailsFullscreen
							? "Exit fullscreen"
							: "Expand fullscreen"}
						onclick={() =>
							(tokenUsageDetailsFullscreen = !tokenUsageDetailsFullscreen)}
					>
						{#if tokenUsageDetailsFullscreen}
							<Minimize2Icon class="size-4" />
						{:else}
							<Maximize2Icon class="size-4" />
						{/if}
					</Button>
					<Dialog.Header>
						<Dialog.Title>Conversation token details</Dialog.Title>
						<Dialog.Description>
							Full token accounting for this thread, including the numbers used
							by the composer summary.
						</Dialog.Description>
					</Dialog.Header>

					<div class="min-h-0 overflow-y-auto pr-1">
						{#if tokenUsageDetailsLoading}
							<div class="py-8 text-center text-sm text-muted-foreground">
								Loading token details…
							</div>
						{:else if tokenUsageDetailsError}
							<div
								class="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
							>
								{tokenUsageDetailsError}
							</div>
						{:else if tokenUsageDetails}
							<div class="space-y-4">
								<div class="rounded-lg border border-border p-3">
									<div class="mb-2 text-sm font-medium text-foreground">
										Composer summary sources
									</div>
									<div
										class="grid grid-cols-[auto_auto_auto] gap-x-4 gap-y-1.5 text-xs"
									>
										<div></div>
										<div class="font-medium text-foreground">Last turn</div>
										<div class="font-medium text-foreground">Thread total</div>
										{#each visibleTokenUsageRows(tokenUsageRows(tokenUsageDetails.summary?.lastTurn), tokenUsageRows(tokenUsageDetails.summary?.total)) as row (row.label)}
											<div class="text-muted-foreground">{row.label}</div>
											<div class="text-right tabular-nums">
												{formatFullTokenCount(row.lastValue)}
											</div>
											<div class="text-right tabular-nums">
												{formatFullTokenCount(row.totalValue)}
											</div>
										{/each}
									</div>
									<div
										class="mt-3 grid gap-1 border-t border-border pt-3 text-xs text-muted-foreground sm:grid-cols-2"
									>
										<div>
											Last context input:
											<span class="tabular-nums text-foreground">
												{formatFullTokenCount(
													tokenUsageDetails.summary?.lastStep?.inputTokens
														?.total ?? 0,
												)}
											</span>
										</div>
										<div>
											Model max:
											<span class="tabular-nums text-foreground">
												{formatFullTokenCount(
													tokenUsageDetails.summary?.modelMaxTokens ?? 0,
												)}
											</span>
										</div>
										<div>
											Max output:
											<span class="tabular-nums text-foreground">
												{formatFullTokenCount(
													tokenUsageDetails.summary?.maxOutputTokens ?? 0,
												)}
											</span>
										</div>
										<div>
											Estimated total cost:
											<span class="tabular-nums text-foreground">
												{formatEstimatedCost(
													estimateUsageCost(
														tokenUsageDetails.summary?.total,
														tokenUsageDetails.summary?.prices,
													),
												) ?? "n/a"}
											</span>
										</div>
									</div>
								</div>

								{#if visibleTokenUsageChart}
									<div class="rounded-lg border border-border p-3">
										<div
											class="mb-3 flex flex-wrap items-start justify-between gap-2"
										>
											<div>
												<div class="text-sm font-medium text-foreground">
													Token lanes
												</div>
												<div class="text-xs text-muted-foreground">
													Click a turn header to expand it into step columns.
													Bars are scaled within each lane.
												</div>
											</div>
											<div class="text-xs text-muted-foreground">
												{tokenUsageChart?.turns.length ?? 0} turns ·
												{tokenUsageChart?.steps.length ?? 0} completed steps
											</div>
										</div>

										<div class="mb-4 space-y-2">
											{#each visibleTokenUsageChart.lanes as lane (lane.key)}
												<div
													class="grid grid-cols-[8.5rem_1fr_6rem] items-center gap-3 text-xs"
												>
													<div class="min-w-0">
														<div class="truncate font-medium text-foreground">
															{lane.label}
														</div>
														<div
															class="truncate text-muted-foreground"
															title={lane.description}
														>
															{lane.description}
														</div>
													</div>
													<div
														class="h-2 overflow-hidden rounded-full bg-muted"
													>
														<div
															class="h-full rounded-full"
															style={`width: ${tokenUsageBarPercent(lane.total, visibleTokenUsageChart.maxLaneTotal)}%; background-color: ${lane.color}`}
														></div>
													</div>
													<div class="text-right tabular-nums text-foreground">
														{formatFullTokenCount(lane.total)}
													</div>
												</div>
											{/each}
										</div>

										<div
											class="overflow-x-auto rounded-md border border-border"
										>
											<div
												class="min-w-max text-xs"
												style={`grid-template-columns: ${tokenUsageChartGridColumns(visibleTokenUsageChart.columns.length)}`}
											>
												<div
													class="grid border-b border-border bg-muted/40"
													style={`grid-template-columns: ${tokenUsageChartGridColumns(visibleTokenUsageChart.columns.length)}`}
												>
													<div
														class="sticky left-0 z-10 bg-muted/95 px-3 py-2 font-medium text-muted-foreground"
													>
														Lane
													</div>
													{#each visibleTokenUsageChart.turns as usageTurn (usageTurn.id)}
														<div
															class="border-l border-border text-center"
															style={`grid-column: span ${usageTurn.columnCount} / span ${usageTurn.columnCount}`}
															title={usageTurn.subtitle}
														>
															<button
																type="button"
																class="hover:bg-muted focus-visible:ring-ring flex h-full w-full flex-col items-center justify-center px-2 py-2 outline-hidden transition-colors focus-visible:ring-2"
																aria-expanded={usageTurn.expanded}
																aria-label={`${usageTurn.expanded ? "Collapse" : "Expand"} ${usageTurn.label} token steps`}
																disabled={usageTurn.stepCount <= 1}
																onclick={() =>
																	toggleTokenUsageTurn(usageTurn.id)}
															>
																<div class="font-medium text-foreground">
																	{usageTurn.label}
																</div>
																<div class="tabular-nums text-muted-foreground">
																	{formatTokenCount(usageTurn.total)}
																	{#if usageTurn.stepCount > 1}
																		<span aria-hidden="true">
																			{usageTurn.expanded ? " ▲" : " ▼"}
																		</span>
																	{/if}
																</div>
															</button>
														</div>
													{/each}
													<div
														class="sticky right-0 z-10 border-l border-border bg-muted/95 px-3 py-2 text-right font-medium text-muted-foreground"
													>
														Total
													</div>
												</div>

												<div
													class="grid border-b border-border bg-background"
													style={`grid-template-columns: ${tokenUsageChartGridColumns(visibleTokenUsageChart.columns.length)}`}
												>
													<div
														class="sticky left-0 z-10 bg-background px-3 py-1.5 text-muted-foreground"
													>
														Column
													</div>
													{#each visibleTokenUsageChart.columns as usageColumn (usageColumn.id)}
														<div
															class="border-l border-border px-2 py-1.5 text-center text-muted-foreground"
														>
															{usageColumn.type === "step"
																? usageColumn.label.replace("Step ", "S")
																: "Total"}
														</div>
													{/each}
													<div
														class="sticky right-0 z-10 border-l border-border bg-background px-3 py-1.5 text-right text-muted-foreground"
													>
														Tokens
													</div>
												</div>

												{#each visibleTokenUsageChart.lanes as lane (lane.key)}
													<div
														class="grid border-b border-border last:border-b-0"
														style={`grid-template-columns: ${tokenUsageChartGridColumns(visibleTokenUsageChart.columns.length)}`}
													>
														<div
															class="sticky left-0 z-10 border-r border-border bg-background px-3 py-2 font-medium text-foreground"
														>
															{lane.label}
														</div>
														{#each visibleTokenUsageChart.columns as usageColumn (usageColumn.id)}
															<div
																class="border-l border-border px-2 py-2"
																title={tokenUsageCellTitle(lane, usageColumn)}
															>
																<div class="h-5 rounded bg-muted p-0.5">
																	<div
																		class="h-full rounded-sm"
																		style={`width: ${tokenUsageBarPercent(usageColumn.values[lane.key], lane.maxStepValue)}%; background-color: ${lane.color}`}
																	></div>
																</div>
																<div
																	class="mt-1 text-right tabular-nums text-muted-foreground"
																>
																	{formatTokenCount(
																		usageColumn.values[lane.key],
																	)}
																</div>
															</div>
														{/each}
														<div
															class="sticky right-0 z-10 border-l border-border bg-background px-3 py-2 text-right tabular-nums text-foreground"
														>
															{formatFullTokenCount(lane.total)}
														</div>
													</div>
												{/each}

												<div
													class="grid bg-muted/30"
													style={`grid-template-columns: ${tokenUsageChartGridColumns(visibleTokenUsageChart.columns.length)}`}
												>
													<div
														class="sticky left-0 z-10 border-r border-border bg-muted/95 px-3 py-2 font-medium text-foreground"
													>
														Column total
													</div>
													{#each visibleTokenUsageChart.columns as usageColumn (usageColumn.id)}
														<div
															class="border-l border-border px-2 py-2 text-right tabular-nums text-foreground"
														>
															{formatTokenCount(usageColumn.total)}
														</div>
													{/each}
													<div
														class="sticky right-0 z-10 border-l border-border bg-muted/95 px-3 py-2 text-right tabular-nums text-foreground"
													>
														{formatTokenCount(
															usageTotal(tokenUsageDetails.summary?.total),
														)}
													</div>
												</div>
											</div>
										</div>
									</div>
								{/if}

								{#if (tokenUsageDetails.turns?.length ?? 0) === 0}
									<div
										class="rounded-lg border border-border p-4 text-sm text-muted-foreground"
									>
										No completed turn steps have recorded token usage yet.
									</div>
								{:else}
									<div class="space-y-3">
										{#each tokenUsageDetails.turns ?? [] as usageTurn, turnIndex (usageTurn.id)}
											<div class="rounded-lg border border-border p-3">
												<div
													class="flex flex-wrap items-start justify-between gap-2"
												>
													<div class="min-w-0">
														<div class="font-medium text-foreground">
															Turn {turnIndex + 1}
														</div>
														{#if tokenUsageTurnSubtitle(usageTurn.model, usageTurn.reasoning, usageTurn.serviceTier)}
															<div
																class="break-all text-xs text-muted-foreground"
															>
																{tokenUsageTurnSubtitle(
																	usageTurn.model,
																	usageTurn.reasoning,
																	usageTurn.serviceTier,
																)}
															</div>
														{/if}
													</div>
													<div class="text-right text-xs text-muted-foreground">
														<div class="tabular-nums text-foreground">
															{formatFullTokenCount(
																usageTotal(usageTurn.usage),
															)}
															tokens
														</div>
														{#if formatUsageDate(usageTurn.startedAt)}
															<div>{formatUsageDate(usageTurn.startedAt)}</div>
														{/if}
													</div>
												</div>

												<div class="mt-3 grid gap-3 text-xs sm:grid-cols-2">
													<div>
														<div class="mb-1 font-medium text-muted-foreground">
															Turn breakdown
														</div>
														<div class="space-y-1">
															{#each visibleUsageRows(usageTurn.usage) as row (row.label)}
																<div class="flex justify-between gap-3">
																	<span>{row.label}</span>
																	<span class="tabular-nums text-foreground">
																		{formatFullTokenCount(row.value)}
																	</span>
																</div>
															{/each}
														</div>
													</div>
													<div>
														<div class="mb-1 font-medium text-muted-foreground">
															Turn limits and cost
														</div>
														<div class="space-y-1">
															<div class="flex justify-between gap-3">
																<span>Model max</span>
																<span class="tabular-nums text-foreground">
																	{formatFullTokenCount(
																		usageTurn.modelMaxTokens ?? 0,
																	)}
																</span>
															</div>
															<div class="flex justify-between gap-3">
																<span>Max output</span>
																<span class="tabular-nums text-foreground">
																	{formatFullTokenCount(
																		usageTurn.maxOutputTokens ?? 0,
																	)}
																</span>
															</div>
															<div class="flex justify-between gap-3">
																<span>Estimated cost</span>
																<span class="tabular-nums text-foreground">
																	{formatEstimatedCost(
																		estimateUsageCost(
																			usageTurn.usage,
																			usageTurn.prices,
																		),
																	) ?? "n/a"}
																</span>
															</div>
														</div>
													</div>
												</div>

												{#if (usageTurn.steps?.length ?? 0) > 0}
													<div
														class="mt-3 space-y-2 border-t border-border pt-3"
													>
														{#each usageTurn.steps ?? [] as usageStep (usageStep.index)}
															<div class="rounded-md bg-muted/40 p-2 text-xs">
																<div
																	class="mb-2 flex flex-wrap items-center justify-between gap-2"
																>
																	<div class="font-medium text-foreground">
																		Step {usageStep.index + 1}
																	</div>
																	<div
																		class="tabular-nums text-muted-foreground"
																	>
																		{formatFullTokenCount(
																			usageTotal(usageStep.usage),
																		)}
																		tokens
																	</div>
																</div>
																<div
																	class="grid grid-cols-2 gap-x-4 gap-y-1 sm:grid-cols-4"
																>
																	{#each visibleUsageRows(usageStep.usage) as row (row.label)}
																		<div class="flex justify-between gap-2">
																			<span class="text-muted-foreground">
																				{row.label}
																			</span>
																			<span
																				class="tabular-nums text-foreground"
																			>
																				{formatFullTokenCount(row.value)}
																			</span>
																		</div>
																	{/each}
																</div>
																{#if (usageStep.toolCalls?.length ?? 0) > 0}
																	<div class="mt-2 text-muted-foreground">
																		Tool calls:
																		{usageStep.toolCalls
																			?.map((call) => call.name || call.id)
																			.join(", ")}
																	</div>
																{/if}
															</div>
														{/each}
													</div>
												{/if}
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/if}
					</div>
				</Dialog.Content>
			</Dialog.Root>
		</div>
	</div>
</div>
