<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import {
		clearDebugLogs,
		installDebugInstrumentation,
	} from "$lib/context/debug";
	import { useContext } from "$lib/context";

	type DebugTab =
		| "subscriptions"
		| "events"
		| "logs"
		| "commands"
		| "network"
		| "changes"
		| "state";
	type Point = {
		x: number;
		y: number;
	};
	type JsonRow = {
		path: string;
		key: string;
		depth: number;
		expandable: boolean;
		expanded: boolean;
		value: unknown;
	};

	const PANEL_WIDTH = 544;
	const PANEL_HEIGHT = 512;
	const LAUNCHER_SIZE = 44;
	const PANEL_GAP = 12;
	const VIEWPORT_PADDING = 12;

	const context = useContext();
	const tabs: DebugTab[] = [
		"subscriptions",
		"events",
		"logs",
		"commands",
		"network",
		"changes",
		"state",
	];
	let open = $state(false);
	let tab = $state<DebugTab>("subscriptions");
	let panelElement = $state<HTMLDivElement | null>(null);
	let panelPosition = $state<Point>({
		x: VIEWPORT_PADDING,
		y: VIEWPORT_PADDING,
	});
	let searchQuery = $state("");
	let dragOffset = $state<Point | null>(null);
	let copiedState = $state(false);
	let copyStateTimeout: ReturnType<typeof setTimeout> | undefined;
	let expandedCommandIds = $state<Record<number, boolean>>({});
	let expandedEventIds = $state<Record<number, boolean>>({});
	let expandedLogIds = $state<Record<number, boolean>>({});
	let expandedRequestIds = $state<Record<number, boolean>>({});
	let expandedStateChangeIds = $state<Record<number, boolean>>({});
	let expandedSubscriptionIds = $state<Record<string, boolean>>({});
	let expandedJsonPaths = $state<Record<string, boolean>>({
		$: true,
		"$.data": true,
		"$.view": true,
	});

	const debug = $derived(context.view.app.debug);
	const subscriptions = $derived(
		Object.values(debug.subscriptions).sort((left, right) =>
			right.openedAt.localeCompare(left.openedAt),
		),
	);
	const activeCount = $derived(
		subscriptions.filter(
			(subscription) =>
				subscription.status === "active" || subscription.status === "opening",
		).length,
	);
	const runningCommands = $derived(
		debug.commands.filter((command) => command.status === "running").length,
	);
	const visibleLogs = $derived(
		debug.logs.filter((log) => log.kind === "console" || log.level === "error"),
	);
	const normalizedSearchQuery = $derived(searchQuery.trim().toLowerCase());
	const hasSearchQuery = $derived(normalizedSearchQuery.length > 0);
	const filteredSubscriptions = $derived(
		filterDebugEntries(subscriptions, subscriptionSearchText),
	);
	const filteredEvents = $derived(
		filterDebugEntries(debug.events, eventSearchText),
	);
	const filteredLogs = $derived(filterDebugEntries(visibleLogs, logSearchText));
	const filteredCommands = $derived(
		filterDebugEntries(debug.commands, commandSearchText),
	);
	const filteredRequests = $derived(
		filterDebugEntries(debug.requests, requestSearchText),
	);
	const filteredStateChanges = $derived(
		filterDebugEntries(debug.stateChanges, stateChangeSearchText),
	);
	const hasErrors = $derived(
		debug.logs.some((log) => log.level === "error") ||
			debug.commands.some((command) => command.status === "error") ||
			debug.requests.some((request) => request.status === "error"),
	);
	const stateSnapshotValue = $derived.by(() => ({
		view: context.view,
		data: context.data,
	}));
	const stateSnapshot = $derived.by(() => stringifyJson(stateSnapshotValue));
	const stateRows = $derived.by(() =>
		flattenJsonRows(stateSnapshotValue, "$", "root", 0),
	);
	const filteredStateRows = $derived(
		filterDebugEntries(stateRows, stateRowSearchText),
	);

	function formatTime(value: string | null) {
		if (!value) return "—";
		return new Date(value).toLocaleTimeString();
	}

	function formatCollapsedChangePath(path: string | undefined) {
		if (!path) return "state changed";
		const trimmedPath = path.replace(/^\$\./, "").replace(/\.\$$/, "");
		const parts = trimmedPath.split(".").filter(Boolean);
		if (parts.length <= 2) return trimmedPath;
		return parts.slice(-2).join(".");
	}

	function formatRequestPath(url: string) {
		try {
			const parsedUrl = new URL(url, window.location.origin);
			return `${parsedUrl.pathname}${parsedUrl.search}`;
		} catch {
			return url;
		}
	}

	function statusClass(status: string) {
		switch (status) {
			case "active":
			case "success":
				return "text-emerald-400";
			case "opening":
			case "running":
			case "warn":
				return "text-amber-300";
			case "error":
				return "text-destructive";
			default:
				return "text-muted-foreground";
		}
	}

	function tabHasErrors(item: DebugTab) {
		switch (item) {
			case "subscriptions":
				return subscriptions.some(
					(subscription) => subscription.status === "error",
				);
			case "logs":
				return visibleLogs.some((log) => log.level === "error");
			case "commands":
				return debug.commands.some((command) => command.status === "error");
			case "network":
				return debug.requests.some((request) => request.status === "error");
			default:
				return false;
		}
	}

	function filterDebugEntries<T>(
		entries: T[],
		getSearchText: (entry: T) => string,
	): T[] {
		if (!hasSearchQuery) return entries;
		return entries.filter((entry) =>
			getSearchText(entry).toLowerCase().includes(normalizedSearchQuery),
		);
	}

	function searchText(...values: unknown[]) {
		return values
			.filter((value) => value !== null && value !== undefined)
			.map((value) => String(value))
			.join(" ");
	}

	function subscriptionSearchText(
		subscription: (typeof subscriptions)[number],
	) {
		return searchText(
			subscription.label,
			subscription.stream,
			subscription.status,
			subscription.eventCount,
			subscription.lastEvent,
			subscription.openedAt,
			subscription.closedAt,
			subscription.lastEventAt,
		);
	}

	function eventSearchText(event: (typeof debug.events)[number]) {
		return searchText(
			event.direction,
			event.stream,
			event.type,
			event.event,
			event.label,
			event.payload,
			event.at,
		);
	}

	function logSearchText(log: (typeof visibleLogs)[number]) {
		return searchText(
			log.kind,
			log.level,
			log.message,
			log.detail,
			log.stack,
			log.at,
		);
	}

	function commandSearchText(command: (typeof debug.commands)[number]) {
		return searchText(
			command.name,
			command.status,
			command.args,
			command.callStack,
			command.error,
			command.durationMs,
			command.startedAt,
			command.finishedAt,
		);
	}

	function requestSearchText(request: (typeof debug.requests)[number]) {
		return searchText(
			request.method,
			request.url,
			formatRequestPath(request.url),
			request.status,
			request.statusCode,
			request.statusText,
			request.error,
			request.durationMs,
			request.startedAt,
			request.finishedAt,
		);
	}

	function stateChangeSearchText(entry: (typeof debug.stateChanges)[number]) {
		return searchText(
			entry.changeCount,
			entry.at,
			...entry.changes.flatMap((change) => [
				change.type,
				change.path,
				change.before,
				change.after,
			]),
		);
	}

	function stateRowSearchText(row: JsonRow) {
		return searchText(row.path, row.key, jsonValuePreview(row.value));
	}

	function toggleOpen() {
		if (!open) {
			resetPanelPosition();
		}
		open = !open;
	}

	function resetPanelPosition() {
		if (typeof window === "undefined") return;
		panelPosition = clampPanelPosition({
			x: VIEWPORT_PADDING,
			y:
				window.innerHeight -
				VIEWPORT_PADDING -
				LAUNCHER_SIZE -
				PANEL_GAP -
				PANEL_HEIGHT,
		});
	}

	function startDrag(event: MouseEvent) {
		if (event.button !== 0) return;

		dragOffset = {
			x: event.clientX - panelPosition.x,
			y: event.clientY - panelPosition.y,
		};
		window.addEventListener("mousemove", dragPanel);
		window.addEventListener("mouseup", stopDrag, { once: true });
	}

	function dragPanel(event: MouseEvent) {
		if (!dragOffset) return;
		panelPosition = clampPanelPosition({
			x: event.clientX - dragOffset.x,
			y: event.clientY - dragOffset.y,
		});
	}

	function stopDrag() {
		dragOffset = null;
		window.removeEventListener("mousemove", dragPanel);
	}

	function toggleJsonPath(path: string) {
		expandedJsonPaths[path] = !expandedJsonPaths[path];
	}

	function toggleEvent(eventId: number) {
		expandedEventIds[eventId] = !expandedEventIds[eventId];
	}

	async function copyStateJson() {
		try {
			await navigator.clipboard.writeText(stateSnapshot);
			copiedState = true;
			if (copyStateTimeout) {
				clearTimeout(copyStateTimeout);
			}
			copyStateTimeout = setTimeout(() => {
				copiedState = false;
			}, 1500);
		} catch {
			copiedState = false;
		}
	}

	function flattenJsonRows(
		value: unknown,
		path: string,
		key: string,
		depth: number,
	): JsonRow[] {
		const expandable = isExpandableJsonValue(value);
		const expanded = expandedJsonPaths[path] ?? false;
		const rows: JsonRow[] = [
			{
				path,
				key,
				depth,
				expandable,
				expanded,
				value,
			},
		];

		if (!expandable || !expanded) {
			return rows;
		}

		const entries = Array.isArray(value)
			? value.map((entry, index) => [String(index), entry] as const)
			: Object.entries(value as Record<string, unknown>);
		for (const [entryKey, entryValue] of entries) {
			rows.push(
				...flattenJsonRows(
					entryValue,
					`${path}.${escapeJsonPathPart(entryKey)}`,
					entryKey,
					depth + 1,
				),
			);
		}
		return rows;
	}

	function jsonValuePreview(value: unknown) {
		if (Array.isArray(value)) return `Array(${value.length})`;
		if (isRecord(value)) {
			const propertyCount = Object.keys(value).length;
			if (isCollectionState(value)) {
				return `{${propertyCount}} ${value.allIds.length} items`;
			}
			return `{${propertyCount}}`;
		}
		if (typeof value === "string") return JSON.stringify(value);
		if (value === null) return "null";
		return String(value);
	}

	function jsonValueClass(value: unknown) {
		if (typeof value === "string") return "text-emerald-400";
		if (typeof value === "number") return "text-sky-400";
		if (typeof value === "boolean") return "text-purple-400";
		if (value === null || value === undefined) return "text-muted-foreground";
		return "text-foreground";
	}

	function stringifyJson(value: unknown) {
		try {
			return JSON.stringify(value, jsonReplacer, "\t");
		} catch (error) {
			return JSON.stringify(
				{
					error: error instanceof Error ? error.message : String(error),
				},
				null,
				"\t",
			);
		}
	}

	function jsonReplacer(_key: string, value: unknown) {
		if (typeof value === "function") return "[Function]";
		if (value instanceof Event) return `[${value.constructor.name}]`;
		return value;
	}

	function isExpandableJsonValue(value: unknown) {
		return Array.isArray(value) || isRecord(value);
	}

	function isRecord(value: unknown): value is Record<string, unknown> {
		return typeof value === "object" && value !== null && !Array.isArray(value);
	}

	function isCollectionState(
		value: Record<string, unknown>,
	): value is Record<string, unknown> & { allIds: unknown[] } {
		return Array.isArray(value.allIds) && isRecord(value.byId);
	}

	function escapeJsonPathPart(value: string) {
		return value.replaceAll("\\", "\\\\").replaceAll(".", "\\.");
	}

	function clampPanelPosition(position: Point): Point {
		if (typeof window === "undefined") return position;
		const width = panelElement?.offsetWidth ?? PANEL_WIDTH;
		const height = panelElement?.offsetHeight ?? PANEL_HEIGHT;
		return {
			x: Math.min(
				Math.max(VIEWPORT_PADDING, position.x),
				Math.max(
					VIEWPORT_PADDING,
					window.innerWidth - width - VIEWPORT_PADDING,
				),
			),
			y: Math.min(
				Math.max(VIEWPORT_PADDING, position.y),
				Math.max(
					VIEWPORT_PADDING,
					window.innerHeight - height - VIEWPORT_PADDING,
				),
			),
		};
	}

	onDestroy(() => {
		window.removeEventListener("mousemove", dragPanel);
		if (copyStateTimeout) {
			clearTimeout(copyStateTimeout);
		}
	});

	onMount(() => installDebugInstrumentation(context));
