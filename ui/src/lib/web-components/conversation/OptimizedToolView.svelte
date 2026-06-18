<script lang="ts">
	import BotIcon from "@lucide/svelte/icons/bot";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import CheckIcon from "@lucide/svelte/icons/check";
	import CheckCircleIcon from "@lucide/svelte/icons/circle-check";
	import ClipboardIcon from "@lucide/svelte/icons/clipboard";
	import ClockIcon from "@lucide/svelte/icons/clock";
	import CodeIcon from "@lucide/svelte/icons/code";
	import FilePenIcon from "@lucide/svelte/icons/file-pen";
	import FileTextIcon from "@lucide/svelte/icons/file-text";
	import FolderSearchIcon from "@lucide/svelte/icons/folder-search";
	import GitCommitHorizontalIcon from "@lucide/svelte/icons/git-commit-horizontal";
	import GlobeIcon from "@lucide/svelte/icons/globe";
	import KeyRoundIcon from "@lucide/svelte/icons/key-round";
	import ListTodoIcon from "@lucide/svelte/icons/list-todo";
	import LoaderCircleIcon from "@lucide/svelte/icons/loader-circle";
	import PencilIcon from "@lucide/svelte/icons/pencil";
	import SearchIcon from "@lucide/svelte/icons/search";
	import TerminalIcon from "@lucide/svelte/icons/terminal";
	import WrenchIcon from "@lucide/svelte/icons/wrench";
	import XCircleIcon from "@lucide/svelte/icons/circle-x";
	import type { Component } from "svelte";

	type Props = {
		toolKind: string;
		partId?: string;
		callId?: string;
		state?: string;
		title?: string;
		input?: string;
		output?: string;
		errorText?: string;
		defaultOpen?: boolean;
	};

	let {
		toolKind,
		partId,
		callId,
		state: toolState = "input-available",
		title,
		input = "",
		output = "",
		errorText = "",
		defaultOpen = false,
	}: Props = $props();

	let open = $derived(defaultOpen);
	let rawOpen = $state(false);
	let copiedTarget = $state<"raw" | "body" | `summary:${string}` | null>(null);
	let copyTimeout = $state<ReturnType<typeof setTimeout> | null>(null);

	function parseValue(value: string): unknown {
		if (!value.trim()) {
			return undefined;
		}
		try {
			return JSON.parse(value);
		} catch {
			return value;
		}
	}

	function valueRecord(value: unknown): Record<string, unknown> {
		return value && typeof value === "object" && !Array.isArray(value)
			? (value as Record<string, unknown>)
			: {};
	}

	function stringValue(value: unknown): string {
		return typeof value === "string" ? value : "";
	}

	function numberValue(value: unknown): number | undefined {
		return typeof value === "number" ? value : undefined;
	}

	function basename(path: string): string {
		return path.split(/[\\/]/).filter(Boolean).pop() || path;
	}

	function shortenPath(path: string): string {
		if (path.length <= 80) {
			return path;
		}
		const start = path.slice(0, 24);
		const end = path.slice(-48);
		return `${start}…${end}`;
	}

	function truncate(value: string, length = 80): string {
		return value.length > length ? `${value.slice(0, length)}…` : value;
	}

	function lineCount(value: string): number {
		if (!value) {
			return 0;
		}
		return value.split(/\r?\n/).length;
	}

	function formatValue(value: unknown): string {
		if (value === undefined || value === null || value === "") {
			return "";
		}
		if (typeof value === "string") {
			return value;
		}
		try {
			return JSON.stringify(value, null, 2);
		} catch {
			return String(value);
		}
	}

	function outputText(value: unknown): string {
		const record = valueRecord(value);
		const candidates = [
			record.output,
			record.stdout,
			record.content,
			Array.isArray(record.lines) ? record.lines.join("\n") : undefined,
			record.result,
			record.error,
		];
		for (const candidate of candidates) {
			if (typeof candidate === "string" && candidate.length > 0) {
				return candidate;
			}
		}
		return formatValue(value);
	}

	const inputValue = $derived.by(() => parseValue(input));
	const outputValue = $derived.by(() => parseValue(output));
	const inputRecord = $derived.by(() => valueRecord(inputValue));
	const filePath = $derived.by(() => stringValue(inputRecord.file_path));
	const pattern = $derived.by(() => stringValue(inputRecord.pattern));
	const command = $derived.by(() => stringValue(inputRecord.command));
	const description = $derived.by(() => stringValue(inputRecord.description));
	const displayTitle = $derived.by(() => title || summarizeTitle());
	const statusMeta = $derived.by(() => statusFor(toolState));
	const ToolIcon = $derived.by(() => iconFor(toolKind));
	const rawPayload = $derived.by(() =>
		JSON.stringify(
			{
				toolName: toolKind,
				partId,
				callId,
				state: toolState,
				input: inputValue,
				output: outputValue,
				errorText: errorText || undefined,
			},
			null,
			2,
		),
	);
	const summaryRows = $derived.by(() => buildSummaryRows());
	const bodyText = $derived.by(() => buildBodyText());
	const hasBodyText = $derived.by(() => bodyText.trim().length > 0);

	type IconComponent = Component<{
		class?: string;
		"aria-hidden"?: boolean | "true" | "false";
	}>;

	function statusFor(state: string): {
		label: string;
		tone: "running" | "approval" | "success" | "error" | "muted";
		Icon: IconComponent;
		spinning?: boolean;
	} {
		switch (state) {
			case "input-streaming":
				return {
					label: "Preparing",
					tone: "running",
					Icon: LoaderCircleIcon,
					spinning: true,
				};
			case "input-available":
				return {
					label: "Running",
					tone: "running",
					Icon: LoaderCircleIcon,
					spinning: true,
				};
			case "approval-requested":
				return {
					label: "Awaiting Approval",
					tone: "approval",
					Icon: ClockIcon,
				};
			case "approval-responded":
				return { label: "Responded", tone: "success", Icon: CheckCircleIcon };
			case "output-available":
				return { label: "Completed", tone: "success", Icon: CheckCircleIcon };
			case "output-error":
				return { label: "Error", tone: "error", Icon: XCircleIcon };
			case "output-denied":
				return { label: "Denied", tone: "error", Icon: XCircleIcon };
			default:
				return { label: state || "Running", tone: "muted", Icon: ClockIcon };
		}
	}

	function iconFor(tool: string): IconComponent {
		switch (tool) {
			case "Bash":
			case "PowerShell":
				return TerminalIcon;
			case "Read":
			case "read":
				return FileTextIcon;
			case "Write":
				return FilePenIcon;
			case "Edit":
				return PencilIcon;
			case "Grep":
				return SearchIcon;
			case "Glob":
				return FolderSearchIcon;
			case "WebSearch":
			case "WebFetch":
				return GlobeIcon;
			case "TodoWrite":
				return ListTodoIcon;
			case "Task":
				return BotIcon;
			case "Skill":
				return WrenchIcon;
			case "RequestUserCredential":
				return KeyRoundIcon;
			case "RequestCommitPull":
				return GitCommitHorizontalIcon;
			case "apply_patch":
				return CodeIcon;
			default:
				return WrenchIcon;
		}
	}

	function summarizeTitle(): string {
		switch (toolKind) {
			case "Bash":
			case "PowerShell":
				return description || command || "Command";
			case "Read":
			case "read":
				return filePath ? basename(filePath) : "Reading file";
			case "Write":
				return filePath ? basename(filePath) : "Write file";
			case "Edit":
				return filePath ? basename(filePath) : "Edit file";
			case "Grep":
				return pattern ? `Search: ${truncate(pattern, 50)}` : "Search files";
			case "Glob":
				return pattern ? `Find: ${truncate(pattern, 50)}` : "Find files";
			case "WebSearch":
				return stringValue(inputRecord.query)
					? `Search: ${truncate(stringValue(inputRecord.query), 50)}`
					: "Web search";
			case "WebFetch":
				return stringValue(inputRecord.url)
					? `Fetch: ${truncate(stringValue(inputRecord.url), 50)}`
					: "Fetch page";
			case "TodoWrite": {
				const todos = inputRecord.todos;
				return Array.isArray(todos)
					? `Track: ${todos.length} ${todos.length === 1 ? "task" : "tasks"}`
					: "Track tasks";
			}
			case "Task":
				return stringValue(inputRecord.description)
					? `Launch: ${truncate(stringValue(inputRecord.description), 50)}`
					: "Launch task";
			case "Skill":
				return stringValue(inputRecord.skill)
					? `Run: ${stringValue(inputRecord.skill)}`
					: "Run skill";
			case "apply_patch":
				return "Apply patch";
			case "RequestUserCredential":
				return "Credential request";
			case "RequestCommitPull":
				return "Pull sandbox commit";
			default:
				return toolKind;
		}
	}

	function addRow(
		rows: Array<[string, string]>,
		label: string,
		value: unknown,
	) {
		const formatted = formatValue(value).trim();
		if (formatted) {
			rows.push([label, formatted]);
		}
	}

	function buildSummaryRows(): Array<[string, string]> {
		const rows: Array<[string, string]> = [];
		switch (toolKind) {
			case "Bash":
			case "PowerShell":
				addRow(rows, "Description", inputRecord.description);
				addRow(rows, "Command", inputRecord.command);
				break;
			case "Read":
			case "read":
				addRow(rows, "Path", filePath ? shortenPath(filePath) : "");
				addRow(rows, "Offset", numberValue(inputRecord.offset));
				addRow(rows, "Limit", numberValue(inputRecord.limit));
				addRow(rows, "Pages", inputRecord.pages);
				break;
			case "Write":
				addRow(rows, "Path", filePath ? shortenPath(filePath) : "");
				addRow(
					rows,
					"Content",
					`${formatValue(inputRecord.content).length} chars, ${lineCount(formatValue(inputRecord.content))} lines`,
				);
				break;
			case "Edit":
				addRow(rows, "Path", filePath ? shortenPath(filePath) : "");
				addRow(
					rows,
					"Mode",
					inputRecord.replace_all ? "Replace all" : "Single replace",
				);
				addRow(
					rows,
					"Old",
					`${lineCount(formatValue(inputRecord.old_string))} lines`,
				);
				addRow(
					rows,
					"New",
					`${lineCount(formatValue(inputRecord.new_string))} lines`,
				);
				break;
			case "Grep":
				addRow(rows, "Pattern", inputRecord.pattern);
				addRow(rows, "Path", inputRecord.path);
				addRow(rows, "Glob", inputRecord.glob);
				addRow(rows, "Mode", inputRecord.output_mode);
				break;
			case "Glob":
				addRow(rows, "Pattern", inputRecord.pattern);
				addRow(rows, "Path", inputRecord.path);
				break;
			case "WebSearch":
				addRow(rows, "Query", inputRecord.query);
				break;
			case "WebFetch":
				addRow(rows, "URL", inputRecord.url);
				break;
			case "TodoWrite": {
				const todos = inputRecord.todos;
				if (Array.isArray(todos)) {
					addRow(rows, "Todos", `${todos.length}`);
				}
				break;
			}
			case "Task":
				addRow(rows, "Description", inputRecord.description);
				addRow(rows, "Agent", inputRecord.subagent_type);
				break;
			case "Skill":
				addRow(rows, "Skill", inputRecord.skill);
				addRow(rows, "Args", inputRecord.args);
				break;
			case "RequestUserCredential": {
				const credentials = inputRecord.credentials;
				if (Array.isArray(credentials)) {
					addRow(rows, "Credentials", `${credentials.length}`);
				}
				break;
			}
			case "RequestCommitPull":
				addRow(rows, "Base commit", inputRecord.baseCommit);
				addRow(rows, "Notes", inputRecord.notes);
				break;
			case "apply_patch":
				addRow(rows, "Patch", `${lineCount(formatValue(inputValue))} lines`);
				break;
		}
		return rows;
	}

	function buildBodyText(): string {
		switch (toolKind) {
			case "Bash":
			case "PowerShell":
				return outputText(outputValue) || errorText;
			case "Read":
			case "read":
				return outputText(outputValue);
			case "Write":
				return (
					formatValue(inputRecord.content).slice(0, 2000) ||
					outputText(outputValue)
				);
			case "Edit":
				return [
					"--- old",
					formatValue(inputRecord.old_string),
					"+++ new",
					formatValue(inputRecord.new_string),
				]
					.filter(Boolean)
					.join("\n");
			case "TodoWrite": {
				const todos = inputRecord.todos;
				if (Array.isArray(todos)) {
					return todos
						.map((todo, index) => {
							const record = valueRecord(todo);
							return `${index + 1}. [${record.status ?? "pending"}] ${record.content ?? ""}`;
						})
						.join("\n");
				}
				return outputText(outputValue);
			}
			case "RequestUserCredential": {
				const credentials = inputRecord.credentials;
				if (Array.isArray(credentials)) {
					return credentials
						.map((credential, index) => {
							const record = valueRecord(credential);
							return `${index + 1}. ${record.name ?? record.envVar ?? "Credential"}\n${record.justification ?? ""}`;
						})
						.join("\n\n");
				}
				return outputText(outputValue);
			}
			case "WebSearch":
			case "WebFetch":
			case "Grep":
			case "Glob":
			case "Task":
			case "Skill":
			case "RequestCommitPull":
			case "apply_patch":
				return outputText(outputValue) || formatValue(inputValue);
			default:
				return outputText(outputValue) || formatValue(inputValue);
		}
	}

	async function copyToClipboard(
		target: "raw" | "body" | `summary:${string}`,
		text: string,
	) {
		if (!text || typeof navigator === "undefined" || !navigator.clipboard) {
			return;
		}
		await navigator.clipboard.writeText(text);
		copiedTarget = target;
		if (copyTimeout) {
			clearTimeout(copyTimeout);
		}
		copyTimeout = setTimeout(() => {
			copiedTarget = null;
		}, 1800);
	}

	$effect(() => {
		return () => {
			if (copyTimeout) {
				clearTimeout(copyTimeout);
			}
		};
	});
