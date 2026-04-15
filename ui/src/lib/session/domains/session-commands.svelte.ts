import { api } from "$lib/api-client";
import type {
	AgentCommand,
	AgentCommandCredentialRequest,
	CredentialInfo,
	SessionCredentialAssignment,
} from "$lib/api-types";
import type { AppContext } from "$lib/context/app-context.svelte";
import {
	buildAssignmentUses,
	credentialDisplayName,
	findCredentialMatches,
	preferredSourceEnvVar,
} from "$lib/components/ai/tool-renderers/requestusercredential-helpers";
import { createUserMessage } from "$lib/session/domains/session-domain.helpers";
import type { SessionCommandsDomain } from "$lib/session/session-context.types";

type CreateSessionCommandsDomainArgs = {
	app: AppContext;
	sessionId: string;
	hasSession: () => boolean;
	getSelectedThreadId: () => string;
};

export function createSessionCommandsDomain(
	args: CreateSessionCommandsDomainArgs,
): SessionCommandsDomain {
	let list = $state<AgentCommand[]>([]);
	let startingName = $state<string | null>(null);
	let credentialDialogOpen = $state(false);
	let credentialDialogCommand = $state<AgentCommand | null>(null);
	let credentialDialogRequests = $state<AgentCommandCredentialRequest[]>([]);
	let credentialDialogProjectCredentials = $state<CredentialInfo[]>([]);
	let credentialDialogSessionAssignments = $state<
		SessionCredentialAssignment[]
	>([]);
	let selectedCredentialIdsByEnvVar = $state<Record<string, string>>({});
	let credentialDialogError = $state<string | null>(null);

	const uiVisible = $derived.by(() =>
		[...list]
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

	async function refresh() {
		if (!args.hasSession()) {
			list = [];
			return;
		}
		const response = await api.getSessionCommands(args.sessionId);
		list = response.commands;
	}

	async function sendMessages(
		messages: ReturnType<typeof createUserMessage>[],
	) {
		await args.app.chat({
			sessionId: args.sessionId,
			threadId: args.getSelectedThreadId(),
			messages,
		});
	}

	function resetCredentialDialog() {
		credentialDialogOpen = false;
		credentialDialogCommand = null;
		credentialDialogRequests = [];
		credentialDialogProjectCredentials = [];
		credentialDialogSessionAssignments = [];
		selectedCredentialIdsByEnvVar = {};
		credentialDialogError = null;
	}

	async function prepareCommandCredentialDialog(command: AgentCommand) {
		if (args.app.credentials.list.length === 0) {
			await args.app.credentials.refresh();
		}
		const assignmentsResponse = await api.getSessionCredentials(args.sessionId);
		const sessionAssignments = assignmentsResponse.credentials;
		const requests = command.discobot?.credentialRequest ?? [];
		const initialSelections = Object.fromEntries(
			requests.map((request) => {
				const matches = findCredentialMatches(
					request.envVar,
					args.app.credentials.list,
					sessionAssignments,
				);
				return [request.envVar, matches[0]?.credential.id ?? ""];
			}),
		);

		credentialDialogCommand = command;
		credentialDialogRequests = requests;
		credentialDialogProjectCredentials = args.app.credentials.list;
		credentialDialogSessionAssignments = sessionAssignments;
		selectedCredentialIdsByEnvVar = initialSelections;
		credentialDialogError = null;
		credentialDialogOpen = true;
	}

	async function prepareCommandCredentials(
		requests: AgentCommandCredentialRequest[],
		sessionAssignments: SessionCredentialAssignment[],
		projectCredentials: CredentialInfo[],
		selectedCredentialIds: Record<string, string>,
	) {
		const nextAssignments = [...sessionAssignments];

		for (const request of requests) {
			const credentialId = selectedCredentialIds[request.envVar]?.trim();
			if (!credentialId) {
				throw new Error(`Select a credential for ${request.envVar}.`);
			}
			const credential = projectCredentials.find(
				(item) => item.id === credentialId,
			);
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
					...buildAssignmentUses({
						envVar: request.envVar,
						name: request.name,
						justification: request.justification,
						approvedUses: request.approvedUses ?? [],
					}),
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
		startingName = command.name;
		try {
			await sendMessages([createUserMessage(`/${command.name}`)]);
		} finally {
			startingName = null;
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
		const selectedCredentialIds = selectedCredentialIdsByEnvVar;

		try {
			await prepareCommandCredentials(
				requests,
				sessionAssignments,
				projectCredentials,
				selectedCredentialIds,
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
		if (!args.hasSession() || startingName || credentialDialogOpen) {
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
			return list;
		},
		get uiVisible() {
			return uiVisible;
		},
		get startingName() {
			return startingName;
		},
		get credentialDialog() {
			return {
				open: credentialDialogOpen,
				command: credentialDialogCommand,
				requests: credentialDialogRequests,
				projectCredentials: credentialDialogProjectCredentials,
				sessionAssignments: credentialDialogSessionAssignments,
				selectedCredentialIdsByEnvVar,
				error: credentialDialogError,
				selectCredential: (envVar: string, credentialId: string) => {
					selectedCredentialIdsByEnvVar = {
						...selectedCredentialIdsByEnvVar,
						[envVar]: credentialId,
					};
				},
				close: resetCredentialDialog,
				confirm: confirmCredentialDialog,
			};
		},
		refresh,
		run,
	};
}
