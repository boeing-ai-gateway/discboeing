import type { CommitOperation, Session } from "../../api-types";
import { CommitStatus } from "../../api-constants";

export type SessionToolbarAction = CommitOperation | "rebase";

export type SessionToolbarOperationState = {
	hasChanges: boolean;
	showSplitButton: boolean;
	primaryAction: SessionToolbarAction;
	primaryLabel: string;
	secondaryAction: SessionToolbarAction | null;
	secondaryLabel: string | null;
	activeOperation: SessionToolbarAction;
	showPending: boolean;
	showBusy: boolean;
	buttonLabel: string;
};

export function getSessionToolbarOperationState(args: {
	filesChanged: number;
	session: Session | null;
	startingOperation: SessionToolbarAction | null;
}): SessionToolbarOperationState {
	const hasChanges = args.filesChanged > 0;
	const primaryAction = "commit";
	const primaryLabel = "Commit";
	const isPending = args.session?.commitStatus === CommitStatus.PENDING;
	const showBusy = args.startingOperation !== null || isPending;
	const activeOperation = args.startingOperation ?? primaryAction;
	const progressLabel =
		args.startingOperation === "rebase"
			? "Rebasing..."
			: args.startingOperation === "commit"
				? "Committing..."
				: "Working...";

	return {
		hasChanges,
		showSplitButton: true,
		primaryAction,
		primaryLabel,
		secondaryAction: "rebase",
		secondaryLabel: "Rebase",
		activeOperation,
		showPending: isPending,
		showBusy,
		buttonLabel: isPending
			? "Pending..."
			: args.startingOperation !== null
				? progressLabel
				: primaryLabel,
	};
}
