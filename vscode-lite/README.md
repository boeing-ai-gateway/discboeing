# vscode-lite

Standalone lightweight IDE for a single local workspace.

`vscode-lite` runs a Go server that owns local file access through a VFS
abstraction and starts language servers for Monaco/LSP features. The frontend is
a separate Svelte/Vite/Monaco app under `web/`.

## Requirements

The server validates these dependencies on startup:

- `gopls`
- `node`
- `typescript-language-server`

For TypeScript, the resolver checks
`{workspace}/node_modules/.bin/typescript-language-server` first, then falls back
to `PATH`.

## Development

```bash
# from repo root
pnpm install
pnpm vscode-lite:web:dev

# in another terminal
go run ./vscode-lite/cmd/vscode-lite --workspace /path/to/repo --addr :3333
```

- Go server: <http://localhost:3333>
- Vite dev server: <http://localhost:3334>

The Vite dev server proxies `/api` and LSP WebSockets to the Go server.

## Discobot service

A workspace service is available as **VSCode Lite**. It exposes the Vite frontend
on port `3334` and starts the Go API/LSP server on port `3333`.

The service runs against the current workspace root. It runs `pnpm install`, adds
`vscode-lite/web/node_modules/.bin` and `GOPATH/bin` to `PATH`, and installs
`gopls` into `GOPATH/bin` if it is missing so the service can start in fresh dev
containers.

## Build/check

```bash
pnpm vscode-lite:web:build
pnpm vscode-lite:web:check
go test ./vscode-lite/...
go build ./vscode-lite/cmd/vscode-lite
```

## MVP scope

Implemented scope targets:

- local workspace VFS,
- file tree/read/write/search APIs,
- Monaco editor tabs,
- IntelliJ-style autosave,
- Go and TypeScript/JavaScript LSP WebSocket bridge,
- diagnostics, go-to-definition, references plumbing, problems panel, and
  outline panel basics.

Deferred:

- terminal,
- Git UI,
- code actions,
- rename,
- formatting,
- LSP process pooling/reuse.
