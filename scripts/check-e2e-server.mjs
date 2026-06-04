const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3100";

const controller = new AbortController();
const timeout = setTimeout(() => controller.abort(), 5_000);

try {
  const response = await fetch(baseURL, { signal: controller.signal });
  if (!response.ok) {
    throw new Error(`received HTTP ${response.status}`);
  }
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`E2E server is not reachable at ${baseURL}: ${message}`);
  console.error("Start the app first, then rerun pnpm test:e2e.");
  process.exit(1);
} finally {
  clearTimeout(timeout);
}
