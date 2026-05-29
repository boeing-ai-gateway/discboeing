import { expect, type Locator, type Page } from "@playwright/test";

export const shell = (page: Page) => page.locator("#app-shell");
export const workspaceGrid = (page: Page) => page.locator(".app-workspace-grid");
export const sessionPanel = (page: Page) => page.locator("#panel-session");
export const editorPanel = (page: Page) => page.locator("#panel-editor");
export const composerPanel = (page: Page) => page.locator("#panel-composer");
export const terminalPanel = (page: Page) => page.locator("#panel-terminal");

export const openDefaultLayout = async (page: Page) => {
	await page.goto("/");
	await expect(shell(page)).toBeVisible();
};

export const workspaceGridBox = async (page: Page) => {
	const box = await workspaceGrid(page).boundingBox();
	const padding = await workspaceGrid(page).evaluate((element) => {
		const style = window.getComputedStyle(element);
		return {
			top: Number.parseFloat(style.paddingTop),
			right: Number.parseFloat(style.paddingRight),
			bottom: Number.parseFloat(style.paddingBottom),
			left: Number.parseFloat(style.paddingLeft),
		};
	});

	expect(box).not.toBeNull();
	return {
		x: (box?.x ?? 0) + padding.left,
		y: (box?.y ?? 0) + padding.top,
		width: (box?.width ?? 0) - padding.left - padding.right,
		height: (box?.height ?? 0) - padding.top - padding.bottom,
	};
};

export const expectPanelFillsGridVertically = async (page: Page, panel: Locator) => {
	const gridBox = await workspaceGridBox(page);
	const panelBox = await panel.boundingBox();

	expect(panelBox).not.toBeNull();
	expect(Math.abs((panelBox?.y ?? 0) - gridBox.y)).toBeLessThanOrEqual(2);
	expect(Math.abs((panelBox?.height ?? 0) - gridBox.height)).toBeLessThanOrEqual(2);
};

export const expectPanelMountedButHidden = async (panel: Locator) => {
	await expect(panel).toHaveCount(1);
	await expect(panel).toBeHidden();
	await expect(panel).toHaveClass(/panel--hidden/);
};

export const expectPanelReachesGridRightEdge = async (page: Page, panel: Locator) => {
	const gridBox = await workspaceGridBox(page);
	const panelBox = await panel.boundingBox();

	expect(panelBox).not.toBeNull();
	expect(
		Math.abs(((panelBox?.x ?? 0) + (panelBox?.width ?? 0)) - (gridBox.x + gridBox.width)),
	).toBeLessThanOrEqual(2);
};

export const showPanel = async (page: Page, label: string, panel: Locator) => {
	const button = page.getByRole("button", { name: `Show ${label}` });
	if ((await button.count()) > 0) {
		await button.click();
	}
	await expect(panel).toBeVisible();
};

export const hidePanel = async (page: Page, label: string, panel: Locator) => {
	const button = page.getByRole("button", { name: `Hide ${label}` });
	if ((await button.count()) > 0) {
		await button.click();
	}
	await expect(panel).toBeHidden();
};

export const restoreMaximizedPanel = async (page: Page) => {
	const restore = page.getByRole("button", { name: /^Restore .* panel$/ });
	if ((await restore.count()) > 0) {
		await restore.first().click();
	}
	await expect(workspaceGrid(page).locator("> .panel--maximized")).toHaveCount(0);
};

export const setSessionsPanelVisible = async (page: Page, visible: boolean) => {
	await restoreMaximizedPanel(page);
	const button = page.locator("#sessions-sidebar-toggle");
	if (visible) {
		if ((await button.count()) > 0 && (await button.getAttribute("aria-label")) === "Show sessions panel") {
			await button.click();
		}
		await expect(sessionPanel(page)).toBeVisible();
		return;
	}

	if ((await button.count()) > 0 && (await button.getAttribute("aria-label")) === "Hide sessions panel") {
		await button.click();
	}
	await expect(sessionPanel(page)).toBeHidden();
};

export const setPanelVisible = async (
	page: Page,
	label: string,
	panel: Locator,
	visible: boolean,
) => {
	await restoreMaximizedPanel(page);
	if (visible) {
		await showPanel(page, label, panel);
		return;
	}
	await hidePanel(page, label, panel);
};

export const maximizePanel = async (page: Page, label: string, panel: Locator) => {
	await restoreMaximizedPanel(page);
	await showPanel(page, label.toLowerCase(), panel);
	await page.getByRole("button", { name: `Maximize ${label} panel` }).click();
	await expect(panel).toHaveClass(/panel--maximized/);
};
