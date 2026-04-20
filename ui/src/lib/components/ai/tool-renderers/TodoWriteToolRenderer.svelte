<script lang="ts">
	import ListTodoIcon from "@lucide/svelte/icons/list-todo";
	import {
		ToolContent,
		ToolHeaderControls,
		ToolHeaderStatus,
	} from "$lib/components/ai/tool";
	import {
		type TodoWriteToolInput,
		type TodoWriteToolOutput,
		validateTodoWriteInput,
		validateTodoWriteOutput,
	} from "$lib/components/ai/tool-schemas/todowrite-schema";
	import { CollapsibleTrigger } from "$lib/components/ui/collapsible";
	import type { ToolRendererComponentProps } from "./types";
	import { renderToolValue } from "./utils";

	let {
		toolPart,
		isRaw,
		onToggleRaw,
		previousTodoEntries = [],
	}: ToolRendererComponentProps = $props();

	function getTodoKey(todo: {
		content?: string;
		activeForm?: string;
		status?: string;
	}): string {
		return `${todo.content ?? ""}::${todo.activeForm ?? ""}::${todo.status ?? ""}`;
	}

	function getTodoLabel(todo: {
		content?: string;
		activeForm?: string;
	}): string {
		return todo.activeForm || todo.content || "Untitled task";
	}

	const isStreaming = $derived.by(
		() =>
			toolPart.state === "input-streaming" ||
			toolPart.state === "input-available",
	);
	const inputValidation = $derived.by(() =>
		validateTodoWriteInput(toolPart.input),
	);
	const validInput = $derived.by(() =>
		inputValidation.success
			? (inputValidation.data as TodoWriteToolInput)
			: undefined,
	);
	const outputValidation = $derived.by(() =>
		toolPart.output ? validateTodoWriteOutput(toolPart.output) : null,
	);
	const validOutput = $derived.by(() =>
		outputValidation?.success
			? (outputValidation.data as TodoWriteToolOutput)
			: undefined,
	);
	const todos = $derived.by(() => validInput?.todos ?? []);
	const totalTodos = $derived.by(() => todos.length);
	const todoCounts = $derived.by(() => {
		const counts = { completed: 0, in_progress: 0, pending: 0 };
		for (const todo of todos) {
			if (todo.status === "completed") {
				counts.completed += 1;
			} else if (todo.status === "in_progress") {
				counts.in_progress += 1;
			} else {
				counts.pending += 1;
			}
		}
		return counts;
	});
	const progressPercent = $derived.by(() =>
		totalTodos > 0 ? Math.round((todoCounts.completed / totalTodos) * 100) : 0,
	);
	const currentTodo = $derived.by(
		() =>
			todos.find((todo) => todo.status === "in_progress") ??
			todos.find((todo) => todo.status === "pending"),
	);
	const currentTaskLabel = $derived.by(() =>
		currentTodo ? getTodoLabel(currentTodo) : null,
	);
	const completedTodos = $derived.by(() =>
		todos.filter((todo) => todo.status === "completed"),
	);
	const previousCompletedTodoKeys = $derived.by(
		() =>
			new Set(
				previousTodoEntries
					.filter((todo) => todo.status === "completed")
					.map((todo) => getTodoKey(todo)),
			),
	);
	const newlyCompletedTodos = $derived.by(() =>
		completedTodos.filter(
			(todo) => !previousCompletedTodoKeys.has(getTodoKey(todo)),
		),
	);
	const remainingTodos = $derived.by(() =>
		todos.filter((todo) => todo.status !== "completed"),
	);
	const latestCompletedTodos = $derived.by(() =>
		newlyCompletedTodos.length > 0
			? newlyCompletedTodos
			: completedTodos.slice(Math.max(completedTodos.length - 2, 0)),
	);
	const hasSuccessState = $derived.by(() =>
		Boolean(validOutput?.success || validOutput?.content),
	);
	const todoError = $derived.by(() => toolPart.errorText || validOutput?.error);
	const rawOutputText = $derived.by(() => renderToolValue(toolPart.output));
	const headerSummary = $derived.by(() => {
		if (totalTodos > 0) {
			const progress = `${todoCounts.completed}/${totalTodos} done`;
			return currentTaskLabel ? `${progress} • ${currentTaskLabel}` : progress;
		}

		return isStreaming ? "Loading todos..." : "Todo write";
	});
</script>

<div class="flex items-center justify-between gap-4 px-4 pt-4">
	<CollapsibleTrigger
		class="flex min-w-0 flex-1 items-center gap-2 text-left text-muted-foreground"
	>
		<ListTodoIcon class="size-4 shrink-0 text-muted-foreground" />
		<span class="truncate font-medium text-sm">{headerSummary}</span>
		<ToolHeaderStatus state={toolPart.state} />
	</CollapsibleTrigger>
	<ToolHeaderControls {isRaw} {onToggleRaw} />
