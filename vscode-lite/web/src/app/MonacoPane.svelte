<script lang="ts">
	import * as monaco from "monaco-editor";
	import { onDestroy, onMount, tick } from "svelte";

	export let model: monaco.editor.ITextModel | null = null;
	export let selection: monaco.IRange | null = null;
	export let onBlur: () => void;
	export let onDefinition: (model: monaco.editor.ITextModel, position: monaco.Position) => void;

	let container: HTMLDivElement;
	let editor: monaco.editor.IStandaloneCodeEditor;

	onMount(() => {
		editor = monaco.editor.create(container, {
			model,
			theme: "vs-dark",
			automaticLayout: true,
			minimap: { enabled: false },
			fontSize: 13
		});
		editor.onDidBlurEditorText(onBlur);
		editor.addAction({
			id: "vscode-lite.goto-definition",
			label: "Go to Definition",
			keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.F12],
			run: () => {
				const active = editor.getModel();
				const position = editor.getPosition();
				if (active && position) onDefinition(active, position);
			}
		});
	});

	$: if (editor && model) {
		editor.setModel(model);
		if (selection) {
			editor.setSelection(selection);
			editor.revealRangeInCenter(selection);
		}
		void tick().then(() => editor.focus());
	}

	onDestroy(() => editor?.dispose());
</script>

<div bind:this={container} class="monaco"></div>
