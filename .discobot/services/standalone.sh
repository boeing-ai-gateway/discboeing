#!/bin/bash
#---
# name: Discobot Standalone UI
# description: Embedded standalone server serving the built Svelte SPA
# order: 3
# http: 3001
# path: /
#---

set -euo pipefail

SQL_DUMP="${WORKSPACE_ORIGIN_PATH}/test.db.sql"
DB="${HOME}/.local/share/discobot/discobot.db"
ENV_FILE="./server/.env"

seed_database() {
    if [ ! -e "$DB" ] && [ -e "$SQL_DUMP" ]; then
        mkdir -p "$(dirname "$DB")"
        sqlite3 "$DB" < "$SQL_DUMP"
    fi
}

ensure_server_env() {
    if ! grep -q "^SANDBOX_IMAGE=" "$ENV_FILE" 2>/dev/null; then
        echo "SANDBOX_IMAGE=ghcr.io/obot-platform/discobot:nonexistent" >> "$ENV_FILE"
    fi
}

binary_name() {
    local os arch ext
    os=$(uname -s)
    arch=$(uname -m)
    ext=""

    case "$os" in
        Linux) os="unknown-linux-gnu" ;;
        Darwin) os="apple-darwin" ;;
        MINGW*|MSYS*|CYGWIN*)
            os="pc-windows-msvc"
            ext=".exe"
            ;;
        *)
            echo "Unsupported OS: $os" >&2
            exit 1
            ;;
    esac

    case "$arch" in
        x86_64|amd64) arch="x86_64" ;;
        arm64|aarch64) arch="aarch64" ;;
        *)
            echo "Unsupported architecture: $arch" >&2
            exit 1
            ;;
    esac

    printf 'discobot-server-%s-%s%s\n' "$arch" "$os" "$ext"
}

seed_database
ensure_server_env

pnpm install
pnpm build:server

SERVER_BIN="./src-tauri/binaries/$(binary_name)"
if [ ! -x "$SERVER_BIN" ]; then
    echo "Standalone server binary not found: $SERVER_BIN" >&2
    exit 1
fi

cd server
exec "../${SERVER_BIN#./}" 
