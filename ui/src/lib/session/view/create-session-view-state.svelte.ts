import type { WorkspaceValidationResult } from "$lib/api-types";
import type { SessionActiveView } from "$lib/session/session-view.types";
import {
	getDefaultActiveView,
	getSelectedFileFromView,
} from "$lib/session/session-view.types";

type CreateSessionViewStateArgs = {
	getFiles: () => string[];
	getServices: () => string[];
};

export function resolveOpenFileState(
	file: string | undefined,
	selectedFile: string,
): { activeView: SessionActiveView; selectedFile: string } {
	if (file !== undefined) {
		return {
			activeView: { kind: "file", path: file },
			selectedFile: file,
		};
	}

	if (selectedFile.length === 0) {
		return {
			activeView: { kind: "file", path: "" },
			selectedFile: "",
		};
	}
	return {
		activeView: { kind: "file", path: selectedFile },
		selectedFile,
	};
}

export type SessionViewState = {
	activeView: SessionActiveView;
	selectedFile: string;
	activeServiceId: string | null;
	terminalRootEnabled: boolean;
	dockMaximized: boolean;
	composerDraft: string;
	hooksExpanded: boolean;
	queueExpanded: boolean;
	hookDialogOpen: boolean;
	selectedHookId: string | null;
	pendingWorkspaceOption: string;
	pendingWorkspaceBranch: string;
	pendingWorkspaceSourceInput: string;
	pendingWorkspaceValidation: WorkspaceValidationResult | null;
	pendingWorkspaceValidating: boolean;
	pendingWorkspaceSetupMessage: string | null;
	pendingWorkspaceRequiresSourceInput: boolean;
	pendingWorkspaceSourceType: "local" | "git";
	pendingWorkspaceSourceIsValid: boolean;
	pendingWorkspaceValidationMessage: string | null;
	openChat: () => void;
	openTerminal: () => void;
	openDesktop: () => void;
	openDiffReview: () => void;
	openFile: (file?: string) => void;
	openServices: () => void;
	openService: (serviceId: string) => void;
	setTerminalRootEnabled: (value: boolean) => void;
	toggleDockMaximized: () => void;
	setComposerDraft: (value: string) => void;
	setPendingWorkspaceOption: (value: string) => void;
	setPendingWorkspaceBranch: (value: string) => void;
	setPendingWorkspaceSourceInput: (value: string) => void;
	setPendingWorkspaceValidation: (
		value: WorkspaceValidationResult | null,
	) => void;
	setPendingWorkspaceValidating: (value: boolean) => void;
	setPendingWorkspaceSetupMessage: (value: string | null) => void;
	resetPendingWorkspaceSetup: () => void;
	setHooksExpanded: (value: boolean) => void;
	setQueueExpanded: (value: boolean) => void;
	openHookDialog: (hookId: string) => void;
	closeHookDialog: () => void;
	resetForSession: (selectedFile: string) => void;
};

