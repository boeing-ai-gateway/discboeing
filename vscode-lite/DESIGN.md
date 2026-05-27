# vscode-lite Design and Implementation Plan

`vscode-lite` is a standalone lightweight IDE subsystem that lives under
`./vscode-lite`. It is intentionally independent from Discobot sessions for the
MVP. The subsystem runs one local Go server process for one local workspace and
serves a Svelte/Vite/Monaco frontend.

## Locked product decisions

- Standalone subsystem under `./vscode-lite`.
- Standalone Go binary plus Svelte/Vite frontend.
- Run with a workspace argument:

  ```bash
  vscode-lite --workspace /path/to/repo
  ```

- Single workspace per process.
- The vscode-lite Go server owns LSP process lifecycle.
- The MVP does not depend on Discobot project/session APIs.
- File operations go through a VFS abstraction.
- Initial VFS implementation is a local filesystem VFS rooted at the configured
  workspace.
- Go and TypeScript are the required MVP languages.
- Required startup dependencies:
  - `gopls`
  - `node`
  - `typescript-language-server`
- For TypeScript, resolve `{workspace}/node_modules/.bin/typescript-language-server`
  before falling back to `PATH`.
- Do not auto-install language servers yet. During development/testing they are
  expected to be installed locally or globally.
- Autosave should behave like IntelliJ:
  - send LSP `textDocument/didChange` immediately as Monaco changes,
  - write to disk after a debounce,
  - force save on editor blur, tab switch, and window blur,
  - send LSP `textDocument/didSave` after disk writes.
- Suggested ports:
  - Go server: `3333`
  - Vite dev server: `3334`
- Frontend is independent from existing `./ui`; do not import main app
  components.
- MVP excludes terminal and Git UI.

## Run model

Development:

```bash
pnpm --dir vscode-lite/web dev
# in another terminal
go run ./vscode-lite/cmd/vscode-lite --workspace /path/to/repo --addr :3333
```

Production/build target:

```bash
pnpm --dir vscode-lite/web build
go build ./vscode-lite/cmd/vscode-lite
./vscode-lite --workspace /path/to/repo --addr :3333
```

When `web/dist` exists, the Go server should be able to serve it. During dev,
Vite serves the frontend and proxies `/api` and LSP WebSocket requests to the Go
server.

## Proposed package layout

```text
vscode-lite/
├── DESIGN.md
├── README.md
├── cmd/
│   └── vscode-lite/
│       └── main.go
├── internal/
│   ├── lsp/
│   │   ├── framing.go
│   │   ├── manager.go
│   │   ├── resolver.go
│   │   └── server.go
│   ├── server/
│   │   ├── api.go
│   │   ├── lsp.go
│   │   ├── server.go
│   │   └── static.go
│   └── vfs/
│       ├── local.go
│       ├── path.go
│       └── vfs.go
└── web/
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    ├── index.html
    └── src/
        ├── main.ts
        ├── app/
        ├── api/
        ├── editor/
        ├── lsp/
        └── styles.css
```

## VFS design

All file APIs must use `VFS`, not direct ad hoc filesystem access.

Suggested interface:

```go
type VFS interface {
    Root() string
    List(ctx context.Context, path string, opts ListOptions) (*ListResult, error)
    Read(ctx context.Context, path string) (*ReadResult, error)
    Write(ctx context.Context, path string, content []byte) error
    Stat(ctx context.Context, path string) (*FileInfo, error)
    Rename(ctx context.Context, oldPath, newPath string) error
    Delete(ctx context.Context, path string) error
}
```

Initial implementation: `LocalVFS`.

Path rules:

- API paths are workspace-relative slash paths.
- `.` means workspace root.
- Reject absolute paths.
- Reject `..` traversal.
- Reject paths that resolve outside the workspace root.
- Reject symlink escapes for existing targets.
- For writes to new files, validate the resolved parent directory stays within
  root.

This makes room for future implementations such as:

- `DiscobotSessionVFS`
- remote VFS
- overlay VFS
- Git-aware VFS

## HTTP API

Minimal API surface:

```text
GET /api/workspace
GET /api/files/tree?path=.
GET /api/files/content?path=src/main.go
PUT /api/files/content
POST /api/files/rename
POST /api/files/delete
GET /api/files/search?q=foo&limit=50
GET /api/lsp/{language}
```

