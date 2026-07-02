import { readStorage } from "$lib/local-storage";

const SIDEBAR_LAYOUT_STORAGE_KEY = "paneforge:discboeing-ui-sidebar-layout";

export const sidebarLayoutStore = {
	hasSavedDesktopLayout(): boolean {
		return readStorage(SIDEBAR_LAYOUT_STORAGE_KEY) !== null;
	},
};
