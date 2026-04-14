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
});
