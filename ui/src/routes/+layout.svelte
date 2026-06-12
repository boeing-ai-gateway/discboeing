<script lang="ts">
	import { onMount, type Snippet } from "svelte";
	import "../app.css";
	import "katex/dist/katex.min.css";

	import StartupGate from "$lib/components/app/StartupGate.svelte";
	import { Toaster } from "$lib/components/ui/sonner";
	import { ideOptions, windowControls } from "$lib/shell/app-shell-config";
	import { createContext, initializeApp, setContext } from "$lib/context";

	type Props = {
		children: Snippet;
	};

	let { children }: Props = $props();

	const context = setContext(
		createContext({
			ideOptions,
			windowControls,
		}),
	);
	initializeApp();

	onMount(() => {
		return () => {
			context.commands.lifecycle.shutdown();
		};
	});
</script>

<Toaster />

<StartupGate>
	{@render children()}
</StartupGate>
