<script lang="ts">
	import {
		Tool,
		ToolContent,
		ToolHeader,
		ToolInput,
		ToolOutput,
	} from "$lib/components/ai/tool";
	import type { DynamicToolPart } from "$lib/components/ai/types";
	import type { PlanEntry } from "$lib/plan-entry";
	import type { ResolvedTheme } from "$lib/theme";
	import { getToolRenderer, getToolTitle } from "./registry";

	type Props = {
		toolPart: DynamicToolPart;
		queued?: boolean;
		forceRaw?: boolean;
		defaultOpen?: boolean;
		sessionId?: string | null;
		threadId?: string | null;
		resolvedTheme?: ResolvedTheme;
		previousTodoEntries?: PlanEntry[];
		approvalResponse?: { approved: boolean; reason?: string };
		onToolApprovalResponse?: (payload: {
			id: string;
			approved: boolean;
			reason?: string;
		}) => void;
	};

	let {
		toolPart,
		queued = false,
		forceRaw = false,
		defaultOpen = false,
		sessionId,
		threadId,
		resolvedTheme,
		previousTodoEntries,
		approvalResponse,
		onToolApprovalResponse,
	}: Props = $props();

	const isApprovalAnswered = $derived(approvalResponse !== undefined);
	const isPendingApproval = $derived(
		toolPart.state === "approval-requested" && !isApprovalAnswered,
	);
	const getInitialOpen = () =>
		defaultOpen ||
		toolPart.toolName === "AskUserQuestion" ||
		(toolPart.toolName === "RequestCommitPull" && isPendingApproval) ||
		(toolPart.toolName === "RequestUserCredential" && isPendingApproval);

	let isRaw = $derived(forceRaw);
	let open = $state(getInitialOpen());

	$effect(() => {
		if (
			toolPart.toolName === "AskUserQuestion" ||
			(toolPart.toolName === "RequestCommitPull" && isPendingApproval) ||
			(toolPart.toolName === "RequestUserCredential" && isPendingApproval)
		) {
			open = true;
		}
	});

	const renderedInput = $derived.by(() => toolPart.input);
	const renderedOutput = $derived.by(() => toolPart.output);
	const renderedError = $derived.by(() => toolPart.errorText);
	const Renderer = $derived.by(() => getToolRenderer(toolPart.toolName));
	const hasOptimizedView = $derived.by(() => Boolean(Renderer));
	const isAlwaysExpanded = $derived.by(
		() =>
			toolPart.toolName === "AskUserQuestion" ||
			(toolPart.toolName === "RequestCommitPull" && isPendingApproval) ||
			(toolPart.toolName === "RequestUserCredential" && isPendingApproval),
	);
	const showRaw = $derived.by(() => !hasOptimizedView || isRaw);
	const title = $derived.by(() => getToolTitle(toolPart));
</script>

{#if showRaw}
	<Tool bind:open {defaultOpen} {queued} showBorder={false}>
		<ToolHeader
			type="dynamic-tool"
			toolName={toolPart.toolName}
			state={toolPart.state}
			{title}
			isRaw={showRaw}
			onToggleRaw={hasOptimizedView ? () => (isRaw = !isRaw) : undefined}
			canCollapse={!isAlwaysExpanded}
		/>
		<ToolContent>
			<ToolInput input={renderedInput} />
			<ToolOutput output={renderedOutput} errorText={renderedError} />
		</ToolContent>
	</Tool>
{:else if Renderer}
	<Tool bind:open {defaultOpen} {queued} showBorder={false}>
		<Renderer
			{toolPart}
			{queued}
			{sessionId}
			{threadId}
			{resolvedTheme}
			{previousTodoEntries}
			{approvalResponse}
			{onToolApprovalResponse}
			{isRaw}
			onToggleRaw={() => (isRaw = !isRaw)}
		/>
	</Tool>
{/if}
