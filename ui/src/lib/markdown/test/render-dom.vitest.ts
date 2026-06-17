import { expect, test } from "vitest";

import { parseMarkdownToHast } from "../pipeline";
import { renderMarkdownTree } from "../render-dom";
import type { CodeHighlighterPlugin, RenderMarkdownOptions } from "../types";

function renderMarkdown(
	markdown: string,
	options: RenderMarkdownOptions = {},
): HTMLDivElement {
	const container = document.createElement("div");
	container.append(renderMarkdownTree(parseMarkdownToHast(markdown), options));
	return container;
}

async function waitForRender() {
	await new Promise((resolve) => window.setTimeout(resolve, 20));
}

test("renderMarkdownTree keeps loose ordered and unordered list markers outside the content block", () => {
	const container =
		renderMarkdown(`1. **Restart during question phase with no cached snapshot at all**

   - This should now be fine because only persisted history exists.
`);

	const orderedList = container.querySelector("ol");
	expect(orderedList).toBeTruthy();
	expect(orderedList!.className).toMatch(/list-outside/);
	expect(orderedList!.className).toMatch(/list-decimal/);
	expect(orderedList!.className).toMatch(/pl-6/);
	expect(orderedList!.className).not.toMatch(/list-inside/);

	const unorderedList = container.querySelector("ul");
	expect(unorderedList).toBeTruthy();
	expect(unorderedList!.className).toMatch(/list-outside/);
	expect(unorderedList!.className).toMatch(/list-disc/);
	expect(unorderedList!.className).toMatch(/pl-6/);
	expect(unorderedList!.className).not.toMatch(/list-inside/);

	const orderedListItem = orderedList!.querySelector("li");
	expect(orderedListItem?.querySelector("p")).toBeTruthy();
});

test("renderMarkdownTree omits the default text label for unlabeled code fences", () => {
	const container = renderMarkdown("```\nplain text\n```");
	const header = container.querySelector(
		'[data-streamdown="code-block-header"]',
	);

	expect(header).toBeTruthy();
	expect(header?.textContent?.trim()).toBe("");
});

test("renderMarkdownTree shows explicit code fence languages", () => {
	const container = renderMarkdown("```yaml\nkey: value\n```");
	const header = container.querySelector(
		'[data-streamdown="code-block-header"]',
	);

	expect(header).toBeTruthy();
	expect(header?.textContent?.trim()).toBe("yaml");
});

test("renderMarkdownTree renders YAML front matter as a table", () => {
	const container = renderMarkdown(`---
title: Release notes
draft: false
tags:
  - ui
  - markdown
metadata:
  owner: discobot
---

# Hello`);

	const table = container.querySelector("table");
	expect(table).toBeTruthy();

	const rows = Array.from(table?.querySelectorAll("tr") ?? []).map((row) =>
		Array.from(row.querySelectorAll("th, td")).map(
			(cell) => cell.textContent?.trim() ?? "",
		),
	);

	expect(rows).toEqual([
		["Field", "Value"],
		["title", "Release notes"],
		["draft", "false"],
		["tags", "ui, markdown"],
		["metadata.owner", "discobot"],
	]);

	const heading = container.querySelector("h1");
	expect(heading).toBeTruthy();
	expect(heading?.textContent?.trim()).toBe("Hello");
});

