<svelte:options
	customElement={{
		tag: "disco-tool-input",
		props: {
			format: { attribute: "format", type: "String" },
		},
	}}
/>

<script lang="ts">
	import { onMount } from "svelte";
	import { getCustomElementHost, readJsonScript, writeJsonScript } from "./dom";

	type Props = {
		format?: "json" | "text";
	};

	let { format = "json" }: Props = $props();
	type ValueHost = HTMLElement & {
		value?: unknown;
		setValue?: (value: unknown) => void;
	};

	let root = $state<HTMLElement | null>(null);
	let textValue = $state("");

	function getHost(): ValueHost | null {
		return root ? getCustomElementHost<ValueHost>(root) : null;
	}

	function readValue(): unknown {
		const host = getHost();
		if (!host) {
			return undefined;
		}
		if (format === "json") {
			return readJsonScript(host);
		}
		return host.textContent ?? "";
	}

	function refresh() {
		const host = getHost();
		if (!host) {
			return;
		}
		const nextValue = readValue();
		host.value = nextValue;
		textValue =
			format === "json"
				? JSON.stringify(nextValue, null, "\t")
				: String(nextValue ?? "");
	}

	function setValue(value: unknown) {
		const host = getHost();
		if (!host) {
			return;
		}
		if (format === "json") {
			writeJsonScript(host, value);
		} else {
			host.textContent = String(value ?? "");
		}
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

<pre bind:this={root} part="content" class="content"><code>{textValue}</code
	></pre>
<span class="source"><slot></slot></span>

<style>
	:host {
		display: block;
		min-width: 0;
	}

	.content {
		max-height: 16rem;
		margin: 0;
		overflow: auto;
		white-space: pre-wrap;
		border-radius: 0.375rem;
		border: 1px solid var(--disco-border, #e5e7eb);
		background: var(--disco-background, #fff);
		padding: 0.625rem 0.75rem;
		color: var(--disco-foreground, #111827);
		font-family: var(--disco-font-mono, ui-monospace, monospace);
		font-size: 0.75rem;
		line-height: 1.45;
		overflow-wrap: anywhere;
	}

	.source {
		display: none;
	}
</style>
