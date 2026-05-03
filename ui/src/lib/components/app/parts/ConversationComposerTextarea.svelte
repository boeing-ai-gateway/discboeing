<script lang="ts">
	import type { AgentCommand } from "$lib/api-types";
	import ConversationPromptHistoryDropdown from "$lib/components/app/ConversationPromptHistoryDropdown.svelte";
	import ConversationFileMentionDropdown from "$lib/components/app/parts/ConversationFileMentionDropdown.svelte";
	import ConversationSlashCommandDropdown from "$lib/components/app/parts/ConversationSlashCommandDropdown.svelte";
	import { InputGroupTextarea } from "$lib/components/ui/input-group";

	type FileMentionDropdownHandle = {
		handleInput: (value: string, cursor: number) => void;
		handleKeydown: (event: KeyboardEvent) => boolean;
		closeDropdown: () => void;
	};

	type SlashCommandDropdownHandle = {
		handleInput: (value: string, cursor: number) => void;
		handleKeydown: (event: KeyboardEvent) => boolean;
		closeDropdown: () => void;
	};

	type PromptHistoryDropdownHandle = {
		handleKeydown: (event: KeyboardEvent) => boolean;
		closePromptHistoryDropdown: () => void;
	};

	type Props = {
		draft: string;
		disabled?: boolean;
		onDraftChange: (value: string) => void;
		sessionId: string | null;
		commands: AgentCommand[];
		commandsLoading: boolean;
		attachmentCount: number;
		onAddFiles: (files: File[] | FileList) => void;
		onRemoveLastAttachment: () => void;
		onSubmit: () => void | Promise<void>;
		onRequestAutocompleteSession?: () => void | Promise<boolean>;
	};

	let {
		draft,
		disabled = false,
		onDraftChange,
		sessionId,
		commands,
		commandsLoading,
		attachmentCount,
		onAddFiles,
		onRemoveLastAttachment,
		onSubmit,
		onRequestAutocompleteSession,
	}: Props = $props();

	let isComposing = $state(false);
	let fileMentionDropdownRef = $state<FileMentionDropdownHandle | null>(null);
	let slashCommandDropdownRef = $state<SlashCommandDropdownHandle | null>(null);
	let promptHistoryDropdownRef = $state<PromptHistoryDropdownHandle | null>(
		null,
	);
	let fileMentionTextareaRef = $state<HTMLTextAreaElement | null>(null);

	function shouldSubmitComposerOnEnter(draft: string): boolean {
		return draft.trim().length > 0;
	}

	function hasActiveFileMention(value: string, cursor: number): boolean {
		return /@([^\s@]*)$/.test(value.slice(0, cursor));
	}

	function hasActiveSlashCommand(value: string, cursor: number): boolean {
		return /^\/([^\s/]*)$/.test(value.slice(0, cursor));
	}

	function syncAutocompleteDropdowns() {
		const textarea = fileMentionTextareaRef;
		if (!textarea) {
			return;
		}

		const cursor = textarea.selectionStart ?? textarea.value.length;
		slashCommandDropdownRef?.handleInput(textarea.value, cursor);
		fileMentionDropdownRef?.handleInput(textarea.value, cursor);
	}

	function handleTextareaKeydown(event: KeyboardEvent) {
		if (slashCommandDropdownRef?.handleKeydown(event)) {
			return;
		}

		if (fileMentionDropdownRef?.handleKeydown(event)) {
			return;
		}

		if (promptHistoryDropdownRef?.handleKeydown(event)) {
			return;
		}

		if (event.key === "Enter") {
			if (isComposing || event.isComposing || event.shiftKey) {
				return;
			}
			if (!shouldSubmitComposerOnEnter(draft)) {
				return;
			}
			event.preventDefault();
			void onSubmit();
			return;
		}

		if (
			event.key === "Backspace" &&
			draft.length === 0 &&
			attachmentCount > 0
		) {
			event.preventDefault();
			onRemoveLastAttachment();
		}
	}

	function handleTextareaInput(event: Event) {
		const textarea = event.currentTarget as HTMLTextAreaElement;
		onDraftChange(textarea.value);
		promptHistoryDropdownRef?.closePromptHistoryDropdown();
		const cursor = textarea.selectionStart ?? textarea.value.length;
		if (
			!sessionId &&
			(hasActiveFileMention(textarea.value, cursor) ||
				hasActiveSlashCommand(textarea.value, cursor))
		) {
			void onRequestAutocompleteSession?.();
		}
		slashCommandDropdownRef?.handleInput(textarea.value, cursor);
		fileMentionDropdownRef?.handleInput(textarea.value, cursor);
	}

	function handleTextareaPaste(event: ClipboardEvent) {
		const items = event.clipboardData?.items;
		if (!items) {
			return;
		}

		const files: File[] = [];
		for (const item of items) {
			if (item.kind !== "file") {
				continue;
			}
			const file = item.getAsFile();
			if (file) {
				files.push(file);
			}
		}

		if (files.length > 0) {
			event.preventDefault();
			onAddFiles(files);
		}
	}

	export function closeMentionDropdown() {
		fileMentionDropdownRef?.closeDropdown();
	}

	export function closeSlashCommandDropdown() {
		slashCommandDropdownRef?.closeDropdown();
	}

	export function closePromptHistoryDropdown() {
		promptHistoryDropdownRef?.closePromptHistoryDropdown();
	}

	export function focus() {
		const textarea = fileMentionTextareaRef;
		if (!textarea) {
			return;
		}
		textarea.focus();
		const cursor = textarea.value.length;
		textarea.setSelectionRange(cursor, cursor);
		syncAutocompleteDropdowns();
	}
</script>

<ConversationSlashCommandDropdown
	bind:this={slashCommandDropdownRef}
	{sessionId}
	{commands}
	{commandsLoading}
	textareaRef={fileMentionTextareaRef}
	onDraftChange={(value) => onDraftChange(value)}
/>

<ConversationFileMentionDropdown
	bind:this={fileMentionDropdownRef}
	{sessionId}
	textareaRef={fileMentionTextareaRef}
	onDraftChange={(value) => onDraftChange(value)}
/>

<ConversationPromptHistoryDropdown
	bind:this={promptHistoryDropdownRef}
	textareaRef={fileMentionTextareaRef}
	onDraftChange={(value) => onDraftChange(value)}
/>

<InputGroupTextarea
	bind:ref={fileMentionTextareaRef}
	rows={2}
	class="field-sizing-content max-h-48 min-h-16 transition-all"
	value={draft}
	{disabled}
	placeholder="Type a message, @file, /command, or ↑ for history"
	oncompositionstart={() => {
		isComposing = true;
	}}
	oncompositionend={() => {
		isComposing = false;
	}}
	onkeydown={handleTextareaKeydown}
	onpaste={handleTextareaPaste}
	oninput={handleTextareaInput}
/>