</script>

{#if debug.enabled}
	<div class="text-xs">
		{#if open}
			<div
				bind:this={panelElement}
				class="fixed z-50 flex h-[32rem] w-[34rem] max-w-[calc(100vw-1.5rem)] flex-col overflow-hidden rounded-xl border border-border bg-background/95 shadow-2xl backdrop-blur"
				style={`left: ${panelPosition.x}px; top: ${panelPosition.y}px;`}
			>
				<div
					class="flex items-center justify-between border-b border-border px-3 py-2"
				>
					<button
						type="button"
						class="cursor-move touch-none select-none text-left"
						aria-label="Drag context debug panel"
						onmousedown={startDrag}
					>
						<div class="font-semibold">Context debug</div>
						<div class="text-muted-foreground">
							{activeCount} active subscriptions · {runningCommands} running commands
							· {debug.events.length} events · {debug.requests.length} requests ·
							{debug.stateChanges.length} state changes
						</div>
					</button>
					<div class="flex items-center gap-2">
						<button
							type="button"
							class="rounded border border-border px-2 py-1 text-muted-foreground hover:bg-tree-hover hover:text-foreground"
							onclick={() => clearDebugLogs(context)}
						>
							Clear
						</button>
						<button
							type="button"
							class="rounded border border-border px-2 py-1 text-muted-foreground hover:bg-tree-hover hover:text-foreground"
							onclick={() => (open = false)}
						>
							Close
						</button>
					</div>
				</div>

				<div class="flex border-b border-border p-1">
					{#each tabs as item (item)}
						<button
							type="button"
							class={`rounded px-3 py-1 capitalize ${
								tab === item
									? "bg-tree-hover text-foreground"
									: "text-muted-foreground hover:text-foreground"
							}`}
							onclick={() => (tab = item)}
						>
							<span class="inline-flex items-center gap-1.5">
								<span>{item}</span>
								{#if tabHasErrors(item)}
									<span
										class="inline-block size-1.5 rounded-full bg-destructive"
										aria-label={`${item} has errors`}
									></span>
								{/if}
							</span>
						</button>
					{/each}
				</div>

				<div class="border-b border-border p-2">
					<input
						type="search"
						class="w-full rounded border border-border bg-background px-2 py-1 font-sans text-xs text-foreground outline-none placeholder:text-muted-foreground focus:border-ring"
						placeholder="Search debug entries..."
						bind:value={searchQuery}
					/>
				</div>

				<div class="min-h-0 flex-1 overflow-auto p-3 font-mono">
					{#if tab === "subscriptions"}
						{#if subscriptions.length === 0}
							<div class="text-muted-foreground">
								No subscriptions recorded.
							</div>
						{:else if filteredSubscriptions.length === 0}
							<div class="text-muted-foreground">
								No subscriptions match your search.
							</div>
						{:else}
							<div class="space-y-2">
								{#each filteredSubscriptions as subscription (subscription.id)}
									<div class="rounded-lg border border-border px-2 py-1.5">
										<button
											type="button"
											class="flex w-full min-w-0 items-center gap-2 text-left"
											aria-expanded={expandedSubscriptionIds[subscription.id] ??
												false}
											onclick={() =>
												(expandedSubscriptionIds[subscription.id] =
													!expandedSubscriptionIds[subscription.id])}
										>
											<span class="w-3 shrink-0 text-muted-foreground">
												{expandedSubscriptionIds[subscription.id] ? "▾" : "▸"}
											</span>
											<span class="truncate font-semibold">
												{subscription.label}
											</span>
											<span
												class={`shrink-0 ${statusClass(subscription.status)}`}
											>
												{subscription.status}
											</span>
											<span class="shrink-0 text-muted-foreground">
												{subscription.eventCount} events
											</span>
											<span class="ml-auto shrink-0 text-muted-foreground">
												{formatTime(subscription.lastEventAt)}
											</span>
										</button>
										{#if expandedSubscriptionIds[subscription.id]}
											<div
												class="mt-2 grid grid-cols-2 gap-2 text-muted-foreground"
											>
												<div>opened: {formatTime(subscription.openedAt)}</div>
												<div>closed: {formatTime(subscription.closedAt)}</div>
												<div>last: {formatTime(subscription.lastEventAt)}</div>
												<div>stream: {subscription.stream}</div>
											</div>
											{#if subscription.lastEvent}
												<div class="mt-1 text-muted-foreground">
													last event: {subscription.lastEvent}
												</div>
											{/if}
										{/if}
									</div>
								{/each}
							</div>
						{/if}
					{:else if tab === "events"}
						{#if debug.events.length === 0}
							<div class="text-muted-foreground">
								No socket events recorded.
							</div>
						{:else if filteredEvents.length === 0}
							<div class="text-muted-foreground">
								No socket events match your search.
							</div>
						{:else}
							<div class="space-y-2">
								{#each filteredEvents as event (event.id)}
									<div class="rounded-lg border border-border px-2 py-1.5">
										<button
											type="button"
											class="flex w-full min-w-0 items-center gap-2 text-left"
											aria-expanded={expandedEventIds[event.id] ?? false}
											onclick={() => toggleEvent(event.id)}
										>
											<span class="w-3 shrink-0 text-muted-foreground">
												{expandedEventIds[event.id] ? "▾" : "▸"}
											</span>
											<span
												class={`shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold ${
													event.direction === "out"
														? "bg-sky-500/15 text-sky-300"
														: "bg-emerald-500/15 text-emerald-300"
												}`}
											>
												{event.direction === "out" ? "SEND" : "RECV"}
											</span>
											<span class="truncate font-semibold">{event.stream}</span>
											<span class="shrink-0 text-muted-foreground">
												{event.type}
											</span>
											{#if event.event}
												<span class="truncate text-amber-300"
													>{event.event}</span
												>
											{/if}
											<span class="ml-auto shrink-0 text-muted-foreground">
												{formatTime(event.at)}
											</span>
										</button>
										{#if expandedEventIds[event.id]}
											<pre
												class="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded border border-border bg-muted/20 p-2 text-muted-foreground">{event.payload}</pre>
										{/if}
									</div>
								{/each}
							</div>
						{/if}
					{:else if tab === "logs"}
						{#if visibleLogs.length === 0}
							<div class="text-muted-foreground">
								No console logs or errors.
							</div>
						{:else if filteredLogs.length === 0}
							<div class="text-muted-foreground">
								No logs match your search.
							</div>
						{:else}
							<div class="space-y-1">
								{#each filteredLogs as log (log.id)}
									<div class="rounded border border-border px-2 py-1.5">
										<button
											type="button"
											class="flex w-full min-w-0 items-center gap-2 text-left"
											aria-expanded={expandedLogIds[log.id] ?? false}
											onclick={() =>
												(expandedLogIds[log.id] = !expandedLogIds[log.id])}
										>
											<span class="w-3 shrink-0 text-muted-foreground">
												{expandedLogIds[log.id] ? "▾" : "▸"}
											</span>
											<span class={statusClass(log.level)}>
												[{log.kind}]
											</span>
											<span class="truncate">{log.message}</span>
											<span class="ml-auto shrink-0 text-muted-foreground">
												{formatTime(log.at)}
											</span>
										</button>
										{#if expandedLogIds[log.id] && (log.detail || log.stack)}
											<div class="mt-2 space-y-2">
												{#if log.detail}
													<div>
														<div
															class="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground"
														>
															detail
														</div>
														<pre
															class="max-h-48 overflow-auto whitespace-pre-wrap rounded border border-border bg-muted/20 p-2 text-muted-foreground">{log.detail}</pre>
													</div>
												{/if}
												{#if log.stack}
													<div>
														<div
															class="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground"
														>
															stack
														</div>
														<pre
															class="max-h-64 overflow-auto whitespace-pre-wrap rounded border border-border bg-muted/20 p-2 text-muted-foreground">{log.stack}</pre>
													</div>
												{/if}
											</div>
										{/if}
									</div>
								{/each}
							</div>
						{/if}
					{:else if tab === "commands"}
						{#if debug.commands.length === 0}
							<div class="text-muted-foreground">No commands recorded.</div>
						{:else if filteredCommands.length === 0}
							<div class="text-muted-foreground">
								No commands match your search.
							</div>
						{:else}
							<div class="space-y-2">
								{#each filteredCommands as command (command.id)}
									<div class="rounded-lg border border-border px-2 py-1.5">
										<button
											type="button"
											class="flex w-full min-w-0 items-center gap-2 text-left"
											aria-expanded={expandedCommandIds[command.id] ?? false}
											onclick={() =>
												(expandedCommandIds[command.id] =
													!expandedCommandIds[command.id])}
										>
											<span class="w-3 shrink-0 text-muted-foreground">
												{expandedCommandIds[command.id] ? "▾" : "▸"}
											</span>
											<span class="truncate font-semibold">{command.name}</span>
											<span class={`shrink-0 ${statusClass(command.status)}`}>
												{command.status}
											</span>
											<span class="shrink-0 text-muted-foreground">
												{command.durationMs ?? "—"}ms
											</span>
											<span class="ml-auto shrink-0 text-muted-foreground">
												{formatTime(command.startedAt)}
											</span>
										</button>
										{#if expandedCommandIds[command.id]}
											<div class="mt-2 space-y-2">
												<div>
													<div
														class="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground"
													>
														args
													</div>
													<pre
														class="max-h-48 overflow-auto whitespace-pre-wrap rounded border border-border bg-muted/20 p-2 text-muted-foreground">{command.args}</pre>
												</div>
												{#if command.callStack}
													<div>
														<div
															class="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground"
														>
															initiation stack
														</div>
														<pre
															class="max-h-64 overflow-auto whitespace-pre-wrap rounded border border-border bg-muted/20 p-2 text-muted-foreground">{command.callStack}</pre>
													</div>
												{/if}
											</div>
											{#if command.error}
												<div class="mt-1 text-destructive">{command.error}</div>
											{/if}
										{/if}
									</div>
								{/each}
							</div>
						{/if}
					{:else if tab === "network"}
						{#if debug.requests.length === 0}
							<div class="text-muted-foreground">No requests recorded.</div>
						{:else if filteredRequests.length === 0}
							<div class="text-muted-foreground">
								No requests match your search.
							</div>
						{:else}
							<div class="space-y-2">
								{#each filteredRequests as request (request.id)}
									<div class="rounded-lg border border-border px-2 py-1.5">
										<button
											type="button"
											class="flex w-full min-w-0 items-center gap-2 text-left"
											aria-expanded={expandedRequestIds[request.id] ?? false}
											onclick={() =>
												(expandedRequestIds[request.id] =
													!expandedRequestIds[request.id])}
										>
											<span class="w-3 shrink-0 text-muted-foreground">
												{expandedRequestIds[request.id] ? "▾" : "▸"}
											</span>
											<span class="shrink-0 font-semibold"
												>{request.method}</span
											>
											<span class="truncate" title={request.url}
												>{formatRequestPath(request.url)}</span
											>
											<span class={`shrink-0 ${statusClass(request.status)}`}>
												{request.statusCode ?? request.status}
											</span>
											<span class="shrink-0 text-muted-foreground">
												{request.durationMs ?? "—"}ms
											</span>
											<span class="ml-auto shrink-0 text-muted-foreground">
												{formatTime(request.startedAt)}
											</span>
										</button>
										{#if expandedRequestIds[request.id]}
											{#if request.statusText}
												<div class="mt-1 text-muted-foreground">
													{request.statusText}
												</div>
											{/if}
											{#if request.error}
												<div class="mt-1 text-destructive">{request.error}</div>
											{/if}
											<div class="mt-1 break-all text-muted-foreground">
												{request.method}
												{request.url}
											</div>
											<div class="mt-1 text-muted-foreground">
												finished: {formatTime(request.finishedAt)}
											</div>
										{/if}
									</div>
								{/each}
							</div>
						{/if}
					{:else if tab === "changes"}
						{#if debug.stateChanges.length === 0}
							<div class="text-muted-foreground">
								No state changes recorded yet.
							</div>
						{:else if filteredStateChanges.length === 0}
							<div class="text-muted-foreground">
								No state changes match your search.
							</div>
						{:else}
							<div class="space-y-2">
								{#each filteredStateChanges as entry (entry.id)}
									<div class="rounded-lg border border-border px-2 py-1.5">
										<button
											type="button"
											class="flex w-full min-w-0 items-center gap-2 text-left"
											aria-expanded={expandedStateChangeIds[entry.id] ?? false}
											onclick={() =>
												(expandedStateChangeIds[entry.id] =
													!expandedStateChangeIds[entry.id])}
										>
											<span class="w-3 shrink-0 text-muted-foreground">
												{expandedStateChangeIds[entry.id] ? "▾" : "▸"}
											</span>
											<span class="shrink-0 font-semibold">
												{entry.changeCount} paths
											</span>
											<span
												class="truncate text-left text-muted-foreground"
												dir="rtl"
												title={entry.changes[0]?.path ?? "state changed"}
											>
												{formatCollapsedChangePath(entry.changes[0]?.path)}
											</span>
											<span class="ml-auto shrink-0 text-muted-foreground">
												{formatTime(entry.at)}
											</span>
										</button>
										{#if expandedStateChangeIds[entry.id]}
											<div class="mt-2 space-y-2">
												{#each entry.changes as change (`${change.type}:${change.path}`)}
													<div
														class="rounded border border-border bg-muted/20 p-2"
													>
														<div class="flex items-center gap-2">
															<span
																class={`rounded px-1.5 py-0.5 text-[10px] font-semibold ${
																	change.type === "added"
																		? "bg-emerald-500/15 text-emerald-300"
																		: change.type === "removed"
																			? "bg-destructive/15 text-destructive"
																			: "bg-amber-500/15 text-amber-300"
																}`}
															>
																{change.type}
															</span>
															<span
																class="truncate text-left text-sky-300"
																dir="rtl"
																title={change.path}
															>
																{change.path}
															</span>
														</div>
														<div
															class="mt-2 grid grid-cols-2 gap-2 text-muted-foreground"
														>
															<div class="min-w-0">
																<div
																	class="mb-1 font-sans text-[10px] uppercase"
																>
																	before
																</div>
																<pre
																	class="max-h-32 overflow-auto whitespace-pre-wrap rounded border border-border bg-background/60 p-1">{change.before}</pre>
															</div>
															<div class="min-w-0">
																<div
																	class="mb-1 font-sans text-[10px] uppercase"
																>
																	after
																</div>
																<pre
																	class="max-h-32 overflow-auto whitespace-pre-wrap rounded border border-border bg-background/60 p-1">{change.after}</pre>
															</div>
														</div>
													</div>
												{/each}
											</div>
										{/if}
									</div>
								{/each}
							</div>
						{/if}
					{:else}
						<div class="mb-2 flex items-center justify-between gap-3">
							<div class="text-muted-foreground">
								{`{ view: context.view, data: context.data }`}
							</div>
							<button
								type="button"
								class="rounded border border-border px-2 py-1 font-sans text-muted-foreground hover:bg-tree-hover hover:text-foreground"
								onclick={copyStateJson}
							>
								{copiedState ? "Copied" : "Copy JSON"}
							</button>
						</div>
						<div
							class="rounded-lg border border-border bg-muted/20 py-2 text-[11px]"
						>
							{#each filteredStateRows as row (row.path)}
								<div
									class="flex min-w-max items-start gap-1 px-2 leading-5 hover:bg-tree-hover"
									style={`padding-left: ${row.depth * 14 + 8}px;`}
								>
									{#if row.expandable}
										<button
											type="button"
											class="w-4 shrink-0 text-muted-foreground hover:text-foreground"
											aria-label={`${row.expanded ? "Collapse" : "Expand"} ${row.key}`}
											onclick={() => toggleJsonPath(row.path)}
										>
											{row.expanded ? "▾" : "▸"}
										</button>
									{:else}
										<span class="w-4 shrink-0"></span>
									{/if}
									<button
										type="button"
										class="min-w-0 text-left"
										onclick={() => row.expandable && toggleJsonPath(row.path)}
									>
										<span class="text-sky-300">{row.key}</span>
										<span class="text-muted-foreground">: </span>
										<span class={jsonValueClass(row.value)}
											>{jsonValuePreview(row.value)}</span
										>
									</button>
								</div>
							{/each}
							{#if hasSearchQuery && filteredStateRows.length === 0}
								<div class="px-2 text-muted-foreground">
									No state rows match your search.
								</div>
							{/if}
						</div>
					{/if}
				</div>
			</div>
		{/if}

		<button
			type="button"
			class="fixed bottom-3 left-3 z-50 flex h-11 w-11 items-center justify-center rounded-full border border-border bg-background/95 font-semibold shadow-lg backdrop-blur hover:bg-tree-hover"
			aria-label="Open context debug panel"
			onclick={toggleOpen}
		>
			<span class="relative flex h-3 w-3">
				<span
					class={`absolute inline-flex h-full w-full animate-ping rounded-full opacity-75 ${
						hasErrors ? "bg-destructive" : "bg-emerald-400"
					}`}
				></span>
				<span
					class={`relative inline-flex h-3 w-3 rounded-full ${
						hasErrors ? "bg-destructive" : "bg-emerald-500"
					}`}
				></span>
			</span>
		</button>
	</div>
{/if}
