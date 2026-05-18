type ComposerControl = {
	control: string;
	menu: string;
	trigger: string;
	option: string;
};

const controls: Record<string, ComposerControl> = {
	attachment: {
		control: "[data-composer-attachment-button]",
		menu: "[data-composer-attachment-menu]",
		trigger: "[data-composer-attachment-trigger]",
		option: "[data-composer-attachment-add-files]",
	},
	model: {
		control: "[data-composer-model-control]",
		menu: "[data-composer-model-menu]",
		trigger: "[data-composer-model-trigger]",
		option: "[data-composer-model-option]",
	},
	reasoning: {
		control: "[data-composer-reasoning-control]",
		menu: "[data-composer-reasoning-menu]",
		trigger: "[data-composer-reasoning-trigger]",
		option: "[data-composer-reasoning-option]",
	},
	serviceTier: {
		control: "[data-composer-service-tier-control]",
		menu: "[data-composer-service-tier-menu]",
		trigger: "[data-composer-service-tier-trigger]",
		option: "[data-composer-service-tier-option]",
	},
};

function closeControl(config: ComposerControl, except?: HTMLElement) {
	for (const menu of document.querySelectorAll<HTMLElement>(config.menu)) {
		if (except && menu === except) {
			continue;
		}
		menu.classList.add("hidden");
		const trigger = menu
			.closest<HTMLElement>(config.control)
			?.querySelector<HTMLButtonElement>(config.trigger);
		trigger?.setAttribute("aria-expanded", "false");
	}
}

function toggleControl(trigger: HTMLButtonElement, config: ComposerControl) {
	const wrapper = trigger.closest<HTMLElement>(config.control);
	if (!wrapper || wrapper.dataset.composerAttachmentDisabled === "true") {
		return;
	}
	const menu = wrapper.querySelector<HTMLElement>(config.menu);
	if (!menu) {
		return;
	}
	const open = menu.classList.contains("hidden");
	composerControls.closeAll(menu);
	menu.classList.toggle("hidden", !open);
	trigger.setAttribute("aria-expanded", String(open));
	if (open) {
		menu.querySelector<HTMLElement>(`[aria-checked='true'], ${config.option}`)?.focus();
	}
}

export const composerControls = {
	closeAll(except?: HTMLElement) {
		for (const config of Object.values(controls)) {
			closeControl(config, except);
		}
	},

	closeAttachment() {
		closeControl(controls.attachment);
	},

	closeModel() {
		closeControl(controls.model);
	},

	closeReasoning() {
		closeControl(controls.reasoning);
	},

	closeServiceTier() {
		closeControl(controls.serviceTier);
	},

	toggleAttachment(trigger: HTMLButtonElement) {
		toggleControl(trigger, controls.attachment);
	},

	toggleModel(trigger: HTMLButtonElement) {
		toggleControl(trigger, controls.model);
	},

	toggleReasoning(trigger: HTMLButtonElement) {
		toggleControl(trigger, controls.reasoning);
	},

	toggleServiceTier(trigger: HTMLButtonElement) {
		toggleControl(trigger, controls.serviceTier);
	},
};
