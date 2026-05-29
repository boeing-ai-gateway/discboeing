import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.DISCOBOT_E2E_PORT ?? 3330);

export default defineConfig({
	testDir: "./tests/e2e",
	timeout: 30_000,
	fullyParallel: false,
	workers: 1,
	use: {
		baseURL: `http://127.0.0.1:${port}`,
		trace: "on-first-retry",
	},
	webServer: {
		command: `DISCOBOT_PORT=${port} DISCOBOT_STATIC_DIR=static go run ./cmd/discobot`,
		url: `http://127.0.0.1:${port}`,
		reuseExistingServer: !process.env.CI,
		timeout: 30_000,
	},
	projects: [
		{
			name: "chromium",
			use: { ...devices["Desktop Chrome"] },
		},
	],
});
