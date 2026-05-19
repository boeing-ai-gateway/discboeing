import { composerControls } from "./composer-controls";

export type ComposerAttachmentsAPI = {
	addFiles(button: HTMLButtonElement): void;
	backspace(textarea: HTMLTextAreaElement, event: KeyboardEvent): void;
	install(): void;
	keydown(textarea: HTMLTextAreaElement, event: KeyboardEvent): void;
	paste(textarea: HTMLTextAreaElement, event: ClipboardEvent): void;
	remove(button: HTMLButtonElement): void;
	upload(input: HTMLInputElement): void;
};

function composerFormData(form: HTMLFormElement | null): FormData {
	const data = new FormData();
	const textarea = form?.querySelector<HTMLTextAreaElement>('textarea[name="prompt"]');
	data.set("prompt", textarea?.value ?? "");
	return data;
}

async function postComposerAttachments(input: HTMLInputElement) {
	const files = Array.from(input.files ?? []);
	if (files.length === 0) {
		return;
	}
	const form = input.closest<HTMLFormElement>("form");
	const data = composerFormData(form);
	for (const file of files) {
		data.append("attachments", file, file.name);
	}
	input.value = "";
	await fetch("/ui/commands/composer-attachments", {
		method: "POST",
		body: data,
	});
}

async function removeComposerAttachmentByID(form: HTMLFormElement | null, id: string) {
	const formData = composerFormData(form);
	const data = new URLSearchParams();
	data.set("prompt", String(formData.get("prompt") ?? ""));
	data.set("id", id);
	await fetch("/ui/commands/composer-attachment-remove", {
		method: "POST",
		body: data,
	});
}

function composerAttachmentInput(root: ParentNode = document) {
	return root.querySelector<HTMLInputElement>("input[data-composer-attachment-input]");
}

function composerHasSubmitContent(textarea: HTMLTextAreaElement) {
	const attachmentCount = Number.parseInt(textarea.dataset.composerAttachmentCount ?? "0", 10);
	return textarea.value.trim().length > 0 || attachmentCount > 0;
}

function submitOnEnter(textarea: HTMLTextAreaElement, event: KeyboardEvent) {
	if (
		event.defaultPrevented ||
		event.key !== "Enter" ||
		event.shiftKey ||
		event.altKey ||
		event.ctrlKey ||
		event.metaKey ||
		event.isComposing ||
		textarea.dataset.composerSubmitOnEnter !== "true" ||
		!composerHasSubmitContent(textarea)
	) {
		return;
	}
	event.preventDefault();
	textarea.closest<HTMLFormElement>("form")?.requestSubmit();
}

async function postComposerSubmit(form: HTMLFormElement) {
	const data = new URLSearchParams();
	const formData = new FormData(form);
	data.set("prompt", String(formData.get("prompt") ?? ""));
	data.set("run_after", String(formData.get("run_after") ?? ""));
	await fetch("/ui/commands/composer-submit", {
		method: "POST",
		headers: { "content-type": "application/x-www-form-urlencoded" },
		body: data,
	});
}

function isComposerForm(form: HTMLFormElement | null): form is HTMLFormElement {
	return !!form?.closest("[data-conversation-composer]");
}

let installed = false;

export const composerAttachments: ComposerAttachmentsAPI = {
	addFiles(button) {
		composerControls.closeAttachment();
		composerAttachmentInput(button.closest("form") ?? document)?.click();
	},

	backspace(textarea, event) {
		if (
			event.key !== "Backspace" ||
			textarea.value.length > 0 ||
			textarea.dataset.composerBackspaceRemovesLastAttachment !== "true"
		) {
			return;
		}
		const form = textarea.closest<HTMLFormElement>("form");
		const attachments = form?.querySelectorAll<HTMLElement>("[data-composer-attachment-id]");
		const lastAttachment = attachments?.item(attachments.length - 1);
		const id = lastAttachment?.dataset.composerAttachmentId;
		if (!id) {
			return;
		}
		event.preventDefault();
		void removeComposerAttachmentByID(form, id);
	},

	keydown(textarea, event) {
		submitOnEnter(textarea, event);
		if (!event.defaultPrevented) {
			composerAttachments.backspace(textarea, event);
		}
	},

	install() {
		if (installed) {
			return;
		}
		installed = true;
		document.addEventListener(
			"keydown",
			(event) => {
				const textarea = event.target instanceof HTMLTextAreaElement ? event.target : null;
				if (!textarea?.matches("[data-composer-textarea]")) {
					return;
				}
				submitOnEnter(textarea, event);
			},
			true,
		);
		document.addEventListener(
			"submit",
			(event) => {
				const form = event.target instanceof HTMLFormElement ? event.target : null;
				if (!isComposerForm(form)) {
					return;
				}
				event.preventDefault();
				event.stopImmediatePropagation();
				void postComposerSubmit(form);
			},
			true,
		);
	},

	paste(textarea, event) {
		const files = Array.from(event.clipboardData?.files ?? []);
		if (files.length === 0) {
			return;
		}
		const input = composerAttachmentInput(textarea.closest("form") ?? document);
		if (!input) {
			return;
		}
		const dataTransfer = new DataTransfer();
		for (const file of files) {
			dataTransfer.items.add(file);
		}
		input.files = dataTransfer.files;
		void postComposerAttachments(input);
	},

	remove(button) {
		const id = button.dataset.composerAttachmentRemove;
		if (!id) {
			return;
		}
		void removeComposerAttachmentByID(button.closest<HTMLFormElement>("form"), id);
	},

	upload(input) {
		void postComposerAttachments(input);
	},
};
