import { expect, test } from "../fixtures/test";

test("mocked e2e framework imports and registers routes", async ({ page, fakeApi }) => {
	await page.goto("about:blank");

	const sessions = await page.evaluate(async () => {
		const response = await fetch(
			"http://localhost:3001/api/projects/local/sessions",
		);
		return {
			status: response.status,
			body: await response.json(),
		};
	});

	expect(sessions.status).toBe(200);
	expect(sessions.body).toEqual({
		sessions: expect.arrayContaining([expect.objectContaining({ id: "session-1" })]),
	});
	expect(fakeApi.projectId).toBe("local");
});
