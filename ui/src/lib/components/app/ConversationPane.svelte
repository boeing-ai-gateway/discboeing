<script lang="ts">
	import AppWindowIcon from "@lucide/svelte/icons/app-window";
	import ArrowDownIcon from "@lucide/svelte/icons/arrow-down";
	import GalleryHorizontalEndIcon from "@lucide/svelte/icons/gallery-horizontal-end";
	import ListTreeIcon from "@lucide/svelte/icons/list-tree";
	import { tick } from "svelte";
	import { api } from "$lib/api-client";
	import { isSessionTransitioningStatus } from "$lib/api-constants";
	import type {
		BrowserEventChunkData,
		BrowserEventFile,
		ChatMessage,
	} from "$lib/api-types";
	import type { ChatWidthMode } from "$lib/app/app-context.types";
	import type {
		AssistantConversationPaneRenderablePart,
		HookFailureMessageMetadata,
		UserConversationPaneRenderablePart,
	} from "$lib/components/app/conversation-pane-message-parts";
	import {
		getAssistantMessagePartGroups,
		getHookFailureCollapsedSummary,
		getHookFailureMessageMetadata,
		getHookPathDisplayLabel,
		getUserMessageRenderableParts,
		isAssistantToolPartQueued,
	} from "$lib/components/app/conversation-pane-message-parts";
	import {
		Attachment,
		AttachmentInfo,
		AttachmentPreview,
		Attachments,
		Loader,
	} from "$lib/components/ai";
	import {
		Message,
		MessageContent,
		MessageResponse,
	} from "$lib/components/ai/message";
	import {
		Reasoning,
		ReasoningContent,
		ReasoningTrigger,
	} from "$lib/components/ai/reasoning";
	import OptimizedToolRenderer from "$lib/components/ai/tool-renderers/OptimizedToolRenderer.svelte";
	import type { DynamicToolPart } from "$lib/components/ai/types";
	import ConversationComposer from "$lib/components/app/ConversationComposer.svelte";
	import AssistantMessageCopyActions from "$lib/components/app/parts/AssistantMessageCopyActions.svelte";
	import BrowserScreenshotPreviewDialog from "$lib/components/app/parts/BrowserScreenshotPreviewDialog.svelte";
	import ConversationSelectionComment from "$lib/components/app/parts/ConversationSelectionComment.svelte";
	import HookPreviewDialog from "$lib/components/app/parts/HookPreviewDialog.svelte";
	import MessageResponseWithCommand from "$lib/components/app/parts/MessageResponseWithCommand.svelte";
	import {
		type ConversationTurn,
		getReservedTurnMinHeight,
		groupMessagesIntoTurns,
		isCompactionMessage,
	} from "$lib/components/app/conversation-pane-layout";
	import { Alert, AlertDescription } from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import {
		Collapsible,
		CollapsibleContent,
		CollapsibleTrigger,
	} from "$lib/components/ui/collapsible";
	import { getErrorMessage } from "$lib/error-message";
	import { useContext } from "$lib/context/context.svelte";
	import { openFile } from "$lib/context/commands/file";
	import {
		refreshThread,
		setConversationScrollTop,
		addThreadPendingComment,
		submitThread,
	} from "$lib/context/commands/thread";
	import type {
		SessionContextValue,
		ThreadContextValue,
		ConversationComment,
	} from "$lib/session/session-context.types";
	import {
		buildUserMessageParts,
		formatConversationComments,
		getTodoWriteEntries,
	} from "$lib/session/domains/session-domain.helpers";

	type ConversationPaneStatus = ThreadContextValue["status"] | "streaming";
	type ConversationPaneErrorBannerKey = "session" | "thread";
	type BrowserActivityViewMode = "simple" | "details";
	type BrowserTimelineStep = {
		index: number;
		file: BrowserEventFile;
		event: BrowserEventChunkData;
		artifactURI: string;
	};

	type Props = {
		session?: SessionContextValue;
		thread?: ThreadContextValue;
		contentTopPadding?: number;
		messages?: ChatMessage[];
		status?: ConversationPaneStatus;
		isStreaming?: boolean;
		threadError?: string | null;
		sessionError?: string | null;
		chatWidthMode?: ChatWidthMode;
		showComposer?: boolean;
		toolDefaultOpen?: boolean;
		visible?: boolean;
	};

	const SCROLL_TO_BOTTOM_BUFFER = 64;
	const BROWSER_SCREENSHOT_MAX_LOAD_ATTEMPTS = 4;
	const BROWSER_SCREENSHOT_RETRY_DELAY_MS = 200;

	let {
		session,
		thread,
		contentTopPadding = 0,
		messages,
		status,
		isStreaming: isStreamingOverride,
		threadError: threadErrorOverride = null,
		sessionError: sessionErrorOverride = null,
		chatWidthMode,
		showComposer = true,
		toolDefaultOpen = false,
		visible = true,
	}: Props = $props();

	const context = useContext();
	const activeSessionId = $derived.by(
		() => session?.sessionId ?? context.view.app.selection.sessionId ?? null,
	);
	const activeThreadId = $derived.by(() => thread?.threadId ?? null);
	const conversationMessages = $derived.by(
		() => messages ?? thread?.messages ?? [],
	);
	const visibleConversationMessages = $derived.by(() =>
		conversationMessages.filter(
			(message) => !message.synthetic || isCompactionMessage(message),
		),
	);
	const conversationStatus = $derived.by(() => {
		const nextStatus = status ?? thread?.status ?? "ready";
		return nextStatus === "streaming" ? "ready" : nextStatus;
	});
	const conversationTurns = $derived.by(() =>
		groupMessagesIntoTurns(visibleConversationMessages),
	);
	const browserEventsByTurnId = $derived.by(
		() => thread?.browserEventsByTurnId ?? {},
	);
	const previousTodoEntriesByToolCallId = $derived.by(() => {
		const entriesByToolCallId: Record<
			string,
			NonNullable<ReturnType<typeof getTodoWriteEntries>>
		> = {};
		let previousEntries: NonNullable<ReturnType<typeof getTodoWriteEntries>> =
			[];

		for (const message of visibleConversationMessages) {
			for (const part of message.parts) {
				if (part.type !== "dynamic-tool" || part.toolName !== "TodoWrite") {
					continue;
				}

				entriesByToolCallId[part.toolCallId] = previousEntries;

				if (part.state !== "output-available") {
					continue;
				}

				const entries = getTodoWriteEntries(part.input);
				if (entries) {
					previousEntries = entries;
				}
			}
		}

		return entriesByToolCallId;
	});
	const activeTurnId = $derived.by(
		() => conversationTurns.at(-1)?.renderId ?? null,
	);
	const effectiveChatWidthMode = $derived.by(
		() => chatWidthMode ?? context.view.app.preferences.chatWidthMode ?? "full",
	);
	const hasMessages = $derived.by(() => visibleConversationMessages.length > 0);
	const isLoading = $derived.by(() => conversationStatus === "loading");
	const isStreaming = $derived.by(
		() =>
			isStreamingOverride ??
			(status === "streaming" ? true : (thread?.isStreaming ?? false)),
	);
	const sessionError = $derived.by(() =>
		getErrorMessage(sessionErrorOverride ?? session?.current?.errorMessage),
	);
	const shouldShowSessionError = $derived.by(
		() => !isSessionTransitioningStatus(session?.current?.sandboxStatus),
	);
	const visibleSessionError = $derived.by(() =>
		shouldShowSessionError ? sessionError : null,
	);
	const threadError = $derived.by(() =>
		getErrorMessage(threadErrorOverride ?? thread?.error),
	);
	const canShowComposer = $derived.by(
		() => showComposer && Boolean(session) && Boolean(thread),
	);
	const latestConversationMessageId = $derived.by(
		() => visibleConversationMessages.at(-1)?.id ?? null,
	);
	const savedScrollTop = $derived.by(() => {
		if (!activeSessionId || !activeThreadId) {
			return null;
		}
		return (
			context.view.sessions[activeSessionId]?.threads[activeThreadId]
				?.conversation.scrollTop ?? null
		);
	});

	let viewport = $state<HTMLDivElement | null>(null);
	let contentEl = $state<HTMLDivElement | null>(null);
	let composerContainer = $state<HTMLDivElement | null>(null);
	let hasInitialBottomScroll = $state(false);
	let isNearBottom = $state(true);
	let expandedAssistantStepMessages = $state<Record<string, boolean>>({});
	let expandedGeneratedUserMessages = $state<Record<string, boolean>>({});
	let expandedHookFailureMessages = $state<Record<string, boolean>>({});
	let expandedBrowserActivityMessages = $state<Record<string, boolean>>({});
	let expandedCompactionMessages = $state<Record<string, boolean>>({});
	let expandedBrowserDetailEvents = $state<Record<string, boolean>>({});
	let browserActivityViewModes = $state<
		Record<string, BrowserActivityViewMode>
	>({});
	let lastReservedSubmitMessageId = $state<string | null>(null);
	let reservedTurnMinHeight = $state(0);
	let hookPreviewOpen = $state(false);
	let hookPreviewMetadata = $state<HookFailureMessageMetadata | null>(null);
	let hookPreviewContent = $state("");
	let hookPreviewLoading = $state(false);
	let hookPreviewError = $state<string | null>(null);
	let browserScreenshotPreviewOpen = $state(false);
	let browserScreenshotPreviewFile = $state<BrowserEventFile | null>(null);
	let browserScreenshotPreviewURL = $state<string | null>(null);
	let browserScreenshotPreviewLoading = $state(false);
	let browserScreenshotPreviewError = $state<string | null>(null);
	let browserScreenshotLoadErrors = $state<Record<string, string>>({});
	let browserScreenshotPreviewCache = $state<Record<string, string>>({});
	let browserScreenshotLoadPromises: Partial<Record<string, Promise<string>>> =
		{};
	let expandedErrorBanners = $state<
		Partial<Record<ConversationPaneErrorBannerKey, boolean>>
	>({});
	let lastRestoredVisibleThreadId = $state<string | null>(null);

	async function openHookPreview(metadata: HookFailureMessageMetadata) {
		hookPreviewMetadata = metadata;
		hookPreviewContent = "";
		hookPreviewError = null;
		hookPreviewOpen = true;

		if (!activeSessionId || !metadata.hookPath) {
			return;
		}

		hookPreviewLoading = true;
		try {
			const response = await api.readSessionFile(
				activeSessionId,
				metadata.hookPath,
			);
			hookPreviewContent = response.content;
		} catch (error) {
			hookPreviewError =
				error instanceof Error ? error.message : "Failed to load hook file.";
		} finally {
			hookPreviewLoading = false;
		}
	}

	async function editHookFile() {
		const hookPath = hookPreviewMetadata?.hookPath;
		if (!hookPath) {
			return;
		}
		if (activeSessionId) {
			await openFile(activeSessionId, hookPath);
		}
		hookPreviewOpen = false;
	}

	function isProvisionalUserMessage(
		message: ChatMessage | undefined,
	): message is ChatMessage & { role: "user"; provisional: true } {
		return message?.role === "user" && message.provisional === true;
	}

	function isAssistantStepMessageExpanded(messageId: string): boolean {
		return expandedAssistantStepMessages[messageId] ?? false;
	}

	function isGeneratedUserMessageExpanded(messageId: string): boolean {
		return expandedGeneratedUserMessages[messageId] ?? false;
	}

	function isHookFailureMessageExpanded(messageId: string): boolean {
		return expandedHookFailureMessages[messageId] ?? false;
	}

	function setGeneratedUserMessageExpanded(messageId: string, open: boolean) {
		expandedGeneratedUserMessages = {
			...expandedGeneratedUserMessages,
			[messageId]: open,
		};
	}

	function setHookFailureMessageExpanded(messageId: string, open: boolean) {
		expandedHookFailureMessages = {
			...expandedHookFailureMessages,
			[messageId]: open,
		};
	}

	function setAssistantStepMessageExpanded(messageId: string, open: boolean) {
		expandedAssistantStepMessages = {
			...expandedAssistantStepMessages,
			[messageId]: open,
		};
	}

	function isBrowserActivityExpanded(turnId: string): boolean {
		return expandedBrowserActivityMessages[turnId] ?? turnId === activeTurnId;
	}

	function setBrowserActivityExpanded(messageId: string, open: boolean) {
		expandedBrowserActivityMessages = {
			...expandedBrowserActivityMessages,
			[messageId]: open,
		};
	}

	function isBrowserDetailEventExpanded(eventId: string): boolean {
		return expandedBrowserDetailEvents[eventId] ?? false;
	}

	function setBrowserDetailEventExpanded(eventId: string, open: boolean) {
		expandedBrowserDetailEvents = {
			...expandedBrowserDetailEvents,
			[eventId]: open,
		};
	}

	function getBrowserActivityViewMode(turnId: string): BrowserActivityViewMode {
		return browserActivityViewModes[turnId] ?? "simple";
	}

	function setBrowserActivityViewMode(
		turnId: string,
		mode: BrowserActivityViewMode,
	) {
		browserActivityViewModes = {
			...browserActivityViewModes,
			[turnId]: mode,
		};
	}

	function getBrowserEventMethodLabel(event: BrowserEventChunkData): string {
		return event.event.method?.trim() || "Unknown browser event";
	}

	function isBrowserEventDetailExpandable(
		event: BrowserEventChunkData,
	): boolean {
		const collapsed = getBrowserEventDetails(event);
		const expanded = getBrowserEventDetails(event, { expanded: true });
		return Boolean(collapsed && expanded && collapsed !== expanded);
	}

	function getBrowserEventDetailToggleLabel(eventId: string): string {
		return isBrowserDetailEventExpanded(eventId) ? "Show less" : "Show full";
	}

	function getBrowserEventTimestampLabel(
		event: BrowserEventChunkData,
	): string | null {
		if (!event.event.recordedAt) {
			return null;
		}
		const recordedAt = new Date(event.event.recordedAt);
		return Number.isNaN(recordedAt.getTime())
			? null
			: recordedAt.toLocaleTimeString([], {
					hour: "numeric",
					minute: "2-digit",
					second: "2-digit",
				});
	}

	function getBrowserActivityStepCount(
		events: BrowserEventChunkData[],
	): number {
		const screenshotKeys: string[] = [];

		for (const event of events) {
			for (const file of event.event.files ?? []) {
				const key =
					file.uri?.trim() || file.path?.trim() || file.filename?.trim();
				if (key && !screenshotKeys.includes(key)) {
					screenshotKeys.push(key);
				}
			}
		}

		return screenshotKeys.length;
	}

	function getBrowserEventDetails(
		event: BrowserEventChunkData,
		options: { expanded?: boolean } = {},
	): string | null {
		const payload =
			event.event.payload && typeof event.event.payload === "object"
				? (event.event.payload as Record<string, unknown>)
				: null;
		if (!payload) {
			return null;
		}
		const data =
			event.event.direction === "response"
				? payload.result && typeof payload.result === "object"
					? (payload.result as Record<string, unknown>)
					: null
				: payload.params && typeof payload.params === "object"
					? (payload.params as Record<string, unknown>)
					: null;
		if (!data) {
			return null;
		}

		const maxLength = options.expanded ? null : undefined;

		switch (event.event.method) {
			case "Page.navigate":
				return getBrowserDetailText(data.url, maxLength);
			case "Target.createTarget":
				return getBrowserDetailText(
					event.event.direction === "response" ? data.targetId : data.url,
					maxLength,
				);
			case "Target.activateTarget":
			case "Target.attachToTarget":
			case "Target.closeTarget":
				return getBrowserDetailText(
					event.event.direction === "response"
						? (data.sessionId ?? data.success)
						: data.targetId,
					maxLength,
				);
			case "Runtime.evaluate":
				return getBrowserRuntimeEvaluateDetails(
					event.event.direction,
					data,
					maxLength,
				);
			case "Input.dispatchMouseEvent":
				return getBrowserInputDetails(
					data,
					["type", "x", "y", "button", "clickCount"],
					maxLength,
				);
			case "Input.dispatchKeyEvent":
				return getBrowserInputDetails(
					data,
					["type", "key", "code", "text"],
					maxLength,
				);
			default:
				return getBrowserInputDetails(
					data,
					event.event.direction === "response"
						? ["targetId", "sessionId", "frameId", "loaderId", "success", "url"]
						: [
								"url",
								"targetId",
								"sessionId",
								"type",
								"key",
								"button",
								"x",
								"y",
							],
					maxLength,
				);
		}
	}

	function getBrowserRuntimeEvaluateDetails(
		direction: string,
		data: Record<string, unknown>,
		maxLength?: number | null,
	): string | null {
		if (direction === "request") {
			return getBrowserDetailText(data.expression, maxLength ?? 120);
		}
		const result =
			data.result && typeof data.result === "object"
				? (data.result as Record<string, unknown>)
				: null;
		if (!result) {
			return null;
		}
		return getBrowserDetailText(
			result.value ?? result.description ?? result.type,
			maxLength ?? 120,
		);
	}

	function getBrowserInputDetails(
		data: Record<string, unknown>,
		keys: string[],
		maxLength?: number | null,
	): string | null {
		const parts = keys
			.map((key) => {
				const value = getBrowserDetailText(data[key], maxLength ?? 60);
				return value ? `${key}: ${value}` : null;
			})
			.filter((value): value is string => Boolean(value));
		return parts.length > 0 ? parts.join(" • ") : null;
	}

	function getBrowserDetailText(
		value: unknown,
		maxLength: number | null = 80,
	): string | null {
		if (value === null || value === undefined) {
			return null;
		}
		const text =
			typeof value === "string"
				? value
				: typeof value === "number" || typeof value === "boolean"
					? String(value)
					: null;
		if (!text) {
			return null;
		}
		const trimmed = text.trim();
		if (!trimmed) {
			return null;
		}
		if (maxLength === null) {
			return trimmed;
		}
		return trimmed.length > maxLength
			? `${trimmed.slice(0, maxLength - 1)}…`
			: trimmed;
	}

	function getBrowserArtifactURI(file: BrowserEventFile): string {
		return file.uri ?? `artifacts://${file.path}`;
	}

	function getBrowserScreenshotURL(file: BrowserEventFile): string | null {
		return browserScreenshotPreviewCache[getBrowserArtifactURI(file)] ?? null;
	}

	async function loadBrowserScreenshot(
		file: BrowserEventFile,
	): Promise<string> {
		const artifactURI = getBrowserArtifactURI(file);
		const cachedURL = browserScreenshotPreviewCache[artifactURI];
		if (cachedURL) {
			return cachedURL;
		}
		if (browserScreenshotLoadPromises[artifactURI]) {
			return browserScreenshotLoadPromises[artifactURI];
		}

		const loadPromise = loadBrowserScreenshotWithRetry(file);
		browserScreenshotLoadPromises = {
			...browserScreenshotLoadPromises,
			[artifactURI]: loadPromise,
		};

		try {
			return await loadPromise;
		} finally {
			const rest = { ...browserScreenshotLoadPromises };
			delete rest[artifactURI];
			browserScreenshotLoadPromises = rest;
		}
	}

	async function loadBrowserScreenshotWithRetry(
		file: BrowserEventFile,
	): Promise<string> {
		const artifactURI = getBrowserArtifactURI(file);
		for (
			let attempt = 1;
			attempt <= BROWSER_SCREENSHOT_MAX_LOAD_ATTEMPTS;
			attempt++
		) {
			const cachedURL = browserScreenshotPreviewCache[artifactURI];
			if (cachedURL) {
				return cachedURL;
			}
			if (!activeSessionId || !activeThreadId) {
				throw new Error("No active thread.");
			}

			try {
				const response = await api.readSessionThreadArtifact(
					activeSessionId,
					activeThreadId,
					artifactURI,
				);
				const base64Content =
					response.encoding === "base64"
						? response.content
						: btoa(response.content);
				const nextURL = `data:${file.mediaType || "application/octet-stream"};base64,${base64Content}`;
				browserScreenshotPreviewCache = {
					...browserScreenshotPreviewCache,
					[artifactURI]: nextURL,
				};
				if (browserScreenshotLoadErrors[artifactURI]) {
					const rest = { ...browserScreenshotLoadErrors };
					delete rest[artifactURI];
					browserScreenshotLoadErrors = rest;
				}
				return nextURL;
			} catch (error) {
				if (attempt === BROWSER_SCREENSHOT_MAX_LOAD_ATTEMPTS) {
					throw error;
				}
				await new Promise((resolve) =>
					setTimeout(resolve, BROWSER_SCREENSHOT_RETRY_DELAY_MS),
				);
			}
		}

		throw new Error("Failed to load browser screenshot.");
	}

	async function ensureBrowserScreenshotLoaded(file: BrowserEventFile) {
		const artifactURI = getBrowserArtifactURI(file);
		if (browserScreenshotPreviewCache[artifactURI]) {
			return;
		}

		try {
			await loadBrowserScreenshot(file);
		} catch (error) {
			browserScreenshotLoadErrors = {
				...browserScreenshotLoadErrors,
				[artifactURI]:
					error instanceof Error
						? error.message
						: "Failed to load browser screenshot.",
			};
		}
	}

	async function preloadBrowserTimelineSteps(steps: BrowserTimelineStep[]) {
		for (const step of steps) {
			await ensureBrowserScreenshotLoaded(step.file);
		}
	}

	function getBrowserScreenshotLoadError(
		file: BrowserEventFile,
	): string | null {
		return browserScreenshotLoadErrors[getBrowserArtifactURI(file)] ?? null;
	}

	function getBrowserTimelineSteps(
		events: BrowserEventChunkData[],
	): BrowserTimelineStep[] {
		const steps: BrowserTimelineStep[] = [];
		const seenArtifactURIs: string[] = [];

		for (const event of events) {
			for (const file of event.event.files ?? []) {
				const artifactURI = getBrowserArtifactURI(file);
				if (seenArtifactURIs.includes(artifactURI)) {
					continue;
				}
				steps.push({
					index: steps.length + 1,
					file,
					event,
					artifactURI,
				});
				seenArtifactURIs.push(artifactURI);
			}
		}

		return steps;
	}

	async function openBrowserScreenshotPreview(file: BrowserEventFile) {
		browserScreenshotPreviewFile = file;
		browserScreenshotPreviewError = null;
		browserScreenshotPreviewOpen = true;
		browserScreenshotPreviewURL = null;
		browserScreenshotPreviewLoading = true;

		try {
			browserScreenshotPreviewURL = await loadBrowserScreenshot(file);
		} catch (error) {
			browserScreenshotPreviewError =
				error instanceof Error
					? error.message
					: "Failed to load browser screenshot.";
		} finally {
			browserScreenshotPreviewLoading = false;
		}
	}

	function getCollapsedStepLabel(stepCount: number): string {
		return `${stepCount} ${stepCount === 1 ? "step" : "steps"}`;
	}

	function isReplacedAssistantMessage(message: ChatMessage): boolean {
		return message.role === "assistant" && Boolean(message.replacedByMessageId);
	}

	function getCollapsedAssistantHeaderLabel(
		turn: ConversationTurn,
		partGroups: ReturnType<typeof getAssistantMessagePartGroups> | null,
	): string {
		const groupedAssistantMessages = getTurnGroupedAssistantMessages(turn);
		if (
			groupedAssistantMessages.length === 1 &&
			partGroups?.collapsedParts.length === 0 &&
			isReplacedAssistantMessage(groupedAssistantMessages[0])
		) {
			return "Failed message";
		}

		return getCollapsedStepLabel(getTurnGroupedStepCount(turn, partGroups));
	}

	function getBrowserActivityStepLabel(stepCount: number): string {
		return `${stepCount} browser ${stepCount === 1 ? "step" : "steps"}`;
	}

	function getTurnFinalAssistantMessage(turn: ConversationTurn) {
		return turn.assistantMessages.at(-1) ?? null;
	}

	function isCompactionTurn(turn: ConversationTurn): boolean {
		return (
			turn.userMessages.length > 0 &&
			turn.assistantMessages.length > 0 &&
			turn.userMessages.every(isCompactionMessage) &&
			turn.assistantMessages.every(isCompactionMessage)
		);
	}

	function getMessageText(message: ChatMessage | null | undefined): string {
		if (!message) {
			return "";
		}
		return message.parts
			.filter((part) => part.type === "text")
			.map((part) => part.text)
			.join("");
	}

	function isCompactionExpanded(turnId: string): boolean {
		return expandedCompactionMessages[turnId] ?? false;
	}

	function setCompactionExpanded(turnId: string, expanded: boolean): void {
		expandedCompactionMessages[turnId] = expanded;
	}

	function getTurnGroupedAssistantMessages(
		turn: ConversationTurn,
	): ConversationTurn["assistantMessages"] {
		return turn.assistantMessages.slice(0, -1);
	}

	function getAssistantMessageAllRenderableParts(message: ChatMessage) {
		const partGroups = getAssistantMessagePartGroups(message, {
			isMessageComplete: !isActiveStreamingAssistantMessage(message),
		});
		return [...partGroups.collapsedParts, ...partGroups.visibleParts];
	}

	function getAssistantMessagesAllRenderableParts(messages: ChatMessage[]) {
		return messages.flatMap((message) =>
			getAssistantMessageAllRenderableParts(message),
		);
	}

	function extractURLsFromText(text: string) {
		const urls: string[] = [];
		for (const match of text.matchAll(/https?:\/\/[^\s<>)\]]+/g)) {
			const url = match[0]?.replace(/[.,;:!?]+$/, "");
			if (url && !urls.includes(url)) {
				urls.push(url);
			}
		}
		return urls.map((url) => ({ title: url, url }));
	}

	function getWebSearchFallbackResults(
		parts: AssistantConversationPaneRenderablePart[],
		index: number,
	) {
		const results: { title: string; url: string }[] = [];
		for (const part of parts.slice(index + 1)) {
			if (part.type === "dynamic-tool" && part.toolName === "WebSearch") {
				break;
			}
			if (part.type === "text") {
				results.push(...extractURLsFromText(part.text));
			}
		}
		return results.filter(
			(result, resultIndex) =>
				results.findIndex((current) => current.url === result.url) ===
				resultIndex,
		);
	}

	function getRenderableToolPart(
		parts: AssistantConversationPaneRenderablePart[],
		toolPart: DynamicToolPart,
	): DynamicToolPart {
		const index = parts.findIndex(
			(part) =>
				part.type === "dynamic-tool" && part.toolCallId === toolPart.toolCallId,
		);
		const part = toolPart;
		if (part.toolName !== "WebSearch") {
			return part;
		}
		if (index < 0) {
			return part;
		}

		const output =
			part.output &&
			typeof part.output === "object" &&
			!Array.isArray(part.output)
				? (part.output as Record<string, unknown>)
				: {};
		if (Array.isArray(output.results) && output.results.length > 0) {
			return part;
		}

		const results = getWebSearchFallbackResults(parts, index);
		if (results.length === 0) {
			return part;
		}

		return {
			...part,
			output: {
				...output,
				results,
			},
		};
	}

	function getTurnGroupedStepCount(
		turn: ConversationTurn,
		partGroups: ReturnType<typeof getAssistantMessagePartGroups> | null,
	): number {
		return (
			turn.assistantMessages.length - 1 + (partGroups?.collapsedStepCount ?? 0)
		);
	}

	function shouldCollapseErrorBanner(errorText: string): boolean {
		return errorText.split(/\r?\n/).length > 3 || errorText.length > 240;
	}

	function isDroppedModelConnectionError(errorText: string): boolean {
		const normalized = errorText.toLowerCase();
		return normalized.includes("websocket read") && normalized.includes("eof");
	}

	function getErrorBannerDisplayText(
		key: ConversationPaneErrorBannerKey,
		errorText: string,
	): string {
		if (key === "thread" && isDroppedModelConnectionError(errorText)) {
			return "The model connection dropped unexpectedly. Your work is saved. Retry to reconnect and continue.";
		}
		return errorText;
	}

	function getErrorBannerDetailsText(
		key: ConversationPaneErrorBannerKey,
		errorText: string,
	): string | null {
		if (key === "thread" && isDroppedModelConnectionError(errorText)) {
			return errorText;
		}
		return null;
	}

	function isErrorBannerExpanded(key: ConversationPaneErrorBannerKey): boolean {
		return expandedErrorBanners[key] ?? false;
	}

	function setErrorBannerExpanded(
		key: ConversationPaneErrorBannerKey,
		expanded: boolean,
	) {
		expandedErrorBanners = {
			...expandedErrorBanners,
			[key]: expanded,
		};
	}

	async function submitSelectionComment(
		comment: Omit<ConversationComment, "id">,
	) {
		if (!thread) {
			return;
		}
		const text = formatConversationComments([comment]);
		if (!activeSessionId) {
			return;
		}
		await submitThread(activeSessionId, thread.threadId, {
			parts: buildUserMessageParts(text),
		});
	}

	function getErrorBannerToggleLabel(
		key: ConversationPaneErrorBannerKey,
		hasDetails = false,
	): string {
		if (hasDetails) {
			return isErrorBannerExpanded(key) ? "Hide details" : "Show details";
		}
		return isErrorBannerExpanded(key) ? "Show less" : "Show full error";
	}

	function getErrorBannerAction(
		key: ConversationPaneErrorBannerKey,
	): { label: string; run: () => void } | null {
		if (key !== "thread" || !thread) {
			return null;
		}

		return {
			label: "Retry",
			run: () => {
				if (activeSessionId) {
					void refreshThread(activeSessionId, thread.threadId);
				}
			},
		};
	}

	function isActiveStreamingAssistantMessage(message: ChatMessage): boolean {
		return (
			isStreaming &&
			message.role === "assistant" &&
			message.id === latestConversationMessageId
		);
	}

	function updateIsNearBottom() {
		const element = viewport;
		if (!element) {
			isNearBottom = true;
			return;
		}

		const distanceToBottom =
			element.scrollHeight - element.scrollTop - element.clientHeight;
		isNearBottom = distanceToBottom <= SCROLL_TO_BOTTOM_BUFFER;
	}

	function saveScrollPosition(element: HTMLDivElement | null = viewport) {
		if (!element || !activeThreadId || !activeSessionId) {
			return;
		}
		setConversationScrollTop(
			activeSessionId,
			activeThreadId,
			element.scrollTop,
		);
	}

	function scrollToBottom(behavior: globalThis.ScrollBehavior = "auto") {
		const element = viewport;
		if (!element) {
			return;
		}

		element.scrollTo({ top: element.scrollHeight, behavior });
		if (behavior === "auto") {
			requestAnimationFrame(() => {
				saveScrollPosition(element);
				updateIsNearBottom();
			});
		}
	}

	function getTurnElement(turnId: string) {
		if (!viewport) {
			return null;
		}

		return viewport.querySelector<HTMLElement>(
			`[data-conversation-turn-id="${CSS.escape(turnId)}"]`,
		);
	}

	function getMessageElement(messageId: string) {
		if (!viewport) {
			return null;
		}

		return viewport.querySelector<HTMLElement>(
			`[data-conversation-message-id="${CSS.escape(messageId)}"]`,
		);
	}

	function scrollElementToViewportTop(target: HTMLElement) {
		const element = viewport;
		if (!element) {
			return;
		}

		const styles = window.getComputedStyle(element);
		const paddingTop = Number.parseFloat(styles.paddingTop) || 0;
		const viewportTop = element.getBoundingClientRect().top;
		const targetTop = target.getBoundingClientRect().top;
		const nextScrollTop = Math.min(
			Math.max(0, element.scrollHeight - element.clientHeight),
			Math.max(0, element.scrollTop + targetTop - viewportTop - paddingTop),
		);

		element.scrollTo({ top: nextScrollTop, behavior: "auto" });
		requestAnimationFrame(() => {
			saveScrollPosition(element);
			updateIsNearBottom();
		});
	}

	function scrollMessageToViewportTop(messageId: string) {
		const messageElement = getMessageElement(messageId);
		if (!messageElement) {
			return;
		}

		scrollElementToViewportTop(messageElement);
	}

	function captureReservedTurnHeight(turnId: string) {
		const element = viewport;
		const turnElement = getTurnElement(turnId);
		if (!element || !turnElement) {
			return 0;
		}

		const styles = window.getComputedStyle(element);
		const turnStyles = window.getComputedStyle(turnElement);
		const paddingTop = Number.parseFloat(styles.paddingTop) || 0;
		const paddingBottom = Number.parseFloat(styles.paddingBottom) || 0;
		const turnTopPadding = Number.parseFloat(turnStyles.paddingTop) || 0;

		return getReservedTurnMinHeight({
			currentTurnHeight: turnElement.getBoundingClientRect().height,
			contentTopPadding,
			turnTopPadding,
			viewportClientHeight: element.clientHeight,
			viewportPaddingBottom: paddingBottom,
			viewportPaddingTop: paddingTop,
		});
	}

	function refreshReservedTurnHeight(turnId: string) {
		reservedTurnMinHeight = captureReservedTurnHeight(turnId);
	}

	function getTurnStyle(isLastTurn: boolean) {
		if (!isLastTurn || reservedTurnMinHeight <= 0) {
			return undefined;
		}

		return `min-height: ${reservedTurnMinHeight}px;`;
	}

	$effect(() => {
		const element = viewport;
		if (!element) {
			isNearBottom = true;
			return;
		}

		const handleScroll = () => {
			saveScrollPosition(element);
			updateIsNearBottom();
		};

		updateIsNearBottom();
		element.addEventListener("scroll", handleScroll);

		return () => {
			element.removeEventListener("scroll", handleScroll);
		};
	});

	$effect(() => {
		if (!visible) {
			lastRestoredVisibleThreadId = null;
		}
	});

	$effect(() => {
		const element = viewport;
		const threadId = activeThreadId;
		if (!visible || !element || !threadId) {
			return;
		}
		if (savedScrollTop === null) {
			return;
		}
		if (lastRestoredVisibleThreadId === threadId) {
			return;
		}

		lastRestoredVisibleThreadId = threadId;
		void tick().then(() => {
			if (!visible || viewport !== element || activeThreadId !== threadId) {
				return;
			}
			element.scrollTop = Math.min(
				savedScrollTop,
				Math.max(0, element.scrollHeight - element.clientHeight),
			);
			saveScrollPosition(element);
			updateIsNearBottom();
		});
	});

	$effect(() => {
		for (const turn of conversationTurns) {
			if (!isBrowserActivityExpanded(turn.renderId)) {
				continue;
			}
			const browserEvents = browserEventsByTurnId[turn.id] ?? [];
			const browserTimelineSteps = getBrowserTimelineSteps(browserEvents);
			if (browserTimelineSteps.length === 0) {
				continue;
			}
			void preloadBrowserTimelineSteps(browserTimelineSteps);
		}
	});

	$effect(() => {
		if (conversationMessages.length > 0) {
			return;
		}

		hasInitialBottomScroll = false;
		expandedGeneratedUserMessages = {};
		lastReservedSubmitMessageId = null;
		reservedTurnMinHeight = 0;
		updateIsNearBottom();
	});

	$effect(() => {
		if (hasInitialBottomScroll) {
			return;
		}
		if (!viewport || conversationMessages.length === 0) {
			return;
		}
		if (conversationStatus !== "ready") {
			return;
		}
		if (savedScrollTop !== null) {
			hasInitialBottomScroll = true;
			return;
		}

		hasInitialBottomScroll = true;
		void tick().then(() => {
			if (conversationMessages.length > 0) {
				scrollToBottom("auto");
			}
		});
	});

	$effect(() => {
		const element = viewport;
		const composerElement = composerContainer;
		const turnId = activeTurnId;
		const latestMessage = visibleConversationMessages.at(-1);
		if (!element || !turnId || !isProvisionalUserMessage(latestMessage)) {
			return;
		}

		const messageId = latestMessage.id;
		const observer = new ResizeObserver(() => {
			refreshReservedTurnHeight(turnId);
			void tick().then(() => {
				if (activeTurnId === turnId) {
					scrollMessageToViewportTop(messageId);
				}
			});
		});

		observer.observe(element);
		if (composerElement) {
			observer.observe(composerElement);
		}

		void tick().then(() => {
			if (activeTurnId !== turnId) {
				return;
			}
			refreshReservedTurnHeight(turnId);
			void tick().then(() => {
				scrollMessageToViewportTop(messageId);
			});
		});

		return () => {
			observer.disconnect();
		};
	});

	$effect(() => {
		const latestMessage = visibleConversationMessages.at(-1);
		const turnId = activeTurnId;
		if (!viewport || !turnId || !isProvisionalUserMessage(latestMessage)) {
			return;
		}
		if (latestMessage.id === lastReservedSubmitMessageId) {
			return;
		}

		lastReservedSubmitMessageId = latestMessage.id;
		void tick()
			.then(() => {
				refreshReservedTurnHeight(turnId);
				return tick();
			})
			.then(() => {
				scrollMessageToViewportTop(latestMessage.id);
			});
	});

	// Auto-scroll to bottom during streaming when the user is near the bottom.
	// Uses a ResizeObserver on the content div so it fires as streamed text
	// grows the layout, not just when the messages array reference changes.
	// Scrolling up pauses auto-scroll; scrolling back to the bottom resumes it.
	// Disabled when the autoScrollOnStream preference is off.
	$effect(() => {
		const element = contentEl;
		const autoScroll = context.view.app.preferences.autoScrollOnStream;

		if (!element || !autoScroll) {
			return;
		}

		const observer = new ResizeObserver(() => {
			if (isStreaming && isNearBottom) {
				scrollToBottom("auto");
			}
		});

		observer.observe(element);

		return () => {
			observer.disconnect();
		};
	});