`PUT /api/files/content` body:

```json
{
  "path": "src/main.go",
  "content": "..."
}
```

`GET /api/workspace` should return at least:

```json
{
  "root": "/abs/path/to/workspace",
  "languages": {
    "go": { "available": true, "command": "gopls" },
    "typescript": { "available": true, "command": ".../typescript-language-server" }
  }
}
```

## LSP design

The browser speaks JSON-RPC over WebSocket. Language servers speak
Content-Length framed JSON-RPC over stdio. The Go server bridges them:

```text
Monaco / LSP client
  <-> WebSocket JSON-RPC
  <-> Go LSP bridge
  <-> Content-Length framed stdio
  <-> language server process
```

Supported MVP language routes:

```text
/api/lsp/go          -> gopls
/api/lsp/typescript  -> typescript-language-server --stdio
/api/lsp/javascript  -> typescript-language-server --stdio
```

Command resolution:

- Go: require `gopls` in `PATH`.
- TypeScript/JavaScript:
  1. prefer `{workspace}/node_modules/.bin/typescript-language-server`,
  2. fallback to `typescript-language-server` in `PATH`,
  3. require `node` in `PATH`.

Startup validation should fail if required dependencies are missing. Error
messages should identify the missing binary and suggested install command, but
should not auto-install.

LSP lifecycle for MVP:

- A WebSocket connection starts one language server process.
- Disconnect shuts down/kills that process.
- Reuse/pooling can come later.
- Start language server with working directory set to workspace root.
- Pass real `file://` URIs based on the local workspace path.

## Frontend MVP

Use Svelte + Vite + Monaco. The UI should include:

- file explorer,
- tab strip,
- Monaco editor,
- problems panel,
- outline panel,
- basic status bar.

Editor behavior:

- Click file -> read content -> create/reuse Monaco model -> open tab.
- Each open tab tracks:
  - relative path,
  - `file://` URI,
  - language ID,
  - Monaco model,
  - dirty/pending-save state,
  - last save error.
- LSP `didOpen` when model opens.
- LSP `didChange` immediately on model edit.
- Debounced autosave, default around `750ms`.
- Force save on editor blur, tab switch, and window blur.
- LSP `didSave` after successful write.
- LSP `didClose` when model closes.

LSP-backed MVP features:

- diagnostics rendered as Monaco markers and in Problems panel,
- hover,
- go to definition,
- find references,
- document symbols/outline.

Defer until after MVP:

- terminal,
- Git UI/decorations,
- code actions,
- rename,
- formatting,
- workspace symbol search,
- persistent settings,
- process reuse/pooling.

## Implementation checklist

1. Create/finish `vscode-lite` package structure.
2. Add root workspace/package integration and convenient scripts where useful.
3. Implement CLI parsing for `--workspace` and `--addr`.
4. Implement dependency validation for `gopls`, `node`, and
   `typescript-language-server`.
5. Implement VFS interface and LocalVFS with path safety.
6. Add VFS unit tests for traversal, absolute paths, symlink escape, read,
   write, and list behavior.
7. Implement LSP framing helpers and tests.
8. Implement LSP command resolver and tests.
9. Implement HTTP server and file APIs.
10. Implement LSP WebSocket bridge.
11. Create Svelte/Vite frontend.
12. Implement workspace API client and file explorer.
13. Implement Monaco editor, model registry, tabs, and autosave.
14. Implement LSP client connections for Go and TypeScript/JavaScript.
15. Wire diagnostics, hover, definition, references, and outline.
16. Add Problems and Outline panels.
17. Run focused checks:
    - `go test ./vscode-lite/...`
    - `go build ./vscode-lite/cmd/vscode-lite`
    - `pnpm --dir vscode-lite/web build`
    - frontend typecheck/check if configured.
18. Record any limitations in `vscode-lite/README.md`.

## Validation expectations

Because external LSP binaries may not exist in CI/sandbox environments, tests
should cover pure Go logic without requiring live `gopls` or TypeScript server
processes. Runtime startup validation may fail locally if dependencies are not
installed; that should be reported clearly rather than bypassed.
