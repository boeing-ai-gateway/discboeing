#!/bin/bash
#---
# name: Install Templ
# type: session
#---
set -euo pipefail

module_dir="${DISCBOEING_WORKSPACE:-$(pwd)}/discboeing"

if [[ ! -f "$module_dir/go.mod" ]]; then
	echo "discboeing/go.mod not found; skipping Templ install"
	exit 0
fi

version="$(
	cd "$module_dir"
	go list -m -f '{{.Version}}' github.com/a-h/templ 2>/dev/null || true
)"
if [[ -z "$version" ]]; then
	echo "github.com/a-h/templ is not listed in discboeing/go.mod; skipping Templ install"
	exit 0
fi

gopath="$(go env GOPATH)"
install_dir="${gopath}/bin"
mkdir -p "$install_dir"

echo "Installing Templ $version to $install_dir"
GOBIN="$install_dir" go install "github.com/a-h/templ/cmd/templ@$version"