</script>

<div
	part="container"
	class="container"
	data-state={toolState}
	data-tool-name={toolKind}
>
	<div part="header" class="header">
		<button
			part="trigger"
			class="trigger"
			type="button"
			aria-expanded={open}
			onclick={() => (open = !open)}
		>
			<ChevronDownIcon
				class={open ? "chevron open" : "chevron"}
				aria-hidden="true"
			/>
			<ToolIcon class="tool-icon" aria-hidden="true" />
			<span part="title" class="title">{displayTitle}</span>
			<span part="status" class={`status ${statusMeta.tone}`}>
				<statusMeta.Icon
					class={statusMeta.spinning ? "status-icon spinning" : "status-icon"}
					aria-hidden="true"
				/>
				{statusMeta.label}
			</span>
		</button>
		<button
			part="raw-toggle"
			class:active={rawOpen}
			class="raw-toggle"
			type="button"
			aria-pressed={rawOpen}
			aria-label={rawOpen ? "Show optimized view" : "Show raw view"}
			title={rawOpen ? "Show optimized view" : "Show raw view"}
			onclick={() => (rawOpen = !rawOpen)}
		>
			<CodeIcon class="raw-icon" aria-hidden="true" />
		</button>
	</div>

	{#if open}
		{#if rawOpen}
			<div part="raw" class="raw-panel">
				<div class="panel-header">
					<span>Raw</span>
					<button
						type="button"
						class="copy-button"
						aria-label="Copy raw tool payload"
						title="Copy raw tool payload"
						onclick={() => copyToClipboard("raw", rawPayload)}
					>
						{#if copiedTarget === "raw"}
							<CheckIcon class="copy-icon" aria-hidden="true" />
							Copied
						{:else}
							<ClipboardIcon class="copy-icon" aria-hidden="true" />
							Copy
						{/if}
					</button>
				</div>
				<pre class="raw">{rawPayload}</pre>
			</div>
		{:else}
			<div part="content" class="content">
				{#if summaryRows.length > 0}
					<div part="summary" class="summary">
						{#each summaryRows as [label, value] (`${label}:${value}`)}
							<div class="summary-row">
								<span class="summary-label">{label}</span>
								<code>{value}</code>
								<button
									type="button"
									class="copy-button inline"
									aria-label={`Copy ${label.toLowerCase()}`}
									title={`Copy ${label.toLowerCase()}`}
									onclick={() => copyToClipboard(`summary:${label}`, value)}
								>
									{#if copiedTarget === `summary:${label}`}
										<CheckIcon class="copy-icon" aria-hidden="true" />
										<span class="sr-only">Copied</span>
									{:else}
										<ClipboardIcon class="copy-icon" aria-hidden="true" />
										<span class="sr-only">Copy</span>
									{/if}
								</button>
							</div>
						{/each}
					</div>
				{/if}

				{#if hasBodyText}
					<div part="body" class="body-panel">
						<div class="body-header">
							<span>{errorText ? "Error" : "Output"}</span>
							<div class="panel-actions">
								<span
									>{lineCount(bodyText)}
									{lineCount(bodyText) === 1 ? "line" : "lines"}</span
								>
								<button
									type="button"
									class="copy-button"
									aria-label="Copy output"
									title="Copy output"
									onclick={() => copyToClipboard("body", bodyText)}
								>
									{#if copiedTarget === "body"}
										<CheckIcon class="copy-icon" aria-hidden="true" />
										Copied
									{:else}
										<ClipboardIcon class="copy-icon" aria-hidden="true" />
										Copy
									{/if}
								</button>
							</div>
						</div>
						<pre class="body">{bodyText}</pre>
					</div>
				{/if}

				{#if errorText}
					<div part="error" class="error">{errorText}</div>
				{/if}
			</div>
		{/if}
	{/if}
</div>

<style>
	:host {
		display: block;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-family: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
	}

	:global(.container) {
		display: flex;
		width: 100%;
		min-width: 0;
		flex-direction: column;
		gap: 0;
		margin-bottom: 1rem;
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
	}

	:global(.header) {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		min-width: 0;
		padding: 1rem 1rem 0;
	}

	:global(.trigger) {
		display: flex;
		min-width: 0;
		flex: 1 1 auto;
		align-items: center;
		gap: 0.5rem;
		border: 0;
		background: transparent;
		padding: 0;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	:global(.trigger:hover) {
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.title) {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 0.875rem;
		font-weight: 500;
		line-height: 1.25rem;
	}

	:global(.status) {
		display: inline-flex;
		flex: 0 0 auto;
		align-items: center;
		gap: 0.25rem;
		border-radius: 999px;
		background: var(
			--disco-conversation-muted,
			var(--disco-muted, var(--muted, #f3f4f6))
		);
		padding: 0.125rem 0.5rem;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-size: 0.75rem;
		font-weight: 500;
		line-height: 1rem;
	}

	:global(.status.success) {
		background: color-mix(in oklab, #16a34a 12%, transparent);
		color: #15803d;
	}

	:global(.status.running) {
		background: color-mix(in oklab, #2563eb 12%, transparent);
		color: #1d4ed8;
	}

	:global(.status.approval) {
		background: color-mix(in oklab, #d97706 14%, transparent);
		color: #b45309;
	}

	:global(.status.error) {
		background: color-mix(in oklab, #dc2626 12%, transparent);
		color: #b91c1c;
	}

	:global(.raw-toggle) {
		display: inline-flex;
		width: 1.75rem;
		height: 1.75rem;
		align-items: center;
		justify-content: center;
		border: 0;
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.375rem);
		background: transparent;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		cursor: pointer;
	}

	:global(.raw-toggle:hover),
	:global(.raw-toggle:focus-visible),
	:global(.raw-toggle.active) {
		background: var(
			--disco-conversation-muted,
			var(--disco-muted, var(--muted, #f3f4f6))
		);
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.chevron),
	:global(.tool-icon),
	:global(.status-icon),
	:global(.raw-icon),
	:global(.copy-icon) {
		width: 1rem;
		height: 1rem;
		flex: 0 0 auto;
	}

	:global(.chevron) {
		transition: transform 150ms ease;
	}

	:global(.chevron.open) {
		transform: rotate(180deg);
	}

	:global(.tool-icon) {
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
	}

	:global(.status-icon.spinning) {
		animation: disco-tool-spin 1s linear infinite;
	}

	:global(.content) {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		margin: 0;
		padding: 0.75rem 1rem 1rem;
	}

	:global(.summary) {
		display: grid;
		gap: 0.5rem;
	}

	:global(.summary-row) {
		display: flex;
		min-width: 0;
		align-items: baseline;
		gap: 0.5rem;
		font-size: 0.875rem;
		line-height: 1.25rem;
	}

	:global(.summary-label) {
		flex: 0 0 auto;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.75rem;
	}

	:global(code),
	:global(.body),
	:global(.raw) {
		font-family: var(
			--disco-conversation-font-mono,
			var(--disco-font-mono, var(--font-mono, monospace))
		);
	}

	:global(code) {
		min-width: 0;
		overflow-wrap: anywhere;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-size: 0.8125rem;
		white-space: pre-wrap;
	}

	:global(.copy-button) {
		display: inline-flex;
		height: 1.5rem;
		align-items: center;
		justify-content: center;
		gap: 0.25rem;
		border: 0;
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.5rem);
		background: transparent;
		padding: 0 0.375rem;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-family: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
		font-size: 0.75rem;
		line-height: 1rem;
		cursor: pointer;
	}

	:global(.copy-button:hover),
	:global(.copy-button:focus-visible) {
		background: var(
			--disco-conversation-background,
			var(--disco-background, var(--background, #fff))
		);
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.copy-button.inline) {
		width: 1.5rem;
		flex: 0 0 auto;
		padding: 0;
		opacity: 0;
		transition: opacity 120ms ease;
	}

	:global(.summary-row:hover .copy-button.inline),
	:global(.copy-button.inline:focus-visible) {
		opacity: 1;
	}

	:global(.raw-panel),
	:global(.body-panel) {
		overflow: hidden;
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.375rem);
		background: var(
			--disco-conversation-muted,
			var(--disco-muted, var(--muted, #f3f4f6))
		);
	}

	:global(.raw-panel) {
		margin: 0.75rem 1rem 1rem;
	}

	:global(.panel-header),
	:global(.body-header) {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		border-bottom: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		padding: 0.5rem 0.75rem;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.75rem;
		line-height: 1rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	:global(.panel-actions) {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
	}

	:global(.raw),
	:global(.body) {
		max-height: 24rem;
		overflow: auto;
		margin: 0;
		padding: 0.75rem;
		border: 0;
		background: transparent;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-size: 0.75rem;
		line-height: 1rem;
		white-space: pre-wrap;
	}

	:global(.error) {
		border: 1px solid color-mix(in oklab, #dc2626 35%, transparent);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.375rem);
		background: color-mix(in oklab, #dc2626 10%, transparent);
		padding: 0.75rem;
		color: #dc2626;
		font-size: 0.875rem;
		line-height: 1.25rem;
	}

	:global(.sr-only) {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}

	@keyframes disco-tool-spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
