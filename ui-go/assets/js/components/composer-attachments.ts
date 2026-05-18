import { composerControls } from "./composer-controls";

export type ComposerAttachmentsAPI = {
	addFiles(button: HTMLButtonElement): void;
	backspace(textarea: HTMLTextAreaElement, event: KeyboardEvent): void;
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
