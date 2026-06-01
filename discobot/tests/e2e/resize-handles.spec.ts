import { expect, test, type Locator, type Page } from "@playwright/test";

import {
	composerPanel,
	editorPanel,
	hidePanel,
	openDefaultLayout,
	showPanel,
	terminalPanel,
} from "./helpers/layout";

const layoutResizePath = "/ui/commands/layout/resize";

const sessionsResizeHandle = (page: Page) => page.getByRole("separator", { name: "Resize sessions panel" });
const workspaceResizeHandle = (page: Page) => page.getByRole("separator", { name: "Resize session workspace panel" });
const terminalResizeHandle = (page: Page) => page.getByRole("separator", { name: "Resize terminal panel" });

const sizeOf = async (locator: Locator, axis: "x" | "y") => {
	const box = await locator.boundingBox();
	expect(box).not.toBeNull();
	return axis === "x" ? (box?.width ?? 0) : (box?.height ?? 0);
};

const dragHandle = async (page: Page, handle: Locator, deltaX: number, deltaY: number) => {
	const box = await handle.boundingBox();
	expect(box).not.toBeNull();
	const startX = (box?.x ?? 0) + (box?.width ?? 0) / 2;
	const startY = (box?.y ?? 0) + (box?.height ?? 0) / 2;

	await page.mouse.move(startX, startY);
	await page.mouse.down();
	await page.mouse.move(startX + deltaX, startY + deltaY, { steps: 4 });
	await page.mouse.up();
};

const expectResizeCommand = (page: Page, action: () => Promise<void>) =>
	Promise.all([
		page.waitForResponse((response) => response.url().includes(layoutResizePath) && response.status() === 204),
		action(),
	]);

test.beforeEach(async ({ page }) => {
	await openDefaultLayout(page);
});

test("drags the sessions panel resize handle", async ({ page }) => {
	const handle = sessionsResizeHandle(page);
	const panel = page.locator("#panel-session");
	await expect(handle).toBeVisible();
	await expect(handle).toHaveAttribute("aria-orientation", "vertical");

	const initialWidth = await sizeOf(panel, "x");
	await expectResizeCommand(page, () => dragHandle(page, handle, 80, 0));

	await expect(handle).toHaveAttribute("aria-valuenow", String(Math.round(initialWidth + 80)));
	await expect.poll(() => sizeOf(panel, "x")).toBeGreaterThan(initialWidth + 60);
});

test("supports keyboard resizing for the session workspace split", async ({ page }) => {
	await showPanel(page, "editor", editorPanel(page));
	await showPanel(page, "composer", composerPanel(page));
	const handle = workspaceResizeHandle(page);
	await expect(handle).toBeVisible();
	await expect(handle).toHaveAttribute("aria-orientation", "vertical");

	const initialWidth = await sizeOf(composerPanel(page), "x");
	await handle.focus();
	await expectResizeCommand(page, async () => {
		await page.keyboard.press("ArrowRight");
	});

	await expect(handle).toHaveAttribute("aria-valuenow", String(Math.round(initialWidth + 16)));
	await expect.poll(() => sizeOf(composerPanel(page), "x")).toBeGreaterThan(initialWidth + 8);
});

test("drags the terminal resize handle in reverse vertical direction", async ({ page }) => {
	await showPanel(page, "editor", editorPanel(page));
	await showPanel(page, "terminal", terminalPanel(page));
	const handle = terminalResizeHandle(page);
	await expect(handle).toBeVisible();
	await expect(handle).toHaveAttribute("aria-orientation", "horizontal");

	const initialHeight = await sizeOf(terminalPanel(page), "y");
	await expectResizeCommand(page, () => dragHandle(page, handle, 0, -64));

	await expect(handle).toHaveAttribute("aria-valuenow", String(Math.round(initialHeight + 64)));
	await expect.poll(() => sizeOf(terminalPanel(page), "y")).toBeGreaterThan(initialHeight + 48);
});

test("clamps dragged panel sizes to declared bounds", async ({ page }) => {
	const handle = sessionsResizeHandle(page);
	const panel = page.locator("#panel-session");
	await expect(handle).toBeVisible();

	await expectResizeCommand(page, () => dragHandle(page, handle, 1000, 0));
	await expect(handle).toHaveAttribute("aria-valuenow", "460");
	await expect.poll(() => sizeOf(panel, "x")).toBeLessThanOrEqual(461);

	await expectResizeCommand(page, () => dragHandle(page, handle, -1000, 0));
	await expect(handle).toHaveAttribute("aria-valuenow", "220");
	await expect.poll(() => sizeOf(panel, "x")).toBeLessThanOrEqual(221);
});

test("hides inactive resize handles", async ({ page }) => {
	await showPanel(page, "editor", editorPanel(page));
	await showPanel(page, "terminal", terminalPanel(page));
	await expect(sessionsResizeHandle(page)).toBeVisible();
	await expect(workspaceResizeHandle(page)).toBeVisible();
	await expect(terminalResizeHandle(page)).toBeVisible();

	await hidePanel(page, "terminal", terminalPanel(page));
	await expect(terminalResizeHandle(page)).toBeHidden();

	await hidePanel(page, "editor", editorPanel(page));
	await expect(workspaceResizeHandle(page)).toBeHidden();

	await page.getByRole("button", { name: "Maximize Session Workspace panel" }).click();
	await expect(composerPanel(page)).toHaveClass(/panel--maximized/);
	await expect(sessionsResizeHandle(page)).toBeHidden();
});
