import { sendFormData } from "./command.js";

const uploadURL = "/ui/commands/composer/attachments";

const filesFromList = (files) => Array.from(files ?? []).filter((file) => file instanceof File);

const uploadFiles = async (files) => {
	const incoming = filesFromList(files);
	if (incoming.length === 0) {
		return;
	}

	const formData = new FormData();
	for (const file of incoming) {
		formData.append("files", file);
	}
	await sendFormData({ url: uploadURL, formData });
};

const isFileDrag = (event) => Array.from(event.dataTransfer?.types ?? []).includes("Files");

const openFilePicker = (trigger, event) => {
	event.preventDefault();
	const root = trigger.closest(".composer-attachment-button");
	root?.querySelector("[data-composer-attachment-input]")?.click();
};

const uploadSelectedFiles = (input) => {
	void uploadFiles(input.files).finally(() => {
		input.value = "";
	});
};

const markDragTarget = (target, event) => {
	if (!isFileDrag(event)) {
		return;
	}

	event.preventDefault();
	target.toggleAttribute("data-composer-attachment-dragging", true);
};

const markDragOverTarget = (target, event) => {
	if (!isFileDrag(event)) {
		return;
	}

	event.preventDefault();
	event.dataTransfer.dropEffect = "copy";
	target.toggleAttribute("data-composer-attachment-dragging", true);
};

const clearDragTarget = (target, event) => {
	if (target.contains(event.relatedTarget)) {
		return;
	}

	target.removeAttribute("data-composer-attachment-dragging");
};

const dropFiles = (target, event) => {
	if (!isFileDrag(event)) {
		return;
	}

	event.preventDefault();
	target.removeAttribute("data-composer-attachment-dragging");
	void uploadFiles(event.dataTransfer.files);
};

export const setupComposerAttachments = () => {
	window.discobot = window.discobot ?? {};
	window.discobot.composerAttachments = {
		...(window.discobot.composerAttachments ?? {}),
		clearDragTarget,
		dropFiles,
		markDragOverTarget,
		markDragTarget,
		openFilePicker,
		uploadSelectedFiles,
	};
};
