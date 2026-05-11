#!/usr/bin/env bash
set -euo pipefail

# Validate exe.dev API key access by creating a small VM through the exe.dev
# command endpoint. The token is read from EXEDEV_TOKEN and is never printed.
#
# Usage:
#   EXEDEV_TOKEN='your-exe-dev-token' scripts/validate-exedev-apikey.sh
#
# Optional:
#   EXEDEV_ENDPOINT=https://exe.dev/exec
#   EXEDEV_VM_NAME=discobot-curl-test-123
#   EXEDEV_IMAGE=ubuntu:22.04
#   KEEP_EXEDEV_VM=1

endpoint="${EXEDEV_ENDPOINT:-https://exe.dev/exec}"
token="${EXEDEV_TOKEN:-}"
vm_name="${EXEDEV_VM_NAME:-discobot-curl-test-$(date +%s)-$RANDOM}"
image="${EXEDEV_IMAGE:-}"
keep_vm="${KEEP_EXEDEV_VM:-0}"

if [[ -z "$token" ]]; then
	echo "EXEDEV_TOKEN is required" >&2
	exit 2
fi

if ! command -v curl >/dev/null 2>&1; then
	echo "curl is required" >&2
	exit 2
fi

run_exedev() {
	local command="$1"
	curl --fail-with-body --silent --show-error \
		--request POST "$endpoint" \
		--header "Authorization: Bearer $token" \
		--header "Content-Type: text/plain" \
		--data-binary "$command"
}

cleanup() {
	local status=$?
	if [[ "$keep_vm" == "1" ]]; then
		echo "Keeping VM: $vm_name"
		exit "$status"
	fi
	if [[ "${created:-0}" == "1" ]]; then
		echo "Removing VM: $vm_name"
		if ! run_exedev "rm --json $vm_name"; then
			echo "Warning: failed to remove VM $vm_name" >&2
		fi
	fi
	exit "$status"
}
trap cleanup EXIT

create_command="new --json --name=$vm_name"
if [[ -n "$image" ]]; then
	create_command="$create_command --image=$image"
fi

echo "Creating exe.dev VM: $vm_name"
create_response="$(run_exedev "$create_command")"
created=1
printf '%s\n' "$create_response"

echo "Checking exe.dev VM: $vm_name"
run_exedev "ls --json --l $vm_name"
printf '\n'

echo "exe.dev API key validation succeeded."
