<script lang="ts">
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import type { BrowserEventChunkData, ChatMessage } from "$lib/api-types";
	import {
		getAssistantMessagePartGroups,
		getUserMessageOriginalCommandDisplay,
		getUserMessageOriginalText,
		type AssistantConversationPaneRenderablePart,
		type UserOriginalCommandDisplay,
	} from "$lib/components/app/conversation-pane-message-parts";
	import { groupMessagesIntoTurns } from "$lib/components/app/conversation-pane-layout";
	import type { DynamicToolPart } from "$lib/components/ai/types";
	import type { PlanEntry } from "$lib/plan-entry";
	import type { ResolvedTheme } from "$lib/theme";
	import "$lib/web-components/conversation/define";

	type QuestionSubmitDetail = {
		callId?: string;
		approvalId?: string;
		answers?: Record<string, string>;
	};

	type Props = {
		messages: ChatMessage[];
		browserEventsByTurnId?: Record<string, BrowserEventChunkData[]>;
		status?: "idle" | "loading" | "ready" | "error";
		chatWidth?: "full" | "constrained";
		renderTurns?: boolean;
		toolDefaultOpen?: boolean;
		optimizedTools?: boolean;
		resolvedTheme?: ResolvedTheme;
		previousTodoEntriesByToolCallId?: Record<string, PlanEntry[]>;
		approvalResponsesByToolCallId?: Record<
			string,
			{ approved: boolean; reason?: string }
		>;
		onQuestionSubmit?: (detail: QuestionSubmitDetail) => void;
		onToolApprovalResponse?: (payload: {
			id: string;
			approved: boolean;
			reason?: string;
		}) => void;
	};

	let {
		messages,
		browserEventsByTurnId = {},
		status = "ready",
		chatWidth = "full",
		renderTurns = false,
		toolDefaultOpen = false,
		optimizedTools = true,
		onQuestionSubmit,
		onToolApprovalResponse,
	}: Props = $props();

	let expandedGeneratedUserMessages = $state<Record<string, boolean>>({});
	let root = $state<HTMLDivElement | null>(null);

	const turns = $derived.by(() => groupMessagesIntoTurns(messages));
	const OPTIMIZED_TOOL_TAGS: Record<string, string> = {
		Bash: "disco-tool-bash",
		PowerShell: "disco-tool-powershell",
		Read: "disco-tool-read",
		read: "disco-tool-read",
		Write: "disco-tool-write",
		Edit: "disco-tool-edit",
		Grep: "disco-tool-grep",
		Glob: "disco-tool-glob",
		RequestUserCredential: "disco-tool-request-user-credential",
		RequestCommitPull: "disco-tool-request-commit-pull",
		apply_patch: "disco-tool-apply-patch",
		WebSearch: "disco-tool-web-search",
		WebFetch: "disco-tool-web-fetch",
		TodoWrite: "disco-tool-todo-write",
		Task: "disco-tool-task",
		Skill: "disco-tool-skill",
	};

	function messageState(
		message: ChatMessage,
	): "pending" | "streaming" | "complete" | "error" {
		if (message.status === "streaming") {
			return "streaming";
		}
		if (message.status === "error") {
			return "error";
		}
		return message.provisional ? "pending" : "complete";
	}

	function partState(
		state: unknown,
	): "pending" | "streaming" | "complete" | "error" {
		if (state === "streaming") {
			return "streaming";
		}
		if (state === "error") {
			return "error";
		}
		if (state === "pending") {
			return "pending";
		}
		return "complete";
	}

	function toolState(state: unknown) {
		return typeof state === "string" && state.length > 0
			? state
			: "input-available";
	}

	function jsonText(value: unknown): string {
		return JSON.stringify(value, null, "\t");
	}

	function browserStepCount(events: BrowserEventChunkData[]): number {
		const screenshotKeys: string[] = [];
		for (const item of events) {
			for (const file of item.event.files ?? []) {
				const key =
					file.uri?.trim() || file.path?.trim() || file.filename?.trim();
				if (key && !screenshotKeys.includes(key)) {
					screenshotKeys.push(key);
				}
			}
		}
		return screenshotKeys.length || events.length;
	}

	function browserStepLabel(count: number): string {
		return count === 1 ? "1 BROWSER STEP" : `${count} BROWSER STEPS`;
	}

	function stepGroupLabel(count: number): string {
		return count === 1 ? "1 STEP" : `${count} STEPS`;
	}

	function optimizedToolTag(part: DynamicToolPart): string | undefined {
		return OPTIMIZED_TOOL_TAGS[part.toolName];
	}

	function toolInputText(part: DynamicToolPart): string {
		return part.input === undefined ? "" : jsonText(part.input);
	}

	function toolOutputText(part: DynamicToolPart): string {
		return part.output === undefined ? "" : jsonText(part.output);
	}

	function defaultOpenForTool(part: DynamicToolPart): boolean {
		return (
			toolDefaultOpen ||
			part.state === "approval-requested" ||
			part.state === "output-error"
		);
	}

	function hasSemanticToolRenderer(part: DynamicToolPart): boolean {
		return part.toolName === "AskUserQuestion";
	}

	function dynamicToolPart(
		part: Extract<ChatMessage["parts"][number], { type: "dynamic-tool" }>,
	): DynamicToolPart {
		return {
			type: "dynamic-tool",
			toolCallId: part.toolCallId,
			toolName: part.toolName,
			state: toolState(part.state) as DynamicToolPart["state"],
			input: part.input,
			approval: part.approval,
			output: part.output,
			errorText: part.errorText,
			title: part.title,
		};
	}

	function userTextParts(message: ChatMessage) {
		return message.parts.filter(
			(part): part is Extract<ChatMessage["parts"][number], { type: "text" }> =>
				part.type === "text" && part.text.length > 0,
		);
	}

	function userFileParts(message: ChatMessage) {
		return message.parts.filter(
			(part): part is Extract<ChatMessage["parts"][number], { type: "file" }> =>
				part.type === "file",
		);
	}

	function generatedTextLabel(
		command: UserOriginalCommandDisplay | null,
	): string {
		return command?.kind === "skill" ? "Skill text" : "Generated text";
	}

	function generatedTextForCommand(
		message: ChatMessage,
		command: UserOriginalCommandDisplay,
	): string[] {
		if (
			(command.kind === "skill" || command.kind === "script") &&
			command.text
		) {
			return [command.text];
		}
		return userTextParts(message).map((part) => part.text);
	}

	function commandLabel(command: UserOriginalCommandDisplay): string {
		const label =
			command.kind === "skill"
				? "Skill"
				: command.kind === "script"
					? "Script"
					: "Command";
		return `${label}: ${command.command}`;
	}

	function isGeneratedUserMessageExpanded(messageId: string): boolean {
		return expandedGeneratedUserMessages[messageId] ?? false;
	}

	function setGeneratedUserMessageExpanded(messageId: string, open: boolean) {
		expandedGeneratedUserMessages = {
			...expandedGeneratedUserMessages,
			[messageId]: open,
		};
	}

	function askUserQuestions(input: unknown): Array<{
		header?: string;
		question: string;
		notes?: string;
		multiSelect?: boolean;
		options: Array<{ label: string; description?: string }>;
	}> {
		if (!input || typeof input !== "object") {
			return [];
		}
		const questions = (input as { questions?: unknown }).questions;
		if (!Array.isArray(questions)) {
			return [];
		}
		return questions.filter(
			(
				question,
			): question is {
				header?: string;
				question: string;
				notes?: string;
				multiSelect?: boolean;
				options: Array<{ label: string; description?: string }>;
			} => {
				if (!question || typeof question !== "object") {
					return false;
				}
				const candidate = question as {
					question?: unknown;
					options?: unknown;
				};
				return (
					typeof candidate.question === "string" &&
					Array.isArray(candidate.options)
				);
			},
		);
	}

	function askUserAnswers(output: unknown): Array<{
		question: string;
		answer: string;
	}> {
		if (!output) {
			return [];
		}
		if (typeof output === "string") {
			try {
				return askUserAnswers(JSON.parse(output));
			} catch {
				return [];
			}
		}
		if (Array.isArray(output)) {
			return output
				.map((item) => {
					if (!item || typeof item !== "object") {
						return null;
					}
					const question = (item as { question?: unknown }).question;
					const answer = (item as { answer?: unknown }).answer;
					return typeof question === "string" && typeof answer === "string"
						? { question, answer }
						: null;
				})
				.filter((item): item is { question: string; answer: string } =>
					Boolean(item),
				);
		}
		if (typeof output === "object") {
			return Object.entries(output)
				.filter(
					(entry): entry is [string, string] =>
						typeof entry[0] === "string" && typeof entry[1] === "string",
				)
				.map(([question, answer]) => ({ question, answer }));
		}
		return [];
	}

	function approvalIdForTool(part: DynamicToolPart): string | undefined {
		const approval = part.approval;
		if (approval && typeof approval === "object" && "id" in approval) {
			const id = approval.id;
			return typeof id === "string" ? id : undefined;
		}
		return part.toolCallId;
	}

	function handleQuestionSubmit(event: Event) {
		const detail = (event as CustomEvent<QuestionSubmitDetail>).detail;
		const approvalId = detail.approvalId ?? detail.callId;
		onQuestionSubmit?.(detail);
		if (!approvalId || !detail.answers) {
			return;
		}

		onToolApprovalResponse?.({
			id: approvalId,
			approved: true,
		});
	}

	$effect(() => {
		const element = root;
		if (!element) {
			return;
		}
		element.addEventListener(
			"disco-tool-question-submit",
			handleQuestionSubmit,
		);
		return () => {
			element.removeEventListener(
				"disco-tool-question-submit",
				handleQuestionSubmit,
			);
		};
	});
