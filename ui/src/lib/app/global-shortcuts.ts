export type GlobalShortcut = {
	id: string;
	label: string;
	keyGroups: string[][];
};

type ShortcutKeyboardEvent = Pick<
	KeyboardEvent,
	"altKey" | "ctrlKey" | "metaKey" | "repeat" | "shiftKey"
> & {
	key?: string;
};

export type SwitcherCommitModifier = "Control" | "Meta";

export type GlobalShortcutAction =
	| {
			id: "switch-recent-thread";
			reverse: boolean;
			commitModifier: SwitcherCommitModifier;
	  }
	| {
			id:
				| "new-session"
				| "new-thread"
				| "keyboard-help"
				| "toggle-terminal"
				| "toggle-desktop"
				| "toggle-editor"
				| "toggle-files"
				| "toggle-diff-review"
				| "toggle-services";
	  };

export function detectIsMacPlatform(): boolean {
	if (typeof navigator === "undefined") {
		return false;
	}

	const nav = navigator as Navigator & {
		userAgentData?: {
			platform?: string;
		};
	};
	const platform = nav.userAgentData?.platform || nav.platform || nav.userAgent;
	return /mac/i.test(platform);
}

export function getSwitcherShortcutHints(isMacPlatform: boolean): string[][] {
	return isMacPlatform
		? [
				["Ctrl", "Tab"],
				["Cmd", "Shift", "]"],
			]
		: [
				["Ctrl", "Tab"],
				["Ctrl", "K"],
			];
}

export function getPrimaryModifierLabel(isMacPlatform: boolean): string {
	return isMacPlatform ? "Cmd" : "Ctrl";
}

function getEventKey(event: Pick<ShortcutKeyboardEvent, "key">): string {
	return typeof event.key === "string" ? event.key : "";
}

function usesPrimaryShortcutModifier(
	event: ShortcutKeyboardEvent,
	isMacPlatform: boolean,
): boolean {
	return isMacPlatform ? event.metaKey && !event.ctrlKey : event.ctrlKey;
}

function getTabSwitcherCommitModifier(
	event: ShortcutKeyboardEvent,
	isMacPlatform: boolean,
): SwitcherCommitModifier | null {
	const eventKey = getEventKey(event);
	const key = eventKey.toLowerCase();
	if (eventKey === "Tab" && event.ctrlKey && !event.metaKey && !event.altKey) {
		return "Control";
	}
	if (isMacPlatform) {
		return key === "]" &&
			event.metaKey &&
			event.shiftKey &&
			!event.ctrlKey &&
			!event.altKey
			? "Meta"
			: null;
	}
	return key === "k" && event.ctrlKey && !event.metaKey && !event.altKey
		? "Control"
		: null;
}

function getWorkspaceViewModifierLabel(isMacPlatform: boolean): string {
	return isMacPlatform ? "Cmd" : "Alt";
}

function usesWorkspaceViewShortcutModifier(
	event: ShortcutKeyboardEvent,
	isMacPlatform: boolean,
): boolean {
	return isMacPlatform
		? event.metaKey && !event.ctrlKey && !event.altKey
		: event.altKey && !event.ctrlKey && !event.metaKey;
}

export function getGlobalShortcuts(isMacPlatform: boolean): GlobalShortcut[] {
	const primaryModifierLabel = getPrimaryModifierLabel(isMacPlatform);
	const workspaceViewModifierLabel =
		getWorkspaceViewModifierLabel(isMacPlatform);

	return [
		{
			id: "switch-recent-thread",
			label: "Switch recent thread",
			keyGroups: getSwitcherShortcutHints(isMacPlatform),
		},
		{
			id: "new-session",
			label: "Start new session",
			keyGroups: [[primaryModifierLabel, "N"]],
		},
		{
			id: "new-thread",
			label: "Start new thread",
			keyGroups: [["Ctrl", "Shift", "N"]],
		},
		{
			id: "keyboard-help",
			label: "Show keyboard shortcuts",
			keyGroups: [[primaryModifierLabel, "/"]],
		},
		{
			id: "toggle-terminal",
			label: "Toggle terminal",
			keyGroups: [[workspaceViewModifierLabel, "T"]],
		},
		{
			id: "toggle-desktop",
			label: "Toggle desktop",
			keyGroups: [[workspaceViewModifierLabel, "D"]],
		},
		{
			id: "toggle-editor",
			label: "Toggle editor",
			keyGroups: [[workspaceViewModifierLabel, "E"]],
		},
		{
			id: "toggle-files",
			label: "Toggle files",
			keyGroups: [[workspaceViewModifierLabel, "F"]],
		},
		{
			id: "toggle-diff-review",
			label: "Toggle diff editor",
			keyGroups: [[workspaceViewModifierLabel, "Shift", "D"]],
		},
		{
			id: "toggle-services",
			label: "Toggle services",
			keyGroups: [[workspaceViewModifierLabel, "S"]],
		},
	];
}

export function matchGlobalShortcutKeydown(
	event: ShortcutKeyboardEvent,
	isMacPlatform: boolean,
): GlobalShortcutAction | null {
	if (event.repeat) {
		return null;
	}

	const eventKey = getEventKey(event);
	const switcherCommitModifier = getTabSwitcherCommitModifier(
		event,
		isMacPlatform,
	);
	if (switcherCommitModifier) {
		return {
			id: "switch-recent-thread",
			reverse: eventKey === "Tab" && event.shiftKey,
			commitModifier: switcherCommitModifier,
		};
	}

	const key = eventKey.toLowerCase();
	if (usesWorkspaceViewShortcutModifier(event, isMacPlatform)) {
		if (key === "t" && !event.shiftKey) {
			return { id: "toggle-terminal" };
		}
		if (key === "d") {
			return { id: event.shiftKey ? "toggle-diff-review" : "toggle-desktop" };
		}
		if (key === "e" && !event.shiftKey) {
			return { id: "toggle-editor" };
		}
		if (key === "f" && !event.shiftKey) {
			return { id: "toggle-files" };
		}
		if (key === "s" && !event.shiftKey) {
			return { id: "toggle-services" };
		}
	}

	if (
		key === "n" &&
		event.ctrlKey &&
		event.shiftKey &&
		!event.metaKey &&
		!event.altKey
	) {
		return { id: "new-thread" };
	}

	if (!usesPrimaryShortcutModifier(event, isMacPlatform)) {
		return null;
	}

	if (key === "n") {
		return { id: "new-session" };
	}
	if (key === "/" || key === "?") {
		return { id: "keyboard-help" };
	}
	return null;
}

export function shouldCommitTabSwitcherOnKeyup(
	event: { key?: string },
	commitModifier: SwitcherCommitModifier | null,
): boolean {
	return commitModifier !== null && getEventKey(event) === commitModifier;
}
