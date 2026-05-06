import { expect, test } from "vitest";

import { parseMarkdownToHast } from "./pipeline";
import { renderMarkdownTree } from "./render-dom";
import type { CodeHighlighterPlugin, RenderMarkdownOptions } from "./types";

function renderMarkdown(
	markdown: string,
	options: RenderMarkdownOptions = {},
): HTMLDivElement {
	const container = document.createElement("div");
	container.append(renderMarkdownTree(parseMarkdownToHast(markdown), options));
	return container;
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
