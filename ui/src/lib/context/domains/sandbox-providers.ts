import { api } from "$lib/api-client";
import type {
	SandboxProviderInstance,
	SandboxProviderType,
} from "$lib/api-types";
import type { CollectionCache } from "$lib/context/cache";
import {
	createCollectionCache,
	createErrorStatus,
	createLoadingStatus,
	createReadyStatus,
} from "$lib/context/cache";
import type { CommandOptions, Context } from "$lib/context/context.types";

export type SandboxProvidersState = {
	types: CollectionCache<SandboxProviderType>;
	instances: CollectionCache<SandboxProviderInstance>;
	defaultProviderId: string;
	projectDefaultProviderId: string;
	systemDefaultProviderId: string;
};

export type SandboxProviderMutationInput = {
	type?: string;
	name?: string;
	icon?: string;
	config?: Record<string, unknown>;
	disabled?: boolean;
};

export type CreateSandboxProviderInput = SandboxProviderMutationInput & {
	type: string;
	name: string;
};

export function createSandboxProvidersState(): SandboxProvidersState {
	return {
		types: createCollectionCache(),
		instances: createCollectionCache(),
		defaultProviderId: "",
		projectDefaultProviderId: "",
		systemDefaultProviderId: "",
	};
}

export async function loadSandboxProvidersIntoCache(
	context: Context,
	options: CommandOptions = {},
): Promise<void> {
	void options;
	context.data.sandboxProviders.types.status = createLoadingStatus();
	context.data.sandboxProviders.instances.status = createLoadingStatus();

	try {
		const [typesResponse, providersResponse] = await Promise.all([
			api.getSandboxProviderTypes(),
			api.getSandboxProviders(),
		]);
		context.data.sandboxProviders.types.byId = Object.fromEntries(
			typesResponse.providerTypes.map((providerType) => [
				providerType.id,
				providerType,
			]),
		);
		context.data.sandboxProviders.types.allIds =
			typesResponse.providerTypes.map((providerType) => providerType.id);
		context.data.sandboxProviders.types.status = createReadyStatus();

		context.data.sandboxProviders.instances.byId = Object.fromEntries(
			providersResponse.providers.map((provider) => [provider.id, provider]),
		);
		context.data.sandboxProviders.instances.allIds =
			providersResponse.providers.map((provider) => provider.id);
		context.data.sandboxProviders.instances.status = createReadyStatus();
		context.data.sandboxProviders.defaultProviderId = providersResponse.default;
		context.data.sandboxProviders.projectDefaultProviderId =
			providersResponse.projectDefault ?? "";
		context.data.sandboxProviders.systemDefaultProviderId =
			providersResponse.systemDefault ?? "";
	} catch (error) {
		context.data.sandboxProviders.types.status = createErrorStatus(error);
		context.data.sandboxProviders.instances.status = createErrorStatus(error);
		throw error;
	}
}

export async function createSandboxProvider(
	context: Context,
	input: CreateSandboxProviderInput,
	options: CommandOptions = {},
): Promise<void> {
	void options;
	await api.createSandboxProvider(input);
	await loadSandboxProvidersIntoCache(context);
}

export async function updateSandboxProvider(
	context: Context,
	id: string,
	input: SandboxProviderMutationInput,
	options: CommandOptions = {},
): Promise<void> {
	void options;
	await api.updateSandboxProvider(id, input);
	await loadSandboxProvidersIntoCache(context);
}

export async function deleteSandboxProvider(
	context: Context,
	id: string,
	options: CommandOptions = {},
): Promise<void> {
	void options;
	await api.deleteSandboxProvider(id);
	await loadSandboxProvidersIntoCache(context);
}

export async function updateDefaultSandboxProvider(
	context: Context,
	providerId: string,
	options: CommandOptions = {},
): Promise<void> {
	void options;
	await api.updateDefaultSandboxProvider(providerId);
	await loadSandboxProvidersIntoCache(context);
}
