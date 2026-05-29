import { expect, test, type Page } from "@playwright/test";

import {
	composerPanel,
	editorPanel,
	openDefaultLayout,
	sessionPanel,
	showPanel,
} from "./helpers/layout";

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("maximizes and restores a default layout panel", async ({ page }) => {
	await showPanel(page, "editor", editorPanel(page));
	await expect(composerPanel(page)).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Maximize Editor panel" }).locator("svg"),
	).toHaveClass(/lucide-maximize-2/);

	const editorBefore = await editorPanel(page).boundingBox();

	await page.getByRole("button", { name: "Maximize Editor panel" }).click();
	await expect(editorPanel(page)).toHaveClass(/panel--maximized/);
	await expect(page.getByRole("button", { name: "Restore Editor panel" })).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Restore Editor panel" }).locator("svg"),
	).toHaveClass(/lucide-minimize-2/);
	await expect(sessionPanel(page)).toBeHidden();
	await expect(composerPanel(page)).toBeHidden();
	expect(await visibleResizeHandles(page)).toBe(0);

	const editorMaximized = await editorPanel(page).boundingBox();
	expect(editorMaximized?.width ?? 0).toBeGreaterThan(editorBefore?.width ?? 0);

	await page.getByRole("button", { name: "Restore Editor panel" }).click();
	await expect(editorPanel(page)).not.toHaveClass(/panel--maximized/);
	await expect(page.getByRole("button", { name: "Maximize Editor panel" })).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Maximize Editor panel" }).locator("svg"),
	).toHaveClass(/lucide-maximize-2/);
	await expect(sessionPanel(page)).toBeVisible();
	await expect(composerPanel(page)).toBeVisible();
});

test("hides resize handles inside a maximized panel", async ({ page }) => {
	await page.getByRole("button", { name: "Maximize Composer panel" }).click();
	await expect(composerPanel(page)).toHaveClass(/panel--maximized/);
	expect(await visibleResizeHandles(page)).toBe(0);
});

const visibleResizeHandles = async (page: Page) => {
	return page.evaluate(() => {
		return Array.from(document.querySelectorAll("[data-discobot-resize]")).filter((element) => {
			const style = window.getComputedStyle(element);
			const rect = element.getBoundingClientRect();
			return (
				style.display !== "none" &&
				style.visibility !== "hidden" &&
				rect.width > 0 &&
				rect.height > 0
			);
		}).length;
	});
};
