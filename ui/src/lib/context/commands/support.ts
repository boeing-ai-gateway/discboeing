import { api } from "$lib/api-client";
import { getCommandContext } from "$lib/context/commands";

export async function fetchSupportInfo(): Promise<void> {
	const context = getCommandContext();
	context.data.supportInfo.status = "loading";
	context.data.supportInfo.error = null;
	try {
		context.data.supportInfo.value = await api.getSupportInfo();
		context.data.supportInfo.status = "ready";
	} catch (error) {
		context.data.supportInfo.error =
			error instanceof Error
				? error.message
				: "Failed to load support information.";
		context.data.supportInfo.status = "error";
	}
}
