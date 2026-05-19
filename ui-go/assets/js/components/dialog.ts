type DialogDismissReason = "escape" | "outside" | "close";

const dismissing = new WeakSet<HTMLElement>();

function dialogDismissURL(dialog: HTMLElement): string {
	return dialog.dataset.dialogDismissUrl ?? "";
}

function dismissibleDialogs(): HTMLElement[] {
	return Array.from(
		document.querySelectorAll<HTMLElement>(
			"[data-slot='dialog-content'][data-dialog-dismiss-url]",
		),
	).filter((dialog) => dialogDismissURL(dialog) !== "");
}

function stackingIndex(dialog: HTMLElement): number {
	const zIndex = Number.parseInt(getComputedStyle(dialog).zIndex, 10);
	return Number.isFinite(zIndex) ? zIndex : 0;
}

function topmostDialog(): HTMLElement | null {
	return dismissibleDialogs().reduce<HTMLElement | null>((topmost, dialog) => {
		if (!topmost) {
			return dialog;
		}
		const topmostZ = stackingIndex(topmost);
		const dialogZ = stackingIndex(dialog);
		if (dialogZ > topmostZ) {
			return dialog;
		}
		if (dialogZ < topmostZ) {
			return topmost;
		}
		return topmost.compareDocumentPosition(dialog) & Node.DOCUMENT_POSITION_FOLLOWING
			? dialog
			: topmost;
	}, null);
}

function shouldDismiss(dialog: HTMLElement, reason: DialogDismissReason, originalEvent: Event): boolean {
	const event = new CustomEvent("discobot:dialog-dismiss", {
		bubbles: true,
		cancelable: true,
		detail: { dialog, originalEvent, reason },
	});
	dialog.dispatchEvent(event);
	return !event.defaultPrevented;
}

async function dismissDialog(
	dialog: HTMLElement,
	reason: DialogDismissReason,
	originalEvent: Event,
) {
	const url = dialogDismissURL(dialog);
	if (!url || dismissing.has(dialog) || !shouldDismiss(dialog, reason, originalEvent)) {
		return;
	}
	dismissing.add(dialog);
	try {
		await fetch(url, { method: "POST", credentials: "same-origin" });
	} finally {
		dismissing.delete(dialog);
	}
}

function handleKeydown(event: KeyboardEvent) {
	if (event.key !== "Escape" || event.defaultPrevented) {
		return;
	}
	const dialog = topmostDialog();
	if (!dialog) {
		return;
	}
	event.preventDefault();
	void dismissDialog(dialog, "escape", event);
}

function handlePointerdown(event: PointerEvent) {
	if (event.defaultPrevented) {
		return;
	}
	const dialog = topmostDialog();
	if (!dialog || dialog.contains(event.target as Node | null)) {
		return;
	}
	void dismissDialog(dialog, "outside", event);
}

function handleClick(event: MouseEvent) {
	if (event.defaultPrevented) {
		return;
	}
	const close = (event.target as Element | null)?.closest<HTMLElement>(
		"[data-slot='dialog-close']",
	);
	const dialog = close?.closest<HTMLElement>("[data-slot='dialog-content']");
	if (!dialog || !dialogDismissURL(dialog)) {
		return;
	}
	void dismissDialog(dialog, "close", event);
}

export const dialog = {
	install() {
		document.addEventListener("keydown", handleKeydown);
		document.addEventListener("pointerdown", handlePointerdown);
		document.addEventListener("click", handleClick);
	},
};
