import { computePosition, flip, offset, shift } from "@floating-ui/dom";

const openPanels = new Set();
const hoverTimers = new WeakMap();
const submenuSources = new WeakMap();
const submenuOpenDelay = 180;

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

const restoreSubmenuPanel = (panel) => {
	const source = submenuSources.get(panel);
	if (!source) {
		return;
	}

	if (source.nextSibling?.parentNode === source.parent) {
		source.parent.insertBefore(panel, source.nextSibling);
	} else {
		source.parent.appendChild(panel);
	}
	submenuSources.delete(panel);
};

const portalSubmenuPanel = (panel) => {
	if (!panel.matches("[data-menu-submenu]") || submenuSources.has(panel)) {
		return;
	}

	submenuSources.set(panel, {
		parent: panel.parentNode,
		nextSibling: panel.nextSibling,
	});
	document.body.appendChild(panel);
};

const closePanel = (panel) => {
	if (!panel) {
		return;
	}

	panel.hidden = true;
	openPanels.delete(panel);

	const trigger = document.querySelector(`[aria-controls="${CSS.escape(panel.id)}"]`);
	trigger?.setAttribute("aria-expanded", "false");

	for (const child of document.querySelectorAll(`[data-menu-parent="${CSS.escape(panel.id)}"]`)) {
		closePanel(child);
	}

	restoreSubmenuPanel(panel);
};

const closeMenu = (menu) => {
	for (const panel of menu.querySelectorAll("[data-menu-panel], [data-menu-submenu]")) {
		closePanel(panel);
	}
	for (const panel of Array.from(openPanels)) {
		const trigger = document.querySelector(`[aria-controls="${CSS.escape(panel.id)}"]`);
		if (trigger?.closest("[data-menu]") === menu) {
			closePanel(panel);
		}
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

	portalSubmenuPanel(panel);
	panel.scrollTop = 0;
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
		await openPanel(trigger, panel, trigger.dataset.menuPlacement || "bottom-end");
	}
};

const openSubmenu = async (trigger) => {
	const panel = panelFor(trigger);
	if (!panel) {
		return;
	}

	const parentPanel = trigger.closest("[data-menu-panel], [data-menu-submenu]");
	for (const sibling of document.querySelectorAll("[data-menu-submenu]")) {
		if (sibling.dataset.menuParent === parentPanel?.id && sibling !== panel) {
			closePanel(sibling);
		}
	}

	await openPanel(trigger, panel, "right-start");
};

const scheduleSubmenuOpen = (trigger) => {
	window.clearTimeout(hoverTimers.get(trigger));
	hoverTimers.set(
		trigger,
		window.setTimeout(() => {
			void openSubmenu(trigger);
		}, submenuOpenDelay),
	);
};

const cancelSubmenuOpen = (trigger) => {
	window.clearTimeout(hoverTimers.get(trigger));
	hoverTimers.delete(trigger);
};

const elementForTarget = (target) => {
	if (target instanceof Element) {
		return target;
	}
	if (target instanceof Node) {
		return target.parentElement;
	}
	return null;
};

const closestForTarget = (target, selector) => elementForTarget(target)?.closest(selector) ?? null;

const isInsideOpenMenu = (target) => {
	const element = elementForTarget(target);
	if (!element) {
		return false;
	}
	if (element.closest("[data-menu]")) {
		return true;
	}
	for (const panel of openPanels) {
		if (panel.contains(element)) {
			return true;
		}
	}
	return false;
};

export const setupMenus = () => {
	window.discobot = window.discobot ?? {};
	window.discobot.menus = {
		...(window.discobot.menus ?? {}),
		cancelSubmenuOpen,
		scheduleSubmenuOpen,
	};

	document.addEventListener("click", async (event) => {
		const trigger = closestForTarget(event.target, "[data-menu-trigger]");
		if (trigger) {
			event.preventDefault();
			event.stopPropagation();
			await toggleRootMenu(trigger, event);
			return;
		}

		const submenuTrigger = closestForTarget(event.target, "[data-menu-submenu-trigger]");
		if (submenuTrigger) {
			event.preventDefault();
			event.stopPropagation();
			await openSubmenu(submenuTrigger);
			return;
		}

		const menuItem = closestForTarget(event.target, ".menu--item");
		if (menuItem && !menuItem.matches("[data-menu-submenu-trigger]") && isInsideOpenMenu(menuItem)) {
			window.setTimeout(closeAllMenus, 0);
			return;
		}

		if (!isInsideOpenMenu(event.target)) {
			closeAllMenus();
		}
	});

	document.addEventListener("keydown", (event) => {
		if (event.key === "Escape") {
			closeAllMenus();
		}
	});
};
