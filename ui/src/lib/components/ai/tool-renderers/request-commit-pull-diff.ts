export type RequestCommitPullDiffEntry = {
	path: string;
	oldPath?: string;
	status: string;
	additions: number;
	deletions: number;
	lineCount: number;
	binary: boolean;
	commitHash: string;
	commitSubject: string;
	params: import("$lib/pierre-diff").DiffRendererParams | null;
};
