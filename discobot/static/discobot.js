import {
	autoUpdate,
	computePosition,
	flip,
	offset,
	shift,
} from "@floating-ui/dom";
import { send, setupCommands } from "./lib/command.js";
import { setupMenus } from "./lib/menu.js";
import { setupResizableHandles } from "./lib/resize.js";
import { setupSessionPanelHover } from "./lib/session-panel-hover.js";
import { setupSessionSidebarDropdown } from "./lib/session-sidebar-dropdown.js";
import { setupTerminalHeaderCopy } from "./lib/terminal-header-copy.js";

setupCommands();
setupMenus();
setupSessionPanelHover();
setupSessionSidebarDropdown();
setupTerminalHeaderCopy();

window.discobot = window.discobot ?? {};

window.discobot.floatingUI = {
	autoUpdate,
	computePosition,
	flip,
	offset,
	shift,
};

setupResizableHandles({ sendCommand: send });
