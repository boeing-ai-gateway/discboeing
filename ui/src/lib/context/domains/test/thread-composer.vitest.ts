import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";

import type { Context } from "$lib/context/context.types";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";
import {
	movePendingComposerDraftToThread,
	setComposerDraft,
} from "$lib/context/domains/thread-composer";
import {
	PENDING_COMPOSER_DRAFT_STORAGE_KEY,
	readComposerDraft,
	resolveComposerDraftStorageKey,
} from "$lib/context/stores/composer-drafts";

type StorageLike = {
	getItem: (key: string) => string | null;
	setItem: (key: string, value: string) => void;
	removeItem: (key: string) => void;
};

function createLocalStorage(): StorageLike {
	const values = new Map<string, string>();
	return {
		getItem: (key) => values.get(key) ?? null,
		setItem: (key, value) => {
			values.set(key, value);
		},
		removeItem: (key) => {
			values.delete(key);
		},
	};
}

function installWindow(storage: StorageLike): Window & typeof globalThis {
	const previousWindow = globalThis.window;
	const nextWindow = {
		...previousWindow,
		location: previousWindow?.location ?? {
			hostname: "localhost",
			host: "localhost",
			protocol: "http:",
			origin: "http://localhost",
		},
		localStorage: storage,
		setTimeout: globalThis.setTimeout.bind(globalThis),
		clearTimeout: globalThis.clearTimeout.bind(globalThis),
	} as unknown as Window & typeof globalThis;
	globalThis.window = nextWindow;
	return previousWindow;
}

function createContext(): Context {
	return {
		data: createInitialDataState({ projectId: "local" }),
		view: createInitialViewState({ projectId: "local" }),
		commands: undefined as unknown as Context["commands"],
	};
}

afterEach(() => {
	vi.useRealTimers();
});

test("moving a pending draft cancels delayed pending draft persistence", async () => {
	vi.useFakeTimers();
	const storage = createLocalStorage();
	const previousWindow = installWindow(storage);
	try {
		const context = createContext();
		const pendingSessionId = context.view.selection.pendingSessionId;
		const pendingThreadId = "pending-thread";
		const createdThreadId = "created-thread";
		const createdThreadKey = resolveComposerDraftStorageKey({
			isPending: false,
			threadId: createdThreadId,
		});

		await setComposerDraft(context, pendingSessionId, "preserved draft");
		assert.equal(storage.getItem(PENDING_COMPOSER_DRAFT_STORAGE_KEY), null);

		await movePendingComposerDraftToThread(
			context,
			pendingThreadId,
			createdThreadId,
			"preserved draft",
		);
		vi.advanceTimersByTime(250);

		assert.equal(storage.getItem(PENDING_COMPOSER_DRAFT_STORAGE_KEY), null);
		assert.equal(readComposerDraft(createdThreadKey), "preserved draft");
	} finally {
		globalThis.window = previousWindow;
	}
});
