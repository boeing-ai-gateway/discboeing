import assert from "node:assert/strict";
import test from "node:test";

import type {
	CredentialInfo,
	RequestedCredential,
	SessionCredentialAssignment,
} from "../../api-types";
import {
	buildAssignmentUses,
	buildGrantedCredentialPayload,
	credentialBindingDescription,
	credentialDisplayName,
	defaultCredentialName,
	findCredentialMatches,
	formatApprovedUses,
	listAnyCredentials,
	preferredSourceEnvVar,
} from "../ai/tool-renderers/requestusercredential-helpers";

const existingCredential: CredentialInfo = {
	id: "cred-1",
	name: "GitHub Token",
	provider: "custom:github-token",
	authType: "api_key",
	isConfigured: true,
	inactive: false,
	agentVisible: false,
	envKeys: ["GITHUB_TOKEN"],
};

const secondCredential: CredentialInfo = {
	id: "cred-2",
	name: "Backup GitHub Token",
	provider: "custom:backup-github-token",
	authType: "api_key",
	isConfigured: true,
	inactive: false,
	agentVisible: false,
	envKeys: ["GITHUB_TOKEN"],
};

const assignment: SessionCredentialAssignment = {
	credentialId: "cred-2",
	sessionCredentialId: "cred_s_abc123",
	agentVisible: true,
	uses: [
		{ id: "use_s_1", description: "create pull requests" },
		{ id: "use_s_2", description: "clone private repositories" },
	],
	credential: secondCredential,
};

const request: RequestedCredential = {
	envVar: "GITHUB_TOKEN",
	name: "GitHub access token",
	justification: "Clone a private repository",
	approvedUses: [
		{ description: "create pull requests" },
		{ description: "clone private repositories" },
	],
};

test("findCredentialMatches prefers credentials already assigned to the session", () => {
	const matches = findCredentialMatches(
		"GITHUB_TOKEN",
		[existingCredential, secondCredential],
		[assignment],
	);

	assert.equal(matches.length, 2);
	assert.equal(matches[0]?.credential.id, "cred-2");
	assert.equal(matches[0]?.assigned, true);
	assert.equal(matches[1]?.credential.id, "cred-1");
	assert.equal(matches[1]?.assigned, false);
});

test("listAnyCredentials includes non-matching credentials sorted after assigned ones", () => {
	const unrelatedCredential: CredentialInfo = {
		id: "cred-3",
		name: "OpenAI Key",
		provider: "custom:openai-key",
		authType: "api_key",
		isConfigured: true,
		inactive: false,
		agentVisible: false,
		envKeys: ["OPENAI_API_KEY"],
	};

	const matches = listAnyCredentials(
		[existingCredential, unrelatedCredential, secondCredential],
		[assignment],
	);

	assert.equal(matches[0]?.credential.id, "cred-2");
	assert.equal(matches[1]?.credential.id, "cred-1");
	assert.equal(matches[2]?.credential.id, "cred-3");
});

test("preferredSourceEnvVar uses exact match when available and first key otherwise", () => {
	assert.equal(
		preferredSourceEnvVar("GITHUB_TOKEN", existingCredential),
		"GITHUB_TOKEN",
	);
	assert.equal(
		preferredSourceEnvVar("GITHUB_TOKEN", {
			...existingCredential,
			envKeys: ["GH_TOKEN", "GITHUB_TOKEN_ALT"],
		}),
		"GH_TOKEN",
	);
});

test("credentialBindingDescription explains session remapping for non-matching creds", () => {
	assert.equal(
		credentialBindingDescription("GITHUB_TOKEN", existingCredential),
		"Available as GITHUB_TOKEN",
	);
	assert.equal(
		credentialBindingDescription("GITHUB_TOKEN", {
			...existingCredential,
			envKeys: ["GH_TOKEN"],
		}),
		"Stored as GH_TOKEN; will be exposed as GITHUB_TOKEN in this session",
	);
});

test("defaultCredentialName combines request name and env var", () => {
	assert.equal(
		defaultCredentialName(request),
		"GitHub access token (GITHUB_TOKEN)",
	);
});

test("credentialDisplayName falls back to env keys for custom credentials", () => {
	assert.equal(credentialDisplayName(existingCredential), "GitHub Token");
	assert.equal(
		credentialDisplayName({
			...existingCredential,
			name: "",
			envKeys: ["GITHUB_TOKEN", "GH_TOKEN"],
		}),
		"GITHUB_TOKEN, GH_TOKEN",
	);
});

test("formatApprovedUses returns trimmed use descriptions", () => {
	assert.deepEqual(formatApprovedUses(request), [
		"create pull requests",
		"clone private repositories",
	]);
});

test("buildAssignmentUses returns request uses without ids", () => {
	assert.deepEqual(buildAssignmentUses(request), [
		{ id: "", description: "create pull requests" },
		{ id: "", description: "clone private repositories" },
	]);
});

test("buildGrantedCredentialPayload returns session-scoped ids and uses", () => {
	assert.deepEqual(
		buildGrantedCredentialPayload([request], { GITHUB_TOKEN: "cred-2" }, [
			assignment,
		]),
		{
			grantedCredentials: [
				{
					credentialId: "cred_s_abc123",
					envVar: "GITHUB_TOKEN",
					name: "GitHub access token",
					approvedUses: [
						{ id: "use_s_1", description: "create pull requests" },
						{ id: "use_s_2", description: "clone private repositories" },
					],
				},
			],
		},
	);
});
