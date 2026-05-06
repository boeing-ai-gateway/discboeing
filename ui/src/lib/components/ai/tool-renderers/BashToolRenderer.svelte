<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import CircleCheckIcon from "@lucide/svelte/icons/circle-check";
	import CircleXIcon from "@lucide/svelte/icons/circle-x";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import KeyRoundIcon from "@lucide/svelte/icons/key-round";
	import TerminalIcon from "@lucide/svelte/icons/terminal";
	import { api } from "$lib/api-client";
	import type { SessionCredentialAssignment } from "$lib/api-types";
	import {
		ToolContent,
		ToolHeaderControls,
		ToolHeaderStatus,
	} from "$lib/components/ai/tool";
	import {
		type BashToolOutput,
		validateBashInput,
		validateBashOutput,
	} from "$lib/components/ai/tool-schemas/bash-schema";
	import { Button } from "$lib/components/ui/button";
	import { CollapsibleTrigger } from "$lib/components/ui/collapsible";
	import { cn } from "$lib/utils";
	import type { ToolRendererComponentProps } from "./types";
	import {
		countLines,
		getToolInputString,
		parseNumberedToolOutput,
		renderToolValue,
		shortenPath,
	} from "./utils";

	let {
		toolPart,
		sessionId = null,
		isRaw,
		onToggleRaw,
	}: ToolRendererComponentProps = $props();
	let copiedTarget = $state<"command" | "stdout" | null>(null);
	let copyTimeout = $state<ReturnType<typeof setTimeout> | null>(null);
	let sessionAssignments = $state<SessionCredentialAssignment[]>([]);

	const isStreaming = $derived.by(
		() =>
			toolPart.state === "input-streaming" ||
			toolPart.state === "input-available",
	);
	const headerDescription = $derived.by(() =>
		getToolInputString(toolPart.input, "description"),
	);
	const headerCommand = $derived.by(() =>
		getToolInputString(toolPart.input, "command"),
	);
	const inputValidation = $derived.by(() => validateBashInput(toolPart.input));
	const validInput = $derived.by(() =>
		inputValidation.success ? inputValidation.data : undefined,
	);
	const outputValidation = $derived.by(() =>
		toolPart.output ? validateBashOutput(toolPart.output) : null,
	);
	const validOutput = $derived.by(() =>
		outputValidation?.success
			? (outputValidation.data as BashToolOutput)
			: undefined,
	);
	const stdout = $derived.by(
		() => validOutput?.output || validOutput?.stdout || "",
	);
	const parsedStdout = $derived.by(() => parseNumberedToolOutput(stdout));
	const hasParsedStdoutLines = $derived.by(() => parsedStdout.lines.length > 0);
	const displayedLineCount = $derived.by(() =>
		hasParsedStdoutLines ? parsedStdout.lines.length : countLines(stdout),
	);
	const rawOutputText = $derived.by(() => renderToolValue(toolPart.output));
	const executionError = $derived.by(
		() => toolPart.errorText || validOutput?.stderr,
	);
	const credentialUses = $derived.by(() => validInput?.credentialUses ?? []);
	const hasCredentialUses = $derived.by(() => credentialUses.length > 0);
	const credentialUseDetails = $derived.by(() =>
		credentialUses.map((binding) => {
			const assignment = sessionAssignments.find(
				(item) => item.sessionCredentialId === binding.credentialId,
			);
			const approvedUse = assignment?.uses?.find(
				(use) => use.id === binding.useId,
			);
			return {
				...binding,
				credentialName:
					assignment?.credential.name ??
					assignment?.credentialId ??
					binding.credentialId,
				useDescription: approvedUse?.description,
			};
		}),
	);

	$effect(() => {
		if (!sessionId || !hasCredentialUses) {
			sessionAssignments = [];
			return;
		}

		let cancelled = false;
		api
			.getSessionCredentials(sessionId)
			.then((response) => {
				if (!cancelled) {
					sessionAssignments = response.credentials;
				}
			})
			.catch(() => {
				if (!cancelled) {
					sessionAssignments = [];
				}
			});

		return () => {
			cancelled = true;
		};
	});

	async function copyToClipboard(
		target: "command" | "stdout",
		text: string | undefined,
	) {
		if (!text || typeof window === "undefined" || !navigator?.clipboard) {
			return;
		}

		await navigator.clipboard.writeText(text);
		copiedTarget = target;
		if (copyTimeout) {
			clearTimeout(copyTimeout);
		}
		copyTimeout = setTimeout(() => {
			copiedTarget = null;
		}, 2000);
	}

	$effect(() => {
		return () => {
			if (copyTimeout) {
				clearTimeout(copyTimeout);
			}
		};
	});
