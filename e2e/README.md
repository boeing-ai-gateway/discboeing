# Browser smoke tests

These Playwright tests cover basic Discobot UI flows against a running app.
They are intended for QA smoke validation after larger frontend changes.

## Run against an already-running app

```bash
pnpm test:e2e
```

The default target is `http://localhost:3100`.

To use a different target:

```bash
E2E_BASE_URL=http://localhost:3100 pnpm test:e2e
```

## Start the app from Playwright

```bash
E2E_START_SERVER=1 pnpm test:e2e
```

By default, Playwright starts `pnpm dev:backend`. If you need to avoid an
already-running or stale dev server, run a fresh frontend and backend on other
ports and point the tests at them:

```bash
E2E_START_SERVER=1 \
E2E_BASE_URL=http://localhost:3101 \
E2E_WEB_SERVER_COMMAND='pnpm exec concurrently -n ui,server -c cyan,green "VITE_DISCOBOT_API_ROOT=/api VITE_DISCOBOT_API_PROXY_TARGET=http://localhost:3023 pnpm --dir ./ui exec vite dev --host 127.0.0.1 --port 3101 --strictPort" "PORT=3023 SSH_PORT=3335 pnpm dev:server"' \
pnpm test:e2e
```

## First-time browser install

If Playwright reports that Chromium is missing, run:

```bash
pnpm exec playwright install chromium
```

## Scope

The smoke suite intentionally avoids destructive or externally visible actions:

- it does not submit prompts;
- it does not create/delete credentials;
- it does not delete sessions, threads, or workspaces.

The suite validates that the app shell boots, primary dialogs open, the sidebar
and new-session affordances are present, and the composer can accept draft text.
