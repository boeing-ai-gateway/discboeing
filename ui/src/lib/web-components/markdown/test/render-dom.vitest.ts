import { expect, test } from "vitest";

import { parseMarkdownToHast } from "../pipeline";
import { renderMarkdownTree } from "../render-dom";
import type { RenderMarkdownOptions } from "../types";

function renderMarkdown(
	markdown: string,
	options: RenderMarkdownOptions = {},
): HTMLDivElement {
	const container = document.createElement("div");
	container.append(
		renderMarkdownTree(parseMarkdownToHast(markdown, options.plugins), options),
	);
	return container;
}

async function waitForRender() {
	await new Promise((resolve) => window.setTimeout(resolve, 20));
}

test("renderMarkdownTree removes Mermaid body artifacts after failures", async () => {
	const container = renderMarkdown("```mermaid\nbroken diagram\n```", {
		plugins: {
			mermaid: {
				language: "mermaid",
				name: "mermaid",
				render: async (id) => {
					const artifact = document.createElement("div");
					artifact.id = `d${id}`;
					artifact.innerHTML = "<svg><text>Syntax error in text</text></svg>";
					document.body.append(artifact);
					throw new Error("Mermaid syntax error");
				},
				type: "diagram",
			},
		},
	});
	document.body.append(container);

	await waitForRender();

	expect(document.body.querySelector('[id^="ddiscboeing-mermaid-"]')).toBeNull();
	expect(document.body.textContent).not.toContain("Syntax error in text");
	expect(container.textContent).toContain("Unable to render Mermaid diagram.");
	expect(container.textContent).toContain("Mermaid syntax error");
	container.remove();
});
