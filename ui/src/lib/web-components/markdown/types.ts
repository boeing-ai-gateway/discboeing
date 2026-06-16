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

export type MarkdownLinkClickDetail = {
	href: string;
	text: string;
};

export type MarkdownImageDownloadDetail = {
	src: string;
	alt: string;
	suggestedFilename: string;
};

export type MarkdownRenderErrorDetail = {
	error: unknown;
	markdown: string;
};

export type DiscoMarkdownEventMap = {
	"disco-link-click": CustomEvent<MarkdownLinkClickDetail>;
	"disco-image-download": CustomEvent<MarkdownImageDownloadDetail>;
	"disco-render-error": CustomEvent<MarkdownRenderErrorDetail>;
};

export type RenderMarkdownOptions = {
	isIncompleteCodeFence?: boolean;
	onImageDownload?: (detail: MarkdownImageDownloadDetail) => boolean | void;
	onLinkClick?: (detail: MarkdownLinkClickDetail) => boolean | void;
	plugins?: MarkdownPluginConfig;
};

export interface DiscoMarkdownElement extends HTMLElement {
	markdown?: string;
	mode: MarkdownMode;
	isAnimating: boolean;
	plugins: MarkdownPluginConfig;
	setMarkdown(value: string): void;
	appendMarkdown(delta: string): void;
	addEventListener<K extends keyof DiscoMarkdownEventMap>(
		type: K,
		listener: (
			this: DiscoMarkdownElement,
			event: DiscoMarkdownEventMap[K],
		) => void,
		options?: boolean | AddEventListenerOptions,
	): void;
	addEventListener(
		type: string,
		listener: EventListenerOrEventListenerObject | null,
		options?: boolean | AddEventListenerOptions,
	): void;
	removeEventListener<K extends keyof DiscoMarkdownEventMap>(
		type: K,
		listener: (
			this: DiscoMarkdownElement,
			event: DiscoMarkdownEventMap[K],
		) => void,
		options?: boolean | EventListenerOptions,
	): void;
	removeEventListener(
		type: string,
		listener: EventListenerOrEventListenerObject | null,
		options?: boolean | EventListenerOptions,
	): void;
}

declare global {
	interface HTMLElementTagNameMap {
		"disco-markdown": DiscoMarkdownElement;
	}
}
