<svelte:options
	customElement={{
		tag: "disco-attachment",
		props: {
			partId: { attribute: "part-id", type: "String" },
			kind: { attribute: "kind", type: "String" },
			src: { attribute: "src", type: "String" },
			filename: { attribute: "filename", type: "String" },
			mediaType: { attribute: "media-type", type: "String" },
		},
	}}
/>

<script lang="ts">
	import FileIcon from "@lucide/svelte/icons/file";
	import ImageIcon from "@lucide/svelte/icons/image";
	import { emitComposedEvent, getCustomElementHost } from "./dom";
	import type { DiscoAttachmentKind } from "./types";

	type Props = {
		partId?: string;
		kind?: DiscoAttachmentKind;
		src?: string;
		filename?: string;
		mediaType?: string;
	};

	let { partId, kind = "file", src, filename, mediaType }: Props = $props();
	let root = $state<HTMLElement | null>(null);
	const isImage = $derived(
		kind === "image" || mediaType?.startsWith("image/") === true,
	);
	const label = $derived(filename || src || "Attachment");

	function openAttachment() {
		if (!root) {
			return;
		}
		const host = getCustomElementHost(root);
		const allowed = emitComposedEvent(
			host,
			"disco-attachment-open-request",
			{
				messageId: host.closest("disco-message")?.id || undefined,
				partId,
				kind,
				src,
				filename,
				mediaType,
			},
			{ cancelable: true },
		);
		if (allowed && src) {
			window.open(src, "_blank", "noopener,noreferrer");
		}
	}
</script>

<button
	bind:this={root}
	part="container"
	class="container"
	type="button"
	onclick={openAttachment}
>
	<span part="preview" class="preview">
		{#if isImage && src}
			<img {src} alt={filename ?? "Attachment preview"} />
		{:else if isImage}
			<ImageIcon class="icon" aria-hidden="true" />
		{:else}
			<FileIcon class="icon" aria-hidden="true" />
		{/if}
	</span>
	<span part="info" class="info">
		<span part="filename" class="filename">{label}</span>
		{#if mediaType}
			<span class="media-type">{mediaType}</span>
		{/if}
	</span>
</button>

<style>
	:host {
		display: inline-block;
		max-width: 100%;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-family: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
	}

	.container {
		display: inline-flex;
		max-width: 100%;
		align-items: center;
		gap: var(--disco-attachment-gap, 0.625rem);
		border: 1px solid
			var(
				--disco-attachment-border,
				var(
					--disco-conversation-border,
					var(--disco-border, var(--border, #e5e7eb))
				)
			);
		border-radius: var(--disco-attachment-radius, 0.625rem);
		background: var(
			--disco-attachment-background,
			var(
				--disco-conversation-background,
				var(--disco-background, var(--background, #fff))
			)
		);
		padding: var(--disco-attachment-padding, 0.5rem 0.625rem);
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	.container:hover {
		background: var(
			--disco-attachment-hover-background,
			var(
				--disco-conversation-accent,
				var(--disco-accent, var(--accent, #f3f4f6))
			)
		);
	}

	.preview {
		display: inline-flex;
		width: var(--disco-attachment-preview-size, 2rem);
		height: var(--disco-attachment-preview-size, 2rem);
		flex: none;
		align-items: center;
		justify-content: center;
		overflow: hidden;
		border-radius: 0.375rem;
		background: var(
			--disco-attachment-preview-background,
			var(--disco-conversation-muted, var(--disco-muted, var(--muted, #f9fafb)))
		);
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
	}

	.preview img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	:global(.icon) {
		width: 1rem;
		height: 1rem;
	}

	.info {
		display: flex;
		min-width: 0;
		flex-direction: column;
	}

	.filename {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: var(--disco-attachment-filename-size, 0.8125rem);
		font-weight: 500;
	}

	.media-type {
		display: var(--disco-attachment-media-display, block);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.75rem;
	}
</style>
