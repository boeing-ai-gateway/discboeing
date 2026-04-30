#!/bin/bash
set -euo pipefail

PLATFORM="${DISCOBOT_GUEST_PLATFORM:-vz}"

if [ "$#" -eq 2 ] && [ "$1" = "platform" ]; then
    EXPECTED_PLATFORM="$2"
    case "${EXPECTED_PLATFORM}" in
        wsl)
            [ "${PLATFORM}" = "wsl" ]
            exit
            ;;
        non-wsl)
            [ "${PLATFORM}" != "wsl" ]
            exit
            ;;
    esac
fi

echo "usage: $0 platform <wsl|non-wsl>" >&2
exit 2
