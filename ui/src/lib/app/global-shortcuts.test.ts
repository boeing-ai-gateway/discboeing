import assert from "node:assert/strict";
import test from "node:test";

import {
	matchGlobalShortcutKeydown,
	shouldCommitTabSwitcherOnKeyup,
	type GlobalShortcutAction,
} from "./global-shortcuts";

function keyboardEvent(
	overrides: Partial<Parameters<typeof matchGlobalShortcutKeydown>[0]> & {
		key: string;
	},
): Parameters<typeof matchGlobalShortcutKeydown>[0] {
	const { key, ...rest } = overrides;
	return {
		altKey: false,
		ctrlKey: false,
		key,
		metaKey: false,
		repeat: false,
		shiftKey: false,
		...rest,
	};
}

function assertShortcutAction(
	action: GlobalShortcutAction | null,
	expected: GlobalShortcutAction,
) {
	assert.deepEqual(action, expected);
}

test("matchGlobalShortcutKeydown ignores event-like objects without a key", () => {
	assert.equal(
		matchGlobalShortcutKeydown(
			{
				altKey: false,
				ctrlKey: true,
				metaKey: false,
				repeat: false,
				shiftKey: false,
			},
			false,
		),
		null,
	);
	assert.equal(shouldCommitTabSwitcherOnKeyup({}, "Control"), false);
});

test("matchGlobalShortcutKeydown tracks Control release for macOS Ctrl+Tab switching", () => {
	assertShortcutAction(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "Tab", ctrlKey: true }),
			true,
		),
		{
			id: "switch-recent-thread",
			reverse: false,
			commitModifier: "Control",
		},
	);
	assert.equal(
		shouldCommitTabSwitcherOnKeyup({ key: "Control" }, "Control"),
		true,
	);
	assert.equal(
		shouldCommitTabSwitcherOnKeyup({ key: "Meta" }, "Control"),
		false,
	);
});

test("matchGlobalShortcutKeydown keeps reverse tab switching on Control+Shift+Tab", () => {
	assertShortcutAction(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "Tab", ctrlKey: true, shiftKey: true }),
			false,
		),
		{
			id: "switch-recent-thread",
			reverse: true,
			commitModifier: "Control",
		},
	);
});

test("matchGlobalShortcutKeydown tracks Meta release for macOS fallback switching", () => {
	assertShortcutAction(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "]", metaKey: true, shiftKey: true }),
			true,
		),
		{
			id: "switch-recent-thread",
			reverse: false,
			commitModifier: "Meta",
		},
	);
	assert.equal(shouldCommitTabSwitcherOnKeyup({ key: "Meta" }, "Meta"), true);
});

test("matchGlobalShortcutKeydown keeps primary shortcuts platform-aware", () => {
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "n", metaKey: true }),
			true,
		),
		{ id: "new-session" },
	);
	assert.equal(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "n", ctrlKey: true }),
			true,
		),
		null,
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "/", ctrlKey: true }),
			false,
		),
		{ id: "keyboard-help" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "f", ctrlKey: true }),
			false,
		),
		{ id: "find-in-page" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "f", metaKey: true }),
			true,
		),
		{ id: "find-in-page" },
	);
});

test("matchGlobalShortcutKeydown starts a new thread with Control+Shift+N", () => {
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "n", ctrlKey: true, shiftKey: true }),
			false,
		),
		{ id: "new-thread" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "n", ctrlKey: true, shiftKey: true }),
			true,
		),
		{ id: "new-thread" },
	);
});

test("matchGlobalShortcutKeydown uses Alt for workspace views off macOS", () => {
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "t", altKey: true }),
			false,
		),
		{ id: "toggle-terminal" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "d", altKey: true }),
			false,
		),
		{ id: "toggle-desktop" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "e", altKey: true }),
			false,
		),
		{ id: "toggle-editor" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "d", altKey: true, shiftKey: true }),
			false,
		),
		{ id: "toggle-diff-review" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "f", altKey: true }),
			false,
		),
		{ id: "toggle-files" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "s", altKey: true }),
			false,
		),
		{ id: "toggle-services" },
	);
});

test("matchGlobalShortcutKeydown uses Command for workspace views on macOS", () => {
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "t", metaKey: true }),
			true,
		),
		{ id: "toggle-terminal" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "d", metaKey: true }),
			true,
		),
		{ id: "toggle-desktop" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "e", metaKey: true }),
			true,
		),
		{ id: "toggle-editor" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "d", metaKey: true, shiftKey: true }),
			true,
		),
		{ id: "toggle-diff-review" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "f", metaKey: true, shiftKey: true }),
			true,
		),
		{ id: "toggle-files" },
	);
	assert.deepEqual(
		matchGlobalShortcutKeydown(
			keyboardEvent({ key: "s", metaKey: true }),
			true,
		),
		{ id: "toggle-services" },
	);
});
