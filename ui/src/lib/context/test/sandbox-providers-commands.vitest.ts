import { describe, expect, test, vi } from "vitest";

import type {
	SandboxProviderInstance,
	SandboxProviderType,
} from "$lib/api-types";
import { createCommands } from "$lib/context/commands";
import type { Context } from "$lib/context/context.types";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";

const apiMock = vi.hoisted(() => ({
	getSandboxProviderTypes: vi.fn(),
	getSandboxProviders: vi.fn(),
	createSandboxProvider: vi.fn(),
	updateSandboxProvider: vi.fn(),
	deleteSandboxProvider: vi.fn(),
	updateDefaultSandboxProvider: vi.fn(),
}));

vi.mock("$lib/api-client", () => ({
	api: apiMock,
}));

describe("ng sandbox provider commands", () => {
	test("refreshes sandbox provider types and instances into cache", async () => {
		const context = createPlainContext();
		mockProviderSnapshot({
			types: [providerType({ id: "docker" })],
			providers: [provider({ id: "provider-1", type: "docker" })],
			defaultProviderId: "provider-1",
			projectDefaultProviderId: "provider-1",
		});

		await context.commands.sandboxProviders.refreshSandboxProviders({
			wait: true,
		});

		expect(context.data.sandboxProviders.types.allIds).toEqual(["docker"]);
		expect(context.data.sandboxProviders.instances.allIds).toEqual([
			"provider-1",
		]);
		expect(context.data.sandboxProviders.defaultProviderId).toBe("provider-1");
		expect(context.data.sandboxProviders.projectDefaultProviderId).toBe(
			"provider-1",
		);
		expect(context.data.sandboxProviders.types.status.state).toBe("ready");
		expect(context.data.sandboxProviders.instances.status.state).toBe("ready");
	});

	test("creates a provider then refreshes cache", async () => {
		const context = createPlainContext();
		apiMock.createSandboxProvider.mockResolvedValueOnce(
			provider({ id: "provider-2" }),
		);
		mockProviderSnapshot({
			types: [providerType({ id: "docker" })],
			providers: [provider({ id: "provider-2", name: "Created" })],
			defaultProviderId: "provider-2",
		});

		await context.commands.sandboxProviders.createSandboxProvider(
			{ type: "docker", name: "Created" },
			{ wait: true },
		);

		expect(apiMock.createSandboxProvider).toHaveBeenCalledWith({
			type: "docker",
			name: "Created",
		});
		expect(
			context.data.sandboxProviders.instances.byId["provider-2"]?.name,
		).toBe("Created");
	});

	test("updates the default provider then refreshes cache", async () => {
		const context = createPlainContext();
		apiMock.updateDefaultSandboxProvider.mockResolvedValueOnce({
			default: "provider-3",
			projectDefault: "provider-3",
		});
		mockProviderSnapshot({
			types: [providerType({ id: "docker" })],
			providers: [provider({ id: "provider-3" })],
			defaultProviderId: "provider-3",
			projectDefaultProviderId: "provider-3",
		});

		await context.commands.sandboxProviders.updateDefaultSandboxProvider(
			"provider-3",
			{
				wait: true,
			},
		);

		expect(apiMock.updateDefaultSandboxProvider).toHaveBeenCalledWith(
			"provider-3",
		);
		expect(context.data.sandboxProviders.defaultProviderId).toBe("provider-3");
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

function mockProviderSnapshot(input: {
	types: SandboxProviderType[];
	providers: SandboxProviderInstance[];
	defaultProviderId: string;
	projectDefaultProviderId?: string;
	systemDefaultProviderId?: string;
}): void {
	apiMock.getSandboxProviderTypes.mockResolvedValueOnce({
		providerTypes: input.types,
	});
	apiMock.getSandboxProviders.mockResolvedValueOnce({
		providers: input.providers,
		default: input.defaultProviderId,
		projectDefault: input.projectDefaultProviderId,
		systemDefault: input.systemDefaultProviderId,
	});
}

function providerType(
	overrides: Partial<SandboxProviderType> = {},
): SandboxProviderType {
	return {
		id: overrides.id ?? "docker",
		name: overrides.name ?? "Docker",
		description: overrides.description ?? "Docker provider",
		available: overrides.available ?? true,
		builtIn: overrides.builtIn ?? true,
		capabilities: overrides.capabilities ?? {
			resources: false,
			inspection: false,
			clearCache: false,
		},
		configFields: overrides.configFields ?? [],
	};
}

function provider(
	overrides: Partial<SandboxProviderInstance> = {},
): SandboxProviderInstance {
	return {
		id: overrides.id ?? "provider-1",
		projectId: overrides.projectId ?? "local",
		type: overrides.type ?? "docker",
		name: overrides.name ?? "Docker",
		builtIn: overrides.builtIn ?? false,
		disabled: overrides.disabled ?? false,
		config: overrides.config ?? {},
		available: overrides.available ?? true,
		default: overrides.default ?? false,
		capabilities: overrides.capabilities ?? {
			resources: false,
			inspection: false,
			clearCache: false,
		},
	};
}
