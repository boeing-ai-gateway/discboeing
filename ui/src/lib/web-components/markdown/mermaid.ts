import mermaid from "mermaid";
import type { DiagramPlugin } from "./types";

mermaid.initialize({
	htmlLabels: false,
	securityLevel: "strict",
	startOnLoad: false,
});

function createRenderContainer() {
	const container = document.createElement("div");
	container.style.height = "0";
	container.style.overflow = "hidden";
	container.style.position = "absolute";
	container.style.visibility = "hidden";
	container.style.width = "0";
	document.body.append(container);
	return container;
}

export const mermaidPlugin: DiagramPlugin = {
	language: "mermaid",
	name: "mermaid",
	render: async (id, code) => {
		const container = createRenderContainer();
		try {
			const result = await mermaid.render(id, code, container);
			return result.svg;
		} finally {
			container.remove();
		}
	},
	type: "diagram",
};
