import type { DesktopRendererBridge } from "$lib/desktop/types";

declare global {
	interface Window {
		__DISCBOEING_DESKTOP__?: DesktopRendererBridge;
	}
}

export {};
