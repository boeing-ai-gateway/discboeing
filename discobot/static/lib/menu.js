import { computePosition, flip, offset, shift } from "@floating-ui/dom";

const openPanels = new Set();

const panelFor = (trigger) => {
	const panelID = trigger.getAttribute("aria-controls");
	if (!panelID) {
		return null;
	}
	return document.getElementById(panelID);
};

const positionPanel = async (reference, panel, placement = "bottom-end") => {
	const { x, y } = await computePosition(reference, panel, {
		placement,
		strategy: "fixed",
		middleware: [offset(4), flip(), shift({ padding: 8 })],
	});

	Object.assign(panel.style, {
		left: `${x}px`,
		top: `${y}px`,
		position: "fixed",
	});
};

const pointReference = (x, y) => ({
	getBoundingClientRect() {
		return {
			x,
			y,
			top: y,
			left: x,
			right: x,
			bottom: y,
			width: 0,
			height: 0,
		};
	},
});

const closePanel = (panel) => {
	if (!panel) {
		return;
	}

	panel.hidden = true;
	openPanels.delete(panel);

	const menu = panel.closest("[data-menu]");
	const trigger = menu?.querySelector(`[aria-controls="${CSS.escape(panel.id)}"]`);
	trigger?.setAttribute("aria-expanded", "false");

	for (const child of panel.querySelectorAll("[data-menu-submenu]")) {
		closePanel(child);
	}
};

const closeMenu = (menu) => {
	for (const panel of menu.querySelectorAll("[data-menu-panel], [data-menu-submenu]")) {
		closePanel(panel);
	}
};

const closeAllMenus = () => {
	for (const panel of Array.from(openPanels)) {
		closePanel(panel);
	}
};

const openPanel = async (trigger, panel, placement, reference = trigger) => {
	if (!panel) {
		return;
	}

	panel.hidden = false;
	openPanels.add(panel);
	trigger.setAttribute("aria-expanded", "true");
	await positionPanel(reference, panel, placement);
};

const toggleRootMenu = async (trigger, event) => {
	const menu = trigger.closest("[data-menu]");
	const panel = panelFor(trigger);
	if (!menu || !panel) {
		return;
	}

	const opening = panel.hidden;
	closeAllMenus();
	if (opening) {
		if (trigger.matches("[data-menu-anchor-click]") && event instanceof MouseEvent && event.detail > 0) {
			await openPanel(trigger, panel, "bottom-start", pointReference(event.clientX, event.clientY));
			return;
		}
		await openPanel(trigger, panel, "bottom-end");
	}
};

const openSubmenu = async (trigger) => {
	const menu = trigger.closest("[data-menu]");
	const panel = panelFor(trigger);
	if (!menu || !panel) {
		return;
	}

	const parentPanel = trigger.closest("[data-menu-panel], [data-menu-submenu]");
	for (const sibling of menu.querySelectorAll("[data-menu-submenu]")) {
		if (sibling.dataset.menuParent === parentPanel?.id && sibling !== panel) {
			closePanel(sibling);
		}
	}

	await openPanel(trigger, panel, "right-start");
};

export const setupMenus = () => {
	document.addEventListener("click", async (event) => {
		const trigger = event.target.closest("[data-menu-trigger]");
		if (trigger) {
			event.preventDefault();
			event.stopPropagation();
			await toggleRootMenu(trigger, event);
			return;
		}

		const submenuTrigger = event.target.closest("[data-menu-submenu-trigger]");
		if (submenuTrigger) {
			event.preventDefault();
			event.stopPropagation();
			await openSubmenu(submenuTrigger);
			return;
		}

		const menuItem = event.target.closest("[data-menu] .menu--item");
		if (menuItem && !menuItem.matches("[data-menu-submenu-trigger]")) {
			window.setTimeout(closeAllMenus, 0);
			return;
		}

		if (!event.target.closest("[data-menu]")) {
			closeAllMenus();
		}
	});

	document.addEventListener("keydown", (event) => {
		if (event.key === "Escape") {
			closeAllMenus();
		}
	});
};
