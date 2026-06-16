<svelte:options
	customElement={{
		tag: "disco-markdown",
		props: {
			isAnimating: { attribute: "is-animating", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import { cjk } from "@streamdown/cjk";
	import { code } from "@streamdown/code";
	import { math } from "@streamdown/math";
	import { getCustomElementHost, emitComposedEvent } from "./dom";
	import { mermaidPlugin } from "./mermaid";
	import { renderMarkdownBlocks } from "./render-blocks";
	import type {
		MarkdownImageDownloadDetail,
		MarkdownLinkClickDetail,
		MarkdownMode,
		MarkdownPluginConfig,
		DiscoMarkdownElement,
	} from "./types";

	type Props = {
		markdown?: string;
		mode?: MarkdownMode;
		isAnimating?: boolean;
		plugins?: MarkdownPluginConfig;
	};

	let {
		markdown,
		mode = "static",
		isAnimating = false,
		plugins = { code, math, cjk, mermaid: mermaidPlugin },
	}: Props = $props();

	let host = $state<HTMLDivElement | null>(null);
	let renderedBlocks: string[] = [];
	let slottedMarkdown = $state("");
	let initializedFromSlot = false;

	function getHost(): DiscoMarkdownElement | null {
		return host ? getCustomElementHost<DiscoMarkdownElement>(host) : null;
	}

	function readSlottedMarkdown() {
		initializedFromSlot = true;
		slottedMarkdown = getHost()?.textContent ?? "";
	}

	function getMarkdown(): string {
		if (markdown !== undefined) {
			return markdown;
		}
		if (!initializedFromSlot) {
			readSlottedMarkdown();
		}
		return slottedMarkdown;
	}

	function dispatchCancelableEvent<T>(type: string, detail: T): boolean {
		const element = getHost();
		if (!element) {
			return true;
		}
		return emitComposedEvent(element, type, detail, { cancelable: true });
	}

	function render() {
		if (!host) {
			return;
		}
		renderedBlocks = renderMarkdownBlocks(host, getMarkdown(), renderedBlocks, {
			isAnimating,
			mode,
			onImageDownload: (detail: MarkdownImageDownloadDetail) =>
				dispatchCancelableEvent("disco-image-download", detail),
			onLinkClick: (detail: MarkdownLinkClickDetail) =>
				dispatchCancelableEvent("disco-link-click", detail),
			onRenderError: (detail) => {
				const element = getHost();
				if (!element) {
					return;
				}
				emitComposedEvent(element, "disco-render-error", detail);
			},
			plugins,
		});
	}

	function setMarkdown(value: string) {
		markdown = value;
		slottedMarkdown = value;
		render();
	}

	function appendMarkdown(delta: string) {
		const nextMarkdown = `${getMarkdown()}${delta}`;
		markdown = nextMarkdown;
		slottedMarkdown = nextMarkdown;
		render();
	}

	function syncResolvedTheme(element: DiscoMarkdownElement) {
		const isDark =
			element.closest(".dark") !== null ||
			document.documentElement.classList.contains("dark") ||
			document.body.classList.contains("dark");

		element.dataset.resolvedTheme = isDark ? "dark" : "light";
	}

	$effect(() => {
		const element = getHost();
		if (!element) {
			return;
		}
		element.setMarkdown = setMarkdown;
		element.appendMarkdown = appendMarkdown;
	});

	$effect(() => {
		const element = getHost();
		if (!element) {
			return;
		}

		syncResolvedTheme(element);

		const observer = new MutationObserver(() => syncResolvedTheme(element));
		observer.observe(document.documentElement, {
			attributeFilter: ["class"],
			attributes: true,
		});
		observer.observe(document.body, {
			attributeFilter: ["class"],
			attributes: true,
		});

		return () => observer.disconnect();
	});

	$effect(() => {
		const element = getHost();
		if (!element || markdown !== undefined) {
			return;
		}
		readSlottedMarkdown();
		const observer = new MutationObserver(() => readSlottedMarkdown());
		observer.observe(element, {
			characterData: true,
			childList: true,
			subtree: true,
		});
		return () => observer.disconnect();
	});

	$effect(() => {
		if (!host) {
			return;
		}
		render();
	});
</script>

<div part="root" class="markdown-root" bind:this={host}></div>

<style>
	:host {
		--disco-markdown-background: var(
			--disco-background,
			var(--background, #ffffff)
		);
		--disco-markdown-foreground: var(
			--disco-foreground,
			var(--foreground, #111827)
		);
		--disco-markdown-muted: var(--disco-muted, var(--muted, #f3f4f6));
		--disco-markdown-muted-foreground: var(
			--disco-muted-foreground,
			var(--muted-foreground, #6b7280)
		);
		--disco-markdown-border: var(--disco-border, var(--border, #e5e7eb));
		--disco-markdown-primary: var(--disco-primary, var(--primary, #2563eb));
		--disco-markdown-code-background: var(
			--disco-sidebar,
			var(--sidebar, #f9fafb)
		);
		--disco-markdown-radius: var(--disco-radius, 0.75rem);
		--disco-markdown-font-sans: var(
			--disco-font-sans,
			var(--font-sans, system-ui, sans-serif)
		);
		--disco-markdown-font-mono: var(
			--disco-font-mono,
			var(
				--font-mono,
				ui-monospace,
				SFMono-Regular,
				Menlo,
				Monaco,
				Consolas,
				monospace
			)
		);
		display: block;
		width: 100%;
		max-width: 100%;
		min-width: 0;
		color: var(--disco-markdown-foreground);
		font-family: var(--disco-markdown-font-sans);
	}

	*,
	*::before,
	*::after {
		box-sizing: border-box;
	}

	.markdown-root {
		width: 100%;
		max-width: 100%;
		min-width: 0;
		white-space: normal;
	}

	:global(.markdown-block) {
		max-width: 100%;
		min-width: 0;
	}

	:global(.markdown-block + .markdown-block) {
		margin-top: 1rem;
	}

	:global(.markdown-fallback) {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	:global(.markdown-root h1),
	:global(.markdown-root h2),
	:global(.markdown-root h3),
	:global(.markdown-root h4),
	:global(.markdown-root h5),
	:global(.markdown-root h6) {
		margin: 1.5rem 0 0.5rem;
		font-weight: 600;
		line-height: 1.25;
	}

	:global(.markdown-root h1) {
		font-size: 1.875rem;
	}
	:global(.markdown-root h2) {
		font-size: 1.5rem;
	}
	:global(.markdown-root h3) {
		font-size: 1.25rem;
	}
	:global(.markdown-root h4) {
		font-size: 1.125rem;
	}
	:global(.markdown-root h5) {
		font-size: 1rem;
	}
	:global(.markdown-root h6) {
		font-size: 0.875rem;
	}

	:global(.markdown-root p) {
		margin: 0.75rem 0;
	}

	:global(.markdown-root a),
	:global(.markdown-root button.wrap-anywhere) {
		appearance: none;
		border: 0;
		background: transparent;
		padding: 0;
		color: var(--disco-markdown-primary);
		cursor: pointer;
		font: inherit;
		font-weight: 500;
		text-align: left;
		text-decoration: underline;
		overflow-wrap: anywhere;
	}

	:global(.markdown-root button:disabled) {
		cursor: default;
		opacity: 0.6;
	}

	:global(.markdown-root blockquote) {
		margin: 1rem 0;
		border-left: 4px solid
			color-mix(
				in srgb,
				var(--disco-markdown-muted-foreground) 30%,
				transparent
			);
		padding-left: 1rem;
		color: var(--disco-markdown-muted-foreground);
		font-style: italic;
	}

	:global(.markdown-root code) {
		border-radius: 0.375rem;
		background: var(--disco-markdown-muted);
		padding: 0.125rem 0.375rem;
		font-family: var(--disco-markdown-font-mono);
		font-size: 0.875em;
	}

	:global(.markdown-root pre code) {
		background: transparent;
		padding: 0;
		font-size: inherit;
	}

	:global(.markdown-root ul),
	:global(.markdown-root ol) {
		margin: 0.75rem 0;
		padding-left: 1.5rem;
	}

	:global(.markdown-root li) {
		padding: 0.25rem 0;
	}

	:global(.markdown-root hr) {
		margin: 1.5rem 0;
		border: 0;
		border-top: 1px solid var(--disco-markdown-border);
	}

	:global(.markdown-root table) {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.875rem;
	}

	:global(.markdown-root th),
	:global(.markdown-root td) {
		border-bottom: 1px solid var(--disco-markdown-border);
		padding: 0.5rem;
		text-align: left;
		vertical-align: middle;
		white-space: nowrap;
	}

	:global(.markdown-root th) {
		font-weight: 500;
	}

	:global(.markdown-root [data-streamdown="code-block"]) {
		box-sizing: border-box;
		margin: 1rem 0;
		display: flex;
		width: auto;
		max-width: 100%;
		min-width: 0;
		overflow: hidden;
		flex-direction: column;
		border: 1px solid var(--disco-markdown-border);
		border-radius: var(--disco-markdown-radius);
		background: var(--disco-markdown-code-background);
		padding: 0.5rem;
	}

	:global(.markdown-root [data-streamdown="code-block-body-container"]) {
		max-width: 100%;
		min-width: 0;
	}

	:global(.markdown-root [data-streamdown="code-block-header"]) {
		display: flex;
		height: 2rem;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		min-width: 0;
		padding: 0 0.25rem 0.5rem;
		color: var(--disco-markdown-muted-foreground);
		font-size: 0.75rem;
	}

	:global(.markdown-root [data-streamdown="code-block-title"]) {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-family: var(--disco-markdown-font-mono);
		text-transform: lowercase;
	}

	:global(.markdown-root [data-streamdown="code-block-body"]) {
		box-sizing: border-box;
		max-width: 100%;
		min-width: 0;
		overflow-x: auto;
		border: 1px solid var(--disco-markdown-border);
		border-radius: 0.375rem;
		background: var(--disco-markdown-background);
		padding: 1rem;
		font-family: var(--disco-markdown-font-mono);
		font-size: 0.875rem;
	}

	:global(.markdown-root [data-streamdown="code-block-body"] pre) {
		margin: 0;
		background: var(--shiki-light-bg, var(--sdm-bg, transparent));
		color: var(--shiki-light, var(--sdm-fg, inherit));
		font-family: inherit;
	}

	:host([data-resolved-theme="dark"])
		:global(.markdown-root [data-streamdown="code-block-body"] pre),
	:host([data-theme="dark"])
		:global(.markdown-root [data-streamdown="code-block-body"] pre) {
		background: var(--shiki-dark-bg, var(--sdm-bg, transparent));
		color: var(--shiki-dark, var(--sdm-fg, inherit));
	}

	:global(.markdown-root [data-streamdown="code-block-body"] code) {
		display: block;
		background: transparent;
		padding: 0;
		font-size: inherit;
		line-height: 1.5;
	}

	:global(.markdown-root [data-streamdown="code-line"]) {
		display: block;
		white-space: pre;
	}

	:global(.markdown-root [data-streamdown="code-token"]) {
		background: var(--shiki-light-bg, var(--sdm-tbg, transparent));
		color: var(--shiki-light, var(--sdm-c, inherit));
	}

	:host([data-resolved-theme="dark"])
		:global(.markdown-root [data-streamdown="code-token"]),
	:host([data-theme="dark"])
		:global(.markdown-root [data-streamdown="code-token"]) {
		background: var(--shiki-dark-bg, var(--sdm-tbg, transparent));
		color: var(--shiki-dark, var(--sdm-c, inherit));
	}

	:global(.markdown-root [data-streamdown="code-line-number"]) {
		display: inline-block;
		width: 1.5rem;
		margin-right: 1rem;
		user-select: none;
		text-align: right;
		font-family: var(--disco-markdown-font-mono);
		font-size: 13px;
		color: color-mix(
			in srgb,
			var(--disco-markdown-muted-foreground) 50%,
			transparent
		);
	}

	:global(.markdown-root [data-streamdown="code-block-actions-row"]) {
		display: flex;
		flex: 0 0 auto;
		align-items: center;
		justify-content: flex-end;
	}

	:global(.markdown-root [data-streamdown="code-block-actions"]) {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		gap: 0.5rem;
	}

	:global(.markdown-root [data-streamdown="image-wrapper"] button) {
		border: 1px solid var(--disco-markdown-border);
		border-radius: 0.375rem;
		background: var(--disco-markdown-background);
		color: var(--disco-markdown-foreground);
		cursor: pointer;
		font: inherit;
		font-size: 0.75rem;
	}

	:global(.markdown-root [data-streamdown="copy-button"]) {
		display: inline-flex;
		min-height: 1.625rem;
		align-items: center;
		justify-content: center;
		border: 0;
		border-radius: 0.375rem;
		background: transparent;
		color: var(--disco-markdown-foreground);
		cursor: pointer;
		font: inherit;
		font-size: 0.75rem;
		padding: 0.25rem 0.5rem;
		line-height: 1;
		white-space: nowrap;
		box-shadow: none;
	}

	:global(.markdown-root [data-streamdown="copy-button"]:hover),
	:global(.markdown-root [data-streamdown="copy-button"]:focus-visible) {
		background: var(--disco-markdown-muted);
	}

	:global(.markdown-root [data-streamdown="image-wrapper"]) {
		position: relative;
		display: inline-block;
		margin: 1rem 0;
	}

	:global(.markdown-root .hidden),
	:global(.markdown-root [hidden]) {
		display: none !important;
	}

	:global(.markdown-root [data-streamdown="image-fallback"]) {
		font-size: 0.75rem;
		font-style: italic;
		color: var(--disco-markdown-muted-foreground);
	}

	:global(.markdown-root [data-streamdown="image-overlay"]) {
		pointer-events: none;
		position: absolute;
		inset: 0;
		display: none;
		border-radius: 0.5rem;
		background: rgb(0 0 0 / 10%);
	}

	:global(
		.markdown-root
			[data-streamdown="image-wrapper"]:hover
			[data-streamdown="image-overlay"]
	) {
		display: block;
	}

	:global(.markdown-root img) {
		max-width: 100%;
		border-radius: 0.5rem;
	}

	:global(.markdown-root [data-streamdown="mermaid"]) {
		margin: 1rem 0;
		overflow-x: auto;
		border: 1px solid var(--disco-markdown-border);
		border-radius: var(--disco-markdown-radius);
		background: var(--disco-markdown-background);
		padding: 1rem;
	}
</style>
