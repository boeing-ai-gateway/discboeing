import { api } from "$lib/api-client";
import type {
	CodexAuthorizeResponse,
	CodexCallbackStatusRequest,
	CodexCallbackStatusResponse,
	CodexDeviceCodeResponse,
	CodexExchangeRequest,
	CodexExchangeResponse,
	CodexPollRequest,
	CodexPollResponse,
	CreateCredentialRequest,
	CredentialInfo,
	CredentialType,
	GitHubAuthorizeRequest,
	GitHubAuthorizeResponse,
	GitHubCallbackStatusRequest,
	GitHubCallbackStatusResponse,
	GitHubDeviceCodeRequest,
	GitHubDeviceCodeResponse,
	GitHubExchangeRequest,
	GitHubExchangeResponse,
	GitHubPollRequest,
	GitHubPollResponse,
	OAuthAuthorizeResponse,
	OAuthExchangeRequest,
	OAuthExchangeResponse,
} from "$lib/api-types";
import type { CollectionCache } from "$lib/context/cache";
import {
	createCollectionCache,
	createErrorStatus,
	createLoadingStatus,
	createReadyStatus,
	removeById,
	upsertById,
} from "$lib/context/cache";
import type { CommandOptions, Context } from "$lib/context/context.types";

export type CredentialsState = CollectionCache<CredentialInfo> & {
	types: CredentialType[];
};

export function createCredentialsState(): CredentialsState {
	return {
		...createCollectionCache<CredentialInfo>(),
		types: [],
	};
}

export async function loadCredentialTypesIntoCache(
	context: Context,
): Promise<void> {
	context.data.credentials.status = createLoadingStatus();

	try {
		const response = await api.getCredentialTypes();
		context.data.credentials.types = response.credentialTypes;
		context.data.credentials.status = createReadyStatus();
	} catch (error) {
		context.data.credentials.status = createErrorStatus(error);
		throw error;
	}
}

export async function loadCredentialsIntoCache(
	context: Context,
	options: CommandOptions = {},
): Promise<void> {
	context.data.credentials.status =
		context.data.credentials.status.state === "ready" && !options.wait
			? { state: "refreshing", refreshingSince: Date.now() }
			: createLoadingStatus();

	try {
		const response = await api.getCredentials();
		context.data.credentials.byId = {};
		context.data.credentials.allIds = [];
		for (const credential of response.credentials) {
			upsertById(context.data.credentials, credential.id, credential);
		}
		context.data.credentials.status = createReadyStatus();
	} catch (error) {
		context.data.credentials.status = createErrorStatus(error);
		throw error;
	}
}

export async function createCredential(
	context: Context,
	input: CreateCredentialRequest,
): Promise<CredentialInfo> {
	const credential = await api.createCredential(input);
	upsertById(context.data.credentials, credential.id, credential);
	notifyCredentialsChanged();
	return credential;
}

export async function deleteCredential(
	context: Context,
	credentialId: string,
): Promise<void> {
	await api.deleteCredential(credentialId);
	removeById(context.data.credentials, credentialId);
	notifyCredentialsChanged();
}

export async function toggleCredentialInactive(
	context: Context,
	credential: CredentialInfo,
): Promise<void> {
	const updated = await api.createCredential({
		provider: credential.provider.startsWith("custom:")
			? "custom"
			: credential.provider,
		credentialId: credential.id,
		name: credential.name,
		description: credential.description,
		authType: credential.authType,
		agentVisible: credential.agentVisible,
		visibility: credential.visibility,
		inactive: !credential.inactive,
	});
	upsertById(context.data.credentials, updated.id, updated);
	notifyCredentialsChanged();
}

export async function codexAuthorize(
	_context: Context,
): Promise<CodexAuthorizeResponse> {
	void _context;
	return api.codexAuthorize();
}

export async function codexDeviceCode(
	_context: Context,
): Promise<CodexDeviceCodeResponse> {
	void _context;
	return api.codexDeviceCode();
}

export async function codexCallbackStatus(
	_context: Context,
	input: CodexCallbackStatusRequest,
): Promise<CodexCallbackStatusResponse> {
	return api.codexCallbackStatus(input);
}

export async function codexPoll(
	_context: Context,
	input: CodexPollRequest,
): Promise<CodexPollResponse> {
	return api.codexPoll(input);
}

export async function codexExchange(
	_context: Context,
	input: CodexExchangeRequest,
): Promise<CodexExchangeResponse> {
	const response = await api.codexExchange(input);
	if (response.success) {
		notifyCredentialsChanged();
	}
	return response;
}

export async function githubAuthorize(
	_context: Context,
	input: GitHubAuthorizeRequest,
): Promise<GitHubAuthorizeResponse> {
	return api.githubAuthorize(input);
}

export async function githubDeviceCode(
	_context: Context,
	input: GitHubDeviceCodeRequest,
): Promise<GitHubDeviceCodeResponse> {
	return api.githubDeviceCode(input);
}

export async function githubCallbackStatus(
	_context: Context,
	input: GitHubCallbackStatusRequest,
): Promise<GitHubCallbackStatusResponse> {
	return api.githubCallbackStatus(input);
}

export async function githubPoll(
	_context: Context,
	input: GitHubPollRequest,
): Promise<GitHubPollResponse> {
	const response = await api.githubPoll(input);
	if (response.status === "success") {
		notifyCredentialsChanged();
	}
	return response;
}

export async function githubExchange(
	_context: Context,
	input: GitHubExchangeRequest,
): Promise<GitHubExchangeResponse> {
	const response = await api.githubExchange(input);
	if (response.success) {
		notifyCredentialsChanged();
	}
	return response;
}

export async function anthropicAuthorize(
	_context: Context,
): Promise<OAuthAuthorizeResponse> {
	void _context;
	return api.anthropicAuthorize();
}

export async function anthropicExchange(
	_context: Context,
	input: OAuthExchangeRequest,
): Promise<OAuthExchangeResponse> {
	const response = await api.anthropicExchange(input);
	if (response.success) {
		notifyCredentialsChanged();
	}
	return response;
}

function notifyCredentialsChanged(): void {
	if (typeof window === "undefined") {
		return;
	}
	window.dispatchEvent(new CustomEvent("discboeing:credentials-changed"));
}
