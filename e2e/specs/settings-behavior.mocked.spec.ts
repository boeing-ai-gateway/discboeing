import type { Page } from "@playwright/test";
import { expect, test } from "../fixtures/test";
import { installSettingsMockRoutes } from "../mocks/settings-routes";

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

  try {
    await page.goto("/");
  } catch (error) {
    test.skip(
      true,
      `Discboeing E2E target is not reachable: ${error instanceof Error ? error.message : String(error)}`,
    );
  }

  const settingsButton = page.getByRole("button", { name: "Settings" });
  const loginLink = page.getByRole("link", { name: /sign in|log in/i });
  await expect(settingsButton.or(loginLink).first()).toBeVisible({
    timeout: 60_000,
  });

  if (await loginLink.isVisible().catch(() => false)) {
    test.skip(true, "Discboeing is showing an authentication screen.");
  }

  await expect(settingsButton).toBeVisible();
  expect(errors, "Browser console/page errors during app boot").toEqual([]);
}

test.describe("Settings mocked behavior", () => {
  test("shows seeded credentials and sandbox providers", async ({ page }) => {
    await installSettingsMockRoutes(page);
    await gotoApp(page);

    await page.getByRole("button", { name: "Settings" }).click();
    await expect(page.getByRole("dialog", { name: "Settings" })).toBeVisible();

    await page.getByRole("tab", { name: "Credentials" }).click();
    await expect(page.getByText("API Credentials")).toBeVisible();
    await expect(page.getByText("E2E OpenAI API key")).toBeVisible();
    await expect(page.getByText("E2E GitHub token")).toBeVisible();

    await page.getByRole("tab", { name: "Providers" }).click();
    await expect(page.getByText("Sandbox Providers")).toBeVisible();
    const providerList = page
      .getByRole("dialog", { name: "Settings" })
      .getByRole("list")
      .filter({ has: page.getByText("E2E Local Docker") })
      .last();
    await expect(providerList.getByText("E2E Local Docker")).toBeVisible();
    await expect(
      providerList.getByText(/Driver: docker · default/),
    ).toBeVisible();
    await expect(providerList.getByText("E2E Remote Linux")).toBeVisible();
  });

  test("can create a mocked sandbox provider instance", async ({ page }) => {
    await installSettingsMockRoutes(page);
    await gotoApp(page);

    await page.getByRole("button", { name: "Settings" }).click();
    await page.getByRole("tab", { name: "Providers" }).click();
    await expect(page.getByText("Active provider instances")).toBeVisible();

    await page.getByRole("button", { name: "Add provider instance" }).click();
    await page.getByRole("button", { name: /Remote Linux/ }).click();
    await page.getByLabel("Name").fill("E2E Created Remote");
    await page.getByLabel("Endpoint").fill("https://created.example.test");
    await page.getByRole("button", { name: "Add provider instance" }).click();

    const providerList = page
      .getByRole("dialog", { name: "Settings" })
      .getByRole("list")
      .filter({ has: page.getByText("E2E Created Remote") })
      .last();
    await expect(providerList.getByText("E2E Created Remote")).toBeVisible();
  });
});
