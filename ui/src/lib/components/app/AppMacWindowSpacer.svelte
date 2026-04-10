<script lang="ts">
	import { onMount } from "svelte";
	import LeftWindowControls from "$lib/components/app/parts/LeftWindowControls.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";

	const environment = useAppContext().environment;
	let isMacFullscreen = $state(false);

	onMount(() => {
		if (!environment.isTauri || environment.windowControlsSide !== "left") {
			return;
		}

		let unlisten: (() => void) | undefined;

		void (async () => {
			const { getCurrentWindow } = await import("@tauri-apps/api/window");
			const appWindow = getCurrentWindow();

			const syncFullscreen = async () => {
				isMacFullscreen = await appWindow.isFullscreen();
			};

			await syncFullscreen();
			unlisten = await appWindow.onResized(() => {
				void syncFullscreen();
			});
		})();

		return () => {
			unlisten?.();
		};
	});

	const showMacSpacer = $derived(
		environment.isTauri &&
			environment.windowControlsSide === "left" &&
			!isMacFullscreen,
	);
</script>

{#if showMacSpacer}
	<LeftWindowControls />
{/if}
