<script lang="ts">
	import {
		LinkSafetyModal,
		LinkSafetyState,
	} from "$lib/components/ai/link-safety-modal";
	import { downloadFile } from "$lib/shell";
	import "$lib/web-components/markdown/define";
	import type {
		MarkdownImageDownloadDetail,
		MarkdownLinkClickDetail,
		MarkdownMode,
		MarkdownPluginConfig,
	} from "$lib/web-components/markdown";

	type Props = {
		text: string;
		class?: string;
		plugins?: MarkdownPluginConfig;
		mode?: MarkdownMode;
		isAnimating?: boolean;
	};

	let {
		text,
		class: className,
		plugins,
		mode = "streaming",
		isAnimating = false,
	}: Props = $props();

	const linkSafety = new LinkSafetyState();

	function handleLinkClick(event: Event) {
		const linkEvent = event as CustomEvent<MarkdownLinkClickDetail>;
		linkEvent.preventDefault();
		linkSafety.requestOpen(linkEvent.detail.href);
	}

	async function handleImageDownload(event: Event) {
		const imageEvent = event as CustomEvent<MarkdownImageDownloadDetail>;
		imageEvent.preventDefault();
		const { src, suggestedFilename } = imageEvent.detail;
		try {
			const response = await fetch(src);
			const blob = await response.blob();
			await downloadFile({
				filename: suggestedFilename,
				content: await blob.arrayBuffer(),
				mimeType: blob.type,
			});
		} catch {
			window.open(src, "_blank", "noopener,noreferrer");
		}
	}
</script>

<disco-markdown
	class={className}
	{mode}
	is-animating={isAnimating}
	{plugins}
	ondisco-link-click={handleLinkClick}
	ondisco-image-download={handleImageDownload}>{text}</disco-markdown
>

<LinkSafetyModal
	isOpen={linkSafety.isOpen}
	onClose={() => linkSafety.close()}
	url={linkSafety.url}
/>
