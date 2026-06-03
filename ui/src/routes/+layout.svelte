<script lang="ts">
	import type { Snippet } from "svelte";
	import "../app.css";
	import "katex/dist/katex.min.css";

	import DevErrorOverlay from "$lib/components/app/parts/DevErrorOverlay.svelte";
	import StartupGate from "$lib/components/app/StartupGate.svelte";
	import { Toaster } from "$lib/components/ui/sonner";
	import { ideOptions, windowControls } from "$lib/app/app-shell-config";
	import {
		initializeAppCommands,
		syncAppNavigationFromBridge,
	} from "$lib/context/commands/app-view";
	import { setDiscobotContext } from "$lib/context/context.svelte";
	import { readInitialThreadSelection } from "$lib/store/recent-threads.store.svelte";

	type Props = {
		children: Snippet;
	};

	let { children }: Props = $props();

	const navEntry = performance.getEntriesByType("navigation")[0] as
		| PerformanceNavigationTiming
		| undefined;
	const isReload = navEntry?.type === "reload";
	const initialSelection = isReload ? readInitialThreadSelection() : null;
	setDiscobotContext({
		ideOptions,
		windowControls,
		selectedSessionId: initialSelection?.sessionId,
		selectedThreadId: initialSelection?.threadId,
	});
	initializeAppCommands({
		selectedSessionId: initialSelection?.sessionId,
		selectedThreadId: initialSelection?.threadId,
	});

	$effect(() => {
		syncAppNavigationFromBridge();
	});
</script>

<Toaster />
<DevErrorOverlay />

<StartupGate>
	{@render children()}
</StartupGate>
