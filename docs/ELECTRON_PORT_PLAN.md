# Electron Desktop App

Discboeing now supports Electron as its only desktop shell. The previous desktop
shell has been removed.

## Runtime layout

```text
electron/
├── main.ts          # Electron main process
├── preload.ts       # Renderer bridge
├── server.ts        # Bundled Go server sidecar boot
├── tray.ts          # Tray and close-to-tray behavior
├── updater.ts       # Electron updater bridge
├── window-state.ts  # Window state persistence
├── assets/          # App icons and macOS entitlements
├── binaries/        # Built discboeing-server sidecar binaries
└── resources/       # Bundled VZ/WSL guest runtime assets
```

The Svelte renderer talks to desktop-only features through
`ui/src/lib/desktop/` and `ui/src/lib/shell.ts`. Browser runs continue to use the
browser adapter fallback, while packaged desktop runs use the Electron preload
bridge.

## Common commands

```bash
pnpm dev        # Backend, frontend, and Electron shell
pnpm dev:app    # Electron shell only, expecting dev backend/frontend
pnpm build      # Build Electron app directory
pnpm dist:app   # Build distributable Electron artifacts
```

`pnpm build:server` writes sidecar binaries to `electron/binaries/`.
Guest-runtime extraction scripts write packaged assets to `electron/resources/`.

## Release behavior

The release workflow builds Electron artifacts for macOS, Windows, and Linux and
uploads Electron updater metadata. Sentry desktop release builds use
`PUBLIC_SENTRY_DIST=electron`.
