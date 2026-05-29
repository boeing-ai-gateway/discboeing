import { expect, test, type Locator, type Page } from "@playwright/test";

import {
	composerPanel,
	editorPanel,
	maximizePanel,
	openDefaultLayout,
	restoreMaximizedPanel,
	sessionPanel,
	setPanelVisible,
	setSessionsPanelVisible,
	terminalPanel,
} from "./helpers/layout";

type PanelID = "editor" | "terminal" | "composer";
type PanelState = "hidden" | "visible" | "maximized";

type MatrixCase = {
	name: string;
	sessionVisible: boolean;
	panels: Record<PanelID, PanelState>;
	maximizedPanel: PanelID | null;
};

const panelOrder: PanelID[] = ["editor", "terminal", "composer"];
const panelLabels: Record<PanelID, string> = {
	editor: "Editor",
	terminal: "Terminal",
	composer: "Composer",
};

const panelLocators: Record<PanelID, (page: Page) => Locator> = {
	editor: editorPanel,
	terminal: terminalPanel,
	composer: composerPanel,
};

const binaryPanelStates = (maximizedPanel: PanelID | null): Record<PanelID, PanelState>[] => {
	const otherPanels = panelOrder.filter((panel) => panel !== maximizedPanel);
	const states: Record<PanelID, PanelState>[] = [];

	for (let mask = 0; mask < 1 << otherPanels.length; mask++) {
		const panels = Object.fromEntries(
			panelOrder.map((panel) => [panel, "hidden"]),
		) as Record<PanelID, PanelState>;
		for (const [index, panel] of otherPanels.entries()) {
			panels[panel] = mask & (1 << index) ? "visible" : "hidden";
		}
		if (maximizedPanel) {
			panels[maximizedPanel] = "maximized";
		}
		states.push(panels);
	}

	return states;
};

const matrixCases = (): MatrixCase[] => {
	const cases: MatrixCase[] = [];
	for (const sessionVisible of [false, true]) {
		for (const panels of binaryPanelStates(null)) {
			if (panelOrder.every((panel) => panels[panel] === "hidden")) {
				continue;
			}
			cases.push({
				name: caseName(sessionVisible, panels),
				sessionVisible,
				panels,
				maximizedPanel: null,
			});
		}
		for (const maximizedPanel of panelOrder) {
			for (const panels of binaryPanelStates(maximizedPanel)) {
				cases.push({
					name: caseName(sessionVisible, panels),
					sessionVisible,
					panels,
					maximizedPanel,
				});
			}
		}
	}
	return cases;
};

const caseName = (sessionVisible: boolean, panels: Record<PanelID, PanelState>) => {
	const session = sessionVisible ? "session-open" : "session-closed";
	const panelState = panelOrder.map((panel) => `${panel}-${panels[panel]}`).join("__");
	return `${session}__${panelState}`;
};

const setMatrixCase = async (page: Page, matrixCase: MatrixCase) => {
	await restoreMaximizedPanel(page);
	await setSessionsPanelVisible(page, matrixCase.sessionVisible);

	for (const panel of panelOrder) {
		if (matrixCase.panels[panel] === "hidden") {
			continue;
		}
		await setPanelVisible(
			page,
			panel,
			panelLocators[panel](page),
			true,
		);
	}

	for (const panel of panelOrder) {
		if (matrixCase.panels[panel] !== "hidden") {
			continue;
		}
		await setPanelVisible(
			page,
			panel,
			panelLocators[panel](page),
			false,
		);
	}

	if (matrixCase.maximizedPanel) {
		await maximizePanel(
			page,
			panelLabels[matrixCase.maximizedPanel],
			panelLocators[matrixCase.maximizedPanel](page),
		);
	}
};

const expectMatrixCase = async (page: Page, matrixCase: MatrixCase) => {
	if (matrixCase.maximizedPanel) {
		await expect(
			page.getByRole("button", {
				name: `Restore ${panelLabels[matrixCase.maximizedPanel]} panel`,
			}).locator("svg"),
		).toHaveClass(/lucide-minimize-2/);
	} else {
		await expect(page.locator(".app-workspace-grid > .panel--maximized")).toHaveCount(0);
	}

	for (const panel of panelOrder) {
		const locator = panelLocators[panel](page);
		const state = matrixCase.panels[panel];

		if (matrixCase.maximizedPanel === panel) {
			await expect(locator).toBeVisible();
			await expect(locator).toHaveClass(/panel--maximized/);
			continue;
		}

		if (matrixCase.maximizedPanel) {
			await expect(locator).toBeHidden();
			continue;
		}

		if (state === "visible") {
			await expect(locator).toBeVisible();
		} else {
			await expect(locator).toBeHidden();
		}
	}

	if (matrixCase.maximizedPanel) {
		await expect(sessionPanel(page)).toBeHidden();
	} else if (matrixCase.sessionVisible) {
		await expect(sessionPanel(page)).toBeVisible();
	} else {
		await expect(sessionPanel(page)).toBeHidden();
	}

	await expectNoWorkspaceHoles(page, matrixCase);
};

