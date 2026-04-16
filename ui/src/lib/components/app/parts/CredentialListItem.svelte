<script lang="ts">
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import PencilIcon from "@lucide/svelte/icons/pencil";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import type { CredentialInfo, Icon } from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import {
		Item,
		ItemActions,
		ItemContent,
		ItemDescription,
		ItemTitle,
	} from "$lib/components/ui/item";

	type Props = {
		credential: CredentialInfo;
		title: string;
		subtitle: string;
		image: Icon | null;
		imageClass?: string;
		monogram: string;
		togglingInactive?: boolean;
		deleting?: boolean;
		onToggleInactive: (credential: CredentialInfo) => void | Promise<void>;
		onEdit: (credential: CredentialInfo) => void | Promise<void>;
		onDelete: (credential: CredentialInfo) => void | Promise<void>;
	};

	let {
		credential,
		title,
		subtitle,
		image,
		imageClass = "",
		monogram,
		togglingInactive = false,
		deleting = false,
		onToggleInactive,
		onEdit,
		onDelete,
	}: Props = $props();
</script>

<Item>
	<ItemContent>
		<div class="flex items-start gap-3">
			{#if image}
				<div
					class="flex size-10 items-center justify-center rounded-md border border-border/70 bg-muted/50 p-1.5"
				>
					<img src={image.src} alt="" class={imageClass} />
				</div>
			{:else}
				<div
					class="flex size-10 items-center justify-center rounded-md border border-border bg-muted text-sm font-semibold"
				>
					{monogram}
				</div>
			{/if}
			<div class="min-w-0">
				<ItemTitle>{title}</ItemTitle>
				<ItemDescription>{subtitle}</ItemDescription>
			</div>
		</div>
	</ItemContent>
	<ItemActions>
		<Button
			variant="outline"
			size="sm"
			disabled={togglingInactive}
			onclick={() => {
				void onToggleInactive(credential);
			}}
		>
			{#if togglingInactive}
				<Loader2Icon class="size-4 animate-spin" />
				{credential.inactive ? "Enabling…" : "Disabling…"}
			{:else}
				{credential.inactive ? "Enable" : "Disable"}
			{/if}
		</Button>
		<Button
			variant="ghost"
			size="icon-sm"
			onclick={() => {
				void onEdit(credential);
			}}
		>
			<PencilIcon class="size-4" />
		</Button>
		<Button
			variant="ghost"
			size="icon-sm"
			disabled={deleting}
			onclick={() => {
				void onDelete(credential);
			}}
		>
			<Trash2Icon class="size-4 text-destructive" />
		</Button>
	</ItemActions>
</Item>