</div>

<ToolContent>
	{#if !toolPart.input || typeof toolPart.input !== "object"}
		<div class="p-4 pt-3 text-muted-foreground text-sm">
			{isStreaming ? "Loading todos..." : "Todo details are unavailable."}
		</div>
	{:else if !inputValidation.success || !validInput?.todos}
		<div class="space-y-3 p-4 pt-3">
			<p class="text-muted-foreground text-sm">
				{isStreaming ? "Loading todos..." : "Could not parse todo details."}
			</p>
			{#if rawOutputText}
				<div class="rounded-md border border-dashed bg-muted/20 p-3">
					<pre
						class="overflow-x-auto whitespace-pre-wrap break-words font-mono text-xs"><code
							>{rawOutputText}</code
						></pre>
				</div>
			{/if}
		</div>
	{:else}
		<div class="space-y-4 p-4 pt-3">
			<div class="space-y-2 rounded-md border bg-muted/20 p-3">
				<div
					class="flex items-center justify-between gap-3 text-xs text-muted-foreground"
				>
					<span>{progressPercent}% complete</span>
					<span>
						{todoCounts.completed}/{totalTodos} done
					</span>
				</div>
				<div class="h-2 overflow-hidden rounded-full bg-muted">
					<div
						class="h-full rounded-full bg-foreground/70 transition-[width]"
						style={`width: ${progressPercent}%`}
					></div>
				</div>
				<div class="flex flex-wrap gap-2 text-xs">
					<span class="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
						{todoCounts.completed} completed
					</span>
					<span class="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
						{todoCounts.in_progress} in progress
					</span>
					<span class="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
						{todoCounts.pending} pending
					</span>
				</div>
			</div>

			{#if currentTodo}
				<div class="rounded-md border bg-background p-3">
					<div
						class="mb-1 text-muted-foreground text-xs uppercase tracking-wide"
					>
						{currentTodo.status === "in_progress"
							? "Currently working on"
							: "Up next"}
					</div>
					<div class="text-sm">{getTodoLabel(currentTodo)}</div>
					{#if currentTodo.activeForm && currentTodo.activeForm !== currentTodo.content}
						<div class="mt-1 text-muted-foreground text-xs">
							{currentTodo.content}
						</div>
					{/if}
				</div>
			{/if}

			{#if latestCompletedTodos.length > 0}
				<div class="rounded-md border bg-background p-3">
					<div
						class="mb-2 text-muted-foreground text-xs uppercase tracking-wide"
					>
						{newlyCompletedTodos.length > 0
							? "Completed since last update"
							: "Recently completed"}
					</div>
					<ul class="space-y-1 text-sm">
						{#each latestCompletedTodos as todo (getTodoKey(todo))}
							<li class="flex items-start gap-2">
								<span class="mt-0.5 text-muted-foreground">✓</span>
								<span>{todo.content || "Untitled task"}</span>
							</li>
						{/each}
					</ul>
				</div>
			{/if}

			{#if remainingTodos.length > 0}
				<div class="rounded-md border bg-muted/20 p-3">
					<div
						class="mb-2 text-muted-foreground text-xs uppercase tracking-wide"
					>
						Remaining work
					</div>
					<ul class="space-y-1">
						{#each remainingTodos as todo (getTodoKey(todo))}
							<li class="flex items-start gap-2 text-xs">
								<span class="mt-0.5 text-muted-foreground"
									>{todo.status === "in_progress" ? "•" : "○"}</span
								>
								<div>
									<div>{todo.content || "Untitled task"}</div>
									{#if todo.activeForm && todo.activeForm !== todo.content}
										<div class="text-muted-foreground">{todo.activeForm}</div>
									{/if}
								</div>
							</li>
						{/each}
					</ul>
				</div>
			{/if}

			{#if hasSuccessState}
				<p class="text-green-700 text-xs">Todo list updated.</p>
			{/if}

			{#if todoError}
				<div
					class="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-destructive text-sm"
				>
					{todoError}
				</div>
			{/if}

			{#if outputValidation && !outputValidation.success && rawOutputText}
				<div class="rounded-md border border-dashed bg-muted/20 p-3">
					<h5
						class="mb-2 font-medium text-muted-foreground text-xs uppercase tracking-wide"
					>
						Unparsed output
					</h5>
					<pre
						class="overflow-x-auto whitespace-pre-wrap break-words font-mono text-xs"><code
							>{rawOutputText}</code
						></pre>
				</div>
			{/if}
		</div>
	{/if}
</ToolContent>