const expectNoWorkspaceHoles = async (page: Page, matrixCase: MatrixCase) => {
	const hasWorkspacePanel = panelOrder.some((panel) => matrixCase.panels[panel] !== "hidden");
	if (!hasWorkspacePanel) {
		return;
	}

	const holes = await page.evaluate(({ maximizedPanel }) => {
		const grid = document.querySelector(".app-workspace-grid");
		if (!grid) {
			return ["missing workspace grid"];
		}

		const gridRect = grid.getBoundingClientRect();
		const gridStyle = window.getComputedStyle(grid);
		const content = {
			left: gridRect.left + Number.parseFloat(gridStyle.paddingLeft),
			top: gridRect.top + Number.parseFloat(gridStyle.paddingTop),
			right: gridRect.right - Number.parseFloat(gridStyle.paddingRight),
			bottom: gridRect.bottom - Number.parseFloat(gridStyle.paddingBottom),
		};

		if (!maximizedPanel) {
			const session = document.querySelector("#panel-session");
			if (session && window.getComputedStyle(session).display !== "none") {
				content.left = Math.max(content.left, session.getBoundingClientRect().right + 10);
			}
		}

		const width = content.right - content.left;
		const height = content.bottom - content.top;
		if (width <= 0 || height <= 0) {
			return [];
		}

		const coveredRects = Array.from(
			document.querySelectorAll(".app-workspace-grid > .panel, [data-discobot-resize]"),
		)
			.filter((element) => window.getComputedStyle(element).display !== "none")
			.map((element) => element.getBoundingClientRect())
			.filter((rect) => rect.width > 0 && rect.height > 0);

		const xEdges = [content.left, content.right];
		const yEdges = [content.top, content.bottom];
		for (const rect of coveredRects) {
			xEdges.push(
				Math.max(content.left, Math.min(content.right, rect.left)),
				Math.max(content.left, Math.min(content.right, rect.right)),
			);
			yEdges.push(
				Math.max(content.top, Math.min(content.bottom, rect.top)),
				Math.max(content.top, Math.min(content.bottom, rect.bottom)),
			);
		}

		const sortedXEdges = [...new Set(xEdges.map((edge) => Math.round(edge)))].sort((a, b) => a - b);
		const sortedYEdges = [...new Set(yEdges.map((edge) => Math.round(edge)))].sort((a, b) => a - b);
		const uncovered: string[] = [];
		for (let xIndex = 0; xIndex < sortedXEdges.length - 1; xIndex++) {
			for (let yIndex = 0; yIndex < sortedYEdges.length - 1; yIndex++) {
				const left = sortedXEdges[xIndex];
				const right = sortedXEdges[xIndex + 1];
				const top = sortedYEdges[yIndex];
				const bottom = sortedYEdges[yIndex + 1];
				if (right - left <= 1 || bottom - top <= 1) {
					continue;
				}

				const x = left + (right - left) / 2;
				const y = top + (bottom - top) / 2;
				const covered = coveredRects.some((rect) =>
					x >= rect.left - 1 &&
					x <= rect.right + 1 &&
					y >= rect.top - 1 &&
					y <= rect.bottom + 1
				);
				if (!covered) {
					uncovered.push(`${left},${top} ${right - left}x${bottom - top}`);
				}
			}
		}

		return uncovered;
	}, { maximizedPanel: matrixCase.maximizedPanel });

	expect(holes, `workspace holes for ${matrixCase.name}`).toEqual([]);
};

test.describe("workspace panel state matrix", () => {
	test.beforeEach(async ({ page }) => {
		await openDefaultLayout(page);
	});

	for (const matrixCase of matrixCases()) {
		test(matrixCase.name, async ({ page }) => {
			await setMatrixCase(page, matrixCase);
			await expectMatrixCase(page, matrixCase);
		});
	}
});
