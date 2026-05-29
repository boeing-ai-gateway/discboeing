const dropdownSelector = "[data-sessions-sidebar-dropdown]";
const triggerSelector = "[data-sessions-sidebar-dropdown-trigger]";
const panelSelector = "[data-sessions-sidebar-dropdown-panel]";

const closeDropdown = (dropdown) => {
	const trigger = dropdown?.querySelector(triggerSelector);
	const panel = dropdown?.querySelector(panelSelector);
	if (!trigger || !panel) {
		return;
	}

	trigger.setAttribute("aria-expanded", "false");
	panel.hidden = true;
};

const closeAllDropdowns = () => {
	for (const dropdown of document.querySelectorAll(dropdownSelector)) {
		closeDropdown(dropdown);
	}
};

const toggleDropdown = (trigger) => {
	const dropdown = trigger.closest(dropdownSelector);
	const panel = dropdown?.querySelector(panelSelector);
	if (!dropdown || !panel) {
		return;
	}

	const opening = panel.hidden;
	closeAllDropdowns();
	trigger.setAttribute("aria-expanded", String(opening));
	panel.hidden = !opening;
};

export const setupSessionSidebarDropdown = () => {
	document.addEventListener("click", (event) => {
		const trigger = event.target.closest(triggerSelector);
		if (trigger) {
			event.preventDefault();
			toggleDropdown(trigger);
			return;
		}

		if (!event.target.closest(dropdownSelector)) {
			closeAllDropdowns();
		}
	});

	document.addEventListener("keydown", (event) => {
		if (event.key === "Escape") {
			closeAllDropdowns();
		}
	});
};
