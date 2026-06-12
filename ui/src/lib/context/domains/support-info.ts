import { api } from "$lib/api-client";
import type { SupportInfoResponse } from "$lib/api-types";
import type { ResourceStatus } from "$lib/context/cache";
import {
	createErrorStatus,
	createIdleStatus,
	createLoadingStatus,
	createReadyStatus,
} from "$lib/context/cache";
import type { Context } from "$lib/context/context.types";

export type SupportInfoState = {
	value: SupportInfoResponse | null;
	status: ResourceStatus;
};

export function createSupportInfoState(): SupportInfoState {
	return {
		value: null,
		status: createIdleStatus(),
	};
}

export async function fetchSupportInfo(context: Context): Promise<void> {
	context.data.supportInfo.status = createLoadingStatus();
	try {
		context.data.supportInfo.value = await api.getSupportInfo();
		context.data.supportInfo.status = createReadyStatus();
	} catch (error) {
		context.data.supportInfo.status = createErrorStatus(error);
	}
}
