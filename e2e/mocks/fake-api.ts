import type {
  CredentialInfo,
  ChatMessage,
  ProjectStreamSocketMessage,
  ProjectStreamSocketRequest,
  Session,
  Thread,
  Workspace,
} from "../../ui/src/lib/api-types";
import {
  mockCredentialTypes,
  mockCredentials,
  mockModels,
  mockSandboxProviders,
  mockSandboxProviderTypes,
  mockServerConfig,
  mockSession,
  mockSupportInfo,
  mockSystemStatus,
  mockThread,
  mockUser,
  mockWorkspace,
  mockWorkspaceProviders,
  MOCK_NOW,
} from "./responses";

export type JsonResponse = { status?: number; body: unknown };

function clone<T>(value: T): T {
  return structuredClone(value);
}

function notFound(path: string): JsonResponse {
  return {
    status: 404,
    body: { error: `Unhandled mocked API request: ${path}` },
  };
}

export class FakeDiscobotApi {
  readonly projectId = "local";
  workspaces = new Map<string, Workspace>();
  sessions = new Map<string, Session>();
  threads = new Map<string, Thread[]>();
  chatMessages = new Map<string, ChatMessage[]>();
  credentials = new Map<string, CredentialInfo>();
  webSocketRequests: ProjectStreamSocketRequest[] = [];
  private seq = 0;

  constructor() {
    this.reset();
  }

  reset() {
    this.workspaces = new Map([[mockWorkspace.id, clone(mockWorkspace)]]);
    this.sessions = new Map([[mockSession.id, clone(mockSession)]]);
    this.threads = new Map([[mockSession.id, [clone(mockThread)]]]);
    this.chatMessages = new Map();
    this.credentials = new Map(
      mockCredentials.map((credential) => [credential.id, clone(credential)]),
    );
    this.webSocketRequests = [];
    this.seq = 0;
  }

  handle(method: string, url: URL): JsonResponse {
    const path = this.apiPath(url);
    if (method === "OPTIONS") return { status: 204, body: null };

    if (path === "/auth/me" && method === "GET")
      return { body: clone(mockUser) };
    if (path === "/auth/logout" && method === "POST")
      return { body: { success: true } };

    if (path === "/api/status" && method === "GET")
      return { body: clone(mockSystemStatus) };
    if (path === "/api/server-config" && method === "GET")
      return { body: clone(mockServerConfig) };
    if (path === "/api/support-info" && method === "GET")
      return { body: mockSupportInfo() };
    if (path === "/api/preferences" && method === "GET")
      return { body: { preferences: [] } };

    const projectPrefix = `/api/projects/${this.projectId}`;
    if (path === projectPrefix || path === `${projectPrefix}/`) {
      return { body: { id: this.projectId, name: "Local" } };
    }
    if (!path.startsWith(`${projectPrefix}/`)) return notFound(path);

    const subpath = path.slice(projectPrefix.length).replace(/\/$/, "") || "/";
    switch (subpath) {
      case "/models":
        return { body: clone(mockModels) };
      case "/workspaces/providers":
        return { body: clone(mockWorkspaceProviders) };
      case "/workspaces":
        if (method === "GET")
          return {
            body: { workspaces: [...this.workspaces.values()].map(clone) },
          };
        break;
      case "/sessions":
        if (method === "GET")
          return { body: { sessions: [...this.sessions.values()].map(clone) } };
        if (method === "POST")
          return { body: { id: `session-${this.sessions.size + 1}` } };
        break;
      case "/credentials":
        if (method === "GET")
          return {
            body: { credentials: [...this.credentials.values()].map(clone) },
          };
        break;
      case "/credentials/types":
        return { body: { credentialTypes: clone(mockCredentialTypes) } };
      case "/sandbox-provider-types":
        return { body: { providerTypes: clone(mockSandboxProviderTypes) } };
      case "/sandbox-providers":
        return {
          body: {
            providers: clone(mockSandboxProviders),
            default: "mock-provider",
            projectDefault: "mock-provider",
            systemDefault: "mock-provider",
          },
        };
      case "/auth-providers":
        return { body: { authProviders: [] } };
      case "/resources":
        return {
          body: {
            provider: "mock-provider",
            vm: {
              cpuCount: 2,
              memoryMB: 4096,
              dataDiskGB: 20,
              canIncreaseDisk: true,
              canDecreaseDisk: true,
              canChangeMemory: true,
              restartRequiredForDisk: false,
              restartRequiredForMemory: false,
            },
          },
        };
      case "/inspection":
        return {
          body: {
            provider: "mock-provider",
            available: true,
            containerName: "mock",
            scope: "host",
          },
        };
    }

    const sessionMatch = subpath.match(/^\/sessions\/([^/]+)(.*)$/);
    if (sessionMatch)
      return this.handleSession(
        method,
        sessionMatch[1],
        sessionMatch[2].replace(/\/$/, ""),
        url,
      );

    const workspaceMatch = subpath.match(/^\/workspaces\/([^/]+)$/);
    if (workspaceMatch && method === "GET") {
      const workspace = this.workspaces.get(workspaceMatch[1]);
      return workspace ? { body: clone(workspace) } : notFound(path);
    }

    return notFound(path);
  }

