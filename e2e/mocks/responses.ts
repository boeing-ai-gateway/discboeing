import type {
	AuthUser,
	CredentialInfo,
	CredentialType,
	ModelsResponse,
	ProvidersResponse,
	SandboxProviderInstance,
	SandboxProviderType,
	ServerConfig,
	Session,
	SupportInfoResponse,
	SystemStatusResponse,
	Thread,
	Workspace,
} from "../../ui/src/lib/api-types";

export const MOCK_NOW = "2026-01-01T00:00:00.000Z";

export const mockUser: AuthUser = {
	id: "mock-user",
	email: "mock@discobot.test",
	name: "Mock User",
	provider: "mock",
};

export const mockSystemStatus: SystemStatusResponse = {
	healthy: true,
	ready: true,
	messages: [],
	startupTasks: [],
} as SystemStatusResponse;

export const mockServerConfig: ServerConfig = {
	ssh_port: 3333,
	http_port: 3001,
	public_base_url: "http://localhost:3001",
};

export function mockSupportInfo(): SupportInfoResponse {
	return {
		version: "mock-e2e",
		runtime: {
			os: "mock",
			arch: "mock",
			go_version: "go1.mock",
			num_cpu: 4,
			num_goroutine: 1,
		},
		config: {
			port: 3001,
			database_driver: "mock",
			auth_enabled: false,
			workspace_dir: "/tmp/discobot-e2e",
			sandbox_image: "mock",
			desktop_mode: false,
			ssh_enabled: true,
			ssh_port: 3333,
			dispatcher_enabled: false,
			available_providers: ["mock"],
		},
		server_log: "mock e2e server log",
		log_path: "/tmp/discobot-e2e.log",
		log_exists: true,
		system_info: mockSystemStatus,
	};
}

export const mockWorkspace: Workspace = {
	id: "workspace-1",
	path: "/workspace/discobot",
	displayName: "Discobot Mock Workspace",
	sourceType: "local",
	status: "ready",
	createdAt: MOCK_NOW,
	updatedAt: MOCK_NOW,
} as Workspace;

export const mockThread: Thread = {
	id: "thread-1",
	name: "Test",
	lastMessage: "Mocked e2e thread",
	activityStatus: { status: "idle" },
};

export const mockSession: Session = {
	id: "session-1",
	name: "Test",
	displayName: "Test",
	workspaceId: mockWorkspace.id,
	providerId: "mock-provider",
	model: "mock-model",
	status: "running",
	sandboxStatus: "ready",
	displayStatus: "ready",
	createdAt: MOCK_NOW,
	updatedAt: MOCK_NOW,
} as Session;

export const mockModels: ModelsResponse = {
	models: [
		{
			id: "mock-model",
			name: "Mock Model",
			provider: "mock",
			description: "Deterministic e2e model fixture",
		},
	],
};

export const mockCredentialTypes: CredentialType[] = [
	{
		id: "mock-api-key",
		provider: "mock",
		backendProvider: "mock",
		name: "Mock API Key",
		description: "Mock API key credential",
		secretLabel: "API key",
		category: "llm",
		group: "model-providers",
		groupName: "Model Providers",
		authType: "api_key",
		env: ["MOCK_API_KEY"],
	},
];

export const mockCredentials: CredentialInfo[] = [
	{
		id: "mock-credential",
		name: "Mock Credential",
		provider: "mock",
		authType: "api_key",
		isConfigured: true,
		inactive: false,
		agentVisible: true,
		visibility: { tools: true, console: false, services: false, hooks: false },
		envKeys: ["MOCK_API_KEY"],
		updatedAt: MOCK_NOW,
	},
];

export const mockSandboxProviderTypes: SandboxProviderType[] = [
	{
		id: "mock",
		name: "Mock Sandbox",
		description: "In-memory e2e sandbox provider",
		available: true,
		builtIn: true,
		capabilities: { resources: true, inspection: true, clearCache: true },
	},
];

export const mockSandboxProviders: SandboxProviderInstance[] = [
	{
		id: "mock-provider",
		projectId: "local",
		type: "mock",
		name: "Mock Sandbox",
		builtIn: true,
		disabled: false,
		available: true,
		default: true,
		capabilities: { resources: true, inspection: true, clearCache: true },
		createdAt: MOCK_NOW,
		updatedAt: MOCK_NOW,
	},
];

export const mockWorkspaceProviders: ProvidersResponse = {
	default: "mock-provider",
	providers: {
		"mock-provider": {
			available: true,
			state: "ready",
			message: "Mock provider ready",
			supportsResources: true,
			supportsInspection: true,
			supportsClearCache: true,
		},
	},
};
