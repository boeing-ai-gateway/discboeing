#!/bin/bash
# Prepare persistent storage for both Apple VZ and WSL.
# - On VZ, /var lives on the attached writable data disk (/dev/vdb).
# - On WSL, /var is mounted from a host-managed VHD attached as a raw block device.
set -euo pipefail

PLATFORM="${DISCBOEING_GUEST_PLATFORM:-vz}"
VAR_DISK_LABEL="${DISCBOEING_VAR_DISK_LABEL:-discboeing-var}"

log() {
    echo "init-var: $*"
}

wait_for_wsl_var_device() {
    local waiting_logged=0

    while true; do
        local device_path
        device_path="$(blkid -o device -t "LABEL=${VAR_DISK_LABEL}" 2>/dev/null | head -n 1 || true)"
        if [ -n "${device_path}" ]; then
            echo "${device_path}"
            return 0
        fi
        if [ "${waiting_logged}" -eq 0 ]; then
            log "waiting for WSL /var disk label ${VAR_DISK_LABEL}"
            waiting_logged=1
        fi
        sleep 0.5
    done
}

ensure_var_mount() {
    mkdir -p /var

    if mountpoint -q /var; then
        log "/var is already mounted"
        return
    fi

    if [ -b /dev/vdb ]; then
        log "using VZ data disk /dev/vdb for /var"
        if ! blkid /dev/vdb >/dev/null 2>&1; then
            log "formatting /dev/vdb as ext4"
            mkfs.ext4 -F /dev/vdb
        fi
        mount -t ext4 -o defaults,discard /dev/vdb /var
        resize2fs /dev/vdb >/dev/null 2>&1 || true
        return
    fi

    if [ "${PLATFORM}" = "wsl" ]; then
        local device_path
        if ! device_path="$(wait_for_wsl_var_device)"; then
            return 1
        fi
        log "using WSL /var disk ${device_path} with label ${VAR_DISK_LABEL} for /var"
        mount -t ext4 -o defaults,discard "${device_path}" /var
        resize2fs "${device_path}" >/dev/null 2>&1 || true
        return
    fi

    log "no supported persistent /var backing store is available"
    return 1
}

initialize_var_contents() {
    if [ ! -d /var/lib ]; then
        log "initializing /var from /var.skel"
        cp -a /var.skel/. /var/
        log "/var initialization complete"
        return
    fi

    log "/var already initialized"
}

ensure_var_mount
initialize_var_contents
