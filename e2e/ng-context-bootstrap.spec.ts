import { expect, test, type Page } from "@playwright/test";

async function gotoBootedApp(page: Page) {
  const errors: string[] = [];
  page.on("pageerror", (error) => {
    errors.push(error.message);
  });
  page.on("console", (message) => {
    if (message.type() === "error") {
      errors.push(message.text());
    }
  });

  await page.goto("/");

  const settingsButton = page.getByRole("button", { name: "Settings" });
  const loginLink = page.getByRole("link", { name: /sign in|log in/i });
  await expect(settingsButton.or(loginLink).first()).toBeVisible({
    timeout: 60_000,
  });

  if (await loginLink.isVisible().catch(() => false)) {
    test.skip(true, "Discobot is showing an authentication screen.");
  }

  await expect(settingsButton).toBeVisible();
  expect(errors, "Browser console/page errors during ng context boot").toEqual([]);
}

test.describe("ng context root bootstrap", () => {
  test("boots the app shell after ng context startup", async ({ page }) => {
    await gotoBootedApp(page);

    await expect(page.getByRole("button", { name: "Settings" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Refresh" })).toBeVisible();
    await expect(
      page.getByRole("button", { name: "New session" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Message" })).toBeVisible();
  });

  test("keeps root dialog behavior working with ng context mounted", async ({
    page,
  }) => {
    await gotoBootedApp(page);

    await page.getByRole("button", { name: "Settings" }).click();
    await expect(page.getByRole("dialog", { name: "Settings" })).toBeVisible();

    await page.getByRole("tab", { name: "Chat" }).click();
    await expect(page.getByRole("tab", { name: "Chat" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await page.getByRole("tab", { name: "Credentials" }).click();
    await expect(page.getByText("API Credentials")).toBeVisible();

    await page.getByRole("button", { name: "Done" }).click();
    await expect(page.getByRole("dialog", { name: "Settings" })).toHaveCount(0);
  });

  test("keeps new-session navigation usable with ng context mounted", async ({
    page,
  }) => {
    await gotoBootedApp(page);

    await page.getByRole("button", { name: "New session" }).first().click();

    const message = page.getByRole("textbox", { name: "Message" });
    await expect(message).toBeVisible();
    await expect(message).toBeEditable();

    const draft = `ng bootstrap draft ${Date.now()}`;
    await message.fill(draft);
    await expect(message).toHaveValue(draft);
    await expect(page.getByRole("button", { name: "Submit" })).toBeVisible();
  });
});
