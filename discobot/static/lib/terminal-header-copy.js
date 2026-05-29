import { showToast } from "./toast.js";

const copyResetMs = 2000;
const sshPort = 3333;
const workspacePath = "/home/discobot/workspace";

const sshHost = () => {
	const { hostname } = window.location;
	if (hostname === "127.0.0.1" || hostname === "::1") {
		return "localhost";
	}
	return hostname || "localhost";
};

const commandForButton = (button) => {
	const kind = button.dataset.terminalCopyKind;
	const text = button.dataset.terminalCopyText || "";
	const match = text.match(/(?:ssh -p 3333 |ssh:\/\/)([^@\s"]+)@/);
	const sessionID = match?.[1] || "";
	if (!sessionID) {
		return text;
	}

	if (kind === "ssh") {
		return `ssh -p ${sshPort} ${sessionID}@${sshHost()}`;
	}
	if (kind === "pull") {
		return `git pull "ssh://${sessionID}@${sshHost()}:${sshPort}${workspacePath}" HEAD`;
	}
	return text;
};

const writeClipboardText = async (text) => {
	if (navigator.clipboard?.writeText) {
		await navigator.clipboard.writeText(text);
		return;
	}

	const textarea = document.createElement("textarea");
	textarea.value = text;
	textarea.setAttribute("readonly", "");
	textarea.style.position = "fixed";
	textarea.style.opacity = "0";
	document.body.append(textarea);
	textarea.select();
	try {
		document.execCommand("copy");
	} finally {
		textarea.remove();
	}
};

const markCopied = (button) => {
	const label = button.querySelector("[data-terminal-copy-label]");
	if (!label) {
		return;
	}

	const originalLabel = label.dataset.originalLabel || label.textContent || "Copy";
	label.dataset.originalLabel = originalLabel;
	label.textContent = "Copied!";
	window.clearTimeout(Number(button.dataset.copyResetTimer || 0));
	const timer = window.setTimeout(() => {
		label.textContent = originalLabel;
		delete button.dataset.copyResetTimer;
	}, copyResetMs);
	button.dataset.copyResetTimer = String(timer);
};

export const setupTerminalHeaderCopy = () => {
	document.addEventListener("click", (event) => {
		const button = event.target.closest?.("[data-terminal-copy-button]");
		if (!button) {
			return;
		}

		event.preventDefault();
		void (async () => {
			try {
				await writeClipboardText(commandForButton(button));
				markCopied(button);
			} catch (error) {
				console.error(error);
				showToast({
					title: "Copy failed",
					message: error instanceof Error ? error.message : "Unable to copy command",
					variant: "error",
				});
			}
		})();
	});
};
