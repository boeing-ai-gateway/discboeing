import type { CommandOptions, Context } from "$lib/context/context.types";
import {
	loadCredentialsIntoCache,
	loadCredentialTypesIntoCache,
} from "$lib/context/domains/credentials";
import { loadModelsIntoCache } from "$lib/context/domains/models";
import {
	activateProject,
	stopProjectWatch,
} from "$lib/context/domains/projects";
import { stopSessionWatches } from "$lib/context/domains/sessions";
import { stopThreadWatches } from "$lib/context/domains/threads";
import { retryUntilSuccess } from "$lib/context/retry";

export async function startup(
	context: Context,
	options?: CommandOptions,
): Promise<void> {
	const startupLoads = Promise.all([
		activateProject(context, context.data.project.id, options),
		retryUntilSuccess(() => loadCredentialTypesIntoCache(context)),
		retryUntilSuccess(() => loadCredentialsIntoCache(context, options)),
		retryUntilSuccess(() => loadModelsIntoCache(context)),
	]);

	await startupLoads;
}

export function shutdown(context: Context): void {
	stopThreadWatches(context);
	stopSessionWatches(context);
	stopProjectWatch(context);
}
