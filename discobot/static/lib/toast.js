const toastRootID = "discobot-toast-root";
const toastDuration = 6000;

const toastRoot = () => {
	let root = document.getElementById(toastRootID);
	if (root) {
		return root;
	}

	root = document.createElement("div");
	root.id = toastRootID;
	root.className = "discobot-toast-root";
	root.setAttribute("aria-live", "polite");
	root.setAttribute("aria-atomic", "false");
	document.body.append(root);
	return root;
};

const errorMessage = (error) => {
	if (error instanceof Error && error.message) {
		return error.message;
	}
	return "Command failed";
};

export const showToast = ({ title = "Discobot", message, variant = "default" } = {}) => {
	const toast = document.createElement("div");
	toast.className = `discobot-toast discobot-toast--${variant}`;
	toast.setAttribute("role", variant === "error" ? "alert" : "status");

	const titleElement = document.createElement("div");
	titleElement.className = "discobot-toast--title";
	titleElement.textContent = title;
	toast.append(titleElement);

	if (message) {
		const messageElement = document.createElement("div");
		messageElement.className = "discobot-toast--message";
		messageElement.textContent = message;
		toast.append(messageElement);
	}

	toastRoot().append(toast);
	window.setTimeout(() => {
		toast.classList.add("discobot-toast--leaving");
		window.setTimeout(() => toast.remove(), 180);
	}, toastDuration);

	return toast;
};

export const showCommandError = (error) => showToast({
	title: "Command failed",
	message: errorMessage(error),
	variant: "error",
});
