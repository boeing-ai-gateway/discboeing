import { expect, test, type Page } from "@playwright/test";

import { openDefaultLayout } from "./helpers/layout";

const openModelMenu = async (page: Page) => {
	const trigger = page.getByRole("button", {
		name: "Select model, thinking effort, and service priority",
	});
	await trigger.click();
	const menu = page.locator("#composer-model-menu-panel");
	await expect(menu).toBeVisible();
	return { trigger, menu };
};

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("opens configured model groups", async ({ page }) => {
	const { trigger, menu } = await openModelMenu(page);

	await expect(trigger).toHaveAttribute("aria-expanded", "true");
	await expect(menu.getByRole("menuitemradio", { name: /Default model/ })).toHaveAttribute("aria-checked", "true");
	await expect(menu.getByText("Anthropic", { exact: true })).toBeVisible();
	await expect(menu.getByRole("menuitem", { name: /Claude Sonnet 4\.5/ })).toBeVisible();
	await expect(menu.getByText("OpenAI", { exact: true })).toBeVisible();
	await expect(menu.getByRole("menuitem", { name: /GPT-5\.1 Codex/ })).toBeVisible();
});

test("selects a model reasoning level and service tier", async ({ page }) => {
	const { trigger, menu } = await openModelMenu(page);

	await menu.getByRole("menuitem", { name: /Claude Sonnet 4\.5/ }).click();
	const reasoningMenu = page.locator("#composer-model-menu-0-0-reasoning-panel");
	await expect(reasoningMenu).toBeVisible();

	await reasoningMenu.getByRole("menuitem", { name: /High/ }).click();
	const serviceTierMenu = page.locator("#composer-model-menu-0-0-high-service-tier-panel");
	await expect(serviceTierMenu).toBeVisible();

	await Promise.all([
		page.waitForResponse((response) =>
			response.url().includes("/ui/commands/composer/model-settings/select") && response.status() === 204,
		),
		serviceTierMenu.getByRole("menuitemradio", { name: /Fast/ }).click(),
	]);

	await expect(trigger).toContainText("Claude Sonnet 4.5 · High · Fast");
});

test("resets selected model settings to default", async ({ page }) => {
	let modelMenu = (await openModelMenu(page)).menu;
	await modelMenu.getByRole("menuitem", { name: /Claude Sonnet 4\.5/ }).click();
	await page.locator("#composer-model-menu-0-0-reasoning-panel").getByRole("menuitem", { name: /High/ }).click();
	await page
		.locator("#composer-model-menu-0-0-high-service-tier-panel")
		.getByRole("menuitemradio", { name: /Fast/ })
		.click();
	await expect(page.getByRole("button", { name: "Select model, thinking effort, and service priority" })).toContainText(
		"Claude Sonnet 4.5 · High · Fast",
	);

	const opened = await openModelMenu(page);
	modelMenu = opened.menu;
	await Promise.all([
		page.waitForResponse((response) =>
			response.url().includes("/ui/commands/composer/model-settings/select") && response.status() === 204,
		),
		modelMenu.getByRole("menuitemradio", { name: /Default model/ }).click(),
	]);

	await expect(opened.trigger).toContainText("Default model");
});
