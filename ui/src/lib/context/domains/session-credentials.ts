import { ApiError, api } from "$lib/api-client";
import type {
	SessionCredentialAssignment,
	SetSessionCredentialsRequest,
} from "$lib/api-types";
import {
	createErrorStatus,
	createLoadingStatus,
	createReadyStatus,
	type ResourceStatus,
} from "$lib/context/cache";
import type { CommandOptions, Context } from "$lib/context/context.types";

export type SessionCredentialsState = {
	assignments: SessionCredentialAssignment[];
	status: ResourceStatus;
};

export function createSessionCredentialsState(): SessionCredentialsState {
	return {
		assignments: [],
		status: { state: "idle" },
	};
}

export function toSetSessionCredentialInputs(
	assignments: SessionCredentialAssignment[],
): SetSessionCredentialsRequest["credentials"] {
	return assignments
		.filter(
			(assignment) =>
				Boolean(assignment.sessionCredentialId) ||
				Boolean(assignment.envVar) ||
				Boolean(assignment.sourceEnvVar) ||
				(assignment.uses?.length ?? 0) > 0 ||
				assignment.visibility.tools !==
					assignment.credential.visibility.tools ||
				assignment.visibility.console !==
					assignment.credential.visibility.console ||
				assignment.visibility.services !==
					assignment.credential.visibility.services ||
				assignment.visibility.hooks !== assignment.credential.visibility.hooks,
		)
		.map((assignment) => ({
			credentialId: assignment.credentialId,
			sessionCredentialId: assignment.sessionCredentialId,
			envVar: assignment.envVar,
			sourceEnvVar: assignment.sourceEnvVar,
			agentVisible: assignment.visibility.tools,
			visibility: assignment.visibility,
			uses: assignment.uses,
		}));
}

export async function loadSessionCredentialsIntoCache(
	context: Context,
	sessionId: string,
	options: CommandOptions = {},
): Promise<void> {
	void options;
	const record = context.data.sessions.byId[sessionId];
	if (!record) {
		return;
	}
	record.credentials.status = createLoadingStatus();

	try {
		const response = await api.getSessionCredentials(sessionId);
		record.credentials.assignments = response.credentials;
		record.credentials.status = createReadyStatus();
	} catch (error) {
		if (error instanceof ApiError && error.status === 404) {
			record.credentials.assignments = [];
			record.credentials.status = createReadyStatus();
			return;
		}
		record.credentials.assignments = [];
		record.credentials.status = createErrorStatus(error);
		throw error;
	}
}

export async function replaceSessionCredentialAssignments(
	context: Context,
	sessionId: string,
	assignments: SessionCredentialAssignment[],
	options: CommandOptions = {},
): Promise<void> {
	void options;
	const response = await api.setSessionCredentials(
		sessionId,
		toSetSessionCredentialInputs(assignments),
	);
	const record = context.data.sessions.byId[sessionId];
	if (record) {
		record.credentials.assignments = response.credentials;
		record.credentials.status = createReadyStatus();
	}
}
