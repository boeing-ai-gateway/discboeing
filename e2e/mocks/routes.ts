import type { Page, Route } from "@playwright/test";
import { FakeDiscobotApi } from "./fake-api";

function jsonHeaders(route: Route) {
  return {
    "access-control-allow-credentials": "true",
    "access-control-allow-headers": "content-type, authorization",
    "access-control-allow-methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
    "access-control-allow-origin":
      route.request().headers().origin ?? "http://localhost:3100",
    "content-type": "application/json",
  };
}

export async function installApiRoutes(page: Page, api: FakeDiscobotApi) {
  await page.route("**/api/**", async (route) => fulfillApiRoute(route, api));
  await page.route("**/auth/**", async (route) => fulfillApiRoute(route, api));
}

async function fulfillApiRoute(route: Route, api: FakeDiscobotApi) {
  const request = route.request();
  const url = new URL(request.url());
  const { status = 200, body } = api.handle(request.method(), url);
  await route.fulfill({
    status,
    headers: jsonHeaders(route),
    body: body === null ? "" : JSON.stringify(body),
  });
}
