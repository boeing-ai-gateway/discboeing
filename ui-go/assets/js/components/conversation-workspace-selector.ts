export type WorkspaceSelectorAPI = {
	rememberSelectValue(select: HTMLSelectElement): void;
	option(select: HTMLSelectElement): void;
	input(input: HTMLInputElement): void;
	keydown(input: HTMLInputElement, event: KeyboardEvent): void;
	reset(element: HTMLElement): void;
	suggestion(button: HTMLButtonElement): void;
};

const workspaceValidationTimers = new WeakMap<HTMLInputElement, number>();
const workspaceSourceInputOptions = new Set(["local-directory", "git-repo"]);
let ignoreWorkspaceResetUntil = 0;

function postWorkspaceSelector(data: URLSearchParams) {
	return fetch("/ui/commands/composer-workspace", {
		method: "POST",
		body: data,
	});
}

function focusWorkspaceInput() {
	requestAnimationFrame(() => {
		document
			.querySelector<HTMLInputElement>("input[data-workspace-source-input]")
			?.focus({ preventScroll: true });
	});
}

function focusWorkspaceSelect(showPicker = false) {
	requestAnimationFrame(() => {
		const select = document.querySelector<HTMLSelectElement>("select[data-workspace-select]");
		select?.focus({ preventScroll: true });
		if (!select || !showPicker || !("showPicker" in select)) {
			return;
		}
		try {
			(select as HTMLSelectElement & { showPicker?: () => void }).showPicker?.();
		} catch {
			select.click();
		}
	});
}

function showWorkspaceSelectPending(select: HTMLSelectElement) {
	const wrapper = select.closest<HTMLElement>("[data-workspace-select-wrapper]");
	if (!wrapper || wrapper.dataset.workspacePending === "true") {
		return;
	}
	const previousValue = select.dataset.workspacePreviousValue;
	if (previousValue) {
		select.value = previousValue;
	}
	wrapper.dataset.workspacePending = "true";
	select.setAttribute("aria-busy", "true");
	select.classList.add("text-transparent", "caret-transparent");

	const spinner = document.createElement("span");
	spinner.dataset.workspaceSelectSpinner = "";
	spinner.className =
		"pointer-events-none absolute end-9 top-1/2 inline-flex size-4 -translate-y-1/2 items-center justify-center text-muted-foreground";
	spinner.innerHTML =
		'<svg class="size-4 animate-spin" viewBox="0 0 24 24" aria-hidden="true"><circle class="opacity-25" cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8v4a4 4 0 0 0-4 4H4z"></path></svg>';
	wrapper.append(spinner);
}

function postWorkspaceSourceInput(input: HTMLInputElement, immediate = false) {
	const existing = workspaceValidationTimers.get(input);
	if (existing !== undefined) {
		window.clearTimeout(existing);
		workspaceValidationTimers.delete(input);
	}
	const run = () => {
		const data = new URLSearchParams();
		data.set("action", "input");
		data.set("source_input", input.value);
		data.set("source_type", input.dataset.workspaceSourceType ?? "");
		void postWorkspaceSelector(data).then(focusWorkspaceInput);
	};
	if (immediate) {
		run();
		return;
	}
	workspaceValidationTimers.set(input, window.setTimeout(run, 250));
}

export const workspaceSelector: WorkspaceSelectorAPI = {
	rememberSelectValue(select) {
		select.dataset.workspacePreviousValue = select.value;
	},

	option(select) {
		const nextValue = select.value;
		if (workspaceSourceInputOptions.has(nextValue)) {
			showWorkspaceSelectPending(select);
			// Native select interactions can finish with a click over the freshly
			// patched reset button. Ignore that transition click so choosing an
			// input-backed source does not immediately reset to the dropdown.
			ignoreWorkspaceResetUntil = Date.now() + 1000;
		}
		const data = new URLSearchParams();
		data.set("action", "option");
		data.set("option", nextValue);
		void postWorkspaceSelector(data).then(focusWorkspaceInput);
	},

	input(input) {
		postWorkspaceSourceInput(input);
	},

	keydown(input, event) {
		if (event.key === "Escape") {
			event.preventDefault();
			event.stopPropagation();
			workspaceSelector.reset(input);
			return;
		}
		if (event.key === "Enter" || event.key === "Tab") {
			const selector = input.closest<HTMLElement>("[data-conversation-workspace-selector]");
			const firstSuggestion = selector?.querySelector<HTMLButtonElement>(
				"button[data-workspace-suggestion]",
			);
			if (firstSuggestion) {
				event.preventDefault();
				workspaceSelector.suggestion(firstSuggestion);
				return;
			}
			postWorkspaceSourceInput(input, true);
		}
	},

	reset(element) {
		if (Date.now() < ignoreWorkspaceResetUntil) {
			return;
		}
		const data = new URLSearchParams();
		data.set("action", "reset");
		void postWorkspaceSelector(data).then(() => focusWorkspaceSelect(true));
		element.blur();
	},

	suggestion(button) {
		const value = button.dataset.workspaceSuggestion;
		if (!value) {
			return;
		}
		const data = new URLSearchParams();
		data.set("action", "suggestion");
		data.set("value", value);
		void postWorkspaceSelector(data).then(focusWorkspaceInput);
	},
};
