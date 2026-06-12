import assert from "node:assert/strict";
import { test } from "vitest";

import type {
	CredentialInfo,
	CredentialType,
	RequestedCredential,
	SessionCredentialAssignment,
} from "../../api-types";
import {
	buildCredentialUseExpiry,
	buildCredentialUseExpiryFromPreset,
	buildAssignmentUses,
	buildGrantedCredentialPayload,
	credentialBindingDescription,
	credentialDisplayName,
	defaultCredentialName,
	findCredentialMatches,
	findPreferredCredentialId,
	formatApprovedUses,
	listAnyCredentials,
	listOAuthCredentialOptions,
	NEVER_EXPIRES_AT,
	oauthCredentialOptionValue,
	parseOAuthCredentialOption,
	preferredSourceEnvVar,
} from "../ai/tool-renderers/requestusercredential-helpers";

const hiddenVisibility = {
	tools: false,
	console: false,
	services: false,
	hooks: false,
} as const;

const existingCredential: CredentialInfo = {
	id: "cred-1",
	name: "GitHub Token",
	provider: "custom:github-token",
	authType: "api_key",
	isConfigured: true,
	inactive: false,
	agentVisible: false,
	visibility: hiddenVisibility,
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
	visibility: hiddenVisibility,
	envKeys: ["GITHUB_TOKEN"],
};

const assignment: SessionCredentialAssignment = {
	credentialId: "cred-2",
	sessionCredentialId: "cred_s_abc123",
	envVar: "GITHUB_TOKEN",
	agentVisible: true,
	visibility: {
		tools: true,
		console: false,
		services: false,
		hooks: false,
	},
	uses: [
		{ id: "use_s_1", description: "create pull requests" },
		{ id: "use_s_2", description: "clone private repositories" },
	],
	credential: secondCredential,
};

const githubOAuthType: CredentialType = {
	id: "github:oauth",
	provider: "github",
	backendProvider: "github-git",
	name: "GitHub",
	description: "GitHub OAuth",
	group: "git-version-control",
	groupName: "Git version control",
	category: "vcs",
	authType: "oauth",
	env: ["GITHUB_TOKEN"],
	oauth: {
		provider: "github-git",
		kind: "device_code",
	},
};

const anthropicOAuthType: CredentialType = {
	id: "anthropic:oauth",
	provider: "anthropic",
	backendProvider: "anthropic",
	name: "Anthropic",
	description: "Anthropic OAuth",
	group: "model-providers",
	groupName: "Model providers",
	category: "llm",
	authType: "oauth",
	env: ["ANTHROPIC_API_KEY"],
	oauth: {
		provider: "anthropic",
		kind: "authorization_code",
	},
};

