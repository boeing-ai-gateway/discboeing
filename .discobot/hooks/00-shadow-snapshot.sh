#!/bin/bash
#---
# name: Shadow workspace snapshot
# type: file
# pattern: "**/*"
# notify_llm: false
#---
set -euo pipefail

workspace=${DISCOBOT_WORKSPACE:-$(pwd)}
cd "$workspace"

git rev-parse --is-inside-work-tree >/dev/null
base=$(git rev-parse HEAD)
base_tree=$(git rev-parse 'HEAD^{tree}')

tmp_index=$(mktemp)
cleanup() {
	rm -f "$tmp_index"
}
trap cleanup EXIT

GIT_INDEX_FILE=$tmp_index git read-tree HEAD
GIT_INDEX_FILE=$tmp_index git add -A
snapshot_tree=$(GIT_INDEX_FILE=$tmp_index git write-tree)

if [ "$snapshot_tree" = "$base_tree" ]; then
	echo "No workspace changes to snapshot."
	exit 0
fi

raw_id=${DISCOBOT_TURN_ID:-${DISCOBOT_STEP_ID:-${DISCOBOT_COMPLETION_ID:-}}}
if [ -z "$raw_id" ]; then
	raw_id="$(date -u +%Y%m%dT%H%M%SZ)-$$"
fi

safe_id=$(printf '%s' "$raw_id" | tr -c 'A-Za-z0-9._-' '-')
session_id=${DISCOBOT_SESSION_ID:-session}
safe_session=$(printf '%s' "$session_id" | tr -c 'A-Za-z0-9._-' '-')
ref="refs/discobot/snapshots/$safe_session/$safe_id"

changed_files=${DISCOBOT_CHANGED_FILES:-}
message=$(cat <<EOF
Discobot shadow snapshot $safe_id

Session: $session_id
Base: $base
Changed files: $changed_files
EOF
)

commit=$(
	GIT_AUTHOR_NAME=${GIT_AUTHOR_NAME:-Discobot Snapshot} \
	GIT_AUTHOR_EMAIL=${GIT_AUTHOR_EMAIL:-discobot-snapshot@localhost} \
	GIT_COMMITTER_NAME=${GIT_COMMITTER_NAME:-Discobot Snapshot} \
	GIT_COMMITTER_EMAIL=${GIT_COMMITTER_EMAIL:-discobot-snapshot@localhost} \
	git commit-tree "$snapshot_tree" -p "$base" -F - <<<"$message"
)

git update-ref "$ref" "$commit"
echo "Created shadow snapshot $commit at $ref"