</script>

<div bind:this={root}>
	<disco-conversation {status} chat-width={chatWidth}>
		{#if renderTurns}
			{#each turns as turn, index (turn.renderId)}
				<disco-turn
					id={turn.id}
					open
					style={index > 0 && turn.userMessages.length > 0
						? "padding-top: var(--disco-turn-spacing, 5rem)"
						: undefined}
				>
					{#each turn.userMessages as message (message.renderId)}
						{@render MessageElement(message)}
					{/each}

					{#each turn.assistantMessages as message, messageIndex (message.renderId)}
						{@render AssistantMessageElement(
							message,
							messageIndex === turn.assistantMessages.length - 1
								? (browserEventsByTurnId[turn.id] ?? [])
								: [],
						)}
					{/each}
				</disco-turn>
			{/each}
		{:else}
			{#each messages as message (message.id)}
				{@render MessageElement(message)}
			{/each}
		{/if}
	</disco-conversation>
</div>

{#snippet MessageElement(message: ChatMessage)}
	<disco-message
		id={message.id}
		from={message.role}
		state={messageState(message)}
		model={message.metadata?.model}
		provisional={message.provisional}
		synthetic={message.synthetic}
		replaces-message-id={message.replacesMessageId}
		replaced-by-message-id={message.replacedByMessageId}
	>
		{#if message.metadata}
			<disco-metadata>
				<svelte:element this={"script"} type="application/json"
					>{jsonText(message.metadata)}</svelte:element
				>
			</disco-metadata>
		{/if}
		{#if message.role === "user"}
			{@render UserMessageParts(message)}
		{:else}
			{#each message.parts as part, index (`${message.id}:${index}`)}
				{@render PartElement(part, message.id, index)}
			{/each}
		{/if}
	</disco-message>
{/snippet}

{#snippet UserMessageParts(message: ChatMessage)}
	{@const originalText = getUserMessageOriginalText(message)}
	{@const originalCommand = getUserMessageOriginalCommandDisplay(message)}
	{@const textParts = userTextParts(message)}
	{@const fileParts = userFileParts(message)}
	{#if originalCommand}
		{@const generatedText = generatedTextForCommand(message, originalCommand)}
		{@const generatedTextExpanded = isGeneratedUserMessageExpanded(message.id)}
		<div
			data-disco-command-block
			data-command-kind={originalCommand.kind}
			data-part-id={`${message.id}:command`}
			class="space-y-2"
		>
			<button
				aria-expanded={generatedTextExpanded}
				aria-label={`${generatedTextExpanded ? "Hide" : "Show"} ${originalCommand.kind === "skill" ? "skill text" : "generated text"}`}
				class="group flex w-full items-center gap-2 rounded-sm text-left text-muted-foreground transition hover:text-foreground"
				disabled={generatedText.length === 0}
				onclick={() =>
					setGeneratedUserMessageExpanded(message.id, !generatedTextExpanded)}
				type="button"
			>
				<ChevronRightIcon class="size-3.5 shrink-0" />
				<span class="text-[11px] uppercase tracking-[0.14em]">
					{commandLabel(originalCommand)}
				</span>
				<ChevronDownIcon
					class={`size-3 transition-all group-hover:opacity-100 ${generatedTextExpanded ? "rotate-180 opacity-100" : "opacity-0"}`}
				/>
			</button>
			{#if originalCommand.args}
				<disco-message-content
					part-id={`${message.id}:command-args`}
					format="markdown">{originalCommand.args}</disco-message-content
				>
			{/if}
			{#if originalCommand.kind === "script" && originalCommand.script?.suppressedLlm}
				<div
					class="rounded-md border border-dashed px-3 py-2 text-muted-foreground text-sm"
				>
					The script completed without output, so no model response was started.
				</div>
			{/if}
			{#if generatedTextExpanded && generatedText.length > 0}
				<disco-generated-text
					part-id={`${message.id}:generated-text`}
					label={generatedTextLabel(originalCommand)}
					content-only
				>
					{generatedText.join("\n\n")}
				</disco-generated-text>
			{/if}
		</div>
	{:else if originalText}
		<disco-message-content
			part-id={`${message.id}:original-text`}
			format="markdown">{originalText}</disco-message-content
		>
		{#if textParts.length > 0}
			<disco-generated-text
				part-id={`${message.id}:generated-text`}
				label="Generated text"
			>
				{textParts.map((part) => part.text).join("\n\n")}
			</disco-generated-text>
		{/if}
	{:else}
		{#each textParts as part, index (`${message.id}:text-${index}`)}
			<disco-message-content
				part-id={`${message.id}:text-${index}`}
				format="markdown">{part.text}</disco-message-content
			>
		{/each}
	{/if}

	{#each fileParts as part, index (`${message.id}:file-${index}`)}
		{@render PartElement(part, message.id, message.parts.indexOf(part))}
	{/each}
{/snippet}

{#snippet AssistantParts(
	parts: AssistantConversationPaneRenderablePart[],
	messageId: string,
	idSuffix = "",
)}
	{#each parts as part, index (`${messageId}${idSuffix}:part-${index}`)}
		{@render PartElement(part, `${messageId}${idSuffix}`, index)}
	{/each}
{/snippet}

{#snippet BrowserActivityElement(
	turnId: string,
	browserEvents: BrowserEventChunkData[],
)}
	{#if browserEvents.length > 0}
		<disco-browser-activity
			part-id={`${turnId}:browser-activity`}
			step-count={browserStepCount(browserEvents)}
			summary={browserStepLabel(browserStepCount(browserEvents))}
		>
			<disco-metadata>
				<svelte:element this={"script"} type="application/json"
					>{jsonText({ events: browserEvents })}</svelte:element
				>
			</disco-metadata>
		</disco-browser-activity>
	{/if}
{/snippet}

{#snippet AssistantMessageElement(
	message: ChatMessage,
	browserEvents: BrowserEventChunkData[] = [],
)}
	{@const partGroups = getAssistantMessagePartGroups(message, {
		isMessageComplete: status === "ready",
	})}
	{@const turnId = message.metadata?.discobot?.turnId ?? message.id}
	<disco-message
		id={message.id}
		from={message.role}
		state={messageState(message)}
		model={message.metadata?.model}
		provisional={message.provisional}
		synthetic={message.synthetic}
		replaces-message-id={message.replacesMessageId}
		replaced-by-message-id={message.replacedByMessageId}
	>
		{#if message.metadata}
			<disco-metadata>
				<svelte:element this={"script"} type="application/json"
					>{jsonText(message.metadata)}</svelte:element
				>
			</disco-metadata>
		{/if}
		{#if partGroups.hasCollapsedSteps}
			<disco-step-group label={stepGroupLabel(partGroups.collapsedStepCount)}>
				{@render AssistantParts(
					partGroups.collapsedParts,
					message.id,
					":grouped",
				)}
			</disco-step-group>
			{@render BrowserActivityElement(turnId, browserEvents)}
			{@render AssistantParts(partGroups.visibleParts, message.id)}
		{:else}
			{@render BrowserActivityElement(turnId, browserEvents)}
			{#each message.parts as part, index (`${message.id}:part-${index}`)}
				{@render PartElement(part, message.id, index)}
			{/each}
		{/if}
	</disco-message>
{/snippet}

{#snippet PartElement(
	part: ChatMessage["parts"][number],
	messageId: string,
	index: number,
)}
	{#if part.type === "text"}
		<disco-message-content
			part-id={`${messageId}:part-${index}`}
			format="markdown">{part.text}</disco-message-content
		>
	{:else if part.type === "reasoning"}
		<disco-reasoning
			part-id={`${messageId}:part-${index}`}
			state={partState(part.state)}
		>
			{part.text}
		</disco-reasoning>
	{:else if part.type === "dynamic-tool"}
		{@const normalizedToolPart = dynamicToolPart(part)}
		{@const optimizedToolElement = optimizedToolTag(normalizedToolPart)}
		{#if hasSemanticToolRenderer(normalizedToolPart)}
			{@render AskUserQuestionElement(normalizedToolPart, messageId, index)}
		{:else if optimizedTools && optimizedToolElement}
			{@render OptimizedToolElement(normalizedToolPart, messageId, index)}
		{:else}
			<disco-tool-call
				part-id={`${messageId}:part-${index}`}
				call-id={part.toolCallId}
				name={part.toolName}
				state={toolState(part.state)}
				title={part.title}
				open={toolDefaultOpen ||
					part.state === "approval-requested" ||
					part.state === "output-error"}
			>
				{#if part.input !== undefined}
					<disco-tool-input format="json">
						<svelte:element this={"script"} type="application/json"
							>{jsonText(part.input)}</svelte:element
						>
					</disco-tool-input>
				{/if}
				{#if part.output !== undefined}
					<disco-tool-output format="json">
						<svelte:element this={"script"} type="application/json"
							>{jsonText(part.output)}</svelte:element
						>
					</disco-tool-output>
				{/if}
			</disco-tool-call>
		{/if}
	{:else if part.type === "file"}
		<disco-attachment
			part-id={`${messageId}:part-${index}`}
			kind="file"
			src={part.url}
			filename={part.filename}
			media-type={part.mediaType}
		></disco-attachment>
	{:else}
		<disco-event
			part-id={`${messageId}:part-${index}`}
			kind={part.type}
			summary="Unsupported message part"
		>
			<disco-metadata>
				<svelte:element this={"script"} type="application/json"
					>{jsonText(part)}</svelte:element
				>
			</disco-metadata>
		</disco-event>
	{/if}
{/snippet}

{#snippet OptimizedToolElement(
	toolPart: DynamicToolPart,
	messageId: string,
	index: number,
)}
	{@const partId = `${messageId}:part-${index}`}
	{@const input = toolInputText(toolPart)}
	{@const output = toolOutputText(toolPart)}
	{@const errorText = toolPart.errorText ?? ""}
	{@const defaultOpen = defaultOpenForTool(toolPart)}
	{#if toolPart.toolName === "Bash"}
		<disco-tool-bash
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-bash>
	{:else if toolPart.toolName === "PowerShell"}
		<disco-tool-powershell
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-powershell>
	{:else if toolPart.toolName === "Read" || toolPart.toolName === "read"}
		<disco-tool-read
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-read>
	{:else if toolPart.toolName === "Write"}
		<disco-tool-write
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-write>
	{:else if toolPart.toolName === "Edit"}
		<disco-tool-edit
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-edit>
	{:else if toolPart.toolName === "Grep"}
		<disco-tool-grep
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-grep>
	{:else if toolPart.toolName === "Glob"}
		<disco-tool-glob
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-glob>
	{:else if toolPart.toolName === "RequestUserCredential"}
		<disco-tool-request-user-credential
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-request-user-credential>
	{:else if toolPart.toolName === "RequestCommitPull"}
		<disco-tool-request-commit-pull
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-request-commit-pull>
	{:else if toolPart.toolName === "apply_patch"}
		<disco-tool-apply-patch
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-apply-patch>
	{:else if toolPart.toolName === "WebSearch"}
		<disco-tool-web-search
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-web-search>
	{:else if toolPart.toolName === "WebFetch"}
		<disco-tool-web-fetch
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-web-fetch>
	{:else if toolPart.toolName === "TodoWrite"}
		<disco-tool-todo-write
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-todo-write>
	{:else if toolPart.toolName === "Task"}
		<disco-tool-task
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-task>
	{:else if toolPart.toolName === "Skill"}
		<disco-tool-skill
			part-id={partId}
			call-id={toolPart.toolCallId}
			state={toolPart.state}
			title={toolPart.title}
			{input}
			{output}
			error-text={errorText}
			default-open={defaultOpen}
		></disco-tool-skill>
	{/if}
{/snippet}

{#snippet AskUserQuestionElement(
	toolPart: DynamicToolPart,
	messageId: string,
	index: number,
)}
	<disco-tool-ask-user-question
		part-id={`${messageId}:part-${index}`}
		call-id={toolPart.toolCallId}
		state={toolPart.state}
		approval-id={approvalIdForTool(toolPart)}
	>
		{#each askUserQuestions(toolPart.input) as question, questionIndex (`${toolPart.toolCallId}:question-${questionIndex}`)}
			<disco-question
				name={question.question}
				header={question.header ?? `Question ${questionIndex + 1}`}
				type={question.multiSelect ? "multiple" : "single"}
			>
				<span slot="question">{question.question}</span>
				{#if question.notes}
					<span slot="notes">{question.notes}</span>
				{/if}
				{#each question.options as option, optionIndex (`${toolPart.toolCallId}:question-${questionIndex}:option-${optionIndex}`)}
					<disco-option value={option.label}>
						<span slot="label">{option.label}</span>
						{#if option.description}
							<span slot="description">{option.description}</span>
						{/if}
					</disco-option>
				{/each}
			</disco-question>
		{/each}
		{#each askUserAnswers(toolPart.output) as answer, answerIndex (`${toolPart.toolCallId}:answer-${answerIndex}`)}
			<disco-answer question={answer.question} answer={answer.answer}
			></disco-answer>
		{/each}
	</disco-tool-ask-user-question>
{/snippet}
