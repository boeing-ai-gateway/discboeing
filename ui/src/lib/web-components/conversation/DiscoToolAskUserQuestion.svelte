<svelte:options
	customElement={{
		tag: "disco-tool-ask-user-question",
		props: {
			partId: { attribute: "part-id", type: "String" },
			callId: { attribute: "call-id", type: "String" },
			state: { attribute: "state", type: "String" },
			approvalId: { attribute: "approval-id", type: "String" },
		},
	}}
/>

<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import ClockIcon from "@lucide/svelte/icons/clock";
	import CodeIcon from "@lucide/svelte/icons/code";
	import MessageSquareQuoteIcon from "@lucide/svelte/icons/message-square-quote";
	import { onMount } from "svelte";
	import { emitComposedEvent, getCustomElementHost } from "./dom";

	type OptionItem = {
		value: string;
		label: string;
		description: string;
	};

	type QuestionItem = {
		name: string;
		header: string;
		question: string;
		notes: string;
		multiSelect: boolean;
		options: OptionItem[];
	};

	type AnswerItem = {
		question: string;
		answer: string;
	};

	type Props = {
		partId?: string;
		callId?: string;
		state?: string;
		approvalId?: string;
	};

	const OTHER_LABEL = "__other__";
	const AUTO_ADVANCE_DELAY = 300;

	let {
		partId,
		callId = "",
		state: toolState = "input-available",
		approvalId,
	}: Props = $props();

	let root = $state<HTMLDivElement | null>(null);
	let questions = $state<QuestionItem[]>([]);
	let completedAnswers = $state<AnswerItem[]>([]);
	let currentStep = $state(0);
	let answers = $state<Record<string, string>>({});
	let otherSelected = $state<Record<string, boolean>>({});
	let otherText = $state<Record<string, string>>({});
	let autoAdvanceTimeout = $state<ReturnType<typeof setTimeout> | null>(null);
	let rawOpen = $state(false);

	const currentQuestion = $derived(questions[currentStep]);
	const notes = $derived(
		questions.find((question) => question.notes)?.notes ?? "",
	);
	const isPending = $derived(toolState === "approval-requested");
	const statusLabel = $derived.by(() => {
		switch (toolState) {
			case "input-streaming":
				return "Preparing";
			case "input-available":
				return "Running";
			case "approval-requested":
				return "Awaiting Approval";
			case "approval-responded":
				return "Responded";
			case "output-available":
				return "Completed";
			case "output-error":
				return "Errored";
			case "output-denied":
				return "Denied";
			default:
				return toolState || "Running";
		}
	});
	const rawPayload = $derived.by(() =>
		JSON.stringify(
			{
				toolName: "AskUserQuestion",
				partId,
				callId,
				approvalId: approvalId ?? callId,
				state: toolState,
				questions,
				answers: completedAnswers,
			},
			null,
			2,
		),
	);

	function getHost(): HTMLElement | null {
		return root ? getCustomElementHost(root) : null;
	}

	function textFromSlot(element: Element, slotName: string): string {
		return (
			element.querySelector(`[slot="${slotName}"]`)?.textContent?.trim() ?? ""
		);
	}

	function ownText(element: Element): string {
		const clone = element.cloneNode(true) as Element;
		clone
			.querySelectorAll(
				'[slot="label"], [slot="description"], [slot="question"], [slot="notes"], disco-option, disco-answer',
			)
			.forEach((child) => child.remove());
		return clone.textContent?.trim() ?? "";
	}

	function readQuestions(host: HTMLElement): QuestionItem[] {
		return Array.from(host.querySelectorAll(":scope > disco-question")).map(
			(questionElement, index) => {
				const question =
					textFromSlot(questionElement, "question") ||
					questionElement.getAttribute("question") ||
					ownText(questionElement);
				const name =
					questionElement.getAttribute("name") ||
					question ||
					`question-${index + 1}`;
				const type = questionElement.getAttribute("type") ?? "single";

				return {
					name,
					header:
						textFromSlot(questionElement, "header") ||
						questionElement.getAttribute("header") ||
						`Question ${index + 1}`,
					question,
					notes:
						textFromSlot(questionElement, "notes") ||
						questionElement.getAttribute("notes") ||
						"",
					multiSelect:
						type === "multiple" ||
						type === "multi" ||
						questionElement.hasAttribute("multiple"),
					options: Array.from(
						questionElement.querySelectorAll(":scope > disco-option"),
					).map((optionElement) => {
						const label =
							textFromSlot(optionElement, "label") ||
							optionElement.getAttribute("label") ||
							ownText(optionElement);
						return {
							value: optionElement.getAttribute("value") || label,
							label,
							description:
								textFromSlot(optionElement, "description") ||
								optionElement.getAttribute("description") ||
								"",
						};
					}),
				};
			},
		);
	}

	function readAnswers(host: HTMLElement): AnswerItem[] {
		return Array.from(host.querySelectorAll(":scope > disco-answer"))
			.map((answerElement) => ({
				question:
					answerElement.getAttribute("question") ||
					textFromSlot(answerElement, "question"),
				answer:
					answerElement.getAttribute("answer") ||
					textFromSlot(answerElement, "answer") ||
					answerElement.textContent?.trim() ||
					"",
			}))
			.filter((answer) => answer.question || answer.answer);
	}

	function refreshFromChildren() {
		const host = getHost();
		if (!host) {
			return;
		}
		questions = readQuestions(host);
		completedAnswers = readAnswers(host);
		if (currentStep >= questions.length) {
			currentStep = Math.max(questions.length - 1, 0);
		}
	}

	function selectedLabels(questionName: string): string[] {
		return (answers[questionName] ?? "")
			.split(",")
			.map((label) => label.trim())
			.filter(Boolean);
	}

	function isQuestionAnswered(question: QuestionItem | undefined): boolean {
		if (!question) {
			return false;
		}
		if (otherSelected[question.name]) {
			return Boolean(otherText[question.name]?.trim());
		}
		return Boolean(answers[question.name]?.trim());
	}

	function allAnswered(): boolean {
		return questions.length > 0 && questions.every(isQuestionAnswered);
	}

	function findNextUnanswered(afterStep: number): number | null {
		for (let index = afterStep + 1; index < questions.length; index += 1) {
			if (!isQuestionAnswered(questions[index])) {
				return index;
			}
		}
		return null;
	}

	function scheduleAutoAdvance() {
		if (autoAdvanceTimeout) {
			clearTimeout(autoAdvanceTimeout);
		}
		autoAdvanceTimeout = setTimeout(() => {
			const next = findNextUnanswered(currentStep);
			if (next !== null) {
				currentStep = next;
			} else if (currentStep < questions.length - 1) {
				currentStep += 1;
			}
		}, AUTO_ADVANCE_DELAY);
	}

	function handleOptionChange(
		question: QuestionItem,
		optionLabel: string,
		checked: boolean,
	) {
		if (optionLabel === OTHER_LABEL) {
			otherSelected = { ...otherSelected, [question.name]: checked };
			if (!question.multiSelect) {
				answers = { ...answers, [question.name]: "" };
			}
			return;
		}

		if (question.multiSelect) {
			const current = selectedLabels(question.name);
			const next = checked
				? [...current, optionLabel]
				: current.filter((label) => label !== optionLabel);
			answers = { ...answers, [question.name]: next.join(", ") };
			return;
		}

		otherSelected = { ...otherSelected, [question.name]: false };
		otherText = { ...otherText, [question.name]: "" };
		answers = { ...answers, [question.name]: optionLabel };
		scheduleAutoAdvance();
	}

	function buildFinalAnswers(): Record<string, string> {
		const finalAnswers: Record<string, string> = {};
		for (const question of questions) {
			const questionKey = question.question || question.name;
			if (otherSelected[question.name]) {
				const customAnswer = otherText[question.name]?.trim() ?? "";
				finalAnswers[questionKey] = question.multiSelect
					? [answers[question.name], customAnswer].filter(Boolean).join(", ")
					: customAnswer;
			} else {
				finalAnswers[questionKey] = answers[question.name] ?? "";
			}
		}
		return finalAnswers;
	}

	function shouldShowSubmitAction(): boolean {
		return currentStep >= questions.length - 1 || allAnswered();
	}

	function submit() {
		const host = getHost();
		if (!host || !allAnswered()) {
			return;
		}
		emitComposedEvent(host, "disco-tool-question-submit", {
			messageId: host.closest("disco-message")?.id || undefined,
			partId,
			callId,
			approvalId: approvalId ?? callId,
			answers: buildFinalAnswers(),
		});
	}

	onMount(() => {
		refreshFromChildren();
		const host = getHost();
		if (!host) {
			return;
		}
		const observer = new MutationObserver(refreshFromChildren);
		observer.observe(host, {
			childList: true,
			subtree: true,
			attributes: true,
			characterData: true,
		});
		return () => {
			observer.disconnect();
			if (autoAdvanceTimeout) {
				clearTimeout(autoAdvanceTimeout);
			}
		};
	});
