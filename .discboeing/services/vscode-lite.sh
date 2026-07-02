#!/bin/bash
#---
# name: VSCode Lite
# description: Standalone Monaco/LSP editor for this workspace
# order: 4
# http: 3334
# path: /
#---

set -euo pipefail

WORKSPACE="${DISCBOEING_WORKSPACE:-$(pwd)}"
GO_ADDR=":3333"
WEB_PORT="3334"

export PATH="${WORKSPACE}/vscode-lite/web/node_modules/.bin:${GOPATH:-${HOME}/go}/bin:${PATH}"

cd "$WORKSPACE"

pnpm install

if ! command -v gopls >/dev/null 2>&1; then
    echo "gopls is required for vscode-lite. Installing into GOPATH/bin..." >&2
    go install golang.org/x/tools/gopls@latest
fi

if ! command -v node >/dev/null 2>&1; then
    echo "node is required for vscode-lite" >&2
    exit 1
fi

if ! command -v typescript-language-server >/dev/null 2>&1; then
    echo "typescript-language-server is required for vscode-lite" >&2
    echo "It should be installed by pnpm from vscode-lite/web/package.json." >&2
    exit 1
fi

cleanup() {
    if [ -n "${SERVER_PID:-}" ]; then
        kill "$SERVER_PID" >/dev/null 2>&1 || true
        wait "$SERVER_PID" >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT INT TERM

go run ./vscode-lite/cmd/vscode-lite \
    --workspace "$WORKSPACE" \
    --addr "$GO_ADDR" &
SERVER_PID=$!

exec pnpm --dir ./vscode-lite/web dev -- --host 0.0.0.0 --port "$WEB_PORT"
