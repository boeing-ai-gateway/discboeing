import { api } from "$lib/api-client";
import type {
	AgentCommand,
	AgentCommandCredentialRequest,
	CredentialInfo,
	CredentialType,
	SessionCredentialAssignment,
} from "$lib/api-types";
import {
	buildAssignmentUses,
	buildCredentialUseExpiryFromPreset,
	CUSTOM_CREDENTIAL_OPTION,
	credentialDisplayName,
	defaultCredentialName,
	findPreferredCredentialId,
	parseOAuthCredentialOption,
	preferredSourceEnvVar,
} from "$lib/components/ai/tool-renderers/requestusercredential-helpers";
import { buildUserMessageParts } from "$lib/conversation-helpers";
import type {
	CredentialValidityPreset,
	CredentialValidityUnit,
	Context,
	SessionCommandCredentialDialogView,
} from "$lib/context/context.types";
import {
	createCredential,
	loadCredentialsIntoCache,
	loadCredentialTypesIntoCache,
} from "$lib/context/domains/credentials";
import { ensureSessionRecord } from "$lib/context/domains/sessions";
import { ensureSessionView } from "$lib/context/domains/view";

function getCredentials(context: Context): CredentialInfo[] {
	return context.data.credentials.allIds.map(
		(id) => context.data.credentials.byId[id],
	);
}

function getCredentialTypes(context: Context): CredentialType[] {
	return context.data.credentials.types;
}

function createEmptyCredentialDialog(): SessionCommandCredentialDialogView {
	return {
		open: false,
		command: null,
		requests: [],
		projectCredentials: [],
		credentialTypes: [],
		sessionAssignments: [],
		selectedOptionByEnvVar: {},
		createCredentialNamesByEnvVar: {},
		createCredentialSecretsByEnvVar: {},
		validityPresetByEnvVar: {},
		validityValueByEnvVar: {},
		validityUnitByEnvVar: {},
		error: null,
	};
}

function getCredentialDialog(
	context: Context,
	sessionId: string,
): SessionCommandCredentialDialogView {
	return ensureSessionView(context, sessionId).commands.credentialDialog;
}

