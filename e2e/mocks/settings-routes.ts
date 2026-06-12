import type { Page, Route } from "@playwright/test";
import {
  createSettingsMockState,
  hiddenFromAgents,
  type CreateCredentialRequest,
  type CredentialInfo,
  type SettingsMockState,
  type SandboxProviderInstance,
  type SandboxProviderType,
  visibleEverywhere,
} from "./settings-fixtures";

export interface InstallSettingsMockRoutesOptions {
  state?: SettingsMockState;
}

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

function noContent(route: Route) {
  return route.fulfill({ status: 204 });
}

async function readJson<T>(route: Route): Promise<T> {
  const body = route.request().postData();
  if (!body) {
    return {} as T;
  }
  return JSON.parse(body) as T;
}

function projectIdFromPath(pathname: string) {
  const match = pathname.match(/\/api\/projects\/([^/]+)/);
  return match?.[1] ?? "local";
}

function pathAfterProject(pathname: string) {
  return pathname.replace(/^.*\/api\/projects\/[^/]+/, "");
}

function timestamp() {
  return new Date().toISOString();
}

function syncProviderDefaults(state: SettingsMockState) {
  for (const provider of state.sandboxProviders) {
    provider.default = provider.id === state.defaultSandboxProviderId;
  }
}

function providerCapabilities(
  state: SettingsMockState,
  typeId: string,
): SandboxProviderType["capabilities"] {
  return (
    state.sandboxProviderTypes.find(
      (providerType) => providerType.id === typeId,
    )?.capabilities ?? {
      resources: false,
      inspection: false,
      clearCache: false,
    }
  );
}

function credentialResponse(credential: CredentialInfo): CredentialInfo {
  return {
    ...credential,
    envVars: credential.envVars?.map((envVar) => ({ ...envVar, value: "" })),
  };
}

function buildCredential(
  state: SettingsMockState,
  request: CreateCredentialRequest,
): CredentialInfo {
  const provider =
    request.provider ?? request.credentialId?.split(":")[0] ?? "custom";
  const id =
    request.credentialId &&
    !state.credentials.some((item) => item.id === request.credentialId)
      ? request.credentialId
      : `cred-${provider}-${Date.now().toString(36)}`;
  const envVars = request.envVars?.map((envVar) => ({ ...envVar, value: "" }));
  const type = state.credentialTypes.find(
    (credentialType) =>
      credentialType.provider === provider &&
      credentialType.authType === request.authType,
  );
  return {
    id,
    name: request.name,
    provider,
    description: request.description,
    authType: request.authType,
    isConfigured: true,
    inactive: request.inactive ?? false,
    agentVisible: request.agentVisible ?? false,
    visibility: request.visibility ?? hiddenFromAgents,
    envKeys: envVars?.map((envVar) => envVar.key) ?? type?.env,
    envVars,
    updatedAt: timestamp(),
  };
}

function buildSandboxProvider(
  state: SettingsMockState,
  projectId: string,
  request: Partial<SandboxProviderInstance>,
): SandboxProviderInstance {
  const type = request.type ?? "docker";
  const providerType = state.sandboxProviderTypes.find(
    (item) => item.id === type,
  );
  const name = request.name?.trim() || providerType?.name || type;
  return {
    id: request.id ?? `sandbox-${type}-${Date.now().toString(36)}`,
    projectId,
    type,
    name,
    icon: request.icon,
    config: request.config,
    builtIn: request.builtIn ?? false,
    disabled: request.disabled ?? false,
    available: request.available ?? providerType?.available ?? true,
    default: false,
    capabilities: providerCapabilities(state, type),
    createdAt: timestamp(),
    updatedAt: timestamp(),
  };
}

