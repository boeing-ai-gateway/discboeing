import type {
	CredentialInfo,
	CredentialType,
	GrantedCredential,
	RequestedCredential,
	SessionCredentialAssignment,
	SessionCredentialUse,
} from "$lib/api-types";

export type CredentialMatch = {
	credential: CredentialInfo;
};

export type OAuthCredentialOption = {
	credentialType: CredentialType;
	label: string;
	value: string;
};

export const CUSTOM_CREDENTIAL_OPTION = "__custom__";
export const OAUTH_CREDENTIAL_OPTION_PREFIX = "__oauth__:";
export const NEVER_EXPIRES_AT = "9999-12-31T23:59:59.000Z";

export type CredentialValidityUnit = "hours" | "days" | "weeks" | "never";
export type CredentialValidityPreset =
	| "15_minutes"
	| "1_hour"
	| "1_day"
	| "1_week"
	| "custom";

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
	void assignments;
	return credentials
		.filter((credential) => credential.envKeys?.includes(envVar))
		.map((credential) => ({ credential }))
		.sort((left, right) =>
			credentialDisplayName(left.credential).localeCompare(
				credentialDisplayName(right.credential),
			),
		);
}

export function findPreferredCredentialId(
	envVar: string,
	credentials: CredentialInfo[],
	assignments: SessionCredentialAssignment[],
): string {
	return (
		findCredentialMatches(envVar, credentials, assignments)[0]?.credential.id ??
		""
	);
}

export function listAnyCredentials(
	credentials: CredentialInfo[],
	assignments: SessionCredentialAssignment[],
): CredentialMatch[] {
	void assignments;
	return [...credentials]
		.map((credential) => ({ credential }))
		.sort((left, right) =>
			credentialDisplayName(left.credential).localeCompare(
				credentialDisplayName(right.credential),
			),
		);
}

export function oauthCredentialOptionValue(
	credentialType: CredentialType,
): string {
	return `${OAUTH_CREDENTIAL_OPTION_PREFIX}${credentialType.id}`;
}

export function parseOAuthCredentialOption(
	value: string,
	credentialTypes: CredentialType[],
): CredentialType | null {
	if (!value.startsWith(OAUTH_CREDENTIAL_OPTION_PREFIX)) {
		return null;
	}
	const credentialTypeId = value.slice(OAUTH_CREDENTIAL_OPTION_PREFIX.length);
	return (
		credentialTypes.find(
			(type) => type.id === credentialTypeId && type.authType === "oauth",
		) ?? null
	);
}

export function listOAuthCredentialOptions(
	envVar: string,
	credentialTypes: CredentialType[],
	credentials: CredentialInfo[],
): OAuthCredentialOption[] {
	return credentialTypes
		.filter(
			(type) =>
				type.authType === "oauth" &&
				type.env?.includes(envVar) &&
				!credentials.some(
					(credential) =>
						credential.provider === type.backendProvider &&
						credential.authType === type.authType,
				),
		)
		.sort((left, right) => left.name.localeCompare(right.name))
		.map((credentialType) => ({
			credentialType,
			label: `New ${credentialType.name} OAuth`,
			value: oauthCredentialOptionValue(credentialType),
		}));
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
	expiresAt?: string,
): SessionCredentialUse[] {
	return formatApprovedUses(request).map((description) => ({
		description,
		id: "",
		...(expiresAt ? { expiresAt } : {}),
	}));
}

export function buildCredentialUseExpiry(
	value: string,
	unit: CredentialValidityUnit,
	now = Date.now(),
): string | undefined {
	if (unit === "never") {
		return NEVER_EXPIRES_AT;
	}
	const amount = Number.parseInt(value.trim(), 10);
	if (!Number.isFinite(amount) || amount <= 0) {
		throw new Error("Enter a valid credential duration.");
	}
	const multiplier =
		unit === "hours"
			? 60 * 60 * 1000
			: unit === "days"
				? 24 * 60 * 60 * 1000
				: 7 * 24 * 60 * 60 * 1000;
	return new Date(now + amount * multiplier).toISOString();
}

export function buildCredentialUseExpiryFromPreset(
	preset: CredentialValidityPreset,
	customValue: string,
	customUnit: CredentialValidityUnit,
	now = Date.now(),
): string | undefined {
	switch (preset) {
		case "15_minutes":
			return new Date(now + 15 * 60 * 1000).toISOString();
		case "1_hour":
			return new Date(now + 60 * 60 * 1000).toISOString();
		case "1_day":
			return new Date(now + 24 * 60 * 60 * 1000).toISOString();
		case "1_week":
			return new Date(now + 7 * 24 * 60 * 60 * 1000).toISOString();
		case "custom":
			return buildCredentialUseExpiry(customValue, customUnit, now);
	}
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
