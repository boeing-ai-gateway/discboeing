import { api } from "$lib/api-client";
import type { AppCredentials } from "$lib/app/app-context.types";
import type {
	CodexAuthorizeResponse,
	CodexCallbackStatusRequest,
	CodexCallbackStatusResponse,
	CodexDeviceCodeResponse,
	CodexExchangeRequest,
	CodexExchangeResponse,
	CodexPollRequest,
	CodexPollResponse,
	CredentialAuthType,
	CredentialVisibility,
	CredentialInfo,
	GitHubDeviceCodeRequest,
	GitHubDeviceCodeResponse,
	GitHubPollRequest,
	OAuthAuthorizeResponse,
	OAuthExchangeRequest,
	OAuthExchangeResponse,
	OAuthRefreshResponse,
} from "$lib/api-types";
import type { CredentialStore } from "$lib/store/credentials.store.svelte";

type CreateAppCredentialsDomainArgs = {
	store: CredentialStore;
	refreshModels: () => Promise<void>;
};

export function createAppCredentialsDomain(
	args: CreateAppCredentialsDomainArgs,
): AppCredentials {
	const { store, refreshModels } = args;

	const refreshModelsAfter = async <T>(task: () => Promise<T>): Promise<T> => {
		const result = await task();
		await refreshModels();
		return result;
	};

	const saveCredential = async (data: {
		provider?: string;
		credentialId?: string;
		name: string;
		description?: string;
		authType: CredentialAuthType;
		apiKey?: string;
		envVars?: { key: string; value: string }[];
		visibility?: CredentialVisibility;
		inactive?: boolean;
	}): Promise<CredentialInfo> => {
		return refreshModelsAfter(() => store.save(data));
	};

	return {
		get list() {
			return store.list;
		},
		get credentialTypes() {
			return store.credentialTypes;
		},
		get: (idOrProvider) => store.get(idOrProvider),
		refresh: () => store.fetch(),
		create: saveCredential,
		update: saveCredential,
		remove: (provider) => refreshModelsAfter(() => store.remove(provider)),
		refreshCredential: async (provider) => {
			const response = await api.refreshCredential(provider);
			await store.fetch();
			await refreshModels();
			return response as OAuthRefreshResponse;
		},
		anthropicAuthorize: (): Promise<OAuthAuthorizeResponse> =>
			api.anthropicAuthorize(),
		anthropicExchange: async (
			data: OAuthExchangeRequest,
		): Promise<OAuthExchangeResponse> => {
			const response = await api.anthropicExchange(data);
			await store.fetch();
			await refreshModels();
			return response;
		},
		githubDeviceCode: (
			data?: GitHubDeviceCodeRequest,
		): Promise<GitHubDeviceCodeResponse> => api.githubDeviceCode(data),
		githubPoll: async (data: GitHubPollRequest) => {
			const response = await api.githubPoll(data);
			if (response.status === "success") {
				await store.fetch();
				await refreshModels();
			}
			return response;
		},
		codexAuthorize: (): Promise<CodexAuthorizeResponse> => api.codexAuthorize(),
		codexExchange: async (
			data: CodexExchangeRequest,
		): Promise<CodexExchangeResponse> => {
			const response = await api.codexExchange(data);
			if (response.success) {
				await store.fetch();
				await refreshModels();
			}
			return response;
		},
		codexDeviceCode: (): Promise<CodexDeviceCodeResponse> =>
			api.codexDeviceCode(),
		codexPoll: async (data: CodexPollRequest): Promise<CodexPollResponse> => {
			const response = await api.codexPoll(data);
			if (response.status === "success") {
				await store.fetch();
				await refreshModels();
			}
			return response;
		},
		codexCallbackStatus: (
			data: CodexCallbackStatusRequest,
		): Promise<CodexCallbackStatusResponse> => api.codexCallbackStatus(data),
	};
}
