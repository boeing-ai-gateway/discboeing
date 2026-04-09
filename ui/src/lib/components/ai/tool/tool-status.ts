import type { ToolState } from "../types";

export type ToolDisplayState = ToolState | "queued";

export function getToolStatusLabel(state: ToolDisplayState): string {
	switch (state) {
		case "input-streaming":
			return "Preparing";
		case "input-available":
			return "Running";
		case "queued":
			return "Queued";
		case "approval-requested":
			return "Awaiting Approval";
		case "approval-responded":
			return "Responded";
		case "output-available":
			return "Completed";
		case "output-error":
			return "Error";
		case "output-denied":
			return "Denied";
	}
}

export function isToolRunningState(state: ToolDisplayState): boolean {
	return state === "input-streaming" || state === "input-available";
}

export function isToolPreparingState(state: ToolDisplayState): boolean {
	return state === "input-streaming";
}
