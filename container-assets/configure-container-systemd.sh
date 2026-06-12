#!/bin/sh
set -eu

mode="${1:-shell}"

ln -sf /opt/discobot/bin/discobot-agent-api /opt/discobot/bin/disco

mask_units="console-getty.service getty@.service serial-getty@.service"
enable_units="tmp.mount discobot-sandbox-init.service discobot-proxy.service docker.socket discobot-nix-restore.service nix-daemon.socket discobot-agent-api.service discobot-vscode.socket"

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
