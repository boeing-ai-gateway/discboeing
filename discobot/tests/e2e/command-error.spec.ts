import { expect, test } from "@playwright/test";

import { composerPanel, openDefaultLayout, showPanel, terminalPanel } from "./helpers/layout";

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("completes when a command returns 204 no content", async ({ page }) => {
	await page.route("**/ui/commands/panels/composer/toggle", async (route) => {
		await route.fulfill({ status: 204, body: "" });
	});

	await showPanel(page, "terminal", terminalPanel(page));
	await expect(composerPanel(page)).toBeVisible();
	const hideComposer = page.getByRole("button", { name: "Hide composer" });

	await Promise.all([
		page.waitForResponse((response) =>
			response.url().includes("/ui/commands/panels/composer/toggle") && response.status() === 204,
		),
		hideComposer.click(),
	]);

	await expect(hideComposer).not.toHaveAttribute("aria-busy", "true");
	await expect(page.getByRole("alert")).toHaveCount(0);
	await expect(composerPanel(page)).toBeVisible();
});

test("shows an error toast when a command request fails", async ({ page }) => {
	await page.route("**/ui/commands/panels/composer/toggle", async (route) => {
		await route.fulfill({ status: 500, body: "" });
	});

	await showPanel(page, "terminal", terminalPanel(page));
	await expect(composerPanel(page)).toBeVisible();
	await page.getByRole("button", { name: "Hide composer" }).click();

	await expect(page.getByRole("alert")).toContainText("Command failed");
	await expect(page.getByRole("alert")).toContainText("Command failed with status 500");
	await expect(composerPanel(page)).toBeVisible();
});