test("renderMarkdownTree falls back to a YAML code block for invalid front matter", () => {
	const container = renderMarkdown(`---
title: [broken
---`);
	const header = container.querySelector(
		'[data-streamdown="code-block-header"]',
	);
	const codeBlock = container.querySelector(
		'[data-streamdown="code-block-body"]',
	);

	expect(header).toBeTruthy();
	expect(header?.textContent?.trim()).toBe("yaml");
	expect(codeBlock).toBeTruthy();
	expect(codeBlock?.textContent ?? "").toMatch(/title: \[broken/);
});

test("renderMarkdownTree falls back to plain text for unsupported code fence languages", () => {
	const unsupportedCodePlugin: CodeHighlighterPlugin = {
		getSupportedLanguages: () => ["typescript"],
		getThemes: () => ["github-light", "github-dark"],
		highlight: () => {
			throw new Error(
				"highlight should not be called for unsupported languages",
			);
		},
		name: "shiki",
		supportsLanguage: () => false,
		type: "code-highlighter",
	};

	const container = renderMarkdown("```gitignore\ndist/\n.env\n```", {
		plugins: { code: unsupportedCodePlugin },
	});
	const header = container.querySelector(
		'[data-streamdown="code-block-header"]',
	);
	const codeBlock = container.querySelector(
		'[data-streamdown="code-block-body"]',
	);

	expect(header).toBeTruthy();
	expect(header?.textContent?.trim()).toBe("gitignore");
	expect(codeBlock).toBeTruthy();
	expect(codeBlock?.textContent ?? "").toMatch(/dist\//);
	expect(codeBlock?.textContent ?? "").toMatch(/\.env/);
});

test("renderMarkdownTree avoids content visibility on incomplete code fences", () => {
	const container = renderMarkdown("```javascript\nconst value = 1;", {
		isIncompleteCodeFence: true,
	});
	const codeBlock = container.querySelector<HTMLElement>(
		'[data-streamdown="code-block"]',
	);

	expect(codeBlock).toBeTruthy();
	expect(codeBlock?.dataset.incomplete).toBe("true");
	expect(codeBlock?.style.contentVisibility).toBe("");
	expect(codeBlock?.style.containIntrinsicSize).toBe("");
});

test("renderMarkdownTree uses content visibility on complete code fences", () => {
	const container = renderMarkdown("```javascript\nconst value = 1;\n```");
	const codeBlock = container.querySelector<HTMLElement>(
		'[data-streamdown="code-block"]',
	);

	expect(codeBlock).toBeTruthy();
	expect(codeBlock?.style.contentVisibility).toBe("auto");
	expect(codeBlock?.style.containIntrinsicSize).toBe("auto 200px");
});

test("renderMarkdownTree renders Mermaid fences with the Mermaid plugin", async () => {
	const container = renderMarkdown("```mermaid\ngraph TD\nA-->B\n```", {
		plugins: {
			mermaid: {
				language: "mermaid",
				name: "mermaid",
				render: async (id, code) =>
					`<svg id="${id}" role="img"><text>${code}</text></svg>`,
				type: "diagram",
			},
		},
	});
	document.body.append(container);
	const diagram = container.querySelector<HTMLElement>(
		'[data-streamdown="mermaid"]',
	);

	expect(diagram).toBeTruthy();
	expect(diagram?.textContent).toContain("Rendering diagram");

	await waitForRender();

	const svg = diagram?.querySelector("svg");
	expect(svg).toBeTruthy();
	expect(svg?.id).toMatch(/^discobot-mermaid-/);
	expect(svg?.textContent).toContain("graph TD");
	container.remove();
});

test("renderMarkdownTree renders incomplete Mermaid fences as code blocks", () => {
	const container = renderMarkdown("```mermaid\ngraph TD\nA-->B", {
		isIncompleteCodeFence: true,
		plugins: {
			mermaid: {
				language: "mermaid",
				name: "mermaid",
				render: () => {
					throw new Error("render should not be called");
				},
				type: "diagram",
			},
		},
	});
	const codeBlock = container.querySelector<HTMLElement>(
		'[data-streamdown="code-block"]',
	);

	expect(codeBlock).toBeTruthy();
	expect(codeBlock?.dataset.language).toBe("mermaid");
});

test("renderMarkdownTree clears Mermaid render output after failures", async () => {
	const container = renderMarkdown("```mermaid\nbroken diagram\n```", {
		plugins: {
			mermaid: {
				language: "mermaid",
				name: "mermaid",
				render: async (_id, _code, container) => {
					container.innerHTML = "<svg><text>Syntax error in text</text></svg>";
					throw new Error("Mermaid syntax error");
				},
				type: "diagram",
			},
		},
	});
	document.body.append(container);
	const diagram = container.querySelector<HTMLElement>(
		'[data-streamdown="mermaid"]',
	);

	await waitForRender();

	expect(diagram?.querySelector("svg")).toBeNull();
	expect(diagram?.textContent).toContain("Unable to render Mermaid diagram.");
	expect(diagram?.textContent).toContain("Mermaid syntax error");
	expect(diagram?.textContent).not.toContain("Syntax error in text");
	container.remove();
});

test("renderMarkdownTree waits to render Mermaid until the container is attached", async () => {
	const renderStates: boolean[] = [];
	const fragment = renderMarkdownTree(
		parseMarkdownToHast("```mermaid\ngraph TD\nA-->B\n```"),
		{
			plugins: {
				mermaid: {
					language: "mermaid",
					name: "mermaid",
					render: async (_id, _code, container) => {
						renderStates.push(container.isConnected);
						return "<svg></svg>";
					},
					type: "diagram",
				},
			},
		},
	);

	await waitForRender();
	expect(renderStates).toEqual([]);

	const container = document.createElement("div");
	document.body.append(container);
	container.append(fragment);
	await waitForRender();

	expect(renderStates).toEqual([true]);
	container.remove();
});
