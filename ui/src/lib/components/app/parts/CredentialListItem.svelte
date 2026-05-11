<script lang="ts">
	import PencilIcon from "@lucide/svelte/icons/pencil";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import type { CredentialInfo, Icon } from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import { Switch } from "$lib/components/ui/switch";
	import {
		Item,
		ItemActions,
		ItemContent,
		ItemDescription,
		ItemMedia,
		ItemTitle,
	} from "$lib/components/ui/item";
	import * as Tooltip from "$lib/components/ui/tooltip";

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

<Item size="sm">
	<ItemContent>
		<div class="flex items-start gap-3">
			<ItemMedia class="h-10 w-10 rounded-md border border-border bg-muted/50">
				{#if image}
					<img src={image.src} alt="" class={imageClass} />
				{:else}
					{monogram}
				{/if}
			</ItemMedia>
			<div class="min-w-0">
				<ItemTitle>{title}</ItemTitle>
				<ItemDescription>{subtitle}</ItemDescription>
			</div>
		</div>
	</ItemContent>
	<ItemActions>
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					<Switch
						{...props}
						checked={!credential.inactive}
						disabled={togglingInactive}
						aria-label={credential.inactive
							? `Enable ${title}`
							: `Disable ${title}`}
						onCheckedChange={(checked) => {
							if (checked !== !credential.inactive) {
								void onToggleInactive(credential);
							}
						}}
					/>
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Content>
				{togglingInactive
					? "Updating"
					: credential.inactive
						? "Enable"
						: "Disable"}
				{title}
			</Tooltip.Content>
		</Tooltip.Root>
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						variant="ghost"
						size="icon-sm"
						aria-label={`Edit ${title}`}
						onclick={() => {
							void onEdit(credential);
						}}
					>
						<PencilIcon class="size-4" />
					</Button>
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Content>Edit {title}</Tooltip.Content>
		</Tooltip.Root>
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						variant="ghost"
						size="icon-sm"
						disabled={deleting}
						aria-label={`Delete ${title}`}
						onclick={() => {
							void onDelete(credential);
						}}
					>
						<Trash2Icon class="size-4 text-destructive" />
					</Button>
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Content>Delete {title}</Tooltip.Content>
		</Tooltip.Root>
	</ItemActions>
</Item>
