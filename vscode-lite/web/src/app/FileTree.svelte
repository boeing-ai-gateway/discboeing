<script lang="ts">
	import type { FileInfo } from "../api/files";

	export let entries: FileInfo[] = [];
	export let onOpen: (path: string) => void;
	export let onExpand: (path: string) => Promise<FileInfo[]>;

	let expanded = new Set<string>();
	let children = new Map<string, FileInfo[]>();

	async function toggle(entry: FileInfo) {
		if (!entry.isDir) {
			onOpen(entry.path);
			return;
		}
		if (expanded.has(entry.path)) {
			expanded.delete(entry.path);
			expanded = new Set(expanded);
			return;
		}
		children.set(entry.path, await onExpand(entry.path));
		expanded.add(entry.path);
		expanded = new Set(expanded);
		children = new Map(children);
	}
</script>

<ul class="tree">
	{#each entries as entry (entry.path)}
		<li>
			<button class:dir={entry.isDir} on:click={() => toggle(entry)} title={entry.path}>
				<span>{entry.isDir ? (expanded.has(entry.path) ? "▾" : "▸") : ""}</span>
				{entry.name}
			</button>
			{#if entry.isDir && expanded.has(entry.path)}
				<svelte:self entries={children.get(entry.path) ?? []} {onOpen} {onExpand} />
			{/if}
		</li>
	{/each}
</ul>
