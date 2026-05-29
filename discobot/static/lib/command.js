import { action as datastarAction } from "@starfederation/datastar";
import { showCommandError } from "./toast.js";

let pendingCommandCount = 0;

const setGlobalCommandPending = (pending) => {
	pendingCommandCount = Math.max(0, pendingCommandCount + (pending ? 1 : -1));
	document.body.toggleAttribute("data-discobot-command-pending", pendingCommandCount > 0);
};

export const send = async ({ method = "POST", url, payload } = {}) => {
	if (!url) {
		throw new Error("Command URL is required");
	}

	const headers = {
		Accept: "application/json",
		"X-Requested-With": "discobot-command",
	};
	const options = { method, headers };
	if (payload !== undefined) {
		headers["Content-Type"] = "application/json";
		options.body = JSON.stringify(payload);
	}

	setGlobalCommandPending(true);
	try {
		const response = await fetch(url, options);
		if (!response.ok) {
			throw new Error(`Command failed with status ${response.status}`);
		}
	} catch (error) {
		showCommandError(error);
		throw error;
	} finally {
		setGlobalCommandPending(false);
	}
};

const commandMethod = (options = {}) => {
	return options.method || "POST";
};

const setCommandPending = (el, pending) => {
	if (pending) {
		el.setAttribute("aria-busy", "true");
		if ("disabled" in el) {
			el.disabled = true;
		}
		return;
	}

	el.removeAttribute("aria-busy");
	if ("disabled" in el) {
		el.disabled = false;
	}
};

export const sendFromElement = async (el, url, options = {}) => {
	if (!url) {
		return;
	}

	if (el.getAttribute("aria-busy") === "true") {
		return;
	}

	setCommandPending(el, true);
	try {
		await send({
			method: commandMethod(options),
			url,
			payload: options.payload,
		});
		el.dispatchEvent(new CustomEvent("discobot-command-complete", {
			bubbles: true,
		}));
	} catch (error) {
		el.dispatchEvent(new CustomEvent("discobot-command-error", {
			bubbles: true,
			detail: { error },
		}));
		throw error;
	} finally {
		setCommandPending(el, false);
	}
};

export const setupCommands = () => {
	datastarAction({
		name: "discobotCommand",
		apply: async ({ el, evt }, url, options = {}) => {
			evt?.preventDefault();
			try {
				await sendFromElement(el, url, options);
			} catch (error) {
				console.error(error);
			}
		},
	});

	window.discobot = window.discobot ?? {};
	window.discobot.command = {
		send,
		sendFromElement,
	};

	return window.discobot.command;
};
