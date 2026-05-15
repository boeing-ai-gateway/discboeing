import type { AppStartupStatus } from "$lib/app/app-context.types";
import type { StartupTaskStore } from "$lib/store/startup-tasks.store";

function isTaskActive(task: { state: string }) {
	return task.state === "pending" || task.state === "in_progress";
}

type CreateAppStartupStatusDomainArgs = {
	store: StartupTaskStore;
};

export function createAppStartupStatusDomain(
	args: CreateAppStartupStatusDomainArgs,
): AppStartupStatus {
	const { store } = args;

	return {
		get tasks() {
			return store.list;
		},
		get visibleTasks() {
			return store.list.filter((task) => task.state !== "completed");
		},
		get hasActiveTasks() {
			return store.list.some(isTaskActive);
		},
		peek: (taskId) => store.peek(taskId),
		ensure: (taskId) => store.ensure(taskId),
		refresh: () => store.fetch(),
	};
}