export function createSessionViewState(
	args: CreateSessionViewStateArgs,
): SessionViewState {
	let activeView = $state<SessionActiveView>({ kind: "chat" });
	let selectedFile = $state("");
	let selectedServiceId = $state<string | null>(null);
	let terminalRootEnabled = $state(false);
	let dockMaximized = $state(false);
	let composerDraft = $state("");
	let hooksExpanded = $state(false);
	let queueExpanded = $state(false);
	let hookDialogOpen = $state(false);
	let selectedHookId = $state<string | null>(null);
	let pendingWorkspaceOption = $state("new-workspace");
	let pendingWorkspaceBranch = $state("");
	let pendingWorkspaceSourceInput = $state("");
	let pendingWorkspaceValidation = $state<WorkspaceValidationResult | null>(
		null,
	);
	let pendingWorkspaceValidating = $state(false);
	let pendingWorkspaceSetupMessage = $state<string | null>(null);

	const openChat = () => {
		dockMaximized = false;
		activeView = { kind: "chat" };
	};

	const openTerminal = () => {
		activeView = { kind: "terminal" };
	};

	const openDesktop = () => {
		activeView = { kind: "desktop" };
	};

	const openDiffReview = () => {
		activeView = { kind: "diff-review" };
	};

	const openFile = (file?: string) => {
		const nextState = resolveOpenFileState(file, selectedFile);
		selectedFile = nextState.selectedFile;
		activeView = nextState.activeView;
	};

	const openServices = () => {
		const serviceIds = args.getServices();
		selectedServiceId =
			selectedServiceId && serviceIds.includes(selectedServiceId)
				? selectedServiceId
				: (serviceIds[0] ?? null);
		activeView = { kind: "services" };
	};

	const openService = (serviceId: string) => {
		selectedServiceId = serviceId;
		activeView = { kind: "services" };
	};

	const closeHookDialog = () => {
		hookDialogOpen = false;
		selectedHookId = null;
	};

	const resetPendingWorkspaceSetup = () => {
		pendingWorkspaceOption = "new-workspace";
		pendingWorkspaceBranch = "";
		pendingWorkspaceSourceInput = "";
		pendingWorkspaceValidation = null;
		pendingWorkspaceValidating = false;
		pendingWorkspaceSetupMessage = null;
	};

	return {
		get activeView() {
			return activeView;
		},
		get selectedFile() {
			return activeView.kind === "file" ? activeView.path : selectedFile;
		},
		get activeServiceId() {
			const serviceIds = args.getServices();
			return selectedServiceId && serviceIds.includes(selectedServiceId)
				? selectedServiceId
				: (serviceIds[0] ?? null);
		},
		get terminalRootEnabled() {
			return terminalRootEnabled;
		},
		get dockMaximized() {
			return dockMaximized;
		},
		get composerDraft() {
			return composerDraft;
		},
		get hooksExpanded() {
			return hooksExpanded;
		},
		set hooksExpanded(value: boolean) {
			hooksExpanded = value;
		},
		get queueExpanded() {
			return queueExpanded;
		},
		set queueExpanded(value: boolean) {
			queueExpanded = value;
		},
		get hookDialogOpen() {
			return hookDialogOpen;
		},
		set hookDialogOpen(value: boolean) {
			hookDialogOpen = value;
			if (!value) {
				selectedHookId = null;
			}
		},
		get selectedHookId() {
			return selectedHookId;
		},
		get pendingWorkspaceOption() {
			return pendingWorkspaceOption;
		},
		get pendingWorkspaceBranch() {
			return pendingWorkspaceBranch;
		},
		get pendingWorkspaceSourceInput() {
			return pendingWorkspaceSourceInput;
		},
		get pendingWorkspaceValidation() {
			return pendingWorkspaceValidation;
		},
		get pendingWorkspaceValidating() {
			return pendingWorkspaceValidating;
		},
		get pendingWorkspaceSetupMessage() {
			return pendingWorkspaceSetupMessage;
		},
		get pendingWorkspaceRequiresSourceInput() {
			return (
				pendingWorkspaceOption === "local-directory" ||
				pendingWorkspaceOption === "git-repo"
			);
		},
		get pendingWorkspaceSourceType() {
			return pendingWorkspaceOption === "git-repo" ? "git" : "local";
		},
		get pendingWorkspaceSourceIsValid() {
			if (
				!(
					pendingWorkspaceOption === "local-directory" ||
					pendingWorkspaceOption === "git-repo"
				)
			) {
				return true;
			}

			if (
				pendingWorkspaceSourceInput.trim().length === 0 ||
				pendingWorkspaceValidating
			) {
				return false;
			}

			return pendingWorkspaceValidation?.valid ?? false;
		},
		get pendingWorkspaceValidationMessage() {
			if (
				!(
					pendingWorkspaceOption === "local-directory" ||
					pendingWorkspaceOption === "git-repo"
				)
			) {
				return null;
			}

			if (pendingWorkspaceSourceInput.trim().length === 0) {
				return null;
			}

			if (pendingWorkspaceValidating) {
				return "Validating workspace...";
			}

			if (!pendingWorkspaceValidation) {
				return null;
			}

			if (!pendingWorkspaceValidation.valid) {
				return (
					pendingWorkspaceValidation.error || "Enter a valid workspace path."
				);
			}

			switch (pendingWorkspaceValidation.classification) {
				case "new":
					return "A new directory will be created and initialized as a git repository.";
				case "empty":
					return "Empty directory is valid. It will be initialized as a git repository.";
				case "existing_git":
					return "Existing git repository detected.";
				case "cloneable":
					return "Repository is cloneable.";
				default:
					return null;
			}
		},
		openChat,
		openTerminal,
		openDesktop,
		openDiffReview,
		openFile,
		openServices,
		openService,
		setTerminalRootEnabled: (value) => {
			terminalRootEnabled = value;
		},
		toggleDockMaximized: () => {
			dockMaximized = !dockMaximized;
		},
		setComposerDraft: (value) => {
			composerDraft = value;
		},
		setPendingWorkspaceOption: (value) => {
			pendingWorkspaceOption = value;
		},
		setPendingWorkspaceBranch: (value) => {
			pendingWorkspaceBranch = value;
		},
		setPendingWorkspaceSourceInput: (value) => {
			pendingWorkspaceSourceInput = value;
		},
		setPendingWorkspaceValidation: (value) => {
			pendingWorkspaceValidation = value;
		},
		setPendingWorkspaceValidating: (value) => {
			pendingWorkspaceValidating = value;
		},
		setPendingWorkspaceSetupMessage: (value) => {
			pendingWorkspaceSetupMessage = value;
		},
		resetPendingWorkspaceSetup,
		setHooksExpanded: (value) => {
			hooksExpanded = value;
		},
		setQueueExpanded: (value) => {
			queueExpanded = value;
		},
		openHookDialog: (hookId) => {
			selectedHookId = hookId;
			hookDialogOpen = true;
		},
		closeHookDialog,
		resetForSession: (nextSelectedFile) => {
			selectedFile = nextSelectedFile;
			selectedServiceId = null;
			activeView = { kind: "chat" };
			terminalRootEnabled = false;
			dockMaximized = false;
			composerDraft = "";
			hooksExpanded = false;
			queueExpanded = false;
			closeHookDialog();
			resetPendingWorkspaceSetup();
		},
	};
}

export function isChatView(activeView: SessionActiveView): boolean {
	return activeView.kind === "chat";
}

export function getSelectedFile(activeView: SessionActiveView): string {
	return getSelectedFileFromView(activeView);
}
