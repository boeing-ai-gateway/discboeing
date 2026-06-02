<script lang="ts">
	import { withCurrentDesktopWindow } from "$lib/shell";
	import { onMount } from "svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";

	const environment = useAppContext().environment;
	let isMacFullscreen = $state(false);

	onMount(() => {
		if (
			!environment.supportsNativeWindowControls ||
			environment.windowControlsSide !== "left"
		) {
			return;
		}

		let unlisten: (() => void) | undefined;
		let disposed = false;

		void withCurrentDesktopWindow(async (window) => {
			const syncFullscreen = async () => {
				const fullscreen = await window.isFullscreen();
				if (!disposed) {
					isMacFullscreen = fullscreen;
				}
			};

			await syncFullscreen();
			const stopListening = await window.onResized(() => {
				void syncFullscreen();
			});

			if (disposed) {
				stopListening();
				return;
			}
			unlisten = stopListening;
		});

		return () => {
			disposed = true;
			unlisten?.();
		};
	});

	const showMacSpacer = $derived(
		environment.supportsNativeWindowControls &&
			environment.windowControlsSide === "left" &&
			!isMacFullscreen,
	);
</script>

{#if showMacSpacer}
	<div aria-hidden="true" class="w-14 shrink-0"></div>
{/if}
