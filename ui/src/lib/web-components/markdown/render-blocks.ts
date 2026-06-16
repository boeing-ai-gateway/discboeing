import { hasIncompleteCodeFence } from "./incomplete-code-utils";
import { parseMarkdownIntoBlocks } from "./parse-blocks";
import { parseMarkdownToHast } from "./pipeline";
import { preprocessMarkdown } from "./preprocess";
import { renderMarkdownTree } from "./render-dom";
import type {
	MarkdownImageDownloadDetail,
	MarkdownLinkClickDetail,
	MarkdownMode,
	MarkdownPluginConfig,
	MarkdownRenderErrorDetail,
} from "./types";

type MarkdownBlockRenderOptions = {
	isAnimating: boolean;
	mode: MarkdownMode;
	plugins: MarkdownPluginConfig;
	onImageDownload?: (detail: MarkdownImageDownloadDetail) => boolean | void;
	onLinkClick?: (detail: MarkdownLinkClickDetail) => boolean | void;
	onRenderError?: (detail: MarkdownRenderErrorDetail) => void;
};

function createBlockElement(
	block: string,
	index: number,
	isIncompleteCodeFence: boolean,
	options: MarkdownBlockRenderOptions,
): HTMLDivElement {
	const wrapper = document.createElement("div");
	wrapper.className = "markdown-block";
	wrapper.dataset.markdownBlock = String(index);
	wrapper.dataset.incompleteCodeFence = String(isIncompleteCodeFence);

	try {
		const tree = parseMarkdownToHast(block, options.plugins);
		wrapper.append(
			renderMarkdownTree(tree, {
				isIncompleteCodeFence,
				onImageDownload: options.onImageDownload,
				onLinkClick: options.onLinkClick,
				plugins: options.plugins,
			}),
		);
	} catch (error) {
		const fallback = document.createElement("div");
		fallback.className = "markdown-fallback";
		fallback.textContent = block;
		wrapper.append(fallback);
		options.onRenderError?.({ error, markdown: block });
	}

	return wrapper;
}

function syncBlocks(
	element: HTMLElement,
	nextBlocks: string[],
	previousBlocks: string[],
	options: MarkdownBlockRenderOptions,
) {
	while (element.children.length > nextBlocks.length) {
		element.lastChild?.remove();
	}

	for (let index = 0; index < nextBlocks.length; index += 1) {
		const block = nextBlocks[index];
		const isLastBlock = index === nextBlocks.length - 1;
		const isIncompleteCodeFence =
			options.isAnimating && isLastBlock && hasIncompleteCodeFence(block);
		const existingBlock = element.children[index] as HTMLDivElement | undefined;
		if (
			previousBlocks[index] === block &&
			existingBlock &&
			existingBlock.dataset.incompleteCodeFence ===
				String(isIncompleteCodeFence)
		) {
			continue;
		}

		const blockElement = createBlockElement(
			block,
			index,
			isIncompleteCodeFence,
			options,
		);
		if (element.children[index]) {
			element.children[index].replaceWith(blockElement);
		} else {
			element.append(blockElement);
		}
	}

	Array.from(element.children).forEach((child, index) => {
		(child as HTMLDivElement).className = "markdown-block";
		(child as HTMLDivElement).dataset.markdownBlock = String(index);
	});
}

export function renderMarkdownBlocks(
	element: HTMLElement,
	markdown: string,
	previousBlocks: string[],
	options: MarkdownBlockRenderOptions,
): string[] {
	const processed = preprocessMarkdown(markdown, {
		isAnimating: options.isAnimating,
		mode: options.mode,
	});
	const nextBlocks =
		options.mode === "streaming"
			? parseMarkdownIntoBlocks(processed)
			: [processed];
	syncBlocks(element, nextBlocks, previousBlocks, options);
	return nextBlocks;
}
