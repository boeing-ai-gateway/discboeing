import type { Page } from "@playwright/test";

import type { MockSessionFixtures } from "./session-fixtures";

export type MockProjectSocketRequest = {
  type: "subscribe" | "unsubscribe";
  stream: "project-events" | "session" | "chat" | string;
  sessionId?: string;
  threadId?: string;
  serviceId?: string;
};

export type MockProjectSocketMessage = {
  type: "subscribed" | "unsubscribed" | "event" | "complete" | "error";
  stream: string;
  sessionId?: string;
  threadId?: string;
  serviceId?: string;
  event?: string;
  data?: unknown;
  error?: string;
};

export const mockWebSocketRequestsBinding = "__discboeingMockWebSocketRequests";

export async function installSessionWebSocketMock(
  page: Page,
  fixtures: MockSessionFixtures,
): Promise<void> {
  await page.addInitScript(
    ({ fixtures, requestsBinding }) => {
      type Listener = (event: Event) => void;
      type Request = {
        type: "subscribe" | "unsubscribe";
        stream: string;
        sessionId?: string;
        threadId?: string;
        serviceId?: string;
      };

      const state = {
        requests: [] as Request[],
      };
      Object.assign(window, { [requestsBinding]: state.requests });

      class MockMessageEvent extends Event {
        data: string;

        constructor(data: unknown) {
          super("message");
          this.data = JSON.stringify(data);
        }
      }

      class MockCloseEvent extends Event {
        code = 1000;
        reason = "";
        wasClean = true;

        constructor() {
          super("close");
        }
      }

      class MockWebSocket extends EventTarget {
        static CONNECTING = 0;
        static OPEN = 1;
        static CLOSING = 2;
        static CLOSED = 3;

        readonly CONNECTING = 0;
        readonly OPEN = 1;
        readonly CLOSING = 2;
        readonly CLOSED = 3;

        binaryType: BinaryType = "blob";
        bufferedAmount = 0;
        extensions = "";
        onclose: ((event: Event) => void) | null = null;
        onerror: ((event: Event) => void) | null = null;
        onmessage: ((event: MessageEvent<string>) => void) | null = null;
        onopen: ((event: Event) => void) | null = null;
        protocol = "";
        readyState = MockWebSocket.CONNECTING;
        url: string;

        constructor(url: string | URL) {
          super();
          this.url = String(url);
          window.setTimeout(() => {
            this.readyState = MockWebSocket.OPEN;
            this.dispatch("open", new Event("open"));
          }, 0);
        }

        send(data: string | ArrayBufferLike | Blob | ArrayBufferView): void {
          if (typeof data !== "string") return;
          const request = JSON.parse(data) as Request;
          state.requests.push(request);
          window.setTimeout(() => this.handleRequest(request), 0);
        }

        close(): void {
          if (this.readyState === MockWebSocket.CLOSED) return;
          this.readyState = MockWebSocket.CLOSED;
          this.dispatch("close", new MockCloseEvent());
        }

        override addEventListener(
          type: string,
          listener: EventListenerOrEventListenerObject | null,
          options?: AddEventListenerOptions | boolean,
        ): void {
          super.addEventListener(type, listener, options);
        }

        override removeEventListener(
          type: string,
          listener: EventListenerOrEventListenerObject | null,
          options?: EventListenerOptions | boolean,
        ): void {
          super.removeEventListener(type, listener, options);
        }

        private handleRequest(request: Request): void {
          if (request.type === "unsubscribe") {
            this.emit({ ...request, type: "unsubscribed" });
            return;
          }

          if (request.type !== "subscribe") return;
          this.emit({ ...request, type: "subscribed" });

          if (request.stream === "project-events") {
            this.emitEvent(request, "history-start", "");
            for (const workspace of fixtures.workspaces) {
              this.emitEvent(request, "workspace_updated", {
                type: "workspace_updated",
                data: workspace,
              });
            }
            for (const session of fixtures.sessions) {
              this.emitEvent(request, "session_updated", {
                type: "session_updated",
                data: session,
              });
            }

            this.emitEvent(request, "history-end", "");
            return;
          }

          if (request.stream === "session" && request.sessionId) {
            const session = fixtures.sessions.find(
              (candidate) => candidate.id === request.sessionId,
            );
            const threads =
              fixtures.threadsBySessionId[request.sessionId] ?? [];
            this.emitEvent(request, "history-start", "");
            if (session) this.emitEvent(request, "session_updated", session);
            this.emitEvent(request, "threads_updated", { threads });
            this.emitEvent(request, "files_updated", { path: "", entries: [] });
            this.emitEvent(request, "commands_updated", { commands: [] });
            this.emitEvent(request, "hooks_updated", {
              hooks: {},
              pendingHooks: [],
              executionPaused: false,
            });
            this.emitEvent(request, "services_updated", { services: [] });
            this.emitEvent(request, "diff_status_updated", { files: [] });
            this.emitEvent(request, "history-end", "");
            return;
          }

          if (request.stream === "chat" && request.threadId) {
            const messages =
              fixtures.chatHistoryByThreadId[request.threadId] ?? [];
            this.emitEvent(request, "history-start", "");
            for (const message of messages) {
              this.emitEvent(
                request,
                "history-message",
                JSON.stringify(message),
              );
            }
            this.emitEvent(request, "history-end", "");
          }
        }

        private emitEvent(
          request: Request,
          event: string,
          data: unknown,
        ): void {
          this.emit({
            type: "event",
            stream: request.stream,
            sessionId: request.sessionId,
            threadId: request.threadId,
            serviceId: request.serviceId,
            event,
            data,
          });
        }

        private emit(message: unknown): void {
          this.dispatch("message", new MockMessageEvent(message));
        }

        private dispatch(type: string, event: Event): void {
          super.dispatchEvent(event);
          const listener = this[`on${type}` as keyof this] as Listener | null;
          listener?.(event);
        }
      }

      Object.assign(window, { WebSocket: MockWebSocket });
    },
    { fixtures, requestsBinding: mockWebSocketRequestsBinding },
  );
}

export async function readMockWebSocketRequests(
  page: Page,
): Promise<MockProjectSocketRequest[]> {
  return page.evaluate(
    (binding) => [
      ...((window as unknown as Record<string, MockProjectSocketRequest[]>)[
        binding
      ] ?? []),
    ],
    mockWebSocketRequestsBinding,
  );
}