async function handleSettingsRoute(route: Route, state: SettingsMockState) {
  const request = route.request();
  const url = new URL(request.url());
  const method = request.method();
  const pathname = url.pathname;
  const projectPath = pathAfterProject(pathname);
  const projectId = projectIdFromPath(pathname);

  if (projectPath === "/credentials/types" && method === "GET") {
    return json(route, { credentialTypes: state.credentialTypes });
  }

  if (projectPath === "/credentials" && method === "GET") {
    return json(route, {
      credentials: state.credentials.map((credential) =>
        credentialResponse(credential),
      ),
    });
  }

  if (projectPath === "/credentials" && method === "POST") {
    const payload = await readJson<CreateCredentialRequest>(route);
    if (!payload.name || !payload.authType) {
      return json(
        route,
        { error: "Credential name and authType are required" },
        400,
      );
    }
    const credential = buildCredential(state, payload);
    state.credentials = [
      ...state.credentials.filter((item) => item.id !== credential.id),
      credential,
    ];
    return json(route, credentialResponse(credential), 201);
  }

  const credentialMatch = projectPath.match(/^\/credentials\/([^/]+)$/);
  if (credentialMatch && method === "GET") {
    const id = decodeURIComponent(credentialMatch[1]);
    const credential = state.credentials.find((item) => item.id === id);
    return credential
      ? json(route, credentialResponse(credential))
      : json(route, { error: "Credential not found" }, 404);
  }

  if (credentialMatch && method === "DELETE") {
    const id = decodeURIComponent(credentialMatch[1]);
    state.credentials = state.credentials.filter((item) => item.id !== id);
    return noContent(route);
  }

  if (projectPath === "/sandbox-provider-types" && method === "GET") {
    return json(route, { providerTypes: state.sandboxProviderTypes });
  }

  if (projectPath === "/sandbox-providers" && method === "GET") {
    syncProviderDefaults(state);
    return json(route, {
      providers: state.sandboxProviders,
      default: state.defaultSandboxProviderId,
      projectDefault: state.defaultSandboxProviderId,
      systemDefault: state.systemDefaultSandboxProviderId,
    });
  }

  if (projectPath === "/sandbox-providers" && method === "POST") {
    const payload = await readJson<Partial<SandboxProviderInstance>>(route);
    if (!payload.type) {
      return json(route, { error: "Sandbox provider type is required" }, 400);
    }
    const provider = buildSandboxProvider(state, projectId, payload);
    state.sandboxProviders = [...state.sandboxProviders, provider];
    syncProviderDefaults(state);
    return json(route, provider, 201);
  }

  if (projectPath === "/sandbox-providers/default" && method === "PATCH") {
    const payload = await readJson<{ providerId?: string }>(route);
    const providerId = payload.providerId ?? "";
    if (
      !state.sandboxProviders.some((provider) => provider.id === providerId)
    ) {
      return json(route, { error: "Sandbox provider not found" }, 404);
    }
    state.defaultSandboxProviderId = providerId;
    syncProviderDefaults(state);
    return json(route, {
      default: state.defaultSandboxProviderId,
      projectDefault: state.defaultSandboxProviderId,
      systemDefault: state.systemDefaultSandboxProviderId,
    });
  }

  const sandboxProviderMatch = projectPath.match(
    /^\/sandbox-providers\/([^/]+)$/,
  );
  if (sandboxProviderMatch && method === "PATCH") {
    const providerId = decodeURIComponent(sandboxProviderMatch[1]);
    const payload = await readJson<Partial<SandboxProviderInstance>>(route);
    const existing = state.sandboxProviders.find(
      (provider) => provider.id === providerId,
    );
    if (!existing) {
      return json(route, { error: "Sandbox provider not found" }, 404);
    }
    Object.assign(existing, {
      ...payload,
      id: existing.id,
      projectId: existing.projectId,
      builtIn: existing.builtIn,
      capabilities: payload.type
        ? providerCapabilities(state, payload.type)
        : existing.capabilities,
      updatedAt: timestamp(),
    });
    syncProviderDefaults(state);
    return json(route, existing);
  }

  if (sandboxProviderMatch && method === "DELETE") {
    const providerId = decodeURIComponent(sandboxProviderMatch[1]);
    state.sandboxProviders = state.sandboxProviders.filter(
      (provider) => provider.id !== providerId,
    );
    if (state.defaultSandboxProviderId === providerId) {
      state.defaultSandboxProviderId = state.sandboxProviders[0]?.id ?? "";
    }
    syncProviderDefaults(state);
    return noContent(route);
  }

  if (projectPath === "/workspaces/providers" && method === "GET") {
    return json(route, {
      default: "docker",
      providers: Object.fromEntries(
        state.sandboxProviderTypes.map((providerType) => [
          providerType.id,
          {
            available: providerType.available,
            state: providerType.available ? "ready" : "not_available",
            message: providerType.description,
            supportsResources: providerType.capabilities.resources,
            supportsInspection: providerType.capabilities.inspection,
            supportsClearCache: providerType.capabilities.clearCache,
          },
        ]),
      ),
    });
  }

  return route.fallback();
}

export async function installSettingsMockRoutes(
  page: Page,
  options: InstallSettingsMockRoutesOptions = {},
): Promise<SettingsMockState> {
  const state = options.state ?? createSettingsMockState();
  await page.route("**/api/projects/*/**", (route) =>
    handleSettingsRoute(route, state),
  );
  return state;
}

export async function installSettingsMockRoutesForCore(
  page: Page,
  state: SettingsMockState = createSettingsMockState(),
): Promise<SettingsMockState> {
  return installSettingsMockRoutes(page, { state });
}

export { createSettingsMockState, visibleEverywhere };
export type { SettingsMockState };
