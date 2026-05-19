import { composerAttachments } from "./composer-attachments";
import { composerControls } from "./composer-controls";
import { workspaceSelector } from "./conversation-workspace-selector";
import { dialog } from "./dialog";
import { pierreDiffViewer } from "./pierre-diff-viewer";
import { popover } from "./popover";
import { theme } from "./theme";

export { pierreDiffViewer, theme };

type DiscobotComponentRegistry = {
	composerAttachments: typeof composerAttachments;
	composerControls: typeof composerControls;
	dialog: typeof dialog;
	pierreDiffViewer: typeof pierreDiffViewer;
	popover: typeof popover;
	theme: typeof theme;
	workspaceSelector: typeof workspaceSelector;
};

export function registerComponents() {
	const discobotGlobal = globalThis as typeof globalThis & {
		discobot?: Partial<DiscobotComponentRegistry>;
	};
	discobotGlobal.discobot = {
		...discobotGlobal.discobot,
		composerAttachments,
		composerControls,
		dialog,
		pierreDiffViewer,
		popover,
		theme,
		workspaceSelector,
	};
	dialog.install();
	popover.install();
	theme.registerLegacyGlobals();
}
