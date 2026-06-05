<script lang="ts">
	import type { AgentCommand } from "$lib/api-types";
	import ConversationPromptHistoryDropdown from "$lib/components/app/ConversationPromptHistoryDropdown.svelte";
	import ConversationFileMentionDropdown from "$lib/components/app/parts/ConversationFileMentionDropdown.svelte";
	import ConversationSlashCommandDropdown from "$lib/components/app/parts/ConversationSlashCommandDropdown.svelte";
	import { InputGroupTextarea } from "$lib/components/ui/input-group";

	type FileMentionItem = {
		path: string;
		type: "file" | "directory";
	};

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
		expanded?: boolean;
		onDraftChange: (value: string) => void;
		sessionId: string | null;
		commands: AgentCommand[];
		commandsLoading: boolean;
		fileMentionSuggestions: FileMentionItem[];
		fileMentionLoading: boolean;
		promptHistory: string[];
		pinnedPrompts: string[];
		attachmentCount: number;
		onAddFiles: (files: File[] | FileList) => void;
		onRemoveLastAttachment: () => void;
		onPinPrompt: (prompt: string) => void;
		onUnpinPrompt: (prompt: string) => void;
		onRemovePromptFromHistory: (prompt: string) => void;
		isPromptPinned: (prompt: string) => boolean;
		onSubmit: () => void | Promise<void>;
		onFileMentionQueryChange: (query: string, open: boolean) => void;
		onRequestAutocompleteSession?: () => void | Promise<boolean>;
	};

	let {
		draft,
		disabled = false,
		expanded = false,
		onDraftChange,
		sessionId,
		commands,
		commandsLoading,
		fileMentionSuggestions,
		fileMentionLoading,
		promptHistory,
		pinnedPrompts,
		attachmentCount,
		onAddFiles,
		onRemoveLastAttachment,
		onPinPrompt,
		onUnpinPrompt,
		onRemovePromptFromHistory,
		isPromptPinned,
		onSubmit,
		onFileMentionQueryChange,
		onRequestAutocompleteSession,
	}: Props = $props();

	let isComposing = $state(false);
	let fileMentionDropdownRef = $state<FileMentionDropdownHandle | null>(null);
	let slashCommandDropdownRef = $state<SlashCommandDropdownHandle | null>(null);
	let promptHistoryDropdownRef = $state<PromptHistoryDropdownHandle | null>(
		null,
	);
	let fileMentionTextareaRef = $state<HTMLTextAreaElement | null>(null);
	let fileMentionActiveOptionId = $state<string | null>(null);
	const fileMentionListboxId = "conversation-file-mentions";

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
	suggestions={fileMentionSuggestions}
	isLoading={fileMentionLoading}
	listboxId={fileMentionListboxId}
	onDraftChange={(value) => onDraftChange(value)}
	onQueryChange={onFileMentionQueryChange}
	onActiveOptionChange={(optionId) => {
		fileMentionActiveOptionId = optionId;
	}}
/>

<ConversationPromptHistoryDropdown
	bind:this={promptHistoryDropdownRef}
	textareaRef={fileMentionTextareaRef}
	onDraftChange={(value) => onDraftChange(value)}
	{promptHistory}
	{pinnedPrompts}
	{onPinPrompt}
	{onUnpinPrompt}
	{onRemovePromptFromHistory}
	{isPromptPinned}
/>

<InputGroupTextarea
	bind:ref={fileMentionTextareaRef}
	rows={2}
	class={expanded
		? "min-h-0 flex-1 pr-12 transition-all"
		: "field-sizing-content max-h-48 min-h-16 pr-12 transition-all"}
	value={draft}
	{disabled}
	placeholder="Type a message, @file, /command, or ↑ for history"
	aria-label="Message"
	aria-autocomplete="list"
	aria-controls={fileMentionActiveOptionId ? fileMentionListboxId : undefined}
	aria-activedescendant={fileMentionActiveOptionId ?? undefined}
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
