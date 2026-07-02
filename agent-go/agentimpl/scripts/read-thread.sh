#!/usr/bin/env bash
set -eu

if [ "$#" -ne 1 ]; then
	echo "Usage: read-thread <thread-id>" >&2
	exit 1
fi

thread_id="$1"
data_dir="${DISCBOEING_DATA_DIR:-$HOME/.discboeing}"
threads_dir="${DISCBOEING_THREADS_DIR:-$data_dir/threads}"
thread_dir="$threads_dir/$thread_id"
config_path="$thread_dir/config.json"
turn_path="$thread_dir/turn.json"
messages_dir="$thread_dir/messages"

if [ ! -d "$thread_dir" ]; then
	echo "Thread not found: $thread_id" >&2
	echo "Expected directory: $thread_dir" >&2
	exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
	echo "python3 is required to format thread output."
	echo "Thread directory: $thread_dir"
	echo "Config file: $config_path"
	echo "Turn state file: $turn_path"
	echo "Messages directory: $messages_dir"
	exit 0
fi

THREAD_ID="$thread_id" THREAD_DIR="$thread_dir" CONFIG_PATH="$config_path" TURN_PATH="$turn_path" MESSAGES_DIR="$messages_dir" python3 - <<'PY'
import json
import os
import pathlib
import sys

thread_id = os.environ["THREAD_ID"]
thread_dir = pathlib.Path(os.environ["THREAD_DIR"])
config_path = pathlib.Path(os.environ["CONFIG_PATH"])
turn_path = pathlib.Path(os.environ["TURN_PATH"])
messages_dir = pathlib.Path(os.environ["MESSAGES_DIR"])


def load_json(path):
    try:
        return json.loads(path.read_text())
    except Exception:
        return None


config = load_json(config_path)
turn = load_json(turn_path)

messages = {}
children = set()

if messages_dir.is_dir():
    for path in sorted(messages_dir.glob("*.json")):
        data = load_json(path)
        if not isinstance(data, dict):
            continue
        msg_id = data.get("id")
        if not msg_id:
            continue
        messages[msg_id] = data
        parent_id = data.get("parentId")
        if parent_id:
            children.add(parent_id)


def choose_leaf():
    for candidate in (
        ((config or {}).get("activeLeafId") if isinstance(config, dict) else None),
        ((turn or {}).get("leafMsgId") if isinstance(turn, dict) else None),
    ):
        if candidate in messages:
            return candidate
    leaves = []
    for msg_id in messages:
        if msg_id not in children:
            path = messages_dir / f"{msg_id}.json"
            try:
                mtime = path.stat().st_mtime
            except OSError:
                mtime = 0
            leaves.append((mtime, msg_id))
    if not leaves:
        return None
    leaves.sort(reverse=True)
    return leaves[0][1]


leaf_id = choose_leaf()
if not leaf_id:
    print(f"# Thread {thread_id}")
    print(f"Directory: {thread_dir}")
    print()
    print("No readable messages found.")
    sys.exit(0)

history = []
seen = set()
current = leaf_id
while current and current not in seen:
    seen.add(current)
    item = messages.get(current)
    if not item:
        break
    history.append(item)
    current = item.get("parentId")
history.reverse()


def format_part(part):
    if not isinstance(part, dict):
        return "[unrecognized part]"
    part_type = part.get("type", "")
    if part_type in ("text", "reasoning"):
        return part.get("text", "").strip()
    if part_type == "tool-call":
        tool_name = part.get("toolName", "tool")
        tool_input = part.get("input")
        if tool_input:
            return f"[tool call] {tool_name} {tool_input}"
        return f"[tool call] {tool_name}"
    if part_type == "tool-result":
        result = f"[tool result] {part.get('toolName', 'tool')}"
        output = format_tool_output(part.get("output"))
        if output:
            return result + "\n" + output
        return result
    if part_type == "tool-approval-request":
        return f"[approval request] {part.get('approvalId', '')}".strip()
    if part_type == "tool-approval-response":
        return f"[approval response] {part.get('approvalId', '')}".strip()
    return f"[{part_type or 'part'}]"


def format_tool_output(output):
    if not isinstance(output, dict):
        return ""
    output_type = output.get("type", "")
    if output_type in ("text", "error-text"):
        return str(output.get("value", "")).strip()
    if output_type in ("json", "error-json"):
        value = output.get("value")
        if value is None:
            return ""
        if isinstance(value, str):
            return value.strip()
        return json.dumps(value, ensure_ascii=False)
    if output_type == "execution-denied":
        reason = str(output.get("reason", "")).strip()
        if reason:
            return f"execution denied: {reason}"
        return "execution denied"
    if output_type == "content":
        rendered = []
        for item in output.get("value", []):
            if not isinstance(item, dict):
                continue
            item_type = item.get("type", "")
            if item_type == "text":
                text = str(item.get("text", "")).strip()
                if text:
                    rendered.append(text)
                continue
            rendered.append(json.dumps(item, ensure_ascii=False))
        return "\n".join(rendered)
    return json.dumps(output, ensure_ascii=False)


def format_message(entry):
    message = entry.get("message", {})
    role = message.get("role", "unknown").upper()
    created_at = message.get("createdAt", "")
    synthetic = " synthetic" if message.get("synthetic") else ""
    print(f"## {role} {entry.get('id', '')}{synthetic}")
    if created_at:
        print(f"createdAt: {created_at}")
    for part in message.get("parts", []):
        text = format_part(part)
        if text:
            print(text)
    print()


print(f"# Thread {thread_id}")
if isinstance(config, dict) and config.get("name"):
    print(f"Name: {config['name']}")
print(f"Directory: {thread_dir}")
print()
for entry in history:
    format_message(entry)
PY
