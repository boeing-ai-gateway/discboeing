import type { DynamicToolPart } from "$lib/components/ai/types";
import type { PlanEntry } from "$lib/plan-entry";
import type { ResolvedTheme } from "$lib/theme";

export type ToolRendererComponentProps = {
	toolPart: DynamicToolPart;
	queued?: boolean;
	sessionId?: string | null;
	threadId?: string | null;
	resolvedTheme?: ResolvedTheme;
	previousTodoEntries?: PlanEntry[];
	onToolApprovalResponse?: (payload: {
		id: string;
		approved: boolean;
		reason?: string;
	}) => void;
	isRaw?: boolean;
	onToggleRaw?: () => void;
};
