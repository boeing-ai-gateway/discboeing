import mermaid from "mermaid";
import type { DiagramPlugin } from "./types";

mermaid.initialize({
	htmlLabels: false,
	securityLevel: "strict",
	startOnLoad: false,
});

export const mermaidPlugin: DiagramPlugin = {
	language: "mermaid",
	name: "mermaid",
	render: async (id, code, container) => {
		const result = await mermaid.render(id, code, container);
		return result.svg;
	},
	type: "diagram",
};
