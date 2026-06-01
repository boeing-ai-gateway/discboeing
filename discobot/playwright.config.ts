import { defineConfig, devices } from "@playwright/test";
import { fileURLToPath } from "node:url";

const port = Number(process.env.DISCOBOT_E2E_PORT ?? 3330);
const testDataURL = new URL("./tests/fixtures/model-selector-data.json", import.meta.url);
const testDataPath = fileURLToPath(testDataURL);
const sessionDirURL = new URL("./test-results/session-store", import.meta.url);
const sessionDirPath = fileURLToPath(sessionDirURL);

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
		command: `rm -rf ${sessionDirPath} && mkdir -p ${sessionDirPath} && DISCOBOT_PORT=${port} DISCOBOT_STATIC_DIR=static DISCOBOT_SERVER_URL=file://${testDataPath} DISCOBOT_SESSION_DIR=${sessionDirPath} go run ./cmd/discobot`,
		url: `http://127.0.0.1:${port}`,
		reuseExistingServer: false,
		timeout: 30_000,
	},
	projects: [
		{
			name: "chromium",
			use: { ...devices["Desktop Chrome"] },
		},
	],
});
