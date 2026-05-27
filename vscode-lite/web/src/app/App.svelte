<script lang="ts">
	import * as monaco from "monaco-editor";
	import { onMount } from "svelte";
	import { listFiles, readFile, workspace, writeFile, type FileInfo } from "../api/files";
	import { languageForPath, lspLanguage } from "../editor/languages";
	import { pathToUri } from "../editor/uri";
	import { LSPClient } from "../lsp/client";
	import FileTree from "./FileTree.svelte";
	import MonacoPane from "./MonacoPane.svelte";

	type Tab = {
		path: string;
		uri: string;
		language: string;
		model: monaco.editor.ITextModel;
		dirty: boolean;
		saving: boolean;
		saveTimer?: number;
	};

	let root = "";
	let rootEntries: FileInfo[] = [];
	let tabs: Tab[] = [];
	let activePath: string | null = null;
	let lspClients = new Map<string, LSPClient>();
	let outline: unknown[] = [];
	let status = "Loading workspace...";
	let selections = new Map<string, monaco.IRange>();

	$: activeTab = tabs.find((tab) => tab.path === activePath) ?? null;
	$: problems = monaco.editor.getModelMarkers({}).map((marker) => ({
		path: marker.resource.path.replace(root, "").replace(/^\//, ""),
		message: marker.message,
		line: marker.startLineNumber
	}));

	onMount(async () => {
		const meta = await workspace();
		root = meta.root;
		rootEntries = (await listFiles(".")).entries;
		status = `Workspace: ${root}`;
		window.addEventListener("blur", saveAllNow);
		window.addEventListener("keydown", handleKeydown);
	});

	async function expand(path: string) {
		return (await listFiles(path)).entries;
	}

	async function openFile(path: string, selection?: monaco.IRange) {
		const existing = tabs.find((tab) => tab.path === path);
		if (existing) {
			if (selection) {
				selections.set(existing.path, selection);
				selections = new Map(selections);
			}
			await switchTo(existing.path);
			return;
		}
		const file = await readFile(path);
		const language = languageForPath(path);
		const uri = pathToUri(root, path);
		const model = monaco.editor.createModel(file.content, language, monaco.Uri.parse(uri));
		const tab: Tab = { path, uri, language, model, dirty: false, saving: false };
		tabs = [...tabs, tab];
		activePath = path;
		const client = clientFor(language);
		client?.didOpen(model, language);
		model.onDidChangeContent(() => {
			tab.dirty = true;
			tabs = [...tabs];
			client?.didChange(model);
			scheduleSave(tab);
		});
		if (selection) {
			selections.set(path, selection);
			selections = new Map(selections);
		}
		await refreshOutline(tab);
	}

	async function switchTo(path: string) {
		if (activeTab) await saveNow(activeTab);
		activePath = path;
		if (activeTab) await refreshOutline(activeTab);
	}

	function closeTab(path: string) {
		const tab = tabs.find((item) => item.path === path);
		if (!tab) return;
		void saveNow(tab).then(() => {
			clientFor(tab.language)?.didClose(tab.model);
			tab.model.dispose();
			tabs = tabs.filter((item) => item.path !== path);
			if (activePath === path) activePath = tabs.at(-1)?.path ?? null;
		});
	}

	function clientFor(language: string) {
		const serverLanguage = lspLanguage(language);
		if (!serverLanguage) return null;
		let client = lspClients.get(serverLanguage);
		if (!client) {
			client = new LSPClient({ root, language: serverLanguage, openFile });
			lspClients.set(serverLanguage, client);
		}
		return client;
	}

	function scheduleSave(tab: Tab) {
		if (tab.saveTimer) window.clearTimeout(tab.saveTimer);
		tab.saveTimer = window.setTimeout(() => void saveNow(tab), 750);
	}

	async function saveNow(tab: Tab) {
		if (!tab.dirty && !tab.saving) return;
		if (tab.saveTimer) window.clearTimeout(tab.saveTimer);
		tab.saving = true;
		tabs = [...tabs];
		try {
			await writeFile(tab.path, tab.model.getValue());
			tab.dirty = false;
			clientFor(tab.language)?.didSave(tab.model);
			status = `Saved ${tab.path}`;
		} catch (error) {
			status = `Save failed: ${String(error)}`;
		} finally {
			tab.saving = false;
			tabs = [...tabs];
		}
	}

	function saveAllNow() {
		void Promise.all(tabs.map(saveNow));
	}

	function handleKeydown(event: KeyboardEvent) {
		if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
			event.preventDefault();
			saveAllNow();
		}
	}

	async function goToDefinition(model: monaco.editor.ITextModel, position: monaco.Position) {
		await clientFor(model.getLanguageId())?.definition(model, position);
	}

	async function refreshOutline(tab: Tab) {
		const symbols = await clientFor(tab.language)?.symbols(tab.model);
		outline = Array.isArray(symbols) ? symbols : [];
	}
</script>

<div class="workbench">
	<aside class="sidebar">
		<h1>vscode-lite</h1>
		<FileTree entries={rootEntries} onOpen={openFile} onExpand={expand} />
	</aside>
	<main class="main">
		<div class="tabs">
			{#each tabs as tab (tab.path)}
				<button class:active={tab.path === activePath} on:click={() => switchTo(tab.path)}>
					{tab.path.split("/").at(-1)}{tab.dirty || tab.saving ? " •" : ""}
				</button>
				<button class="close" on:click={() => closeTab(tab.path)}>×</button>
			{/each}
		</div>
		{#if activeTab}
			<MonacoPane model={activeTab.model} selection={selections.get(activeTab.path) ?? null} onBlur={() => saveNow(activeTab)} onDefinition={goToDefinition} />
		{:else}
			<div class="empty">Open a file from the explorer.</div>
		{/if}
	</main>
	<aside class="rightbar">
		<section>
			<h2>Problems</h2>
			{#each problems as problem}
				<div class="problem">{problem.path}:{problem.line} {problem.message}</div>
			{/each}
		</section>
		<section>
			<h2>Outline</h2>
			<pre>{JSON.stringify(outline, null, 2)}</pre>
		</section>
	</aside>
	<footer>{status}</footer>
</div>