</script>

<div class="flex items-center justify-between gap-4 px-4 pt-4">
	<CollapsibleTrigger
		class={cn(
			"flex min-w-0 flex-1 items-center gap-2 text-left text-muted-foreground",
			hasCredentialUses && "text-amber-700 dark:text-amber-300",
		)}
	>
		<TerminalIcon
			class={cn(
				"size-4 shrink-0 text-muted-foreground",
				hasCredentialUses && "text-amber-600 dark:text-amber-300",
			)}
		/>
		<span class="truncate font-medium text-sm">
			{headerDescription ||
				headerCommand ||
				(isStreaming ? "Loading command details..." : "Command")}
		</span>
		{#if hasCredentialUses}
			<span
				class="inline-flex shrink-0 items-center gap-1 rounded-full border border-amber-500/30 bg-amber-500/10 px-2 py-0.5 font-medium text-amber-700 text-xs dark:text-amber-300"
			>
				<KeyRoundIcon class="size-3" />
				Credential
			</span>
		{/if}
		<ToolHeaderStatus state={toolPart.state} />
	</CollapsibleTrigger>
	<ToolHeaderControls {isRaw} {onToggleRaw} />
</div>

<ToolContent>
	{#if !toolPart.input || typeof toolPart.input !== "object"}
		<div class="p-4 pt-3 text-muted-foreground text-sm">
			{isStreaming
				? "Loading command details..."
				: "Command details are unavailable."}
		</div>
	{:else if !inputValidation.success}
		<div class="space-y-3 p-4 pt-3">
			<p class="text-muted-foreground text-sm">
				{isStreaming
					? "Loading command details..."
					: "Could not parse command details."}
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
			{#if hasCredentialUses}
				<div
					class="space-y-3 rounded-md border border-amber-500/30 bg-amber-500/10 p-3 text-amber-950 text-sm dark:text-amber-100"
				>
					<div class="flex items-center gap-2 font-medium">
						<KeyRoundIcon class="size-4" />
						<span>Credential access used</span>
					</div>
					<ul class="space-y-2">
						{#each credentialUseDetails as binding (`${binding.credentialId}:${binding.useId}:${binding.envVar}`)}
							<li class="space-y-1 rounded-md bg-background/60 p-2">
								<div class="flex flex-wrap items-center gap-2">
									<span class="font-medium">{binding.credentialName}</span>
									<span class="font-mono text-xs text-current/70">
										→ {binding.envVar}
									</span>
								</div>
								<div class="text-current/80 text-xs">
									{#if binding.useDescription}
										Used for: {binding.useDescription}
									{:else}
										Approved use ID: <span class="font-mono"
											>{binding.useId}</span
										>
									{/if}
								</div>
								<div class="font-mono text-current/60 text-[11px]">
									Credential ID: {binding.credentialId}
								</div>
							</li>
						{/each}
					</ul>
				</div>
			{/if}

			<div class="space-y-2">
				<div class="flex flex-wrap items-center gap-2">
					{#if validInput?.run_in_background}
						<span
							class="rounded-full bg-muted px-2 py-0.5 text-muted-foreground text-xs"
							>Background</span
						>
					{/if}
					{#if validInput?.timeout !== undefined}
						<span class="text-muted-foreground text-xs"
							>{validInput.timeout}ms</span
						>
					{/if}
				</div>

				{#if validInput?.description}
					<p class="italic text-muted-foreground text-sm">
						{validInput.description}
					</p>
				{/if}

				<div
					class="overflow-hidden rounded-md border bg-muted/50 font-mono text-sm"
				>
					<div
						class="flex items-center justify-between border-border border-b bg-muted/30 px-3 py-2 text-muted-foreground text-xs"
					>
						<span>$</span>
						<Button
							aria-label="Copy command"
							class="h-6 gap-1 px-2 font-sans text-xs"
							onclick={() => copyToClipboard("command", validInput?.command)}
							size="xs"
							title="Copy command"
							variant="ghost"
						>
							{#if copiedTarget === "command"}
								<CheckIcon class="size-3" />
								Copied
							{:else}
								<CopyIcon class="size-3" />
								Copy
							{/if}
						</Button>
					</div>
					<div class="px-3 py-2">
						<code class="break-all text-foreground">{validInput?.command}</code>
					</div>
				</div>
			</div>

			<div class="space-y-2">
				<div class="flex items-center gap-2">
					{#if executionError}
						<CircleXIcon class="size-4 text-destructive" />
					{:else if validOutput?.exitCode === 0 || validOutput?.exitCode === undefined}
						<CircleCheckIcon class="size-4 text-green-600" />
					{:else}
						<CircleXIcon class="size-4 text-yellow-600" />
					{/if}
					<h4
						class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
					>
						{executionError ? "Error" : "Output"}
					</h4>
					{#if validOutput?.exitCode !== undefined}
						<span
							class={cn(
								"rounded px-2 py-0.5 font-mono text-xs",
								validOutput.exitCode === 0
									? "bg-green-100 text-green-700"
									: "bg-yellow-100 text-yellow-700",
							)}
						>
							exit {validOutput.exitCode}
						</span>
					{/if}
					{#if parsedStdout.isTruncated}
						<span
							class="rounded-full bg-muted px-2 py-0.5 text-muted-foreground text-xs"
						>
							Truncated
						</span>
					{/if}
				</div>

				{#if stdout}
					<div class="rounded-md border bg-muted/30">
						<div
							class="flex items-center justify-between border-b px-3 py-2 text-muted-foreground text-xs uppercase tracking-wide"
						>
							<span>Stdout</span>
							<div class="flex items-center gap-2">
								<span>{displayedLineCount} lines</span>
								<Button
									aria-label="Copy stdout"
									class="h-6 gap-1 px-2 text-xs normal-case tracking-normal"
									onclick={() => copyToClipboard("stdout", stdout)}
									size="xs"
									title="Copy stdout"
									variant="ghost"
								>
									{#if copiedTarget === "stdout"}
										<CheckIcon class="size-3" />
										Copied
									{:else}
										<CopyIcon class="size-3" />
										Copy
									{/if}
								</Button>
							</div>
						</div>
						{#if parsedStdout.isTruncated}
							<div
								class="border-b px-3 py-2 text-[11px] text-muted-foreground normal-case tracking-normal"
							>
								Output truncated{#if parsedStdout.truncationFilePath}
									— full output written to {shortenPath(
										parsedStdout.truncationFilePath,
									)}
								{/if}
							</div>
						{/if}
						{#if hasParsedStdoutLines}
							<div
								class="overflow-x-auto p-3 font-mono text-xs text-foreground"
							>
								<div class="grid min-w-max grid-cols-[auto_1fr] gap-x-3">
									{#each parsedStdout.lines as line}
										<div
											class="select-none text-muted-foreground/60 text-right"
										>
											{line.lineNumber}
										</div>
										<div class="whitespace-pre-wrap break-words">
											{line.text || " "}
										</div>
									{/each}
								</div>
							</div>
						{:else}
							<pre
								class="overflow-x-auto whitespace-pre-wrap break-words p-3 font-mono text-xs text-foreground"><code
									>{stdout}</code
								></pre>
						{/if}
					</div>
				{:else if !executionError && outputValidation?.success}
					<div
						class="rounded-md border border-dashed px-3 py-2 text-muted-foreground text-sm"
					>
						Command completed without output.
					</div>
				{/if}

				{#if validOutput?.stderr && !toolPart.errorText}
					<div class="rounded-md border border-yellow-200 bg-yellow-50 p-3">
						<h5
							class="mb-2 font-medium text-yellow-800 text-xs uppercase tracking-wide"
						>
							Stderr
						</h5>
						<pre
							class="whitespace-pre-wrap break-words font-mono text-xs text-yellow-700"><code
								>{validOutput.stderr}</code
							></pre>
					</div>
				{/if}

				{#if toolPart.errorText}
					<div
						class="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-destructive text-sm"
					>
						{toolPart.errorText}
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
		</div>
	{/if}
</ToolContent>
