import { expect, test } from "@playwright/test";

import {
	composerPanel,
	editorPanel,
	expectPanelFillsGridVertically,
	expectPanelReachesGridRightEdge,
	hidePanel,
	openDefaultLayout,
	showPanel,
	terminalPanel,
} from "./helpers/layout";

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("fills the available space when editor and terminal are hidden", async ({ page }) => {
	await showPanel(page, "editor", editorPanel(page));
	await expect(composerPanel(page)).toBeVisible();
	await hidePanel(page, "terminal", terminalPanel(page));

	const composerWithEditor = await composerPanel(page).boundingBox();

	await showPanel(page, "terminal", terminalPanel(page));
	await hidePanel(page, "editor", editorPanel(page));

	const composerWithTerminal = await composerPanel(page).boundingBox();

	await hidePanel(page, "terminal", terminalPanel(page));
	await expect(composerPanel(page)).toBeVisible();

	const composerWithoutTerminal = await composerPanel(page).boundingBox();
	expect(composerWithoutTerminal?.width ?? 0).toBeGreaterThan(composerWithEditor?.width ?? 0);
	expect(composerWithoutTerminal?.height ?? 0).toBeGreaterThan(composerWithTerminal?.height ?? 0);
	await expectPanelReachesGridRightEdge(page, composerPanel(page));
	await expectPanelFillsGridVertically(page, composerPanel(page));
});