</script>

{#snippet renderUserMessageParts(
	message: ChatMessage,
	parts: UserConversationPaneRenderablePart[],
)}
	{@const fileParts = parts.filter((part) => part.type === "file")}
	{@const isGeneratedTextExpanded = isGeneratedUserMessageExpanded(message.id)}
	<MessageResponseWithCommand
		{message}
		{parts}
		{isGeneratedTextExpanded}
		onGeneratedTextExpandedChange={(open) =>
			setGeneratedUserMessageExpanded(message.id, open)}
	/>
	{#if fileParts.length > 0}
		<Attachments variant="inline" class="max-w-full">
			{#each fileParts as part, index (`${message.id}-file-${index}`)}
				<Attachment
					data={{
						id: `${message.id}-file-${index}`,
						type: "file",
						filename: part.filename,
						mediaType: part.mediaType,
						url: part.url,
					}}
				>
					<AttachmentPreview />
					<AttachmentInfo />
				</Attachment>
			{/each}
		</Attachments>
	{/if}
{/snippet}

{#snippet renderHookFailureMessage(
	messageId: string,
	metadata: HookFailureMessageMetadata,
)}
	{@const isExpanded = isHookFailureMessageExpanded(messageId)}
	{@const collapsedSummary = getHookFailureCollapsedSummary(metadata)}
	<Collapsible
		open={isExpanded}
		onOpenChange={(open) => setHookFailureMessageExpanded(messageId, open)}
	>
		<div
			class="w-full overflow-hidden rounded-xl border border-border bg-card shadow-sm"
		>
			<div
				class="flex items-center justify-between gap-3 bg-muted/20 px-4 py-3"
			>
				<div class="min-w-0 space-y-1">
					<div class="truncate font-medium text-foreground text-sm">
						Hook Failed: <span class="text-muted-foreground"
							>{metadata.hookName}</span
						>
					</div>
					{#if collapsedSummary}
						<div
							class="line-clamp-2 whitespace-pre-wrap break-words text-muted-foreground text-xs [overflow-wrap:anywhere]"
						>
							{collapsedSummary}
						</div>
					{/if}
				</div>
				<div class="flex shrink-0 items-center gap-3">
					<div
						class="rounded-md border border-border bg-background px-2 py-1 font-mono text-foreground text-xs"
					>
						exit {metadata.exitCode}
					</div>
					<CollapsibleTrigger
						aria-expanded={isExpanded}
						class="inline-flex h-8 items-center justify-center rounded-md px-3 text-sm font-medium transition-all hover:bg-accent hover:text-accent-foreground"
						type="button"
					>
						{isExpanded ? "Hide details" : "Show details"}
					</CollapsibleTrigger>
				</div>
			</div>

			<CollapsibleContent class="overflow-hidden border-border border-t">
				{#if isExpanded}
					<div class="space-y-4 p-4">
						<div class="grid gap-3 text-sm sm:grid-cols-2">
							{#if metadata.pattern}
								<div class="space-y-1">
									<div
										class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
									>
										Pattern
									</div>
									<div class="font-mono text-foreground text-xs">
										{metadata.pattern}
									</div>
								</div>
							{/if}

							{#if metadata.hookPath}
								<div class="space-y-1 sm:col-span-2">
									<div
										class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
									>
										Hook file
									</div>
									<Button
										class="h-auto justify-start px-0 font-mono text-xs"
										onclick={() => {
											void openHookPreview(metadata);
										}}
										size="sm"
										variant="link"
									>
										{getHookPathDisplayLabel(metadata.hookPath)}
									</Button>
								</div>
							{/if}

							{#if metadata.files && metadata.files.length > 0}
								<div class="space-y-1 sm:col-span-2">
									<div
										class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
									>
										Files
									</div>
									<div class="space-y-1 font-mono text-foreground text-xs">
										{#each metadata.files as file, __key0 (__key0)}
											<div class="break-all">{file}</div>
										{/each}
										{#if metadata.extraFileCount}
											<div class="text-muted-foreground">
												and {metadata.extraFileCount} more
											</div>
										{/if}
									</div>
								</div>
							{/if}
						</div>

						{#if metadata.output}
							<div class="space-y-2">
								<div
									class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
								>
									Output
								</div>
								<div class="rounded-md border border-border bg-background">
									<pre
										class="whitespace-pre-wrap break-words p-3 font-mono text-foreground text-xs leading-5 [overflow-wrap:anywhere]"><code
											>{metadata.output}</code
										></pre>
								</div>
							</div>
						{:else if metadata.outputPath || metadata.outputTail}
							<div class="space-y-2">
								{#if metadata.outputPath}
									<div class="space-y-2">
										<div
											class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
										>
											Log file
										</div>
										<div
											class="rounded-md border border-border bg-background px-3 py-2 font-mono text-foreground text-xs break-all"
										>
											{metadata.outputPath}
										</div>
									</div>
								{/if}
								{#if metadata.outputTail}
									<div class="space-y-2">
										<div
											class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
										>
											Last 15 lines
										</div>
										<div class="rounded-md border border-border bg-background">
											<pre
												class="whitespace-pre-wrap break-words p-3 font-mono text-foreground text-xs leading-5 [overflow-wrap:anywhere]"><code
													>{metadata.outputTail}</code
												></pre>
										</div>
									</div>
								{/if}
							</div>
						{/if}
					</div>
				{/if}
			</CollapsibleContent>
		</div>
	</Collapsible>
{/snippet}

{#snippet renderAssistantMessageParts(
	message: ChatMessage,
	parts: AssistantConversationPaneRenderablePart[],
	fallbackParts = parts,
)}
	{#each parts as part, index (`${message.id}-${part.type}-${index}`)}
		{#if part.type === "reasoning"}
			<Reasoning defaultOpen={false} isStreaming={part.state === "streaming"}>
				<ReasoningTrigger />
				<ReasoningContent text={part.text} />
			</Reasoning>
		{:else if part.type === "text"}
			<MessageResponse
				text={part.text}
				isAnimating={isActiveStreamingAssistantMessage(message)}
			/>
		{:else if part.type === "dynamic-tool"}
			<OptimizedToolRenderer
				toolPart={getRenderableToolPart(fallbackParts, part as DynamicToolPart)}
				queued={isAssistantToolPartQueued(parts, index)}
				sessionId={activeSessionId}
				threadId={activeThreadId}
				resolvedTheme={context.view.app.preferences.resolvedTheme}
				previousTodoEntries={part.toolName === "TodoWrite"
					? (previousTodoEntriesByToolCallId[part.toolCallId] ?? [])
					: undefined}
				onToolApprovalResponse={thread?.addToolApprovalResponse}
				defaultOpen={toolDefaultOpen}
			/>
		{/if}
	{/each}
{/snippet}

{#snippet renderBrowserActivity(
	turnId: string,
	events: BrowserEventChunkData[],
)}
	{@const browserActivityExpanded = isBrowserActivityExpanded(turnId)}
	{@const browserActivityViewMode = getBrowserActivityViewMode(turnId)}
	{@const browserStepCount = getBrowserActivityStepCount(events)}
	{@const browserTimelineSteps = getBrowserTimelineSteps(events)}
	<Collapsible
		open={browserActivityExpanded}
		onOpenChange={(open) => {
			setBrowserActivityExpanded(turnId, open);
			if (open) {
				void preloadBrowserTimelineSteps(browserTimelineSteps);
			}
		}}
	>
		<CollapsibleTrigger
			aria-label={`${browserActivityExpanded ? "Hide" : "Show"} browser activity`}
			class="flex w-full items-center gap-3 py-1 text-left"
			type="button"
		>
			<span class="h-px flex-1 bg-border"></span>
			<span
				class="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-background px-3 py-1 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.14em] transition-colors hover:border-border hover:text-foreground"
			>
				<AppWindowIcon class="size-3" />
				{getBrowserActivityStepLabel(browserStepCount)}
			</span>
			<span class="h-px flex-1 bg-border"></span>
		</CollapsibleTrigger>
		<CollapsibleContent class="overflow-hidden pt-3">
			{#if browserActivityExpanded}
				<div class="space-y-4">
					<div class="flex justify-end">
						<div
							class="inline-flex items-center overflow-hidden rounded-lg border border-border/70 bg-background/80 p-0.5 shadow-xs"
							aria-label="Browser activity view"
							role="group"
						>
							<Button
								aria-label="Show simple browser activity"
								aria-pressed={browserActivityViewMode === "simple"}
								class="size-6 rounded-md text-muted-foreground hover:text-foreground aria-pressed:text-foreground"
								onclick={() => setBrowserActivityViewMode(turnId, "simple")}
								size="icon-xs"
								title="Simple"
								type="button"
								variant={browserActivityViewMode === "simple"
									? "secondary"
									: "ghost"}
							>
								<GalleryHorizontalEndIcon class="size-3.5" />
							</Button>
							<span class="mx-0.5 h-4 w-px bg-border/70"></span>
							<Button
								aria-label="Show detailed browser activity"
								aria-pressed={browserActivityViewMode === "details"}
								class="size-6 rounded-md text-muted-foreground hover:text-foreground aria-pressed:text-foreground"
								onclick={() => setBrowserActivityViewMode(turnId, "details")}
								size="icon-xs"
								title="Details"
								type="button"
								variant={browserActivityViewMode === "details"
									? "secondary"
									: "ghost"}
							>
								<ListTreeIcon class="size-3.5" />
							</Button>
						</div>
					</div>

					{#if browserActivityViewMode === "simple"}
						<div class="space-y-4 pl-3">
							{#if browserTimelineSteps.length === 0}
								<div
									class="rounded-xl border border-dashed border-border/70 bg-muted/20 px-4 py-6 text-center text-muted-foreground text-sm"
								>
									No screenshots captured.
								</div>
							{:else}
								{#each browserTimelineSteps as step, index (step.artifactURI)}
									<div class="relative flex gap-4">
										<div class="flex w-8 shrink-0 flex-col items-center">
											<div
												class="z-10 flex size-8 items-center justify-center rounded-full border border-border bg-background font-medium text-foreground text-xs shadow-sm"
											>
												{step.index}
											</div>
											{#if index < browserTimelineSteps.length - 1}
												<div class="mt-2 w-px flex-1 bg-border"></div>
											{/if}
										</div>
										<div class="min-w-0 flex-1 pb-4">
											<button
												class="group block w-full overflow-hidden rounded-2xl border border-border/70 bg-card text-left shadow-sm transition-colors hover:border-border"
												onclick={() => {
													void openBrowserScreenshotPreview(step.file);
												}}
												type="button"
											>
												<div
													class="flex items-center gap-2 border-border/70 border-b bg-muted/35 px-3 py-2"
												>
													<span class="size-2 rounded-full bg-rose-400/80"
													></span>
													<span class="size-2 rounded-full bg-amber-400/80"
													></span>
													<span class="size-2 rounded-full bg-emerald-400/80"
													></span>
													<div
														class="ml-2 truncate rounded-md bg-background/80 px-2.5 py-1 text-[11px] text-muted-foreground"
													>
														{getBrowserEventMethodLabel(step.event)}
													</div>
												</div>
												<div
													class="bg-gradient-to-b from-muted/15 to-background p-3"
												>
													{#if getBrowserScreenshotURL(step.file)}
														<img
															alt={step.file.filename ??
																`Browser step ${step.index}`}
															class="w-full rounded-xl border border-border/60 bg-background object-cover shadow-sm"
															src={getBrowserScreenshotURL(step.file) ??
																undefined}
														/>
													{:else if getBrowserScreenshotLoadError(step.file)}
														<div
															class="flex aspect-[16/10] items-center justify-center rounded-xl border border-dashed border-border/70 bg-muted/20 px-4 text-center text-muted-foreground text-sm"
														>
															Screenshot unavailable
														</div>
													{:else}
														<div
															class="flex aspect-[16/10] items-center justify-center rounded-xl border border-dashed border-border/70 bg-muted/20 text-muted-foreground text-sm"
														>
															Loading screenshot…
														</div>
													{/if}
												</div>
											</button>
											<div class="mt-2 pl-1 text-muted-foreground text-xs">
												Step {step.event.stepIndex}
												{#if getBrowserEventTimestampLabel(step.event)}
													• {getBrowserEventTimestampLabel(step.event)}
												{/if}
											</div>
										</div>
									</div>
								{/each}
							{/if}
						</div>
					{:else}
						<div class="space-y-2 pl-11">
							{#each events as browserEvent (browserEvent.event.eventId)}
								{@const browserEventDetails = getBrowserEventDetails(
									browserEvent,
									{
										expanded: isBrowserDetailEventExpanded(
											browserEvent.event.eventId,
										),
									},
								)}
								{@const isBrowserEventDetailsExpandable =
									isBrowserEventDetailExpandable(browserEvent)}
								<div
									class="rounded-md border border-border/60 bg-background/80 px-3 py-2"
								>
									<div
										class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs"
									>
										<span class="font-medium text-foreground">
											{getBrowserEventMethodLabel(browserEvent)}
										</span>
										<span class="text-muted-foreground">
											Step {browserEvent.stepIndex}
										</span>
										<span class="text-muted-foreground uppercase">
											{browserEvent.event.direction}
										</span>
										{#if getBrowserEventTimestampLabel(browserEvent)}
											<span class="text-muted-foreground">
												{getBrowserEventTimestampLabel(browserEvent)}
											</span>
										{/if}
									</div>
									{#if browserEventDetails}
										<div class="mt-1 space-y-1">
											{#if isBrowserEventDetailsExpandable}
												<button
													class={`w-full text-left font-mono text-[11px] text-muted-foreground ${isBrowserDetailEventExpanded(browserEvent.event.eventId) ? "whitespace-pre-wrap break-words [overflow-wrap:anywhere]" : "truncate"}`}
													onclick={() => {
														setBrowserDetailEventExpanded(
															browserEvent.event.eventId,
															!isBrowserDetailEventExpanded(
																browserEvent.event.eventId,
															),
														);
													}}
													type="button"
												>
													{browserEventDetails}
												</button>
											{:else}
												<div
													class="truncate font-mono text-[11px] text-muted-foreground"
												>
													{browserEventDetails}
												</div>
											{/if}
											{#if isBrowserEventDetailsExpandable}
												<Button
													class="h-auto px-0 font-normal text-xs"
													onclick={() =>
														setBrowserDetailEventExpanded(
															browserEvent.event.eventId,
															!isBrowserDetailEventExpanded(
																browserEvent.event.eventId,
															),
														)}
													size="sm"
													type="button"
													variant="link"
												>
													{getBrowserEventDetailToggleLabel(
														browserEvent.event.eventId,
													)}
												</Button>
											{/if}
										</div>
									{/if}
									{#if browserEvent.event.files && browserEvent.event.files.length > 0}
										<div class="mt-2 flex flex-wrap gap-2">
											{#each browserEvent.event.files as file (file.path)}
												<Button
													size="sm"
													type="button"
													variant="outline"
													onclick={() => {
														void openBrowserScreenshotPreview(file);
													}}
												>
													{file.filename ?? "Screenshot"}
												</Button>
											{/each}
										</div>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		</CollapsibleContent>
	</Collapsible>
{/snippet}

{#snippet renderCompactionTurn(turn: ConversationTurn)}
	{@const requestMessage = turn.userMessages[0] ?? null}
	{@const responseMessage = getTurnFinalAssistantMessage(turn)}
	{@const isExpanded = isCompactionExpanded(turn.renderId)}
	<Collapsible
		open={isExpanded}
		onOpenChange={(open) => setCompactionExpanded(turn.renderId, open)}
	>
		<CollapsibleTrigger
			aria-label={`${isExpanded ? "Hide" : "Show"} compaction details`}
			class="flex w-full items-center gap-3 py-1 text-left"
			type="button"
		>
			<span class="h-px flex-1 bg-border"></span>
			<span
				class="rounded-full border border-border/70 bg-background px-3 py-1 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.14em] transition-colors hover:border-border hover:text-foreground"
			>
				Conversation compacted
			</span>
			<span class="h-px flex-1 bg-border"></span>
		</CollapsibleTrigger>
		<CollapsibleContent class="overflow-hidden pt-3">
			{#if isExpanded}
				<div
					class="space-y-3 rounded-xl border border-border bg-card p-4 shadow-sm"
				>
					<div class="space-y-2">
						<div
							class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
						>
							Compaction request
						</div>
						<pre
							class="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md border border-border bg-background p-3 font-mono text-foreground text-xs leading-5 [overflow-wrap:anywhere]"><code
								>{getMessageText(requestMessage)}</code
							></pre>
					</div>
					<div class="space-y-2">
						<div
							class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
						>
							Compaction response
						</div>
						<pre
							class="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md border border-border bg-background p-3 font-mono text-foreground text-xs leading-5 [overflow-wrap:anywhere]"><code
								>{getMessageText(responseMessage)}</code
							></pre>
					</div>
				</div>
			{/if}
		</CollapsibleContent>
	</Collapsible>
{/snippet}

{#snippet renderErrorBanner(
	key: ConversationPaneErrorBannerKey,
	errorText: string,
)}
	{@const displayText = getErrorBannerDisplayText(key, errorText)}
	{@const detailsText = getErrorBannerDetailsText(key, errorText)}
	{@const shouldCollapse = shouldCollapseErrorBanner(displayText)}
	{@const isExpanded = isErrorBannerExpanded(key)}
	{@const action = getErrorBannerAction(key)}
	<Alert variant="destructive">
		<AlertDescription class="min-w-0">
			<div class="flex min-w-0 flex-col items-start gap-2">
				<div
					class={`min-w-0 self-stretch ${shouldCollapse && isExpanded ? "max-h-64 overflow-auto pr-2" : ""}`}
				>
					<p
						class={`min-w-0 whitespace-pre-wrap break-words [overflow-wrap:anywhere] ${shouldCollapse && !isExpanded ? "line-clamp-3" : ""}`}
					>
						{displayText}
					</p>
					{#if detailsText && isExpanded}
						<p
							class="mt-2 min-w-0 whitespace-pre-wrap break-words rounded-md border border-destructive/20 bg-destructive/5 p-2 font-mono text-[11px] [overflow-wrap:anywhere]"
						>
							{detailsText}
						</p>
					{/if}
				</div>
				<div class="flex flex-wrap items-center gap-3">
					{#if action}
						<Button
							class="h-auto px-0 text-xs text-destructive hover:text-destructive"
							onclick={action.run}
							size="sm"
							variant="link"
						>
							{action.label}
						</Button>
					{/if}
					{#if shouldCollapse || detailsText}
						<Button
							aria-expanded={isExpanded}
							class="h-auto px-0 text-xs text-destructive hover:text-destructive"
							onclick={() => setErrorBannerExpanded(key, !isExpanded)}
							size="sm"
							variant="link"
						>
							{getErrorBannerToggleLabel(key, Boolean(detailsText))}
						</Button>
					{/if}
				</div>
			</div>
		</AlertDescription>
	</Alert>
{/snippet}

<div class="flex h-full min-h-0 flex-col overflow-hidden bg-background pt-1">
	{#if visibleSessionError || threadError}
		<div class="flex flex-col gap-2 p-3">
			{#if visibleSessionError}
				{@render renderErrorBanner("session", visibleSessionError)}
			{/if}
			{#if threadError}
				{@render renderErrorBanner("thread", threadError)}
			{/if}
		</div>
	{/if}
	<div
		class={`flex min-h-0 flex-1 flex-col transition-all duration-300 ease-out ${hasMessages ? "" : "justify-end md:justify-center"}`}
	>
		{#if hasMessages}
			<div class="relative min-h-0 min-w-0 flex-1">
				<div
					bind:this={viewport}
					class="scrollbar-gutter-stable h-full min-w-0 overflow-auto p-4"
				>
					<div
						class={`w-full min-w-0 space-y-4 ${effectiveChatWidthMode === "constrained" ? "mx-auto max-w-3xl" : ""}`}
						bind:this={contentEl}
					>
						{#each conversationTurns as turn, index (turn.renderId)}
							<div
								data-active-turn={turn.renderId === activeTurnId}
								data-conversation-turn-id={turn.renderId}
								class={`space-y-4 ${index > 0 && turn.userMessages.length > 0 ? "pt-20" : ""}`}
								style={getTurnStyle(turn.renderId === activeTurnId)}
							>
								{#if isCompactionTurn(turn)}
									{@render renderCompactionTurn(turn)}
								{:else}
									{#each turn.userMessages as message (message.renderId)}
										{@const userParts = getUserMessageRenderableParts(message)}
										{@const hookFailure =
											getHookFailureMessageMetadata(message)}
										<Message
											data-conversation-message-id={message.id}
											from={hookFailure ? "assistant" : "user"}
										>
											<MessageContent>
												{#if hookFailure}
													{@render renderHookFailureMessage(
														message.id,
														hookFailure,
													)}
												{:else}
													{@render renderUserMessageParts(message, userParts)}
												{/if}
											</MessageContent>
										</Message>
									{/each}
									{#if turn.assistantMessages.length > 0}
										{@const assistantMessage =
											getTurnFinalAssistantMessage(turn)}
										{@const browserEvents =
											browserEventsByTurnId[turn.id] ?? []}
										{@const groupedAssistantMessages =
											getTurnGroupedAssistantMessages(turn)}
										{@const turnAssistantRenderableParts =
											getAssistantMessagesAllRenderableParts(
												turn.assistantMessages,
											)}
										{@const partGroups = assistantMessage
											? getAssistantMessagePartGroups(assistantMessage, {
													isMessageComplete:
														!isActiveStreamingAssistantMessage(
															assistantMessage,
														),
												})
											: null}
										{@const groupedStepCount = getTurnGroupedStepCount(
											turn,
											partGroups,
										)}
										{@const collapsedAssistantHeaderLabel =
											getCollapsedAssistantHeaderLabel(turn, partGroups)}
										{#if assistantMessage}
											{@const isCollapsedStepSectionExpanded =
												isAssistantStepMessageExpanded(turn.renderId)}
											{#if groupedStepCount > 0}
												<Collapsible
													open={isCollapsedStepSectionExpanded}
													onOpenChange={(open) =>
														setAssistantStepMessageExpanded(
															turn.renderId,
															open,
														)}
												>
													<CollapsibleTrigger
														aria-label={`${isCollapsedStepSectionExpanded ? "Hide" : "Show"} ${collapsedAssistantHeaderLabel}`}
														class="flex w-full items-center gap-3 py-1 text-left"
														type="button"
													>
														<span class="h-px flex-1 bg-border"></span>
														<span
															class="rounded-full border border-border/70 bg-background px-3 py-1 font-medium text-[11px] text-muted-foreground uppercase tracking-[0.14em] transition-colors hover:border-border hover:text-foreground"
														>
															{collapsedAssistantHeaderLabel}
														</span>
														<span class="h-px flex-1 bg-border"></span>
													</CollapsibleTrigger>
													<CollapsibleContent class="overflow-hidden">
														{#if isCollapsedStepSectionExpanded}
															<div
																class="flex min-w-0 flex-col gap-2 [&>[data-ai-stack]+[data-ai-stack]]:-mt-8"
															>
																{#each groupedAssistantMessages as groupedAssistantMessage (groupedAssistantMessage.renderId)}
																	<Message
																		data-conversation-message-id={groupedAssistantMessage.id}
																		from="assistant"
																	>
																		<MessageContent>
																			{@render renderAssistantMessageParts(
																				groupedAssistantMessage,
																				getAssistantMessageAllRenderableParts(
																					groupedAssistantMessage,
																				),
																				turnAssistantRenderableParts,
																			)}
																		</MessageContent>
																		<AssistantMessageCopyActions
																			message={groupedAssistantMessage}
																		/>
																	</Message>
																{/each}
																{#if partGroups && partGroups.collapsedParts.length > 0}
																	<Message
																		data-conversation-message-id={`${assistantMessage.id}:grouped`}
																		from="assistant"
																	>
																		<MessageContent>
																			{@render renderAssistantMessageParts(
																				assistantMessage,
																				partGroups.collapsedParts,
																				turnAssistantRenderableParts,
																			)}
																		</MessageContent>
																		<AssistantMessageCopyActions
																			message={assistantMessage}
																		/>
																	</Message>
																{/if}
															</div>
														{/if}
													</CollapsibleContent>
												</Collapsible>
											{/if}
											{#if browserEvents.length > 0}
												{@render renderBrowserActivity(
													turn.renderId,
													browserEvents,
												)}
											{/if}
											<Message
												data-conversation-message-id={assistantMessage.id}
												from="assistant"
											>
												<MessageContent>
													{@render renderAssistantMessageParts(
														assistantMessage,
														partGroups
															? partGroups.visibleParts
															: getAssistantMessageAllRenderableParts(
																	assistantMessage,
																),
														turnAssistantRenderableParts,
													)}
												</MessageContent>
												<AssistantMessageCopyActions
													message={assistantMessage}
												/>
											</Message>
										{/if}
									{/if}
									{#if isStreaming && turn.renderId === activeTurnId}
										<Message from="assistant">
											<MessageContent>
												<div class="text-muted-foreground">
													<Loader size={18} />
												</div>
											</MessageContent>
										</Message>
									{/if}
								{/if}
							</div>
						{/each}
					</div>
				</div>
				<ConversationSelectionComment
					conversationRoot={contentEl}
					scrollContainer={viewport}
					onQueueComment={(comment) => {
						if (activeSessionId && activeThreadId) {
							addThreadPendingComment(activeSessionId, activeThreadId, comment);
						}
					}}
					onSubmitComment={submitSelectionComment}
				/>
				{#if !isNearBottom}
					<div
						class="pointer-events-none absolute inset-x-0 bottom-4 flex justify-center"
					>
						<Button
							aria-label="Scroll to bottom"
							class="pointer-events-auto rounded-full shadow-sm"
							onclick={() => scrollToBottom("smooth")}
							size="icon"
							type="button"
							variant="outline"
						>
							<ArrowDownIcon aria-hidden="true" class="size-4" />
						</Button>
					</div>
				{/if}
			</div>
		{:else if isLoading && !canShowComposer}
			<div
				class="flex min-h-0 flex-1 items-center justify-center p-4 text-muted-foreground text-sm"
			>
				Loading conversation...
			</div>
		{/if}

		{#if !hasMessages && isLoading && canShowComposer}
			<div class="px-4 pb-4 text-center text-muted-foreground text-sm">
				Loading conversation...
			</div>
		{/if}

		<HookPreviewDialog
			bind:open={hookPreviewOpen}
			metadata={hookPreviewMetadata}
			content={hookPreviewContent}
			loading={hookPreviewLoading}
			error={hookPreviewError}
			onEdit={() => {
				void editHookFile();
			}}
		/>

		<BrowserScreenshotPreviewDialog
			bind:open={browserScreenshotPreviewOpen}
			file={browserScreenshotPreviewFile}
			url={browserScreenshotPreviewURL}
			loading={browserScreenshotPreviewLoading}
			error={browserScreenshotPreviewError}
		/>

		{#if canShowComposer && session && thread}
			<ConversationComposer
				{session}
				{thread}
				onContainerChange={(element) => (composerContainer = element)}
			/>
		{/if}
	</div>
</div>
