#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./dump-meta.sh [DATABASE_PATH] [OUTPUT_PATH]

Dump Meta SQLite table data as INSERT statements, without DDL.

Arguments:
  DATABASE_PATH  SQLite database path. Defaults to:
                 $HOME/.local/share/discobot/meta.db
  OUTPUT_PATH    Output SQL file. Defaults to stdout.

Examples:
  ./dump-meta.sh
  ./dump-meta.sh ~/.local/share/discobot/meta.db /tmp/meta-data.sql
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

db_path="${1:-$HOME/.local/share/discobot/meta.db}"
out_path="${2:-}"

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "error: sqlite3 is required" >&2
  exit 1
fi

if [[ ! -f "$db_path" ]]; then
  echo "error: database not found: $db_path" >&2
  exit 1
fi

dump_data() {
  sqlite3 "$db_path" "SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;" |
  while IFS= read -r table; do
    [[ -n "$table" ]] || continue
    printf '\n-- %s\n' "$table"
    sqlite3 "$db_path" ".mode insert $table" "SELECT * FROM \"$table\";"
  done
}

if [[ -n "$out_path" ]]; then
  dump_data > "$out_path"
  echo "Wrote $out_path"
else
  dump_data
fi
