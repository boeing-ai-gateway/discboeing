import { readStorage, writeStorage } from "$lib/local-storage";
import type { DiffStyle } from "$lib/pierre-diff";

const APPROVAL_STORAGE_KEY = "discboeing.ui.diff-review.approved";
const DIFF_STYLE_STORAGE_KEY = "discboeing.ui.diff-review.style";

export type DiffReviewApprovals = Record<string, Record<string, string>>;

function readApprovals(): DiffReviewApprovals {
	const stored = readStorage(APPROVAL_STORAGE_KEY);
	if (!stored) {
		return {};
	}

	try {
		const parsed = JSON.parse(stored);
		return typeof parsed === "object" && parsed !== null ? parsed : {};
	} catch {
		return {};
	}
}

function readStyle(): DiffStyle {
	return readStorage(DIFF_STYLE_STORAGE_KEY) === "split" ? "split" : "unified";
}

export const diffReviewPreferencesStore = {
	readApprovals,
	setApprovals(approvals: DiffReviewApprovals): DiffReviewApprovals {
		writeStorage(APPROVAL_STORAGE_KEY, JSON.stringify(approvals));
		return approvals;
	},
	readStyle,
	setStyle(style: DiffStyle): DiffStyle {
		writeStorage(DIFF_STYLE_STORAGE_KEY, style);
		return style;
	},
};
