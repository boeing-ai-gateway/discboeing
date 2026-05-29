import { expect, test } from "@playwright/test";

import { composerPanel, openDefaultLayout, showPanel, terminalPanel } from "./helpers/layout";

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("shows a window chrome spinner while a command is pending", async ({ page }) => {
	let releaseCommand: () => void = () => {};
	const commandPending = new Promise<void>((resolve) => {
		releaseCommand = resolve;
	});

	await page.route("**/ui/commands/panels/composer/toggle", async (route) => {
		await commandPending;
		await route.continue();
	});

	await showPanel(page, "terminal", terminalPanel(page));
	const clickPromise = page.getByRole("button", { name: "Hide composer" }).click();

	await expect(page.getByRole("status", { name: "Command in progress" })).toBeVisible();
	releaseCommand();
	await clickPromise;
	await expect(page.getByRole("status", { name: "Command in progress" })).not.toBeVisible();
	await showPanel(page, "composer", composerPanel(page));
});
