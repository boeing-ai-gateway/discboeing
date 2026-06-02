<script lang="ts">
	import Maximize2Icon from "@lucide/svelte/icons/maximize-2";
	import Minimize2Icon from "@lucide/svelte/icons/minimize-2";
	import { Button } from "$lib/components/ui/button";
	import {
		Popover,
		PopoverContent,
		PopoverTrigger,
	} from "$lib/components/ui/popover";
	import * as Dialog from "$lib/components/ui/dialog";
	import type { ThreadTokenUsageDetails, TokenUsageInfo } from "$lib/api-types";
	import {
		buildTokenUsageChart,
		buildTokenUsageChartRender,
		buildTokenUsageSummary,
		estimateUsageCost,
		formatEstimatedCost,
		formatFullTokenCount,
		formatTokenCount,
		formatUsageDate,
		tokenUsageBarPercent,
		tokenUsageCellTitle,
		tokenUsageChartGridColumns,
		tokenUsageRows,
		tokenUsageTurnSubtitle,
		usageTotal,
		visibleTokenUsageRows,
		visibleUsageRows,
	} from "$lib/components/app/parts/conversation-token-usage.helpers";

	type Props = {
		usage?: TokenUsageInfo;
		onLoadDetails: () => Promise<ThreadTokenUsageDetails>;
	};

	let { usage, onLoadDetails }: Props = $props();

	let summaryOpen = $state(false);
	let tokenUsageDetailsOpen = $state(false);
	let tokenUsageDetailsLoading = $state(false);
	let tokenUsageDetailsError = $state<string | null>(null);
	let tokenUsageDetails = $state<ThreadTokenUsageDetails | null>(null);
	let tokenUsageDetailsFullscreen = $state(false);
	let expandedTokenUsageTurns = $state<Record<string, boolean>>({});

	const tokenUsageSummary = $derived.by(() => buildTokenUsageSummary(usage));
	const tokenUsageChart = $derived.by(() =>
		buildTokenUsageChart(tokenUsageDetails),
	);
	const visibleTokenUsageChart = $derived.by(() =>
		buildTokenUsageChartRender(tokenUsageChart, expandedTokenUsageTurns),
	);

	function toggleTokenUsageTurn(turnId: string) {
		expandedTokenUsageTurns = {
			...expandedTokenUsageTurns,
			[turnId]: !expandedTokenUsageTurns[turnId],
		};
	}

	async function openTokenUsageDetails() {
		summaryOpen = false;
		tokenUsageDetailsOpen = true;
		tokenUsageDetailsLoading = true;
		tokenUsageDetailsError = null;
		try {
			tokenUsageDetails = await onLoadDetails();
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

{#if tokenUsageSummary}
	<Popover bind:open={summaryOpen}>
		<PopoverTrigger>
			<Button
				type="button"
				variant="ghost"
				size="xs"
				class="h-6 gap-1.5 px-2 text-xs font-normal text-muted-foreground"
				aria-label={tokenUsageSummary.ariaLabel}
			>
				<span>{tokenUsageSummary.primaryText}</span>
				<span class="text-border">/</span>
				<span>{tokenUsageSummary.totalText}</span>
			</Button>
		</PopoverTrigger>
		<PopoverContent
			align="start"
			class="w-max max-w-[calc(100vw-2rem)] p-3 text-xs"
		>
			<div class="grid grid-cols-[auto_auto_auto] gap-x-4 gap-y-1.5">
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
						<div>Model max {tokenUsageSummary.modelMaxTokensText} tokens</div>
					{/if}
					{#if tokenUsageSummary.maxOutputTokensText}
						<div>Max output {tokenUsageSummary.maxOutputTokensText} tokens</div>
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
		</PopoverContent>
	</Popover>

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
					Full token accounting for this thread, including the numbers used by
					the composer summary.
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
											tokenUsageDetails.summary?.lastStep?.inputTokens?.total ??
												0,
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
											Click a turn header to expand it into step columns. Bars
											are scaled within each lane.
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
											<div class="h-2 overflow-hidden rounded-full bg-muted">
												<div
													class={`h-full rounded-full ${lane.colorClass}`}
													style={`width: ${tokenUsageBarPercent(lane.total, visibleTokenUsageChart.maxLaneTotal)}%`}
												></div>
											</div>
											<div class="text-right tabular-nums text-foreground">
												{formatFullTokenCount(lane.total)}
											</div>
										</div>
									{/each}
								</div>

								<div class="overflow-x-auto rounded-md border border-border">
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
														onclick={() => toggleTokenUsageTurn(usageTurn.id)}
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
																class={`h-full rounded-sm ${lane.colorClass}`}
																style={`width: ${tokenUsageBarPercent(usageColumn.values[lane.key], lane.maxStepValue)}%`}
															></div>
														</div>
														<div
															class="mt-1 text-right tabular-nums text-muted-foreground"
														>
															{formatTokenCount(usageColumn.values[lane.key])}
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
													<div class="break-all text-xs text-muted-foreground">
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
													{formatFullTokenCount(usageTotal(usageTurn.usage))}
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
											<div class="mt-3 space-y-2 border-t border-border pt-3">
												{#each usageTurn.steps ?? [] as usageStep (usageStep.index)}
													<div class="rounded-md bg-muted/40 p-2 text-xs">
														<div
															class="mb-2 flex flex-wrap items-center justify-between gap-2"
														>
															<div class="font-medium text-foreground">
																Step {usageStep.index + 1}
															</div>
															<div class="tabular-nums text-muted-foreground">
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
																	<span class="tabular-nums text-foreground">
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
{/if}
