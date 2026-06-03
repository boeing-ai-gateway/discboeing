import { expect, test, type Page } from "@playwright/test";

async function gotoApp(page: Page) {
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
  expect(errors, "Browser console/page errors during app boot").toEqual([]);
}

test.describe("Discobot app smoke", () => {
  test("boots to the app shell with primary actions", async ({ page }) => {
    await gotoApp(page);

    await expect(page.getByRole("button", { name: "Settings" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Refresh" })).toBeVisible();
    await expect(
      page.getByRole("button", { name: "New session" }).first(),
    ).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Message" })).toBeVisible();
  });

  test("opens settings, credentials, and support information dialogs", async ({
    page,
  }) => {
    await gotoApp(page);

    await page.getByRole("button", { name: "Settings" }).click();
    await expect(page.getByRole("dialog", { name: "Settings" })).toBeVisible();
    await expect(page.getByRole("tab", { name: "Appearance" })).toBeVisible();

    await page.getByRole("tab", { name: "Credentials" }).click();
    await expect(page.getByText("API Credentials")).toBeVisible();

    await page.getByRole("button", { name: "Support information" }).click();
    await expect(
      page.getByRole("dialog", { name: "Support Information" }),
    ).toBeVisible();
    await expect(
      page.getByText(
        /Loading support information|No support information available|\{/,
      ),
    ).toBeVisible();

    await page.getByRole("button", { name: "Close" }).last().click();
    await expect(
      page.getByRole("dialog", { name: "Support Information" }),
    ).toHaveCount(0);

    await page.getByRole("button", { name: "Done" }).click();
    await expect(page.getByRole("dialog", { name: "Settings" })).toHaveCount(0);
  });

  test("keeps new-session composer usable without submitting", async ({
    page,
  }) => {
    await gotoApp(page);

    await page.getByRole("button", { name: "New session" }).first().click();

    const message = page.getByRole("textbox", { name: "Message" });
    await expect(message).toBeVisible();
    await expect(message).toBeEditable();

    const draft = `Smoke test draft ${Date.now()}`;
    await message.fill(draft);
    await expect(message).toHaveValue(draft);
    await expect(page.getByRole("button", { name: "Submit" })).toBeVisible();
  });

  test("exposes the sessions panel affordance", async ({ page }) => {
    await gotoApp(page);

    const sessionsMenu = page.getByRole("button", {
      name: /Open sessions menu|Expand sessions panel/,
    });
    const sidebarHeading = page.getByText("Sessions", { exact: true });

    await expect(sessionsMenu.or(sidebarHeading).first()).toBeVisible();
  });
});
