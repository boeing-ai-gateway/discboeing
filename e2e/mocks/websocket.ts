import type { Page, WebSocketRoute } from "@playwright/test";
import type { ProjectStreamSocketRequest } from "../../ui/src/lib/api-types";
import { FakeDiscobotApi } from "./fake-api";

export async function installProjectWebSocketMock(
  page: Page,
  api: FakeDiscobotApi,
) {
  await page.routeWebSocket("**/api/projects/local/ws**", async (ws) => {
    ws.onMessage((message) => handleMessage(ws, api, message));
  });
}

function send(ws: WebSocketRoute, value: unknown) {
  ws.send(JSON.stringify(value));
}

function handleMessage(
  ws: WebSocketRoute,
  api: FakeDiscobotApi,
  message: string | Buffer,
) {
  let request: ProjectStreamSocketRequest;
  try {
    request = JSON.parse(String(message)) as ProjectStreamSocketRequest;
  } catch {
    send(ws, { type: "error", error: "Invalid JSON websocket message" });
    return;
  }
  api.webSocketRequests.push(request);

  const ack = {
    type: request.type === "unsubscribe" ? "unsubscribed" : "subscribed",
    stream: request.stream,
    sessionId: request.sessionId,
    threadId: request.threadId,
    serviceId: request.serviceId,
  };
  send(ws, ack);

  if (request.type !== "subscribe") return;
  if (request.stream === "project-events") {
    for (const event of api.projectHistoryMessages()) send(ws, event);
    return;
  }
  if (request.stream === "session" && request.sessionId) {
    for (const event of api.sessionHistoryMessages(request.sessionId))
      send(ws, event);
    return;
  }
  if (request.stream === "chat" && request.sessionId && request.threadId) {
    for (const event of api.chatHistoryMessages(
      request.sessionId,
      request.threadId,
    ))
      send(ws, event);
    return;
  }
  if (request.stream === "service") {
    send(ws, {
      type: "complete",
      stream: "service",
      sessionId: request.sessionId,
      serviceId: request.serviceId,
    });
  }
}
