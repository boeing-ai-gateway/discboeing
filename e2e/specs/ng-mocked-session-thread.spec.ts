import type {
  ChatMessage,
  ProjectStreamSocketRequest,
  Session,
  Thread,
  Workspace,
} from "../../ui/src/lib/api-types";
import { expect, test } from "../fixtures/test";
import type { FakeDiscboeingApi } from "../mocks/fixtures";
import {
  mockedSessionFixtures,
  seededSessionId,
  seededThreadIds,
} from "../mocks/session-fixtures";

function clone<T>(value: T): T {
  return structuredClone(value);
}

function seedSessionFixtures(fakeApi: FakeDiscboeingApi): void {
  fakeApi.workspaces = new Map(
    mockedSessionFixtures.workspaces.map((workspace) => [
      workspace.id,
      clone(workspace) as Workspace,
    ]),
  );
  fakeApi.sessions = new Map(
    mockedSessionFixtures.sessions.map((session) => [
      session.id,
      clone(session) as Session,
    ]),
  );
  fakeApi.threads = new Map(
    Object.entries(mockedSessionFixtures.threadsBySessionId).map(
      ([sessionId, threads]) => [sessionId, clone(threads) as Thread[]],
    ),
  );
  fakeApi.chatMessages = new Map(
    Object.entries(mockedSessionFixtures.chatHistoryByThreadId).map(
      ([threadId, messages]) => [threadId, clone(messages) as ChatMessage[]],
    ),
  );
  fakeApi.webSocketRequests = [];
}

async function waitForRequest(
  fakeApi: FakeDiscboeingApi,
  predicate: (request: ProjectStreamSocketRequest) => boolean,
): Promise<ProjectStreamSocketRequest> {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    const match = fakeApi.webSocketRequests.find(predicate);
    if (match) return match;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error("Timed out waiting for mocked websocket request");
}

test.describe("ng mocked session thread e2e", () => {
  test("boots with seeded websocket data and subscribes to a session thread", async ({
    fakeApi,
    page,
  }) => {
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (message.type() === "error") errors.push(message.text());
    });
    seedSessionFixtures(fakeApi);

    await page.goto("/");

    await expect(page.getByRole("button", { name: "Settings" })).toBeVisible({
      timeout: 60_000,
    });
    await waitForRequest(
      fakeApi,
      (request) =>
        request.type === "subscribe" && request.stream === "project-events",
    );

    await expect(page.getByText("Mocked Session").first()).toBeVisible({
      timeout: 20_000,
    });
    await page.getByText("Mocked Session").first().click();

    const planningThread = page.getByText("Mocked planning thread").first();
    if (await planningThread.isVisible().catch(() => false)) {
      await planningThread.click();
    }

    await waitForRequest(
      fakeApi,
      (request) =>
        request.type === "subscribe" &&
        request.stream === "chat" &&
        request.sessionId === seededSessionId &&
        request.threadId === seededThreadIds[0],
    );

    await expect(
      page
        .getByText("Seeded assistant answer for the planning thread.")
        .first(),
    ).toBeVisible({ timeout: 20_000 });
    expect(
      errors,
      "Browser console/page errors during mocked session e2e",
    ).toEqual([]);
  });
});
