import type { SessionViewState } from "$lib/context/context.types";

type PendingWorkspaceState = SessionViewState["pendingWorkspace"];

export function getPendingWorkspaceRequiresSourceInput(
	pendingWorkspace: Pick<PendingWorkspaceState, "option"> | null | undefined,
): boolean {
	return (
		pendingWorkspace?.option === "local-directory" ||
		pendingWorkspace?.option === "git-repo"
	);
}

export function getPendingWorkspaceSourceIsValid(
	pendingWorkspace: PendingWorkspaceState | null | undefined,
): boolean {
	if (!getPendingWorkspaceRequiresSourceInput(pendingWorkspace)) {
		return true;
	}
	if (
		!pendingWorkspace ||
		pendingWorkspace.sourceInput.trim().length === 0 ||
		pendingWorkspace.validating
	) {
		return false;
	}
	return pendingWorkspace.validation?.valid ?? false;
}

export function getPendingWorkspaceValidationMessage(
	pendingWorkspace: PendingWorkspaceState | null | undefined,
): string | null {
	if (
		!getPendingWorkspaceRequiresSourceInput(pendingWorkspace) ||
		!pendingWorkspace
	) {
		return null;
	}
	if (pendingWorkspace.sourceInput.trim().length === 0) {
		return null;
	}
	if (pendingWorkspace.validating) {
		return "Validating workspace...";
	}
	const validation = pendingWorkspace.validation;
	if (!validation) {
		return null;
	}
	if (!validation.valid) {
		return validation.error || "Enter a valid workspace path.";
	}
	switch (validation.classification) {
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
}
