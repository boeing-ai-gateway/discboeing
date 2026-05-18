#!/bin/bash
#---
# name: Backend build
# type: file
# pattern: "**/*.go"
#---
set -euo pipefail

lock_file="${TMPDIR:-/tmp}/discobot-backend-build.lock"
mkdir -p "$(dirname "$lock_file")"

flock "$lock_file" pnpm build:server
