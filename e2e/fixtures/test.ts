import { test as base } from "@playwright/test";
import { createFakeDiscobotApi, type FakeDiscobotApi } from "../mocks/fixtures";
import { installApiRoutes } from "../mocks/routes";
import { installProjectWebSocketMock } from "../mocks/websocket";

type MockedFixtures = {
	fakeApi: FakeDiscobotApi;
};

export const test = base.extend<MockedFixtures>({
	fakeApi: async ({ page }, use) => {
		const fakeApi = createFakeDiscobotApi();
		await installApiRoutes(page, fakeApi);
		await installProjectWebSocketMock(page, fakeApi);
		await use(fakeApi);
	},
});

export { expect } from "@playwright/test";
