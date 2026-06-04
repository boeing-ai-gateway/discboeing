# Browser smoke tests

These Playwright tests cover basic Discobot UI flows against a running app.
They are intended for QA smoke validation after larger frontend changes.

## Run

Start the app before running the E2E suite. Playwright does not start the server.

For the default local target:

```bash
pnpm dev:backend
```

Then, in another terminal:

```bash
pnpm test:e2e
```

The default target is `http://localhost:3100`. The test script checks that this
URL is reachable before launching Playwright.

To use a different target:

```bash
E2E_BASE_URL=http://localhost:3100 pnpm test:e2e
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
