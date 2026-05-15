import type { Thread } from "$lib/api-types";
import {
	clearComposerDraft,
	readComposerDraft,
	resolveComposerDraftStorageKey,
	writeComposerDraft,
} from "$lib/composer-draft-storage";

const COMPOSER_DRAFT_PERSIST_DELAY_MS = 300;

type ThreadComposerValues = {
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
};

type CreateThreadComposerStateArgs = {
	sessionId: string;
	threadId: string;
	isPending: () => boolean;
	getThread: () => Thread | null;
	getDefaultModel: () => string | null;
	getDraft: () => string;
	setDraft: (draft: string) => void;
};

export function normalizeThreadComposerReasoning(
	reasoning: string | null | undefined,
): string | undefined {
	return reasoning && reasoning.length > 0 ? reasoning : undefined;
}

export function getThreadComposerValues(
	thread: Thread | null,
	defaultModel: string | null,
): ThreadComposerValues {
	return {
		modelId: thread?.model ?? defaultModel,
		reasoning: normalizeThreadComposerReasoning(thread?.reasoning),
		serviceTier: thread?.serviceTier,
	};
}

export function getThreadComposerValuesKey(
	values: ThreadComposerValues,
): string {
	return JSON.stringify(values);
}

export function resolveThreadComposerSubmitValues({
	modelId,
	reasoning,
	serviceTier,
	nextModelId,
	nextReasoning,
	nextServiceTier,
}: ThreadComposerValues & {
	nextModelId: string | null | undefined;
	nextReasoning: string | undefined;
	nextServiceTier: string | null | undefined;
}): ThreadComposerValues {
	const resolvedModelId = nextModelId !== undefined ? nextModelId : modelId;
	return {
		modelId: resolvedModelId,
		reasoning: resolvedModelId
			? normalizeThreadComposerReasoning(nextReasoning ?? reasoning)
			: undefined,
		serviceTier: resolvedModelId
			? ((nextServiceTier !== undefined ? nextServiceTier : serviceTier) ??
				undefined)
			: undefined,
	};
}

export function createThreadComposerState(args: CreateThreadComposerStateArgs) {
	const initialComposerValues = getThreadComposerValues(
		args.getThread(),
		args.getDefaultModel(),
	);
	let sourceComposerValuesKey = $state(
		getThreadComposerValuesKey(initialComposerValues),
	);
	let modelId = $state<string | null>(initialComposerValues.modelId);
	let reasoning = $state<string | undefined>(initialComposerValues.reasoning);
	let serviceTier = $state<string | undefined>(
		initialComposerValues.serviceTier,
	);
	let nextModelId = $state<string | null | undefined>(undefined);
	let nextReasoning = $state<string | undefined>(undefined);
	let nextServiceTier = $state<string | null | undefined>(undefined);
	let loadedComposerDraftStorageKey = $state<string | null>(null);
	let lastStoredComposerDraft = $state("");
	let composerDraftPersistTimer: ReturnType<typeof setTimeout> | null = null;

	const cancelComposerDraftPersist = () => {
		if (composerDraftPersistTimer === null) {
			return;
		}
		clearTimeout(composerDraftPersistTimer);
		composerDraftPersistTimer = null;
	};

	$effect(() => {
		const nextSourceComposerValues = getThreadComposerValues(
			args.getThread(),
			args.getDefaultModel(),
		);
		const nextSourceComposerValuesKey = getThreadComposerValuesKey(
			nextSourceComposerValues,
		);
		if (nextSourceComposerValuesKey === sourceComposerValuesKey) {
			return;
		}
		sourceComposerValuesKey = nextSourceComposerValuesKey;
		modelId = nextSourceComposerValues.modelId;
		reasoning = nextSourceComposerValues.reasoning;
		serviceTier = nextSourceComposerValues.serviceTier;
	});

	function getComposerDraftStorageKey() {
		return resolveComposerDraftStorageKey({
			isPending: args.isPending(),
			threadId: args.threadId,
			sessionId: args.sessionId,
		});
	}
	$effect(() => {
		const storageKey = getComposerDraftStorageKey();
		if (loadedComposerDraftStorageKey !== storageKey) {
			loadedComposerDraftStorageKey = storageKey;
			lastStoredComposerDraft = readComposerDraft(storageKey);
			if (args.getDraft() !== lastStoredComposerDraft) {
				args.setDraft(lastStoredComposerDraft);
			}
			return;
		}

		const draft = args.getDraft();
		if (draft === lastStoredComposerDraft) {
			return;
		}

		cancelComposerDraftPersist();

		composerDraftPersistTimer = setTimeout(() => {
			writeComposerDraft(storageKey, draft);
			lastStoredComposerDraft = draft;
			composerDraftPersistTimer = null;
		}, COMPOSER_DRAFT_PERSIST_DELAY_MS);

		return cancelComposerDraftPersist;
	});

	const clearComposerStateDraft = (
		storageKey = getComposerDraftStorageKey(),
	) => {
		cancelComposerDraftPersist();
		clearComposerDraft(storageKey);
		if (storageKey === getComposerDraftStorageKey()) {
			lastStoredComposerDraft = "";
		}
		args.setDraft("");
	};

	const clearNextValues = () => {
		nextModelId = undefined;
		nextReasoning = undefined;
		nextServiceTier = undefined;
	};

	return {
		get modelId() {
			return modelId;
		},
		get reasoning() {
			return reasoning;
		},
		get serviceTier() {
			return serviceTier;
		},
		get nextModelId() {
			return nextModelId;
		},
		get nextReasoning() {
			return nextReasoning;
		},
		get nextServiceTier() {
			return nextServiceTier;
		},
		setNextModelId: (value: string | null | undefined) => {
			nextModelId = value;
		},
		setNextReasoning: (value: string | undefined) => {
			nextReasoning = value;
		},
		setNextServiceTier: (value: string | null | undefined) => {
			nextServiceTier = value;
		},
		clearNextValues,
		clearDraft: clearComposerStateDraft,
		resolveSubmitValues: () =>
			resolveThreadComposerSubmitValues({
				modelId,
				reasoning,
				serviceTier,
				nextModelId,
				nextReasoning,
				nextServiceTier,
			}),
		applySubmitValues: (values: ThreadComposerValues) => {
			modelId = values.modelId;
			reasoning = values.reasoning;
			serviceTier = values.serviceTier;
			clearNextValues();
		},
		dispose: () => {
			cancelComposerDraftPersist();
		},
	};
}
