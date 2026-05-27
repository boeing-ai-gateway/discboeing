import { action as datastarAction } from "@starfederation/datastar";
import {
	autoUpdate,
	computePosition,
	flip,
	offset,
	shift,
} from "@floating-ui/dom";

const generationSelector = "#app-shell[data-ui-generation]";

const currentGeneration = () => {
	const shell = document.querySelector(generationSelector);
	return Number(shell?.dataset.uiGeneration ?? 0);
};

const waitForGeneration = (generation, timeout = 5000) => {
	if (!generation || currentGeneration() >= generation) {
		return Promise.resolve();
	}

	return new Promise((resolve, reject) => {
		const timeoutID = window.setTimeout(() => {
			observer.disconnect();
			reject(new Error(`Timed out waiting for UI generation ${generation}`));
		}, timeout);

		const observer = new MutationObserver(() => {
			if (currentGeneration() < generation) {
				return;
			}
			window.clearTimeout(timeoutID);
			observer.disconnect();
			resolve();
		});

		observer.observe(document.body, {
			attributeFilter: ["data-ui-generation"],
			attributes: true,
			childList: true,
			subtree: true,
		});

		if (currentGeneration() >= generation) {
			window.clearTimeout(timeoutID);
			observer.disconnect();
			resolve();
		}
	});
};

const send = async ({ method = "POST", url, payload } = {}) => {
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

	const response = await fetch(url, options);
	if (!response.ok) {
		throw new Error(`Command failed with status ${response.status}`);
	}

	const generation = Number(response.headers.get("X-Discobot-UI-Generation") ?? 0);
	await waitForGeneration(generation);
	return { generation };
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

const sendFromElement = async (el, url, options = {}) => {
	if (!url) {
		return { generation: 0, skipped: true };
	}

	if (el.getAttribute("aria-busy") === "true") {
		return { generation: 0, skipped: true };
	}

	setCommandPending(el, true);
	try {
		const result = await send({
			method: commandMethod(options),
			url,
			payload: options.payload,
		});
		el.dispatchEvent(new CustomEvent("discobot-command-complete", {
			bubbles: true,
			detail: result,
		}));
		return result;
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

datastarAction({
	name: "discobotCommand",
	apply: async ({ el, evt }, url, options = {}) => {
		evt?.preventDefault();
		try {
			return await sendFromElement(el, url, options);
		} catch (error) {
			console.error(error);
			return { generation: 0, error };
		}
	},
});

window.discobot = window.discobot ?? {};

window.discobot.floatingUI = {
	autoUpdate,
	computePosition,
	flip,
	offset,
	shift,
};

window.discobot.command = {
	send,
	sendFromElement,
	waitForGeneration,
};
