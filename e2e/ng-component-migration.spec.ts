import { expect, test, type Page } from "@playwright/test";

async function gotoAppWithErrorCapture(page: Page) {
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
    test.skip(true, "Discboeing is showing an authentication screen.");
  }

  await expect(settingsButton).toBeVisible();
  return errors;
}

test.describe("ng migrated app shell components", () => {
  test("mounts the ng-backed mac spacer without boot errors", async ({ page }) => {
    const errors = await gotoAppWithErrorCapture(page);

    await expect(page.getByRole("button", { name: "Settings" })).toBeVisible();
    expect(errors, "Browser console/page errors with ng-backed header spacer").toEqual(
      [],
    );
  });

  test("opens and pins the ng-backed session header dropdown", async ({ page }) => {
    const errors = await gotoAppWithErrorCapture(page);

    const sessionsMenu = page.getByRole("button", { name: "Open sessions menu" });
    if (!(await sessionsMenu.isVisible().catch(() => false))) {
      test.skip(true, "Session header dropdown is not visible in this viewport/state.");
    }

    await sessionsMenu.click();
    await expect(page.getByText("Sessions", { exact: true }).first()).toBeVisible();

    const pinSidebar = page.getByRole("button", { name: "Pin sessions sidebar" });
    await expect(pinSidebar).toBeVisible();
    await pinSidebar.click();
    await expect(pinSidebar).toHaveCount(0);

    expect(errors, "Browser console/page errors from ng-backed dropdown").toEqual(
      [],
    );
  });

  test("opens and closes the ng-backed support info dialog", async ({ page }) => {
    const errors = await gotoAppWithErrorCapture(page);

    await page.getByRole("button", { name: "Settings" }).click();
    await expect(page.getByRole("dialog", { name: "Settings" })).toBeVisible();

    await page.getByRole("button", { name: "Support information" }).click();
    const supportDialog = page.getByRole("dialog", {
      name: "Support Information",
    });
    await expect(supportDialog).toBeVisible();
    await expect(
      supportDialog.getByText(
        /Loading support information|Diagnostic data snapshot|No support information available|Failed to load support information/,
      ).first(),
    ).toBeVisible();

    await supportDialog.getByRole("button", { name: /^Close$/ }).first().click();
    await expect(supportDialog).toHaveCount(0);

    expect(errors, "Browser console/page errors from ng-backed support info").toEqual(
      [],
    );
  });

  test("opens the ng-backed keyboard shortcuts dialog", async ({ page }) => {
    const errors = await gotoAppWithErrorCapture(page);

    await page.keyboard.press("Control+/");
    const shortcutDialog = page.getByRole("dialog", {
      name: "Keyboard shortcuts",
    });
    await expect(shortcutDialog).toBeVisible();
    await expect(shortcutDialog.getByText("Start new session")).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(shortcutDialog).toHaveCount(0);

    expect(
      errors,
      "Browser console/page errors from ng-backed keyboard shortcuts",
    ).toEqual([]);
  });

  test("opens the ng-backed credentials manager", async ({ page }) => {
    const errors = await gotoAppWithErrorCapture(page);

    await page.getByRole("button", { name: "Settings" }).click();
    await expect(page.getByRole("dialog", { name: "Settings" })).toBeVisible();
    await page.getByRole("tab", { name: "Credentials" }).click();

    await expect(page.getByText("Configured credentials")).toBeVisible();
    await expect(page.getByRole("button", { name: "Add credential" })).toBeVisible();

    expect(
      errors,
      "Browser console/page errors from ng-backed credentials manager",
    ).toEqual([]);
  });

  test("opens the ng-backed sandbox providers manager", async ({ page }) => {
    const errors = await gotoAppWithErrorCapture(page);

    await page.getByRole("button", { name: "Settings" }).click();
    await expect(page.getByRole("dialog", { name: "Settings" })).toBeVisible();
    await page.getByRole("tab", { name: "Providers" }).click();

    await expect(page.getByText("Active provider instances")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Refresh provider instances" }),
    ).toBeVisible();

    expect(
      errors,
      "Browser console/page errors from ng-backed sandbox providers manager",
    ).toEqual([]);
  });
});
