import type { HighlightOptions as StreamdownHighlightOptions } from "@streamdown/code";
import type { Pluggable } from "unified";

export type MarkdownMode = "static" | "streaming";

export type HighlightToken = {
	bgColor?: string;
	color?: string;
	content: string;
	htmlAttrs?: Record<string, string>;
	htmlStyle?: Record<string, string>;
	offset?: number;
};

export type HighlightResult = {
	bg?: string;
	fg?: string;
	rootStyle?: string | false;
	tokens: HighlightToken[][];
};

export type CodeTheme = string | object;

export type CodeLanguage = StreamdownHighlightOptions["language"];

export type HighlightOptions = {
	code: string;
	language: CodeLanguage;
	themes: [CodeTheme, CodeTheme];
};

export type MathPlugin = {
	getStyles?: () => string;
	name: "katex";
	rehypePlugin: Pluggable;
	remarkPlugin: Pluggable;
	type: "math";
};

export type CjkPlugin = {
	name: "cjk";
	remarkPlugins: Pluggable[];
	remarkPluginsAfter: Pluggable[];
	remarkPluginsBefore: Pluggable[];
	type: "cjk";
};

export type CodeHighlighterPlugin = {
	getSupportedLanguages?: () => string[];
	getThemes?: () => [CodeTheme, CodeTheme];
	highlight(
		options: HighlightOptions,
		callback?: (result: HighlightResult) => void,
	): HighlightResult | null;
	name: "shiki";
	supportsLanguage?: (language: CodeLanguage) => boolean;
	type: "code-highlighter";
};

export type DiagramPlugin = {
	language: string;
	name: "mermaid";
	render(
		id: string,
		code: string,
		container: Element,
	): Promise<string> | string;
	type: "diagram";
};

export type MarkdownPluginConfig = {
	cjk?: CjkPlugin;
	code?: CodeHighlighterPlugin;
	math?: MathPlugin;
	mermaid?: DiagramPlugin;
};

export type PreprocessMarkdownOptions = {
	isAnimating?: boolean;
	mode?: MarkdownMode;
};

export type RenderMarkdownOptions = {
	isIncompleteCodeFence?: boolean;
	onLinkClick?: (url: string) => void;
	plugins?: MarkdownPluginConfig;
};
