<script lang="ts">
	import "../app.css";
	import "katex/dist/katex.min.css";

	import StartupGate from "$lib/components/app/StartupGate.svelte";
	import { Toaster } from "$lib/components/ui/sonner";
	import { ideOptions, windowControls } from "$lib/app/app-shell-config";
	import { setAppContext } from "$lib/context/app-context.svelte";
	import { readInitialThreadSelection } from "$lib/store/recent-threads.store.svelte";

	type Props = {
		children?: () => any;
	};

	let { children }: Props = $props();

	const navEntry = performance.getEntriesByType("navigation")[0] as
		| PerformanceNavigationTiming
		| undefined;
	const isReload = navEntry?.type === "reload";
	const initialSelection = isReload ? readInitialThreadSelection() : null;
	const app = setAppContext({
		ideOptions,
		windowControls,
		selectedSessionId: initialSelection?.sessionId,
		selectedThreadId: initialSelection?.threadId,
	});
</script>

<Toaster />

<StartupGate {app}>
	{@render children?.()}
</StartupGate>
