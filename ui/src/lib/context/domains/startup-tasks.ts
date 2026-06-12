import type { StartupTask } from "$lib/api-types";
import {
	createCollectionCache,
	type CollectionCache,
} from "$lib/context/cache";

export type StartupTasksState = CollectionCache<StartupTask>;

export function createStartupTasksState(): StartupTasksState {
	return createCollectionCache<StartupTask>();
}
