<script lang="ts">
	import { Collapsible } from "$lib/components/ui/collapsible";
	import { cn } from "$lib/utils";
	import { setToolContext } from "./context";

	type Props = {
		open?: boolean;
		defaultOpen?: boolean;
		queued?: boolean;
		showBorder?: boolean;
		class?: string;
		children?: () => any;
	};

	let {
		defaultOpen = false,
		open = $bindable(defaultOpen),
		queued = false,
		showBorder = true,
		class: className,
		children,
		...restProps
	}: Props = $props();

	const tool = $state({
		isOpen: open,
		queued: false,
		setIsOpen: (next: boolean) => {
			open = next;
		},
	});

	$effect(() => {
		tool.isOpen = open;
		tool.queued = queued;
	});

	setToolContext(tool);
</script>

<Collapsible
	bind:open
	data-ai-tool
	data-ai-stack
	class={cn(
		"group group/tool not-prose mb-4 w-full rounded-md",
		showBorder ? "border" : "",
		className,
	)}
	{...restProps}
>
	{@render children?.()}
</Collapsible>
