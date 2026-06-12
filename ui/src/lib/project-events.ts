import type { Context } from "$lib/context/context.types";
import type { ProjectEventSocket } from "$lib/context/project-subscription";

const projectEventSockets = new WeakMap<Context, ProjectEventSocket>();

export function getProjectEventSocket(
	context: Context,
): ProjectEventSocket | null {
	return projectEventSockets.get(context) ?? null;
}

export function setProjectEventSocket(
	context: Context,
	socket: ProjectEventSocket | null,
): void {
	if (socket) {
		projectEventSockets.set(context, socket);
		return;
	}
	projectEventSockets.delete(context);
}
