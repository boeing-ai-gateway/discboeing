#!/bin/sh
set -eu

ubuntu_mirror="${1:-}"
ubuntu_ports_mirror="${2:-}"
sources_file="/etc/apt/sources.list.d/ubuntu.sources"

if [ ! -f "${sources_file}" ]; then
    exit 0
fi

if [ -n "${ubuntu_mirror}" ]; then
    sed -i \
        -e "s|http://archive.ubuntu.com/ubuntu|${ubuntu_mirror}|g" \
        -e "s|http://security.ubuntu.com/ubuntu|${ubuntu_mirror}|g" \
        "${sources_file}"
fi

if [ -n "${ubuntu_ports_mirror}" ]; then
    sed -i \
        -e "s|http://ports.ubuntu.com/ubuntu-ports|${ubuntu_ports_mirror}|g" \
        "${sources_file}"
fi
