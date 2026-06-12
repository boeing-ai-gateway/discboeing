export type CredentialAuthType = "api_key" | "id" | "oauth";
export type ProviderCategory = "llm" | "vcs" | "sandbox-provider";
export type CredentialTypeGroup =
  | "model-providers"
  | "git-version-control"
  | "tools";

export interface CredentialVisibility {
  tools: boolean;
  console: boolean;
  services: boolean;
  hooks: boolean;
}

export interface CredentialEnvVar {
  key: string;
  value: string;
  originalKey?: string;
}

export interface CredentialInfo {
  id: string;
  name: string;
  provider: string;
  description?: string;
  authType: CredentialAuthType;
  isConfigured: boolean;
  inactive: boolean;
  agentVisible: boolean;
  visibility: CredentialVisibility;
  envKeys?: string[];
  envVars?: CredentialEnvVar[];
  scopes?: string[];
  expiresAt?: string;
  updatedAt?: string;
}

export interface CreateCredentialRequest {
  provider?: string;
  credentialId?: string;
  name: string;
  description?: string;
  authType: CredentialAuthType;
  apiKey?: string;
  envVars?: CredentialEnvVar[];
  agentVisible?: boolean;
  visibility?: CredentialVisibility;
  inactive?: boolean;
}

export interface CredentialType {
  id: string;
  provider: string;
  backendProvider: string;
  name: string;
  description?: string;
  secretLabel?: string;
  secretDescription?: string;
  env?: string[];
  category: ProviderCategory;
  group: CredentialTypeGroup;
  groupName: string;
  authType: CredentialAuthType;
}

export interface SandboxProviderCapabilities {
  resources: boolean;
  inspection: boolean;
  clearCache: boolean;
}

export interface SandboxProviderConfigField {
  key: string;
  label: string;
  type: "text" | "number" | "textarea" | "credential";
  description?: string;
  placeholder?: string;
  required?: boolean;
  advanced?: boolean;
  credentialProvider?: string;
  credentialAuthType?: CredentialAuthType;
}

export interface SandboxProviderType {
  id: string;
  name: string;
  icon?: string;
  description?: string;
  available: boolean;
  builtIn: boolean;
  capabilities: SandboxProviderCapabilities;
  configFields?: SandboxProviderConfigField[];
}

export interface SandboxProviderInstance {
  id: string;
  projectId: string;
  type: string;
  name: string;
  icon?: string;
  config?: Record<string, unknown>;
  builtIn: boolean;
  disabled: boolean;
  available: boolean;
  default: boolean;
  capabilities: SandboxProviderCapabilities;
  createdAt?: string;
  updatedAt?: string;
}

export interface SettingsMockState {
  credentials: CredentialInfo[];
  credentialTypes: CredentialType[];
  sandboxProviderTypes: SandboxProviderType[];
  sandboxProviders: SandboxProviderInstance[];
  defaultSandboxProviderId: string;
  systemDefaultSandboxProviderId: string;
}

export const visibleEverywhere: CredentialVisibility = {
  tools: true,
  console: true,
  services: true,
  hooks: true,
};

export const hiddenFromAgents: CredentialVisibility = {
  tools: false,
  console: false,
  services: false,
  hooks: false,
};

export function createSettingsMockState(): SettingsMockState {
  const now = "2026-06-10T12:00:00.000Z";
  const dockerCapabilities = {
    resources: true,
    inspection: true,
    clearCache: true,
  };
  const remoteCapabilities = {
    resources: false,
    inspection: false,
    clearCache: false,
  };

  return {
    credentials: [
      {
        id: "cred-openai-e2e",
        name: "E2E OpenAI API key",
        provider: "openai",
        description: "Seeded credential for mocked settings tests.",
        authType: "api_key",
        isConfigured: true,
        inactive: false,
        agentVisible: true,
        visibility: visibleEverywhere,
        envKeys: ["OPENAI_API_KEY"],
        updatedAt: now,
      },
      {
        id: "cred-github-e2e",
        name: "E2E GitHub token",
        provider: "github",
        description: "Seeded VCS token for mocked settings tests.",
        authType: "api_key",
        isConfigured: true,
        inactive: false,
        agentVisible: false,
        visibility: hiddenFromAgents,
        envKeys: ["GITHUB_TOKEN"],
        updatedAt: now,
      },
    ],
    credentialTypes: [
      {
        id: "openai:api_key",
        provider: "openai",
        backendProvider: "openai",
        name: "OpenAI",
        description: "OpenAI model provider API key.",
        secretLabel: "API key",
        secretDescription: "A test-only OpenAI API key.",
        env: ["OPENAI_API_KEY"],
        category: "llm",
        group: "model-providers",
        groupName: "Model providers",
        authType: "api_key",
      },
      {
        id: "anthropic:api_key",
        provider: "anthropic",
        backendProvider: "anthropic",
        name: "Anthropic",
        description: "Anthropic model provider API key.",
        secretLabel: "API key",
        env: ["ANTHROPIC_API_KEY"],
        category: "llm",
        group: "model-providers",
        groupName: "Model providers",
        authType: "api_key",
      },
      {
        id: "github:api_key",
        provider: "github",
        backendProvider: "github",
        name: "GitHub",
        description: "GitHub personal access token.",
        secretLabel: "Token",
        env: ["GITHUB_TOKEN"],
        category: "vcs",
        group: "git-version-control",
        groupName: "Git version control",
        authType: "api_key",
      },
    ],
    sandboxProviderTypes: [
      {
        id: "docker",
        name: "Docker",
        icon: "simple:docker",
        description: "Run sandboxes with the local Docker daemon.",
        available: true,
        builtIn: true,
        capabilities: dockerCapabilities,
        configFields: [
          {
            key: "dockerHost",
            label: "Docker host",
            type: "text",
            placeholder: "unix:///var/run/docker.sock",
            advanced: true,
          },
        ],
      },
      {
        id: "remote-linux",
        name: "Remote Linux",
        icon: "simple:linux",
        description: "Connect to a remote Linux sandbox host.",
        available: true,
        builtIn: false,
        capabilities: remoteCapabilities,
        configFields: [
          {
            key: "endpoint",
            label: "Endpoint",
            type: "text",
            placeholder: "https://sandbox.example.test",
            required: true,
          },
          {
            key: "apiKeyCredentialId",
            label: "API key credential",
            type: "credential",
            description:
              "Credential used to authenticate with the sandbox host.",
            credentialProvider: "openai",
            credentialAuthType: "api_key",
          },
        ],
      },
    ],
    sandboxProviders: [
      {
        id: "sandbox-docker-e2e",
        projectId: "local",
        type: "docker",
        name: "E2E Local Docker",
        icon: "simple:docker",
        config: { dockerHost: "unix:///var/run/docker.sock" },
        builtIn: true,
        disabled: false,
        available: true,
        default: true,
        capabilities: dockerCapabilities,
        createdAt: now,
        updatedAt: now,
      },
      {
        id: "sandbox-remote-e2e",
        projectId: "local",
        type: "remote-linux",
        name: "E2E Remote Linux",
        icon: "simple:linux",
        config: {
          endpoint: "https://sandbox.example.test",
          apiKeyCredentialId: "cred-openai-e2e",
        },
        builtIn: false,
        disabled: true,
        available: true,
        default: false,
        capabilities: remoteCapabilities,
        createdAt: now,
        updatedAt: now,
      },
    ],
    defaultSandboxProviderId: "sandbox-docker-e2e",
    systemDefaultSandboxProviderId: "sandbox-docker-e2e",
  };
}
