#!/bin/bash
set -euo pipefail

workspace="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
token="$("${workspace}"/scripts/meta-bootstrap-token.py)"

exec go run ./meta/cmd/metactl --token "$token" "$@"
