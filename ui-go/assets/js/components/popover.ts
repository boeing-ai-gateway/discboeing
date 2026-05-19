const dismissing = new WeakSet<HTMLElement>();

function openScheduleControls(): HTMLElement[] {
	return Array.from(
		document.querySelectorAll<HTMLElement>(
			"[data-composer-schedule-control][data-composer-schedule-open='true'][data-popover-dismiss-url]",
		),
	);
}

function topmostScheduleControl(): HTMLElement | null {
	const controls = openScheduleControls();
	return controls.length > 0 ? controls[controls.length - 1] : null;
}

async function dismiss(control: HTMLElement) {
	const url = control.dataset.popoverDismissUrl ?? "";
	if (!url || dismissing.has(control)) {
		return;
	}
	dismissing.add(control);
	try {
		await fetch(url, {
			method: "POST",
			credentials: "same-origin",
			headers: { "Content-Type": "application/x-www-form-urlencoded" },
		});
	} finally {
		dismissing.delete(control);
	}
}

function handleKeydown(event: KeyboardEvent) {
	if (event.key !== "Escape" || event.defaultPrevented) {
		return;
	}
	const control = topmostScheduleControl();
	if (!control) {
		return;
	}
	event.preventDefault();
	void dismiss(control);
}

function handlePointerdown(event: PointerEvent) {
	if (event.defaultPrevented) {
		return;
	}
	const control = topmostScheduleControl();
	if (!control || control.contains(event.target as Node | null)) {
		return;
	}
	void dismiss(control);
}

export const popover = {
	install() {
		document.addEventListener("keydown", handleKeydown);
		document.addEventListener("pointerdown", handlePointerdown);
	},
};
