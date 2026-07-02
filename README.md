# Discboeing

Discboeing is a coding agent session manager. It runs Discboeing's built-in coding agent inside isolated sandboxed sessions so you can monitor and manage AI-assisted coding work safely.

## Demo

[![Discboeing Demo](https://img.youtube.com/vi/y_hRj_BMY_E/maxresdefault.jpg)](https://youtu.be/y_hRj_BMY_E)

## Features

- **Built-in Coding Agent** — Discboeing ships with its own coding agent instead of depending on third-party coding CLIs
- **Anthropic + OpenAI Support** — Run Discboeing sessions with Anthropic and OpenAI models today, with more model providers coming soon
- **Thread-Level Model Selection** — Choose which model to use for each conversation thread
- **Isolated Sandboxed Sessions** — Run parallel sessions in secure containers with full app debugging capabilities
- **Use Your Own IDE** — Launch remote IDE sessions directly into each sandbox
- **SSH into Sandboxes** — Direct SSH access to every sandbox environment
- **Integrated Lightweight Tools** — Built-in terminal, diff viewer, and editor for quick edits
- **Workspaces** — Organize sessions around git repositories or local folders

## Customization

Automate session setup, enforce code quality with script or AI hooks, and run dev servers with [`.discboeing/hooks/` and `.discboeing/services/`](docs/CUSTOMIZATION.md).

## Install

Download the macOS app from [Releases](https://github.com/boeing-ai-gateway/discboeing/releases).

## Build from source

Use the Node.js version from [`.node-version`](.node-version). CI reads the
same file via `actions/setup-node`.

The root [`package.json`](package.json) pins the pnpm version via
`packageManager`, and CI uses that too.

```bash
pnpm build:app
```

Useful build overrides:

- `VZ_IMAGE_REF` controls the bundled macOS VZ image and defaults to
  `ghcr.io/boeing-ai-gateway/discboeing-vz:main`
- `WSL_IMAGE_REF` controls the bundled Windows WSL image and defaults to
  `ghcr.io/boeing-ai-gateway/discboeing-wsl:main`
- `DISCBOEING_VERSION` controls the app/server version metadata used by the
  build scripts
- `DISCBOEING_TARGET_TRIPLE` overrides the sidecar server target triple in CI and
  cross-compilation builds

## Community

Join the [#discboeing channel](https://discord.gg/tHWRW6PVjP) on the Boeing AI Discord.

## License

Apache-2.0
