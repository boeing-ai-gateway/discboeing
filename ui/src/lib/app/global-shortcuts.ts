export type GlobalShortcut = {
	id: string;
	label: string;
	keyGroups: string[][];
};

type ShortcutKeyboardEvent = Pick<
	KeyboardEvent,
	"altKey" | "ctrlKey" | "key" | "metaKey" | "repeat" | "shiftKey"
>;

export type SwitcherCommitModifier = "Control" | "Meta";

export type GlobalShortcutAction =
	| {
			id: "switch-recent-thread";
			reverse: boolean;
			commitModifier: SwitcherCommitModifier;
	  }
	| {
			id: "new-session" | "keyboard-help";
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
	const key = event.key.toLowerCase();
	if (event.key === "Tab" && event.ctrlKey && !event.metaKey && !event.altKey) {
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

export function getGlobalShortcuts(isMacPlatform: boolean): GlobalShortcut[] {
	const primaryModifierLabel = getPrimaryModifierLabel(isMacPlatform);

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
			id: "keyboard-help",
			label: "Show keyboard shortcuts",
			keyGroups: [[primaryModifierLabel, "/"]],
		},
	];
}

export function matchGlobalShortcutKeydown(
	event: ShortcutKeyboardEvent,
	isMacPlatform: boolean,
): GlobalShortcutAction | null {
	if (event.repeat || event.altKey) {
		return null;
	}

	const switcherCommitModifier = getTabSwitcherCommitModifier(
		event,
		isMacPlatform,
	);
	if (switcherCommitModifier) {
		return {
			id: "switch-recent-thread",
			reverse: event.key === "Tab" && event.shiftKey,
			commitModifier: switcherCommitModifier,
		};
	}

	if (!usesPrimaryShortcutModifier(event, isMacPlatform)) {
		return null;
	}

	const key = event.key.toLowerCase();
	if (key === "n") {
		return { id: "new-session" };
	}
	if (key === "/" || key === "?") {
		return { id: "keyboard-help" };
	}
	return null;
}

export function shouldCommitTabSwitcherOnKeyup(
	event: Pick<KeyboardEvent, "key">,
	commitModifier: SwitcherCommitModifier | null,
): boolean {
	return commitModifier !== null && event.key === commitModifier;
}