  projectHistoryMessages(): ProjectStreamSocketMessage[] {
    return [
      {
        type: "event",
        stream: "project-events",
        event: "connected",
        data: { projectId: this.projectId },
      },
      { type: "event", stream: "project-events", event: "history-start" },
      ...[...this.workspaces.values()].map((workspace) =>
        this.projectEvent("workspace_updated", workspace),
      ),
      ...[...this.sessions.values()].map((session) =>
        this.projectEvent("session_updated", session),
      ),
      { type: "event", stream: "project-events", event: "history-end" },
    ] as ProjectStreamSocketMessage[];
  }

  sessionHistoryMessages(sessionId: string): ProjectStreamSocketMessage[] {
    const session = this.sessions.get(sessionId);
    return [
      { type: "event", stream: "session", sessionId, event: "history-start" },
      ...(session
        ? [
            {
              type: "event",
              stream: "session",
              sessionId,
              event: "session_updated",
              data: clone(session),
            },
          ]
        : []),
      {
        type: "event",
        stream: "session",
        sessionId,
        event: "threads_updated",
        data: { threads: clone(this.threads.get(sessionId) ?? []) },
      },
      {
        type: "event",
        stream: "session",
        sessionId,
        event: "files_updated",
        data: { path: ".", entries: [] },
      },
      {
        type: "event",
        stream: "session",
        sessionId,
        event: "commands_updated",
        data: { commands: [] },
      },
      {
        type: "event",
        stream: "session",
        sessionId,
        event: "hooks_updated",
        data: {
          hooks: {},
          pendingHooks: [],
          lastEvaluatedAt: MOCK_NOW,
          executionPaused: false,
        },
      },
      {
        type: "event",
        stream: "session",
        sessionId,
        event: "services_updated",
        data: { services: [] },
      },
      {
        type: "event",
        stream: "session",
        sessionId,
        event: "diff_updated",
        data: {
          files: [],
          stats: { filesChanged: 0, additions: 0, deletions: 0 },
        },
      },
      { type: "event", stream: "session", sessionId, event: "history-end" },
    ] as ProjectStreamSocketMessage[];
  }

  chatHistoryMessages(
    sessionId: string,
    threadId: string,
  ): ProjectStreamSocketMessage[] {
    return [
      {
        type: "event",
        stream: "chat",
        sessionId,
        threadId,
        event: "history-start",
        data: "",
      },
      ...(this.chatMessages.get(threadId) ?? []).map((message) => ({
        type: "event",
        stream: "chat",
        sessionId,
        threadId,
        event: "history-message",
        data: JSON.stringify(message),
      })),
      {
        type: "event",
        stream: "chat",
        sessionId,
        threadId,
        event: "history-end",
        data: "",
      },
    ] as ProjectStreamSocketMessage[];
  }

  private projectEvent(
    event: "session_updated" | "workspace_updated",
    data: unknown,
  ): ProjectStreamSocketMessage {
    return {
      type: "event",
      stream: "project-events",
      event,
      data: {
        id: `mock-${++this.seq}`,
        seq: this.seq,
        type: event,
        timestamp: MOCK_NOW,
        data,
      },
    } as ProjectStreamSocketMessage;
  }

  private handleSession(
    method: string,
    sessionId: string,
    rest: string,
    url: URL,
  ): JsonResponse {
    const session = this.sessions.get(sessionId);
    if (!session) return notFound(url.pathname);
    if (rest === "" && method === "GET") return { body: clone(session) };
    if (rest === "/credentials" && method === "GET")
      return { body: { credentials: [] } };
    if (rest === "/threads" && method === "GET")
      return { body: { threads: clone(this.threads.get(sessionId) ?? []) } };
    if (rest.match(/^\/threads\/[^/]+$/) && method === "GET") {
      const threadId = decodeURIComponent(rest.split("/").at(-1) ?? "");
      const thread = (this.threads.get(sessionId) ?? []).find(
        (candidate) => candidate.id === threadId,
      );
      return thread ? { body: clone(thread) } : notFound(url.pathname);
    }
    if (rest === "/commands" && method === "GET")
      return { body: { commands: [] } };
    if (rest === "/files" && method === "GET")
      return {
        body: { path: url.searchParams.get("path") ?? ".", entries: [] },
      };
    if (rest === "/files/search" && method === "GET")
      return { body: { query: url.searchParams.get("q") ?? "", results: [] } };
    if (rest === "/diff" && method === "GET")
      return {
        body: {
          files: [],
          stats: { filesChanged: 0, additions: 0, deletions: 0 },
        },
      };
    if (rest === "/services" && method === "GET")
      return { body: { services: [] } };
    if (rest === "/ports" && method === "GET") return { body: { ports: [] } };
    if (rest === "/hooks/status" && method === "GET")
      return {
        body: {
          hooks: {},
          pendingHooks: [],
          lastEvaluatedAt: MOCK_NOW,
          executionPaused: false,
        },
      };
    if (rest === "/hooks/state" && method === "GET")
      return {
        body: {
          hooks: {},
          pendingHooks: [],
          lastEvaluatedAt: MOCK_NOW,
          executionPaused: false,
          outputs: {},
        },
      };
    return notFound(url.pathname);
  }

  private apiPath(url: URL): string {
    return url.pathname.replace(/\/$/, "") || "/";
  }
}
