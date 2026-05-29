import { expect, test } from "@playwright/test";

import {
	composerPanel,
	editorPanel,
	expectPanelFillsGridVertically,
	expectPanelMountedButHidden,
	hidePanel,
	openDefaultLayout,
	sessionPanel,
	setSessionsPanelVisible,
	showPanel,
	terminalPanel,
} from "./helpers/layout";

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("toggles the sessions panel visibility", async ({ page }) => {
	await setSessionsPanelVisible(page, true);
	await expect(page.getByRole("button", { name: "Hide sessions panel" })).toBeVisible();

	await page.locator("#sessions-sidebar-toggle").click();
	await expectPanelMountedButHidden(sessionPanel(page));
	await expect(page.getByRole("button", { name: "Show sessions panel" })).toBeVisible();

	await page.locator("#sessions-sidebar-toggle").click();
	await expect(sessionPanel(page)).toBeVisible();
	await expect(page.getByRole("button", { name: "Hide sessions panel" })).toBeVisible();
});

test("toggles a default layout panel visibility", async ({ page }) => {
	await expect(composerPanel(page)).toBeVisible();
	await expect(page.getByRole("button", { name: "Hide composer" })).toBeVisible();

	await page.getByRole("button", { name: "Hide composer" }).click();
	await expectPanelMountedButHidden(composerPanel(page));
	await expect(page.getByRole("button", { name: "Show composer" })).toBeVisible();

	await page.getByRole("button", { name: "Show composer" }).click();
	await expect(composerPanel(page)).toBeVisible();
	await expect(page.getByRole("button", { name: "Hide composer" })).toBeVisible();
});

test("keeps at least one workspace panel visible", async ({ page }) => {
	await showPanel(page, "terminal", terminalPanel(page));
	await showPanel(page, "composer", composerPanel(page));
	await hidePanel(page, "editor", editorPanel(page));
	await hidePanel(page, "composer", composerPanel(page));

	await expectPanelMountedButHidden(editorPanel(page));
	await expectPanelMountedButHidden(composerPanel(page));
	await expect(terminalPanel(page)).toBeVisible();
	await expect(page.locator("#panel-terminal > .terminal-panel")).toBeVisible();
	await expectPanelFillsGridVertically(page, terminalPanel(page));
	await expectPanelFillsGridVertically(page, page.locator("#panel-terminal > .terminal-panel"));

	await expect(page.getByRole("button", { name: "Hide terminal" })).toBeDisabled();
});
