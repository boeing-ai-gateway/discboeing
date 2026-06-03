import { api } from "$lib/api-client";
import type {
	AgentCommand,
	AgentCommandCredentialRequest,
	CredentialInfo,
	CredentialType,
	SessionCredentialAssignment,
} from "$lib/api-types";
import type { AppRuntime } from "$lib/app/app-runtime.svelte";
import {
	buildCredentialUseExpiryFromPreset,
	buildAssignmentUses,
	CUSTOM_CREDENTIAL_OPTION,
	type CredentialValidityPreset,
	credentialDisplayName,
	findPreferredCredentialId,
	defaultCredentialName,
	parseOAuthCredentialOption,
	preferredSourceEnvVar,
} from "$lib/components/ai/tool-renderers/requestusercredential-helpers";
import { createResource } from "$lib/resource/create-resource.svelte";
import type { SessionCommandsDomain } from "$lib/session/session-context.types";

type CreateSessionCommandsDomainArgs = {
	runtime: AppRuntime;
	sessionId: string;
	hasSession: () => boolean;
	getSelectedThreadId: () => string;
	submit: (text: string, options?: { threadId?: string }) => Promise<unknown>;
};

export function createSessionCommandsDomain(
	args: CreateSessionCommandsDomainArgs,
): SessionCommandsDomain {
	const resource = createResource<AgentCommand[]>({
		owner: "SessionCommands",
		enabled: () => args.hasSession(),
		createEmptyValue: () => [],
		load: async () => {
			const response = await api.getSessionCommands(args.sessionId);
			return response.commands;
		},
		retry: { mode: "background" },
	});
	let isSubmitting = $state(false);
	let credentialDialogOpen = $state(false);
	let credentialDialogCommand = $state<AgentCommand | null>(null);
	let credentialDialogRequests = $state<AgentCommandCredentialRequest[]>([]);
	let credentialDialogProjectCredentials = $state<CredentialInfo[]>([]);
	let credentialDialogCredentialTypes = $state(
		args.runtime.getCredentialTypes(),
	);
	let credentialDialogSessionAssignments = $state<
		SessionCredentialAssignment[]
	>([]);
	let selectedOptionByEnvVar = $state<Record<string, string>>({});
	let createCredentialNamesByEnvVar = $state<Record<string, string>>({});
	let createCredentialSecretsByEnvVar = $state<Record<string, string>>({});
	let validityPresetByEnvVar = $state<Record<string, CredentialValidityPreset>>(
		{},
	);
	let validityValueByEnvVar = $state<Record<string, string>>({});
	let validityUnitByEnvVar = $state<
		Record<string, "hours" | "days" | "weeks" | "never">
	>({});
	let credentialDialogError = $state<string | null>(null);

	const uiVisible = $derived.by(() =>
		[...resource.data]
			.filter((command) => command.discobot?.ui)
			.sort((left, right) => {
				const leftOrder = left.discobot?.order ?? 0;
				const rightOrder = right.discobot?.order ?? 0;
				if (leftOrder !== rightOrder) {
					return leftOrder - rightOrder;
				}
				return left.name.localeCompare(right.name);
			}),
	);

	async function sendMessages(threadId: string, text: string) {
		await args.submit(text, { threadId });
	}

	function resetCredentialDialog() {
		credentialDialogOpen = false;
		credentialDialogCommand = null;
		credentialDialogRequests = [];
		credentialDialogProjectCredentials = [];
		credentialDialogCredentialTypes = [];
		credentialDialogSessionAssignments = [];
		selectedOptionByEnvVar = {};
		createCredentialNamesByEnvVar = {};
		createCredentialSecretsByEnvVar = {};
		validityPresetByEnvVar = {};
		validityValueByEnvVar = {};
		validityUnitByEnvVar = {};
		credentialDialogError = null;
	}

	function findMatchingOAuthCredential(args: {
		requestEnvVar: string;
		credentialType: CredentialType;
		projectCredentials: CredentialInfo[];
	}): CredentialInfo | null {
		return (
			[...args.projectCredentials]
				.filter(
					(credential) =>
						credential.provider === args.credentialType.backendProvider &&
						credential.authType === args.credentialType.authType &&
						credential.isConfigured &&
						!credential.inactive,
				)
				.sort((left, right) =>
					(right.updatedAt ?? "").localeCompare(left.updatedAt ?? ""),
				)
				.find((credential) =>
					credential.envKeys?.includes(args.requestEnvVar),
				) ??
			[...args.projectCredentials]
				.filter(
					(credential) =>
						credential.provider === args.credentialType.backendProvider &&
						credential.authType === args.credentialType.authType &&
						credential.isConfigured &&
						!credential.inactive,
				)
				.sort((left, right) =>
					(right.updatedAt ?? "").localeCompare(left.updatedAt ?? ""),
				)[0] ??
			null
		);
	}

	async function refreshCredentialDialogContext() {
		if (!credentialDialogOpen) {
			return;
		}
		await args.runtime.refreshCredentials();
		const assignmentsResponse = await api.getSessionCredentials(args.sessionId);
		credentialDialogProjectCredentials = args.runtime.getCredentials();
		credentialDialogCredentialTypes = args.runtime.getCredentialTypes();
		credentialDialogSessionAssignments = assignmentsResponse.credentials;
		selectedOptionByEnvVar = Object.fromEntries(
			Object.entries(selectedOptionByEnvVar).map(([envVar, value]) => {
				const selectedOAuthType = parseOAuthCredentialOption(
					value,
					credentialDialogCredentialTypes,
				);
				if (!selectedOAuthType) {
					return [envVar, value];
				}
				const matchingCredential = findMatchingOAuthCredential({
					requestEnvVar: envVar,
					credentialType: selectedOAuthType,
					projectCredentials: credentialDialogProjectCredentials,
				});
				return [envVar, matchingCredential?.id ?? value];
			}),
		);
	}

	async function launchOAuthCredentialWizard(envVar: string) {
		const option = selectedOptionByEnvVar[envVar]?.trim() ?? "";
		const selectedOAuthType = parseOAuthCredentialOption(
			option,
			credentialDialogCredentialTypes,
		);
		if (!selectedOAuthType) {
			return;
		}
		credentialDialogError = null;
		if (selectedOAuthType.backendProvider === "github-git") {
			args.runtime.openCredentialFlow("github-git");
			return;
		}
		if (selectedOAuthType.backendProvider === "codex") {
			args.runtime.openCredentialFlow("codex");
			return;
		}
		credentialDialogError = `${selectedOAuthType.name} OAuth isn't supported in this flow yet.`;
	}

	async function prepareCommandCredentialDialog(command: AgentCommand) {
		if (
			args.runtime.getCredentials().length === 0 ||
			args.runtime.getCredentialTypes().length === 0
		) {
			await args.runtime.refreshCredentials();
		}
		const assignmentsResponse = await api.getSessionCredentials(args.sessionId);
		const sessionAssignments = assignmentsResponse.credentials;
		const requests = command.discobot?.credentialRequest ?? [];
		const initialSelections = Object.fromEntries(
			requests.map((request) => {
				return [
					request.envVar,
					findPreferredCredentialId(
						request.envVar,
						args.runtime.getCredentials(),
						sessionAssignments,
					),
				];
			}),
		);
		const initialNames = Object.fromEntries(
			requests.map((request) => [
				request.envVar,
				defaultCredentialName({
					envVar: request.envVar,
					name: request.name,
					justification: request.justification,
					approvedUses: request.approvedUses ?? [],
				}),
			]),
		);
		const initialValidityPresets = Object.fromEntries(
			requests.map((request) => [request.envVar, "1_hour"]),
		) as Record<string, CredentialValidityPreset>;
		const initialValidityValues = Object.fromEntries(
			requests.map((request) => [request.envVar, "1"]),
		);
		const initialValidityUnits = Object.fromEntries(
			requests.map((request) => [request.envVar, "hours"]),
		) as Record<string, "hours" | "days" | "weeks" | "never">;

		credentialDialogCommand = command;
		credentialDialogRequests = requests;
		credentialDialogProjectCredentials = args.runtime.getCredentials();
		credentialDialogCredentialTypes = args.runtime.getCredentialTypes();
		credentialDialogSessionAssignments = sessionAssignments;
		selectedOptionByEnvVar = initialSelections;
		createCredentialNamesByEnvVar = initialNames;
		createCredentialSecretsByEnvVar = {};
		validityPresetByEnvVar = initialValidityPresets;
		validityValueByEnvVar = initialValidityValues;
		validityUnitByEnvVar = initialValidityUnits;
		credentialDialogError = null;
		credentialDialogOpen = true;
	}

	async function prepareCommandCredentials(
		requests: AgentCommandCredentialRequest[],
		sessionAssignments: SessionCredentialAssignment[],
		projectCredentials: CredentialInfo[],
		credentialTypes: CredentialType[],
		selectedOptions: Record<string, string>,
		customCredentialNames: Record<string, string>,
		customCredentialSecrets: Record<string, string>,
		validityPresets: Record<string, CredentialValidityPreset>,
		validityValues: Record<string, string>,
		validityUnits: Record<string, "hours" | "days" | "weeks" | "never">,
	) {
		const nextAssignments = [...sessionAssignments];

		for (const request of requests) {
			const option = selectedOptions[request.envVar]?.trim();
			if (!option) {
				throw new Error(`Select a credential for ${request.envVar}.`);
			}
			const selectedOAuthType = parseOAuthCredentialOption(
				option,
				credentialTypes,
			);
			if (selectedOAuthType) {
				throw new Error(
					`${selectedOAuthType.name} OAuth isn't wired up here yet.`,
				);
			}
			const expiresAt = buildCredentialUseExpiryFromPreset(
				validityPresets[request.envVar] ?? "1_hour",
				validityValues[request.envVar] ?? "1",
				validityUnits[request.envVar] ?? "hours",
			);
			let credential = projectCredentials.find((item) => item.id === option);
			if (option === CUSTOM_CREDENTIAL_OPTION) {
				const secret = customCredentialSecrets[request.envVar]?.trim() ?? "";
				if (!secret) {
					throw new Error(`Enter a credential value for ${request.envVar}.`);
				}
				credential = await api.createCredential({
					name:
						customCredentialNames[request.envVar]?.trim() ||
						defaultCredentialName({
							envVar: request.envVar,
							name: request.name,
							justification: request.justification,
							approvedUses: request.approvedUses ?? [],
						}),
					description: request.justification.trim() || undefined,
					authType: "api_key",
					envVars: [{ key: request.envVar, value: secret }],
					agentVisible: false,
				});
				projectCredentials = [...projectCredentials, credential];
			}
			if (!credential) {
				throw new Error(
					`Selected credential for ${request.envVar} was not found.`,
				);
			}
			const sourceEnvVar = preferredSourceEnvVar(request.envVar, credential);
			if (!sourceEnvVar) {
				throw new Error(
					`Credential ${credentialDisplayName(credential)} has no usable environment variable binding.`,
				);
			}
			const existingForBinding = nextAssignments.find(
				(item) =>
					item.credentialId === credential.id && item.envVar === request.envVar,
			);
			const existingForCredential = nextAssignments.find(
				(item) => item.credentialId === credential.id,
			);
			nextAssignments.splice(
				0,
				nextAssignments.length,
				...nextAssignments.filter(
					(item) =>
						!(
							item.credentialId === credential.id &&
							item.envVar === request.envVar
						),
				),
			);
			nextAssignments.push({
				credentialId: credential.id,
				sessionCredentialId:
					existingForBinding?.sessionCredentialId ??
					existingForCredential?.sessionCredentialId,
				envVar: request.envVar,
				sourceEnvVar,
				agentVisible: true,
				visibility:
					existingForBinding?.visibility ??
					existingForCredential?.visibility ??
					credential.visibility,
				uses: [
					...(existingForBinding?.uses ?? []),
					...buildAssignmentUses(
						{
							envVar: request.envVar,
							name: request.name,
							justification: request.justification,
							approvedUses: request.approvedUses ?? [],
						},
						expiresAt,
					),
				],
				credential,
			});
		}

		await api.setSessionCredentials(
			args.sessionId,
			nextAssignments.map((assignment) => ({
				credentialId: assignment.credentialId,
				sessionCredentialId: assignment.sessionCredentialId,
				envVar: assignment.envVar,
				sourceEnvVar: assignment.sourceEnvVar,
				agentVisible: assignment.agentVisible,
				visibility: assignment.visibility,
				uses: assignment.uses,
			})),
		);
	}

	async function executeCommand(command: AgentCommand) {
		const threadId = args.getSelectedThreadId();
		const text = `/${command.name}`;
		isSubmitting = true;
		try {
			await sendMessages(threadId, text);
		} finally {
			isSubmitting = false;
		}
	}

	async function confirmCredentialDialog() {
		if (!credentialDialogCommand) {
			return;
		}

		credentialDialogError = null;
		const command = credentialDialogCommand;
		const requests = credentialDialogRequests;
		const projectCredentials = credentialDialogProjectCredentials;
		const sessionAssignments = credentialDialogSessionAssignments;
		const selectedOptions = selectedOptionByEnvVar;

		try {
			await prepareCommandCredentials(
				requests,
				sessionAssignments,
				projectCredentials,
				credentialDialogCredentialTypes,
				selectedOptions,
				createCredentialNamesByEnvVar,
				createCredentialSecretsByEnvVar,
				validityPresetByEnvVar,
				validityValueByEnvVar,
				validityUnitByEnvVar,
			);
			resetCredentialDialog();
			await executeCommand(command);
		} catch (error) {
			credentialDialogError =
				error instanceof Error
					? error.message
					: "Failed to prepare credentials.";
		}
	}

	async function run(command: AgentCommand) {
		if (!args.hasSession() || isSubmitting || credentialDialogOpen) {
			return;
		}

		const requests = command.discobot?.credentialRequest ?? [];
		if (requests.length === 0) {
			await executeCommand(command);
			return;
		}

		await prepareCommandCredentialDialog(command);
	}

	return {
		get list() {
			return resource.data;
		},
		get uiVisible() {
			return uiVisible;
		},
		get status() {
			return resource.status;
		},
		get error() {
			return resource.error;
		},
		get isRefreshing() {
			return resource.isRefreshing;
		},
		get isStale() {
			return resource.isStale;
		},
		get fetchedAt() {
			return resource.fetchedAt;
		},
		get isSubmitting() {
			return isSubmitting;
		},
		get credentialDialog() {
			return {
				open: credentialDialogOpen,
				command: credentialDialogCommand,
				requests: credentialDialogRequests,
				projectCredentials: credentialDialogProjectCredentials,
				credentialTypes: credentialDialogCredentialTypes,
				sessionAssignments: credentialDialogSessionAssignments,
				selectedOptionByEnvVar,
				createCredentialNamesByEnvVar,
				createCredentialSecretsByEnvVar,
				validityPresetByEnvVar,
				validityValueByEnvVar,
				validityUnitByEnvVar,
				error: credentialDialogError,
				selectOption: (envVar: string, value: string) => {
					selectedOptionByEnvVar = {
						...selectedOptionByEnvVar,
						[envVar]: value,
					};
				},
				setCreateCredentialName: (envVar: string, value: string) => {
					createCredentialNamesByEnvVar = {
						...createCredentialNamesByEnvVar,
						[envVar]: value,
					};
				},
				setCreateCredentialSecret: (envVar: string, value: string) => {
					createCredentialSecretsByEnvVar = {
						...createCredentialSecretsByEnvVar,
						[envVar]: value,
					};
				},
				setValidityPreset: (
					envVar: string,
					value: CredentialValidityPreset,
				) => {
					validityPresetByEnvVar = {
						...validityPresetByEnvVar,
						[envVar]: value,
					};
					if (value === "custom") {
						validityValueByEnvVar = {
							...validityValueByEnvVar,
							[envVar]: validityValueByEnvVar[envVar] ?? "1",
						};
						validityUnitByEnvVar = {
							...validityUnitByEnvVar,
							[envVar]: validityUnitByEnvVar[envVar] ?? "hours",
						};
					}
				},
				setValidityValue: (envVar: string, value: string) => {
					validityValueByEnvVar = {
						...validityValueByEnvVar,
						[envVar]: value,
					};
				},
				setValidityUnit: (
					envVar: string,
					value: "hours" | "days" | "weeks" | "never",
				) => {
					validityUnitByEnvVar = {
						...validityUnitByEnvVar,
						[envVar]: value,
					};
				},
				launchOAuthWizard: launchOAuthCredentialWizard,
				refreshAvailableCredentials: refreshCredentialDialogContext,
				close: resetCredentialDialog,
				confirm: confirmCredentialDialog,
			};
		},
		refresh: async () => {
			await resource.refresh();
		},
		invalidate: resource.invalidate,
		run,
	};
}
