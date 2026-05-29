const resizeHandleSelector = "[data-discobot-resize]";
const resizeHandleStep = 16;

const clamp = (value, min, max) => Math.min(max, Math.max(min, value));

const readResizeNumber = (value, fallback) => {
	const parsed = Number.parseFloat(value);
	return Number.isFinite(parsed) ? parsed : fallback;
};

const resizeTarget = (handle) => {
	const selector = handle.dataset.resizeTarget;
	if (!selector) {
		return null;
	}
	return document.querySelector(selector);
};

const resizeBounds = (handle) => ({
	min: readResizeNumber(handle.dataset.resizeMin, 0),
	max: readResizeNumber(handle.dataset.resizeMax, Number.POSITIVE_INFINITY),
});

const resizeSize = (target, axis) => {
	const rect = target.getBoundingClientRect();
	return axis === "y" ? rect.height : rect.width;
};

const applyResizeSize = (handle, target, size) => {
	const axis = handle.dataset.resizeAxis === "y" ? "y" : "x";
	const { min, max } = resizeBounds(handle);
	const nextSize = clamp(size, min, max);
	const value = `${nextSize}px`;
	const sessionWorkspace = target.matches?.(".session-workspace")
		? target
		: target.closest?.(".session-workspace") || target.querySelector?.(".session-workspace");

	if (axis === "y") {
		target.style.height = value;
		target.style.flexBasis = value;
		target.style.setProperty("--terminal-panel-height", value);
		target.style.setProperty("--composer-prompt-height", value);
		if (target.matches(".terminal-panel")) {
			sessionWorkspace?.style.setProperty("--terminal-panel-height", value);
		}
	} else {
		target.style.width = value;
		target.style.flexBasis = value;
		target.style.setProperty("--composer-side-pane-width", value);
		if (target.matches(".session-workspace, .panel-composer")) {
			sessionWorkspace?.style.setProperty("--composer-side-pane-width", value);
		}
	}

	handle.setAttribute("aria-valuenow", String(Math.round(nextSize)));
	return nextSize;
};

const commitResizeSize = async (handle, size, sendCommand) => {
	if (!sendCommand || !handle.dataset.resizeCommand) {
		return;
	}
	if (!handle.dataset.resizeKey && !handle.dataset.resizePanelId) {
		return;
	}

	try {
		await sendCommand({
			url: handle.dataset.resizeCommand,
			payload: {
				key: handle.dataset.resizeKey,
				panelId: handle.dataset.resizePanelId,
				axis: handle.dataset.resizeAxis,
				size: Math.round(size),
			},
		});
	} catch (error) {
		console.error(error);
	}
};

const resizeDelta = (handle, event, start) => {
	const axis = handle.dataset.resizeAxis === "y" ? "y" : "x";
	const position = axis === "y" ? event.clientY : event.clientX;
	const startPosition = axis === "y" ? start.y : start.x;
	const delta = position - startPosition;
	return handle.dataset.resizeDirection === "reverse" ? -delta : delta;
};

const startResize = (handle, event, sendCommand) => {
	if (event.button !== 0) {
		return;
	}

	const target = resizeTarget(handle);
	if (!target) {
		return;
	}

	event.preventDefault();
	const axis = handle.dataset.resizeAxis === "y" ? "y" : "x";
	const start = {
		x: event.clientX,
		y: event.clientY,
		size: resizeSize(target, axis),
	};

	document.body.style.cursor = axis === "y" ? "row-resize" : "col-resize";
	document.body.style.userSelect = "none";

	let nextSize = start.size;
	const onPointerMove = (moveEvent) => {
		nextSize = applyResizeSize(handle, target, start.size + resizeDelta(handle, moveEvent, start));
	};

	const stop = () => {
		document.body.style.cursor = "";
		document.body.style.userSelect = "";
		window.removeEventListener("pointermove", onPointerMove);
		window.removeEventListener("pointerup", stop);
		void commitResizeSize(handle, nextSize, sendCommand);
	};

	window.addEventListener("pointermove", onPointerMove);
	window.addEventListener("pointerup", stop);
};

const resizeWithKeyboard = (handle, event, sendCommand) => {
	const axis = handle.dataset.resizeAxis === "y" ? "y" : "x";
	const direction = handle.dataset.resizeDirection === "reverse" ? -1 : 1;
	let delta = 0;

	if (axis === "x" && event.key === "ArrowRight") {
		delta = resizeHandleStep * direction;
	} else if (axis === "x" && event.key === "ArrowLeft") {
		delta = -resizeHandleStep * direction;
	} else if (axis === "y" && event.key === "ArrowDown") {
		delta = resizeHandleStep * direction;
	} else if (axis === "y" && event.key === "ArrowUp") {
		delta = -resizeHandleStep * direction;
	} else {
		return;
	}

	const target = resizeTarget(handle);
	if (!target) {
		return;
	}

	event.preventDefault();
	const nextSize = applyResizeSize(handle, target, resizeSize(target, axis) + delta);
	void commitResizeSize(handle, nextSize, sendCommand);
};

const initResizableHandles = ({ root = document, sendCommand } = {}) => {
	for (const handle of root.querySelectorAll(resizeHandleSelector)) {
		if (handle.dataset.resizeInitialized === "true") {
			continue;
		}

		handle.dataset.resizeInitialized = "true";
		handle.addEventListener("pointerdown", (event) => startResize(handle, event, sendCommand));
		handle.addEventListener("keydown", (event) => resizeWithKeyboard(handle, event, sendCommand));
	}
};

export const setupResizableHandles = ({ sendCommand } = {}) => {
	initResizableHandles({ sendCommand });

	const observer = new MutationObserver(() => {
		initResizableHandles({ sendCommand });
	});

	observer.observe(document.body, {
		childList: true,
		subtree: true,
	});

	return observer;
};
