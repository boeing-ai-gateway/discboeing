<script lang="ts">
	import type { Snippet } from "svelte";
	import { CommandItem } from "$lib/components/ui/command";
	import { useMicSelectorContext } from "./context";

	type Props = {
		value: string;
		onselect?: (value: string) => void;
		children?: Snippet;
	};

	let { value, onselect, children, ...restProps }: Props = $props();
	const micSelector = useMicSelectorContext();

	function handleSelect() {
		micSelector.setValue(value);
		micSelector.setOpen(false);
		onselect?.(value);
	}
</script>

<CommandItem {value} onSelect={handleSelect} {...restProps}>
	{@render children?.()}
</CommandItem>
