import { expect, test } from "@playwright/test";

import {
	composerPanel,
	editorPanel,
	expectPanelFillsGridVertically,
	expectPanelMountedButHidden,
	expectPanelReachesGridRightEdge,
	openDefaultLayout,
	sessionPanel,
	terminalPanel,
} from "./helpers/layout";

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("uses the default layout", async ({ page }) => {
	await expectPanelMountedButHidden(editorPanel(page));
	await expectPanelMountedButHidden(terminalPanel(page));
	await expect(composerPanel(page)).toBeVisible();
	await expect(sessionPanel(page)).toBeVisible();
	await expect(page.getByRole("button", { name: "Hide sessions panel" })).toBeVisible();
	await expect(page.getByRole("button", { name: "Show editor" })).toBeVisible();
	await expect(page.getByRole("button", { name: "Show terminal" })).toBeVisible();
	await expectPanelReachesGridRightEdge(page, composerPanel(page));
	await expectPanelFillsGridVertically(page, composerPanel(page));
});
