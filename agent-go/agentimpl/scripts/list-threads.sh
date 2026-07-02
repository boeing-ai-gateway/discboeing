#!/usr/bin/env bash
set -eu

data_dir="${DISCBOEING_DATA_DIR:-$HOME/.discboeing}"
threads_dir="${DISCBOEING_THREADS_DIR:-$data_dir/threads}"
current_thread_id="${DISCBOEING_SESSION_ID:-}"

if [ ! -d "$threads_dir" ]; then
	echo "Threads directory not found: $threads_dir"
	exit 0
fi

if ! command -v python3 >/dev/null 2>&1; then
	echo "python3 is required to list thread names."
	echo "Threads directory: $threads_dir"
	exit 0
fi

THREADS_DIR="$threads_dir" CURRENT_THREAD_ID="$current_thread_id" python3 - <<'PY'
import json
import os
import pathlib

threads_dir = pathlib.Path(os.environ["THREADS_DIR"])
current_thread_id = os.environ.get("CURRENT_THREAD_ID", "").strip()

items = []
for path in sorted(threads_dir.iterdir()):
    if not path.is_dir() or path.name == current_thread_id:
        continue
    config_path = path / "config.json"
    name = ""
    try:
        if config_path.exists():
            data = json.loads(config_path.read_text())
            if isinstance(data, dict):
                name = str(data.get("name", "")).strip()
    except Exception:
        name = ""
    items.append((path.name, name))

if not items:
    print("No threads found.")
else:
    for thread_id, name in items:
        if name:
            print(f"{thread_id}\t{name}")
        else:
            print(thread_id)
PY
