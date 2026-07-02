#!/bin/sh
set -eu

mode="${1:-shell}"

ln -sf /opt/discboeing/bin/discboeing-agent-api /opt/discboeing/bin/disco

mask_units="console-getty.service getty@.service serial-getty@.service"
enable_units="tmp.mount discboeing-sandbox-init.service discboeing-proxy.service docker.socket discboeing-nix-restore.service nix-daemon.socket discboeing-agent-api.service discboeing-vscode.socket"

case "${mode}" in
    shell)
        ;;
    gui)
        mask_units="${mask_units} systemd-logind.service"
        enable_units="${enable_units} x11-display.socket x11vnc.socket websockify-proxy.socket"
        ;;
    *)
        echo "usage: $0 [shell|gui]" >&2
        exit 2
        ;;
esac

for unit in ${mask_units}; do
    systemctl mask "${unit}"
done

systemctl disable docker.service containerd.service

for unit in ${enable_units}; do
    systemctl enable "${unit}"
done
