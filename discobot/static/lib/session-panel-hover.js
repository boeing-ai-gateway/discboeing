export function setupSessionPanelHover() {
	document.addEventListener("mouseover", (event) => {
		const conversation = event.target.closest?.(".conversation");
		if (!conversation) {
			return;
		}

		conversation.closest(".session-workspace--center")?.classList.add("session-workspace--center--thread-hovered");
	});

	document.addEventListener("mouseout", (event) => {
		const conversation = event.target.closest?.(".conversation");
		if (!conversation) {
			return;
		}
		if (event.relatedTarget instanceof Node && conversation.contains(event.relatedTarget)) {
			return;
		}

		conversation.closest(".session-workspace--center")?.classList.remove("session-workspace--center--thread-hovered");
	});
}
