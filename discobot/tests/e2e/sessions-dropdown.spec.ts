import { expect, test } from "@playwright/test";

import { openDefaultLayout, setSessionsPanelVisible } from "./helpers/layout";

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("opens the collapsed sessions dropdown from the titlebar", async ({ page }) => {
	await setSessionsPanelVisible(page, false);

	const trigger = page.getByRole("button", { name: "Sessions", exact: true });
	const dropdown = page.locator("[data-sessions-sidebar-dropdown-panel]");

	await expect(trigger).toBeVisible();
	await expect(trigger).toHaveAttribute("aria-expanded", "false");
	await expect(dropdown).toBeHidden();

	await trigger.click();

	await expect(trigger).toHaveAttribute("aria-expanded", "true");
	await expect(dropdown).toBeVisible();
	await expect(dropdown.getByRole("heading", { name: "Sessions" })).toBeVisible();
	await expect(dropdown.getByRole("button", { name: /New/ })).toBeVisible();
});

test("opens the sessions filter menu inside the collapsed dropdown", async ({ page }) => {
	await setSessionsPanelVisible(page, false);

	await page.getByRole("button", { name: "Sessions", exact: true }).click();
	const dropdown = page.locator("[data-sessions-sidebar-dropdown-panel]");
	await expect(dropdown).toBeVisible();

	await dropdown.getByRole("button", { name: "Open session filters" }).click();

	const filterMenu = page.locator("#sessions-filter-menu-panel");
	await expect(filterMenu).toBeVisible();
	await filterMenu.getByRole("menuitem", { name: "Filter" }).click();
	await expect(page.locator("#sessions-filter-menu-submenu-0")).toBeVisible();
	await expect(
		page.locator("#sessions-filter-menu-submenu-0").getByRole("menuitemcheckbox", { name: "Completed" }),
	).toBeVisible();
});
