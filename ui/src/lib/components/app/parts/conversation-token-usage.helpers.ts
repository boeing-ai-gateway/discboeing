import type {
	TokenPrices,
	TokenUsage,
	ThreadTokenUsageDetails,
	TokenUsageInfo,
} from "$lib/api-types";

export type TokenUsageLaneKey =
	| "inputNoCache"
	| "cacheRead"
	| "cacheWrite"
	| "outputText"
	| "outputReasoning";

export type TokenUsageLane = {
	key: TokenUsageLaneKey;
	label: string;
	description: string;
	colorClass: string;
	total: number;
	maxStepValue: number;
};

export type TokenUsageChartStep = {
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

export type TokenUsageChartTurn = {
	id: string;
	label: string;
	subtitle: string;
	stepCount: number;
	values: Record<TokenUsageLaneKey, number>;
	total: number;
};

export type TokenUsageChart = {
	lanes: TokenUsageLane[];
	turns: TokenUsageChartTurn[];
	steps: TokenUsageChartStep[];
	maxLaneTotal: number;
};

export type TokenUsageChartColumn = {
	id: string;
	turnId: string;
	type: "turn" | "step";
	turnLabel: string;
	label: string;
	toolCalls: string[];
	values: Record<TokenUsageLaneKey, number>;
	total: number;
};

export type TokenUsageChartRenderTurn = TokenUsageChartTurn & {
	columnCount: number;
	expanded: boolean;
};

export type TokenUsageChartRender = {
	lanes: TokenUsageLane[];
	turns: TokenUsageChartRenderTurn[];
	columns: TokenUsageChartColumn[];
	maxLaneTotal: number;
};

export function usageTotal(usage: TokenUsage | undefined): number {
	return (usage?.inputTokens?.total ?? 0) + (usage?.outputTokens?.total ?? 0);
}

export function formatRounded(value: number): string {
	return Number.isInteger(value) ? value.toString() : value.toFixed(1);
}

export function formatTokenCount(value: number): string {
	if (value >= 1_000_000) {
		return `${formatRounded(value / 1_000_000)}M`;
	}
	if (value >= 1_000) {
		return `${formatRounded(value / 1_000)}K`;
	}
	return `${value}`;
}

export function formatFullTokenCount(value: number): string {
	return new Intl.NumberFormat().format(value);
}

export function formatPercent(value: number): string {
	return value < 1 ? "<1%" : `${Math.round(value)}%`;
}

export function formatTokenPrice(value: number | undefined): string | null {
	if (!value) {
		return null;
	}
	return `$${value.toFixed(value >= 1 ? 2 : 4)} / 1M`;
}

export function estimateUsageCost(
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

export function formatEstimatedCost(value: number | null): string | null {
	if (value === null) {
		return null;
	}
	if (value > 0 && value < 0.0001) {
		return "<$0.0001";
	}
	return `$${value.toFixed(value < 0.01 ? 4 : 2)}`;
}

export function tokenUsageRows(usage: TokenUsage | undefined) {
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

export function visibleTokenUsageRows(
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

export function buildTokenUsageSummary(usage: TokenUsageInfo | undefined) {
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

export function visibleUsageRows(usage: TokenUsage | undefined) {
	return tokenUsageRows(usage).filter((row) => row.value > 0);
}

const tokenUsageLaneDefinitions: Array<
	Omit<TokenUsageLane, "total" | "maxStepValue">
> = [
	{
		key: "inputNoCache",
		label: "Input",
		description: "Prompt tokens that were not served from cache",
		colorClass: "bg-chart-1",
	},
	{
		key: "cacheRead",
		label: "Cache read",
		description: "Input tokens read from provider cache",
		colorClass: "bg-chart-2",
	},
	{
		key: "cacheWrite",
		label: "Cache write",
		description: "Input tokens written to provider cache",
		colorClass: "bg-chart-3",
	},
	{
		key: "outputText",
		label: "Output text",
		description: "Visible assistant output tokens",
		colorClass: "bg-chart-4",
	},
	{
		key: "outputReasoning",
		label: "Reasoning",
		description: "Reasoning output tokens",
		colorClass: "bg-chart-5",
	},
];

export function tokenLaneValues(
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

export function tokenUsageChartGridColumns(stepCount: number): string {
	return `8.5rem repeat(${stepCount}, minmax(4.75rem, 1fr)) 6rem`;
}

export function tokenUsageBarPercent(value: number, maxValue: number): number {
	if (value <= 0 || maxValue <= 0) {
		return 0;
	}
	return Math.max(4, (value / maxValue) * 100);
}

export function tokenUsageCellTitle(
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

export function buildTokenUsageChartRender(
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

export function buildTokenUsageChart(
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

export function formatUsageDate(value: string | undefined): string | null {
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

export function tokenUsageTurnSubtitle(
	model: string | undefined,
	reasoning: string | undefined,
	serviceTier: string | undefined,
): string {
	return [model, reasoning ? `reasoning ${reasoning}` : null, serviceTier]
		.filter((part): part is string => Boolean(part))
		.join(" · ");
}
