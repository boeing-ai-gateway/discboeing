import { getApiBase } from "$lib/api-config";
import {
	getDesktopRuntimeKind,
	isDesktopShell,
	supportsAppUpdates,
	supportsNativeWindowControls,
} from "$lib/shell";
import type { WindowControlsSide } from "$lib/desktop/types";

function detectWindowControlsSide(): WindowControlsSide {
	if (typeof navigator === "undefined") {
		return "right";
	}

	const nav = navigator as Navigator & {
		userAgentData?: {
			platform?: string;
		};
	};
	const platform = nav.userAgentData?.platform || nav.platform || nav.userAgent;
	return /mac/i.test(platform) ? "left" : "right";
}

export function getAppEnvironment() {
	const runtime = getDesktopRuntimeKind();
	const isDesktop = isDesktopShell();
	return {
		apiBase: getApiBase(),
		runtime,
		isDesktop,
		supportsNativeWindowControls: supportsNativeWindowControls(),
		supportsAppUpdates: supportsAppUpdates(),
		windowControlsSide: detectWindowControlsSide(),
	};
}
