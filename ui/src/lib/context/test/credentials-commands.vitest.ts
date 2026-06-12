import { describe, expect, test, vi } from "vitest";

import type { CreateCredentialRequest, CredentialInfo } from "$lib/api-types";
import { createCommands } from "$lib/context/commands";
import type { Context } from "$lib/context/context.types";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";

const apiMock = vi.hoisted(() => ({
	getCredentials: vi.fn(),
	getCredentialTypes: vi.fn(),
	createCredential: vi.fn(),
	deleteCredential: vi.fn(),
	codexAuthorize: vi.fn(),
	codexDeviceCode: vi.fn(),
	codexCallbackStatus: vi.fn(),
	codexPoll: vi.fn(),
	codexExchange: vi.fn(),
	githubAuthorize: vi.fn(),
	githubDeviceCode: vi.fn(),
	githubCallbackStatus: vi.fn(),
	githubPoll: vi.fn(),
	githubExchange: vi.fn(),
	anthropicAuthorize: vi.fn(),
	anthropicExchange: vi.fn(),
}));

vi.mock("$lib/api-client", () => ({
	api: apiMock,
}));

describe("ng credential commands", () => {
	test("refreshes credentials into the ng cache", async () => {
		const context = createPlainContext();
		apiMock.getCredentials.mockResolvedValueOnce({
			credentials: [credential({ id: "credential-1" })],
		});

		await context.commands.credentials.refreshCredentials({ wait: true });

		expect(context.data.credentials.allIds).toEqual(["credential-1"]);
		expect(context.data.credentials.byId["credential-1"]?.name).toBe(
			"Credential 1",
		);
		expect(context.data.credentials.status.state).toBe("ready");
	});

	test("creates and deletes credentials in the ng cache", async () => {
		const context = createPlainContext();
		const created = credential({ id: "credential-2", name: "Created" });
		apiMock.createCredential.mockResolvedValueOnce(created);
		apiMock.deleteCredential.mockResolvedValueOnce(undefined);

		await context.commands.credentials.createCredential({
			provider: "openai",
			name: "Created",
			authType: "api_key",
			apiKey: "secret",
		} satisfies CreateCredentialRequest);

		expect(context.data.credentials.byId["credential-2"]).toBe(created);
		expect(context.data.credentials.allIds).toEqual(["credential-2"]);

		await context.commands.credentials.deleteCredential("credential-2");

		expect(context.data.credentials.byId["credential-2"]).toBeUndefined();
		expect(context.data.credentials.allIds).toEqual([]);
	});

	test("toggles inactive state through createCredential update", async () => {
		const context = createPlainContext();
		const existing = credential({ id: "credential-3", inactive: false });
		const updated = credential({ id: "credential-3", inactive: true });
		context.data.credentials.byId[existing.id] = existing;
		context.data.credentials.allIds = [existing.id];
		apiMock.createCredential.mockResolvedValueOnce(updated);

		await context.commands.credentials.toggleCredentialInactive(existing);

		expect(apiMock.createCredential).toHaveBeenCalledWith(
			expect.objectContaining({
				credentialId: "credential-3",
				inactive: true,
			}),
		);
		expect(context.data.credentials.byId["credential-3"]?.inactive).toBe(true);
	});
});

function createPlainContext(): Context {
	const context: Context = {
		data: createInitialDataState({ projectId: "local" }),
		view: createInitialViewState(),
		commands: undefined as unknown as Context["commands"],
	};
	context.commands = createCommands(context);
	return context;
}

function credential(overrides: Partial<CredentialInfo> = {}): CredentialInfo {
	const base: CredentialInfo = {
		id: overrides.id ?? "credential-1",
		provider: overrides.provider ?? "openai",
		name: overrides.name ?? "Credential 1",
		description: overrides.description ?? "",
		authType: overrides.authType ?? "api_key",
		isConfigured: overrides.isConfigured ?? true,
		agentVisible: overrides.agentVisible ?? false,
		visibility: overrides.visibility ?? {
			tools: false,
			console: false,
			services: false,
			hooks: false,
		},
		inactive: overrides.inactive ?? false,
	};
	return { ...base, ...overrides };
}
