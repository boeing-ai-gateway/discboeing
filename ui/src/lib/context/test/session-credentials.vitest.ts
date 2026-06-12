import { describe, expect, test, vi } from "vitest";

import { ApiError } from "$lib/api-client";
import type {
	CredentialInfo,
	Session,
	SessionCredentialAssignment,
} from "$lib/api-types";
import type { Context } from "$lib/context/context.types";
import {
	loadSessionCredentialsIntoCache,
	replaceSessionCredentialAssignments,
	toSetSessionCredentialInputs,
} from "$lib/context/domains/session-credentials";
import { createSessionRecord } from "$lib/context/domains/sessions";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";

const apiMock = vi.hoisted(() => ({
	getSessionCredentials: vi.fn(),
	setSessionCredentials: vi.fn(),
}));

vi.mock("$lib/api-client", async (importOriginal) => {
	const original = await importOriginal<typeof import("$lib/api-client")>();
	return {
		...original,
		api: apiMock,
	};
});

describe("ng session credential domain", () => {
	test("loads session credentials into the session record", async () => {
		const context = createPlainContext("session-1");
		const assignment = sessionAssignment({ credentialId: "credential-1" });
		apiMock.getSessionCredentials.mockResolvedValueOnce({
			credentials: [assignment],
		});

		await loadSessionCredentialsIntoCache(context, "session-1");

		expect(
			context.data.sessions.byId["session-1"].credentials.assignments,
		).toEqual([assignment]);
		expect(
			context.data.sessions.byId["session-1"].credentials.status.state,
		).toBe("ready");
	});

	test("treats missing session credentials as ready empty state", async () => {
		const context = createPlainContext("session-1");
		apiMock.getSessionCredentials.mockRejectedValueOnce(
			new ApiError("missing", 404),
		);

		await loadSessionCredentialsIntoCache(context, "session-1");

		expect(
			context.data.sessions.byId["session-1"].credentials.assignments,
		).toEqual([]);
		expect(
			context.data.sessions.byId["session-1"].credentials.status.state,
		).toBe("ready");
	});

	test("filters and saves only session-specific credential changes", async () => {
		const context = createPlainContext("session-1");
		const unchanged = sessionAssignment({ credentialId: "unchanged" });
		const changed = sessionAssignment({
			credentialId: "changed",
			visibility: {
				tools: true,
				console: false,
				services: false,
				hooks: false,
			},
		});
		apiMock.setSessionCredentials.mockResolvedValueOnce({
			credentials: [changed],
		});

		await replaceSessionCredentialAssignments(context, "session-1", [
			unchanged,
			changed,
		]);

		expect(apiMock.setSessionCredentials).toHaveBeenCalledWith("session-1", [
			expect.objectContaining({
				credentialId: "changed",
				agentVisible: true,
			}),
		]);
		expect(
			context.data.sessions.byId["session-1"].credentials.assignments,
		).toEqual([changed]);
	});

	test("keeps uses even when visibility matches global credential", () => {
		const assignment = sessionAssignment({
			credentialId: "credential-1",
			uses: [{ id: "use-1", description: "Tool call" }],
		});

		expect(toSetSessionCredentialInputs([assignment])).toEqual([
			expect.objectContaining({
				credentialId: "credential-1",
				uses: [{ id: "use-1", description: "Tool call" }],
			}),
		]);
	});
});

function createPlainContext(sessionId: string): Context {
	const context: Context = {
		data: createInitialDataState({ projectId: "local" }),
		view: createInitialViewState(),
		commands: undefined as unknown as Context["commands"],
	};
	context.data.sessions.byId[sessionId] = createSessionRecord(sessionId, {
		id: sessionId,
	} as Session);
	context.data.sessions.allIds = [sessionId];
	return context;
}

function sessionAssignment(
	overrides: Partial<SessionCredentialAssignment> = {},
): SessionCredentialAssignment {
	const credential = credentialInfo({
		id: overrides.credentialId ?? "credential-1",
	});
	return {
		credentialId: credential.id,
		agentVisible: overrides.agentVisible ?? false,
		visibility: overrides.visibility ?? {
			tools: false,
			console: false,
			services: false,
			hooks: false,
		},
		credential,
		...overrides,
	};
}

function credentialInfo(
	overrides: Partial<CredentialInfo> = {},
): CredentialInfo {
	return {
		id: overrides.id ?? "credential-1",
		name: overrides.name ?? "Credential",
		provider: overrides.provider ?? "openai",
		description: overrides.description ?? "",
		authType: overrides.authType ?? "api_key",
		isConfigured: overrides.isConfigured ?? true,
		inactive: overrides.inactive ?? false,
		agentVisible: overrides.agentVisible ?? false,
		visibility: overrides.visibility ?? {
			tools: false,
			console: false,
			services: false,
			hooks: false,
		},
	};
}
