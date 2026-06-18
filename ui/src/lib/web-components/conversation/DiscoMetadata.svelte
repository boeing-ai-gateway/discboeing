<svelte:options customElement="disco-metadata" />

<script lang="ts">
	import { onMount } from "svelte";
	import { getCustomElementHost, readJsonScript, writeJsonScript } from "./dom";
	import type { DiscoMetadataValue } from "./types";

	type MetadataHost = HTMLElement & {
		value?: DiscoMetadataValue;
		setValue?: (value: DiscoMetadataValue) => void;
	};
	let root = $state<HTMLSpanElement | null>(null);

	function getHost(): MetadataHost | null {
		return root ? getCustomElementHost<MetadataHost>(root) : null;
	}

	function refresh() {
		const host = getHost();
		if (!host) {
			return;
		}
		const value = readJsonScript(host);
		host.value =
			value && typeof value === "object" ? (value as DiscoMetadataValue) : {};
	}

	function setValue(value: DiscoMetadataValue) {
		const host = getHost();
		if (!host) {
			return;
		}
		writeJsonScript(host, value);
		refresh();
	}

	$effect(() => {
		const host = getHost();
		if (!host) {
			return;
		}
		host.setValue = setValue;
	});

	onMount(() => {
		const host = getHost();
		if (!host) {
			return;
		}
		refresh();
		const observer = new MutationObserver(refresh);
		observer.observe(host, {
			childList: true,
			characterData: true,
			subtree: true,
		});
		return () => observer.disconnect();
	});
</script>

<span hidden bind:this={root}><slot></slot></span>

<style>
	:host {
		display: none;
	}
</style>