</script>

<div part="container" class="container" bind:this={root} data-state={toolState}>
	<div part="header" class="header">
		<div class="header-title">
			<MessageSquareQuoteIcon class="header-icon" aria-hidden="true" />
			<span>Agent question</span>
			<span part="status" class="status">
				<ClockIcon class="status-icon" aria-hidden="true" />
				{statusLabel}
			</span>
		</div>
		<button
			part="raw-toggle"
			class:active={rawOpen}
			class="raw-toggle"
			type="button"
			aria-pressed={rawOpen}
			aria-label={rawOpen ? "Show question view" : "Show raw question data"}
			title={rawOpen ? "Show question view" : "Show raw question data"}
			onclick={() => (rawOpen = !rawOpen)}
		>
			<CodeIcon class="raw-icon" aria-hidden="true" />
		</button>
	</div>

	{#if rawOpen}
		<pre part="raw" class="raw">{rawPayload}</pre>
	{:else}
		<div part="card" class="card">
			<div class="intro">
				<h3>Agent needs input</h3>
				<p>Answer to help the agent continue with your task.</p>
			</div>

			{#if completedAnswers.length > 0 && !isPending}
				<div class="answers" part="answers">
					{#each completedAnswers as item, index (`${item.question}:${index}`)}
						<div class="answer">
							<div class="answer-question">{item.question}</div>
							<div class="answer-value">{item.answer}</div>
						</div>
					{/each}
				</div>
			{:else if questions.length === 0}
				<p class="empty">No questions are available.</p>
			{:else}
				{#if notes}
					<div class="notes" part="notes">{notes}</div>
				{/if}

				{#if questions.length > 1}
					<div class="steps" part="steps">
						{#each questions as question, index (question.name)}
							{@const answered = isQuestionAnswered(question)}
							{@const active = index === currentStep}
							<button
								class:active
								class="step"
								type="button"
								onclick={() => (currentStep = index)}
							>
								{#if answered}
									<CheckIcon class="step-icon" />
								{:else}
									<span class="step-number">{index + 1}</span>
								{/if}
								{question.header}
							</button>
						{/each}
					</div>
				{/if}

				{#if currentQuestion}
					<div class="question" part="question">
						<p class="question-text">{currentQuestion.question}</p>
						<div class="options" part="options">
							{#each currentQuestion.options as option (option.value)}
								{@const selected =
									!otherSelected[currentQuestion.name] &&
									selectedLabels(currentQuestion.name).includes(option.label)}
								<label class:selected class="option">
									<input
										checked={selected}
										name={`question-${currentQuestion.name}`}
										onchange={(event) =>
											handleOptionChange(
												currentQuestion,
												option.label,
												(event.currentTarget as HTMLInputElement).checked,
											)}
										type={currentQuestion.multiSelect ? "checkbox" : "radio"}
										value={option.value}
									/>
									<span class="option-body">
										<span class="option-label">{option.label}</span>
										{#if option.description}
											<span class="option-description"
												>{option.description}</span
											>
										{/if}
									</span>
								</label>
							{/each}

							<label
								class:selected={otherSelected[currentQuestion.name]}
								class="option"
							>
								<input
									checked={otherSelected[currentQuestion.name] ?? false}
									name={`question-${currentQuestion.name}`}
									onchange={(event) =>
										handleOptionChange(
											currentQuestion,
											OTHER_LABEL,
											(event.currentTarget as HTMLInputElement).checked,
										)}
									type={currentQuestion.multiSelect ? "checkbox" : "radio"}
									value={OTHER_LABEL}
								/>
								<span class="option-body">
									<span class="option-label">Other</span>
									{#if otherSelected[currentQuestion.name]}
										<textarea
											oninput={(event) => {
												otherText = {
													...otherText,
													[currentQuestion.name]: (
														event.currentTarget as HTMLTextAreaElement
													).value,
												};
											}}
											placeholder="Type your answer..."
											rows="1"
											value={otherText[currentQuestion.name] ?? ""}
										></textarea>
									{/if}
								</span>
							</label>
						</div>
					</div>
				{/if}

				<div class="actions" part="actions">
					<div>
						{#if currentStep > 0}
							<button
								class="ghost"
								type="button"
								onclick={() => (currentStep -= 1)}
							>
								Back
							</button>
						{/if}
					</div>
					<div>
						{#if shouldShowSubmitAction()}
							<button
								class="primary"
								disabled={!allAnswered()}
								type="button"
								onclick={submit}
							>
								Submit
							</button>
						{:else}
							<button
								class="primary"
								disabled={!isQuestionAnswered(currentQuestion)}
								type="button"
								onclick={() => {
									const next = findNextUnanswered(currentStep);
									if (next !== null) {
										currentStep = next;
									}
								}}
							>
								Continue
							</button>
						{/if}
					</div>
				</div>
			{/if}
		</div>
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

	.container {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.card {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: var(--disco-radius, 0.75rem);
		background: var(
			--disco-conversation-card,
			var(--disco-card, var(--card, #fff))
		);
		padding: 1rem;
	}

	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		min-width: 0;
	}

	.header-title {
		display: flex;
		min-width: 0;
		flex: 1 1 auto;
		align-items: center;
		gap: 0.5rem;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.875rem;
		font-weight: 500;
		line-height: 1.25rem;
	}

	:global(.header-icon),
	:global(.status-icon),
	:global(.raw-icon) {
		width: 1rem;
		height: 1rem;
		flex: 0 0 auto;
	}

	.status {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-size: 0.75rem;
		font-weight: 500;
		line-height: 1rem;
	}

	.raw-toggle {
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
		opacity: 0.85;
		transition:
			background 150ms ease,
			color 150ms ease,
			opacity 150ms ease;
	}

	.raw-toggle:hover,
	.raw-toggle:focus-visible,
	.raw-toggle.active {
		background: var(
			--disco-conversation-muted,
			var(--disco-muted, var(--muted, #f3f4f6))
		);
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		opacity: 1;
	}

	.raw {
		max-height: 24rem;
		overflow: auto;
		margin: 0;
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
		background: var(
			--disco-conversation-background,
			var(--disco-background, var(--background, #fff))
		);
		padding: 0.75rem;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-family: var(
			--disco-conversation-font-mono,
			var(--disco-font-mono, var(--font-mono, monospace))
		);
		font-size: 0.75rem;
		line-height: 1rem;
		white-space: pre-wrap;
	}

	h3,
	p {
		margin: 0;
	}

	.intro h3 {
		font-size: 1rem;
		font-weight: 600;
	}

	.intro p,
	.empty,
	.option-description,
	.answer-question {
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
	}

	.intro p {
		font-size: 0.875rem;
		margin-top: 0.125rem;
	}

	.notes {
		max-height: 16rem;
		overflow: auto;
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
		background: var(
			--disco-conversation-muted,
			var(--disco-muted, var(--muted, #f3f4f6))
		);
		padding: 0.75rem;
		font-size: 0.875rem;
		white-space: pre-wrap;
	}

	.steps {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem;
	}

	.step {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		border: 1px solid transparent;
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
		background: color-mix(
			in srgb,
			var(--disco-conversation-muted, var(--disco-muted, var(--muted, #f3f4f6)))
				50%,
			transparent
		);
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		cursor: pointer;
		font: inherit;
		font-size: 0.75rem;
		font-weight: 500;
		padding: 0.375rem 0.75rem;
	}

	.step.active {
		border-color: color-mix(
			in srgb,
			var(
					--disco-conversation-primary,
					var(--disco-primary, var(--primary, #2563eb))
				)
				30%,
			transparent
		);
		background: color-mix(
			in srgb,
			var(
					--disco-conversation-primary,
					var(--disco-primary, var(--primary, #2563eb))
				)
				10%,
			transparent
		);
		color: var(
			--disco-conversation-primary,
			var(--disco-primary, var(--primary, #2563eb))
		);
	}

	.step-number {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 0.875rem;
		height: 0.875rem;
		border: 1px solid currentColor;
		border-radius: 999px;
		font-size: 0.625rem;
		line-height: 1;
	}

	:global(.step-icon) {
		width: 0.875rem;
		height: 0.875rem;
	}

	.question,
	.options,
	.option-body,
	.answers,
	.answer {
		display: flex;
		flex-direction: column;
	}

	.question {
		gap: 0.75rem;
	}

	.question-text {
		font-size: 0.875rem;
		font-weight: 500;
	}

	.options,
	.answers {
		gap: 0.375rem;
	}

	.option {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
		cursor: pointer;
		padding: 0.625rem 0.75rem;
		transition:
			background 0.15s ease,
			border-color 0.15s ease;
	}

	.option:hover {
		background: color-mix(
			in srgb,
			var(--disco-conversation-muted, var(--disco-muted, var(--muted, #f3f4f6)))
				50%,
			transparent
		);
	}

	.option.selected {
		border-color: var(
			--disco-conversation-primary,
			var(--disco-primary, var(--primary, #2563eb))
		);
		background: color-mix(
			in srgb,
			var(
					--disco-conversation-primary,
					var(--disco-primary, var(--primary, #2563eb))
				)
				5%,
			transparent
		);
	}

	input {
		accent-color: var(
			--disco-conversation-primary,
			var(--disco-primary, var(--primary, #2563eb))
		);
		flex-shrink: 0;
		margin-top: 0.125rem;
	}

	.option-body {
		flex: 1;
		gap: 0.125rem;
		min-width: 0;
	}

	.option-label {
		font-size: 0.875rem;
		font-weight: 500;
		line-height: 1.25;
	}

	.option-description {
		font-size: 0.75rem;
	}

	textarea {
		box-sizing: border-box;
		width: 100%;
		resize: vertical;
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
		background: var(
			--disco-conversation-background,
			var(--disco-background, var(--background, #fff))
		);
		color: inherit;
		font: inherit;
		font-size: 0.875rem;
		margin-top: 0.375rem;
		padding: 0.375rem 0.5rem;
	}

	.actions {
		display: flex;
		justify-content: space-between;
		gap: 0.5rem;
		padding-top: 0.5rem;
	}

	button.primary,
	button.ghost {
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
		cursor: pointer;
		font: inherit;
		font-size: 0.875rem;
		font-weight: 500;
		padding: 0.5rem 0.875rem;
	}

	button.primary {
		border: 1px solid
			var(
				--disco-conversation-primary,
				var(--disco-primary, var(--primary, #2563eb))
			);
		background: var(
			--disco-conversation-primary,
			var(--disco-primary, var(--primary, #2563eb))
		);
		color: var(
			--disco-conversation-primary-foreground,
			var(--primary-foreground, #fff)
		);
	}

	button.ghost {
		border: 1px solid transparent;
		background: transparent;
		color: inherit;
	}

	button:disabled {
		cursor: not-allowed;
		opacity: 0.5;
	}

	.answer {
		gap: 0.25rem;
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
		padding: 0.75rem;
	}

	.answer-question {
		font-size: 0.75rem;
	}

	.answer-value {
		font-size: 0.875rem;
		font-weight: 500;
	}
</style>