const existingGitHubOAuthCredential: CredentialInfo = {
	id: "cred-oauth-1",
	name: "GitHub OAuth",
	provider: "github-git",
	authType: "oauth",
	isConfigured: true,
	inactive: false,
	agentVisible: false,
	visibility: hiddenVisibility,
	envKeys: ["GITHUB_TOKEN"],
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

test("findCredentialMatches sorts matching credentials by display name", () => {
	const matches = findCredentialMatches(
		"GITHUB_TOKEN",
		[existingCredential, secondCredential],
		[assignment],
	);

	assert.equal(matches.length, 2);
	assert.equal(matches[0]?.credential.id, "cred-2");
	assert.equal(matches[1]?.credential.id, "cred-1");
});

test("listAnyCredentials includes non-matching credentials sorted by name", () => {
	const unrelatedCredential: CredentialInfo = {
		id: "cred-3",
		name: "OpenAI Key",
		provider: "custom:openai-key",
		authType: "api_key",
		isConfigured: true,
		inactive: false,
		agentVisible: false,
		visibility: hiddenVisibility,
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

test("listOAuthCredentialOptions offers matching OAuth types that are not yet configured", () => {
	assert.deepEqual(
		listOAuthCredentialOptions(
			"GITHUB_TOKEN",
			[anthropicOAuthType, githubOAuthType],
			[existingCredential],
		),
		[
			{
				credentialType: githubOAuthType,
				label: "New GitHub OAuth",
				value: oauthCredentialOptionValue(githubOAuthType),
			},
		],
	);
	assert.deepEqual(
		listOAuthCredentialOptions(
			"GITHUB_TOKEN",
			[githubOAuthType],
			[existingCredential, existingGitHubOAuthCredential],
		),
		[],
	);
});

test("parseOAuthCredentialOption resolves known OAuth options", () => {
	assert.equal(
		parseOAuthCredentialOption(oauthCredentialOptionValue(githubOAuthType), [
			githubOAuthType,
			anthropicOAuthType,
		])?.id,
		"github:oauth",
	);
	assert.equal(
		parseOAuthCredentialOption("__oauth__:missing", [githubOAuthType]),
		null,
	);
	assert.equal(parseOAuthCredentialOption("cred-1", [githubOAuthType]), null);
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

test("buildAssignmentUses applies the same expiration to each approved use", () => {
	assert.deepEqual(buildAssignmentUses(request, "2026-04-16T00:00:00.000Z"), [
		{
			id: "",
			description: "create pull requests",
			expiresAt: "2026-04-16T00:00:00.000Z",
		},
		{
			id: "",
			description: "clone private repositories",
			expiresAt: "2026-04-16T00:00:00.000Z",
		},
	]);
});

test("findPreferredCredentialId returns the first matching credential by name", () => {
	assert.equal(
		findPreferredCredentialId(
			"GITHUB_TOKEN",
			[existingCredential, secondCredential],
			[assignment],
		),
		"cred-2",
	);
	assert.equal(
		findPreferredCredentialId("MISSING", [existingCredential], [assignment]),
		"",
	);
});

test("buildCredentialUseExpiry converts duration fields into an expiration", () => {
	assert.equal(
		buildCredentialUseExpiry("2", "hours", Date.UTC(2026, 3, 15, 20, 0, 0)),
		"2026-04-15T22:00:00.000Z",
	);
	assert.equal(buildCredentialUseExpiry("1", "never"), NEVER_EXPIRES_AT);
	assert.throws(
		() => buildCredentialUseExpiry("0", "days"),
		/Enter a valid credential duration/,
	);
});

test("buildCredentialUseExpiryFromPreset maps preset durations", () => {
	assert.equal(
		buildCredentialUseExpiryFromPreset(
			"15_minutes",
			"1",
			"hours",
			Date.UTC(2026, 3, 15, 20, 0, 0),
		),
		"2026-04-15T20:15:00.000Z",
	);
	assert.equal(
		buildCredentialUseExpiryFromPreset(
			"1_hour",
			"1",
			"days",
			Date.UTC(2026, 3, 15, 20, 0, 0),
		),
		"2026-04-15T21:00:00.000Z",
	);
	assert.equal(
		buildCredentialUseExpiryFromPreset(
			"custom",
			"2",
			"days",
			Date.UTC(2026, 3, 15, 20, 0, 0),
		),
		"2026-04-17T20:00:00.000Z",
	);
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

test("buildGrantedCredentialPayload matches assignments by credential and env var", () => {
	const ghTokenRequest: RequestedCredential = {
		envVar: "GH_TOKEN",
		name: "GitHub CLI token",
		justification: "Authenticate gh",
		approvedUses: [{ description: "authenticate gh" }],
	};

	assert.deepEqual(
		buildGrantedCredentialPayload(
			[ghTokenRequest, request],
			{ GH_TOKEN: "cred-2", GITHUB_TOKEN: "cred-2" },
			[
				{
					...assignment,
					sessionCredentialId: "cred_s_shared",
					envVar: "GH_TOKEN",
					uses: [{ id: "use_s_gh", description: "authenticate gh" }],
				},
				assignment,
			],
		),
		{
			grantedCredentials: [
				{
					credentialId: "cred_s_shared",
					envVar: "GH_TOKEN",
					name: "GitHub CLI token",
					approvedUses: [{ id: "use_s_gh", description: "authenticate gh" }],
				},
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
