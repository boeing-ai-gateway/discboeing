import { composerAttachments } from "./composer-attachments";
import { composerControls } from "./composer-controls";
import { workspaceSelector } from "./conversation-workspace-selector";
import { pierreDiffViewer } from "./pierre-diff-viewer";
import { theme } from "./theme";

export { pierreDiffViewer, theme };

type DiscobotComponentRegistry = {
	composerAttachments: typeof composerAttachments;
	composerControls: typeof composerControls;
	pierreDiffViewer: typeof pierreDiffViewer;
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
		pierreDiffViewer,
		theme,
		workspaceSelector,
	};
	theme.registerLegacyGlobals();
}
