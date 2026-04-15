import type {
	CredentialInfo,
	GrantedCredential,
	RequestedCredential,
	SessionCredentialAssignment,
	SessionCredentialUse,
} from "$lib/api-types";

export type CredentialMatch = {
	credential: CredentialInfo;
	assigned: boolean;
};

export function credentialDisplayName(credential: CredentialInfo): string {
	const trimmedName = credential.name.trim();
	if (trimmedName.length > 0) {
		return trimmedName;
	}
	if (credential.provider.startsWith("custom:")) {
		return credential.envKeys?.join(", ") || "Custom env vars";
	}
	return credential.provider;
}

export function findCredentialMatches(
	envVar: string,
	credentials: CredentialInfo[],
	assignments: SessionCredentialAssignment[],
): CredentialMatch[] {
	const assignedIds = new Set(
		assignments.map((assignment) => assignment.credentialId),
	);
	return credentials
		.filter((credential) => credential.envKeys?.includes(envVar))
		.map((credential) => ({
			credential,
			assigned: assignedIds.has(credential.id),
		}))
		.sort((left, right) => {
			if (left.assigned !== right.assigned) {
				return left.assigned ? -1 : 1;
			}
			return credentialDisplayName(left.credential).localeCompare(
				credentialDisplayName(right.credential),
			);
		});
}

export function listAnyCredentials(
	credentials: CredentialInfo[],
	assignments: SessionCredentialAssignment[],
): CredentialMatch[] {
	const assignedIds = new Set(
		assignments.map((assignment) => assignment.credentialId),
	);
	return [...credentials]
		.map((credential) => ({
			credential,
			assigned: assignedIds.has(credential.id),
		}))
		.sort((left, right) => {
			if (left.assigned !== right.assigned) {
				return left.assigned ? -1 : 1;
			}
			return credentialDisplayName(left.credential).localeCompare(
				credentialDisplayName(right.credential),
			);
		});
}

export function preferredSourceEnvVar(
	requestEnvVar: string,
	credential: CredentialInfo,
): string | null {
	const envKeys = (credential.envKeys ?? [])
		.map((key) => key.trim())
		.filter(Boolean);
	if (envKeys.includes(requestEnvVar)) {
		return requestEnvVar;
	}
	return envKeys[0] ?? null;
}

export function credentialBindingDescription(
	requestEnvVar: string,
	credential: CredentialInfo,
): string {
	const sourceEnvVar = preferredSourceEnvVar(requestEnvVar, credential);
	if (!sourceEnvVar || sourceEnvVar === requestEnvVar) {
		return `Available as ${requestEnvVar}`;
	}
	return `Stored as ${sourceEnvVar}; will be exposed as ${requestEnvVar} in this session`;
}

export function defaultCredentialName(request: RequestedCredential): string {
	const name = request.name.trim();
	const envVar = request.envVar.trim();
	if (name.length > 0) {
		return `${name} (${envVar})`;
	}
	return envVar;
}

export function formatApprovedUses(request: RequestedCredential): string[] {
	return (request.approvedUses ?? [])
		.map((use) => use.description.trim())
		.filter((description) => description.length > 0);
}

export function buildAssignmentUses(
	request: RequestedCredential,
): SessionCredentialUse[] {
	return formatApprovedUses(request).map((description) => ({
		description,
		id: "",
	}));
}

function isActiveSessionCredentialUse(use: SessionCredentialUse): boolean {
	if (!use.expiresAt) {
		return true;
	}
	return new Date(use.expiresAt).getTime() > Date.now();
}

export function buildGrantedCredentialPayload(
	requests: RequestedCredential[],
	selectedCredentialIdsByEnvVar: Record<string, string>,
	assignments: SessionCredentialAssignment[],
): { grantedCredentials: GrantedCredential[] } {
	const assignmentByBindingKey = new Map(
		assignments.map((assignment) => [
			`${assignment.credentialId}\x00${assignment.envVar ?? ""}`,
			assignment,
		]),
	);
	return {
		grantedCredentials: requests.flatMap((request) => {
			const credentialId =
				selectedCredentialIdsByEnvVar[request.envVar]?.trim();
			if (!credentialId) {
				return [];
			}
			const assignment = assignmentByBindingKey.get(
				`${credentialId}\x00${request.envVar}`,
			);
			if (!assignment?.sessionCredentialId) {
				return [];
			}
			return [
				{
					credentialId: assignment.sessionCredentialId,
					envVar: request.envVar,
					name: request.name,
					approvedUses: (assignment.uses ?? [])
						.filter(isActiveSessionCredentialUse)
						.map((use) => ({
							id: use.id,
							description: use.description,
						})),
				},
			];
		}),
	};
}
