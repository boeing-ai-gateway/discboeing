export type PlanEntryStatus = "pending" | "in_progress" | "completed";

export type PlanEntry = {
	content: string;
	status: PlanEntryStatus;
	activeForm: string;
	priority?: "low" | "medium" | "high";
};