function setCredentialDialog(
	context: Context,
	sessionId: string,
	dialog: SessionCommandCredentialDialogView,
): void {
	ensureSessionView(context, sessionId).commands.credentialDialog = dialog;
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
			.find((credential) => credential.envKeys?.includes(args.requestEnvVar)) ??
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

async function refreshGlobalCredentialCaches(context: Context): Promise<void> {
	await Promise.all([
		loadCredentialTypesIntoCache(context),
		loadCredentialsIntoCache(context, { wait: true }),
	]);
}

async function refreshCredentialDialogContext(
	context: Context,
	sessionId: string,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	if (!dialog.open) {
		return;
	}

	await refreshGlobalCredentialCaches(context);
	const assignmentsResponse = await api.getSessionCredentials(sessionId);
	const projectCredentials = getCredentials(context);
	const credentialTypes = getCredentialTypes(context);
	const selectedOptionByEnvVar = Object.fromEntries(
		Object.entries(dialog.selectedOptionByEnvVar).map(([envVar, value]) => {
			const selectedOAuthType = parseOAuthCredentialOption(
				value,
				credentialTypes,
			);
			if (!selectedOAuthType) {
				return [envVar, value];
			}
			const matchingCredential = findMatchingOAuthCredential({
				requestEnvVar: envVar,
				credentialType: selectedOAuthType,
				projectCredentials,
			});
			return [envVar, matchingCredential?.id ?? value];
		}),
	);

	setCredentialDialog(context, sessionId, {
		...dialog,
		projectCredentials,
		credentialTypes,
		sessionAssignments: assignmentsResponse.credentials,
		selectedOptionByEnvVar,
	});
}

async function launchOAuthCredentialWizard(
	context: Context,
	sessionId: string,
	envVar: string,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	const option = dialog.selectedOptionByEnvVar[envVar]?.trim() ?? "";
	const selectedOAuthType = parseOAuthCredentialOption(
		option,
		dialog.credentialTypes,
	);
	if (!selectedOAuthType) {
		return;
	}
	if (selectedOAuthType.backendProvider === "github-git") {
		context.view.app.dialogs.credentials.open = true;
		context.view.app.dialogs.credentials.flowIntent = "github-git";
		setCredentialDialog(context, sessionId, { ...dialog, error: null });
		return;
	}
	if (selectedOAuthType.backendProvider === "codex") {
		context.view.app.dialogs.credentials.open = true;
		context.view.app.dialogs.credentials.flowIntent = "codex";
		setCredentialDialog(context, sessionId, { ...dialog, error: null });
		return;
	}
	setCredentialDialog(context, sessionId, {
		...dialog,
		error: `${selectedOAuthType.name} OAuth isn't supported in this flow yet.`,
	});
}

async function prepareCommandCredentialDialog(
	context: Context,
	sessionId: string,
	command: AgentCommand,
): Promise<void> {
	if (
		getCredentials(context).length === 0 ||
		getCredentialTypes(context).length === 0
	) {
		await refreshGlobalCredentialCaches(context);
	}
	const assignmentsResponse = await api.getSessionCredentials(sessionId);
	const sessionAssignments = assignmentsResponse.credentials;
	const projectCredentials = getCredentials(context);
	const credentialTypes = getCredentialTypes(context);
	const requests = command.discboeing?.credentialRequest ?? [];
	const initialSelections = Object.fromEntries(
		requests.map((request) => [
			request.envVar,
			findPreferredCredentialId(
				request.envVar,
				projectCredentials,
				sessionAssignments,
			),
		]),
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
	) as Record<string, CredentialValidityUnit>;

	setCredentialDialog(context, sessionId, {
		open: true,
		command,
		requests,
		projectCredentials,
		credentialTypes,
		sessionAssignments,
		selectedOptionByEnvVar: initialSelections,
		createCredentialNamesByEnvVar: initialNames,
		createCredentialSecretsByEnvVar: {},
		validityPresetByEnvVar: initialValidityPresets,
		validityValueByEnvVar: initialValidityValues,
		validityUnitByEnvVar: initialValidityUnits,
		error: null,
	});
}

async function prepareCommandCredentials(
	context: Context,
	sessionId: string,
	requests: AgentCommandCredentialRequest[],
	sessionAssignments: SessionCredentialAssignment[],
	projectCredentials: CredentialInfo[],
	credentialTypes: CredentialType[],
	selectedOptions: Record<string, string>,
	customCredentialNames: Record<string, string>,
	customCredentialSecrets: Record<string, string>,
	validityPresets: Record<string, CredentialValidityPreset>,
	validityValues: Record<string, string>,
	validityUnits: Record<string, CredentialValidityUnit>,
): Promise<void> {
	let availableCredentials = projectCredentials;
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
		let credential = availableCredentials.find((item) => item.id === option);
		if (option === CUSTOM_CREDENTIAL_OPTION) {
			const secret = customCredentialSecrets[request.envVar]?.trim() ?? "";
			if (!secret) {
				throw new Error(`Enter a credential value for ${request.envVar}.`);
			}
			credential = await createCredential(context, {
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
			availableCredentials = [...availableCredentials, credential];
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

	const response = await api.setSessionCredentials(
		sessionId,
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
	const record = context.data.sessions.byId[sessionId];
	if (record) {
		record.credentials.assignments = response.credentials;
	}
}

function resolveSelectedThreadId(
	context: Context,
	sessionId: string,
): string | null {
	if (context.view.selection.sessionId === sessionId) {
		return context.view.selection.threadId;
	}
	return context.view.selection.requestedThreadIdBySessionId[sessionId] ?? null;
}

async function executeCommand(
	context: Context,
	sessionId: string,
	command: AgentCommand,
): Promise<void> {
	const threadId = resolveSelectedThreadId(context, sessionId);
	if (!threadId) {
		return;
	}
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	record.commands.isSubmitting = true;
	try {
		await context.commands.threadComposer.submitThread(sessionId, threadId, {
			parts: buildUserMessageParts(`/${command.name}`),
		});
	} finally {
		record.commands.isSubmitting = false;
	}
}

export async function runAgentCommand(
	context: Context,
	sessionId: string,
	command: AgentCommand,
): Promise<void> {
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	const dialog = getCredentialDialog(context, sessionId);
	if (!record.value || record.commands.isSubmitting || dialog.open) {
		return;
	}

	const requests = command.discboeing?.credentialRequest ?? [];
	if (requests.length === 0) {
		await executeCommand(context, sessionId, command);
		return;
	}

	await prepareCommandCredentialDialog(context, sessionId, command);
}

export async function closeCommandCredentialDialog(
	context: Context,
	sessionId: string,
): Promise<void> {
	setCredentialDialog(context, sessionId, createEmptyCredentialDialog());
}

export async function confirmCommandCredentialDialog(
	context: Context,
	sessionId: string,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	if (!dialog.command) {
		return;
	}

	const command = dialog.command;
	setCredentialDialog(context, sessionId, { ...dialog, error: null });
	try {
		await prepareCommandCredentials(
			context,
			sessionId,
			dialog.requests,
			dialog.sessionAssignments,
			dialog.projectCredentials,
			dialog.credentialTypes,
			dialog.selectedOptionByEnvVar,
			dialog.createCredentialNamesByEnvVar,
			dialog.createCredentialSecretsByEnvVar,
			dialog.validityPresetByEnvVar,
			dialog.validityValueByEnvVar,
			dialog.validityUnitByEnvVar,
		);
		setCredentialDialog(context, sessionId, createEmptyCredentialDialog());
		await executeCommand(context, sessionId, command);
	} catch (error) {
		setCredentialDialog(context, sessionId, {
			...getCredentialDialog(context, sessionId),
			error:
				error instanceof Error
					? error.message
					: "Failed to prepare credentials.",
		});
	}
}

export async function selectCommandCredentialOption(
	context: Context,
	sessionId: string,
	envVar: string,
	value: string,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	setCredentialDialog(context, sessionId, {
		...dialog,
		selectedOptionByEnvVar: {
			...dialog.selectedOptionByEnvVar,
			[envVar]: value,
		},
	});
}

export async function setCommandCredentialCreateName(
	context: Context,
	sessionId: string,
	envVar: string,
	value: string,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	setCredentialDialog(context, sessionId, {
		...dialog,
		createCredentialNamesByEnvVar: {
			...dialog.createCredentialNamesByEnvVar,
			[envVar]: value,
		},
	});
}

export async function setCommandCredentialCreateSecret(
	context: Context,
	sessionId: string,
	envVar: string,
	value: string,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	setCredentialDialog(context, sessionId, {
		...dialog,
		createCredentialSecretsByEnvVar: {
			...dialog.createCredentialSecretsByEnvVar,
			[envVar]: value,
		},
	});
}

export async function setCommandCredentialValidityPreset(
	context: Context,
	sessionId: string,
	envVar: string,
	value: CredentialValidityPreset,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	setCredentialDialog(context, sessionId, {
		...dialog,
		validityPresetByEnvVar: {
			...dialog.validityPresetByEnvVar,
			[envVar]: value,
		},
		validityValueByEnvVar:
			value === "custom"
				? {
						...dialog.validityValueByEnvVar,
						[envVar]: dialog.validityValueByEnvVar[envVar] ?? "1",
					}
				: dialog.validityValueByEnvVar,
		validityUnitByEnvVar:
			value === "custom"
				? {
						...dialog.validityUnitByEnvVar,
						[envVar]: dialog.validityUnitByEnvVar[envVar] ?? "hours",
					}
				: dialog.validityUnitByEnvVar,
	});
}

export async function setCommandCredentialValidityValue(
	context: Context,
	sessionId: string,
	envVar: string,
	value: string,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	setCredentialDialog(context, sessionId, {
		...dialog,
		validityValueByEnvVar: {
			...dialog.validityValueByEnvVar,
			[envVar]: value,
		},
	});
}

export async function setCommandCredentialValidityUnit(
	context: Context,
	sessionId: string,
	envVar: string,
	value: CredentialValidityUnit,
): Promise<void> {
	const dialog = getCredentialDialog(context, sessionId);
	setCredentialDialog(context, sessionId, {
		...dialog,
		validityUnitByEnvVar: {
			...dialog.validityUnitByEnvVar,
			[envVar]: value,
		},
	});
}

export async function launchCommandCredentialOAuthWizard(
	context: Context,
	sessionId: string,
	envVar: string,
): Promise<void> {
	await launchOAuthCredentialWizard(context, sessionId, envVar);
}

export async function refreshCommandCredentialDialogCredentials(
	context: Context,
	sessionId: string,
): Promise<void> {
	await refreshCredentialDialogContext(context, sessionId);
}
