# syntax=docker/dockerfile:1.7

ARG UBUNTU_MIRROR=http://mirrors.edge.kernel.org/ubuntu
ARG UBUNTU_PORTS_MIRROR=http://ports.ubuntu.com/ubuntu-ports

# Root-module Go dependency cache
FROM golang:1.26 AS root-go-deps

WORKDIR /build

# Copy module files first for better caching
# modelsdev/go.mod, controlsocket, and agent-go module files are needed by
# replace directives in the root go.mod
COPY controlsocket/go.mod controlsocket/go.sum ./controlsocket/
COPY modelsdev/go.mod ./modelsdev/
COPY agent-go/go.mod agent-go/go.sum ./agent-go/
COPY go.mod go.sum ./

# Download dependencies
RUN --mount=type=cache,id=discboeing-gomodcache,target=/go/pkg/mod \
    --mount=type=cache,id=discboeing-gobuildcache,target=/root/.cache/go-build \
    go mod download

# Proxy binary builder
FROM root-go-deps AS proxy-builder

# Copy proxy source
COPY proxy/ ./proxy/

# Build the proxy binary
RUN --mount=type=cache,id=discboeing-gomodcache,target=/go/pkg/mod \
    --mount=type=cache,id=discboeing-gobuildcache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /proxy ./proxy/cmd/proxy

# VSOCK port proxy builder
FROM root-go-deps AS vsock-port-proxy-builder

COPY server/cmd/vsock-port-proxy/ ./server/cmd/vsock-port-proxy/
COPY server/internal/sandbox/vm/vsockproxy/ ./server/internal/sandbox/vm/vsockproxy/

RUN --mount=type=cache,id=discboeing-gomodcache,target=/go/pkg/mod \
    --mount=type=cache,id=discboeing-gobuildcache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /discboeing-vsock-port-proxy ./server/cmd/vsock-port-proxy

# gvisor-tap-vsock builders for HCS user-mode networking.
# gvforwarder runs in the Linux guest; gvproxy.exe runs on the Windows host.
FROM golang:1.26 AS gvforwarder-builder

ARG TARGETARCH
ARG GV_FORWARDER_VERSION=v0.8.7

RUN --mount=type=cache,id=discboeing-gomodcache,target=/go/pkg/mod \
    --mount=type=cache,id=discboeing-gobuildcache,target=/root/.cache/go-build \
    set -ex \
    && mkdir -p /tmp/gvbuild \
    && cd /tmp/gvbuild \
    && go mod init discboeing-gvbuild \
    && go get \
    "github.com/containers/gvisor-tap-vsock/cmd/vm@${GV_FORWARDER_VERSION}" \
    "github.com/containers/gvisor-tap-vsock/cmd/gvproxy@${GV_FORWARDER_VERSION}" \
    && CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" \
    go build -o /gvforwarder github.com/containers/gvisor-tap-vsock/cmd/vm \
    && CGO_ENABLED=0 GOOS=windows GOARCH="${TARGETARCH}" \
    go build -o /gvproxy.exe github.com/containers/gvisor-tap-vsock/cmd/gvproxy

# Agent API Go dependency cache
FROM golang:1.26 AS agent-go-deps

WORKDIR /build

# Copy shared module files first — needed by replace directives in
# agent-go/go.mod. replace ../modelsdev resolves to /modelsdev relative to
# WORKDIR /build; replace ../controlsocket resolves to /controlsocket.
COPY controlsocket/go.mod controlsocket/go.sum /controlsocket/
COPY modelsdev/go.mod /modelsdev/

# Copy module files first for better layer caching
COPY agent-go/go.mod agent-go/go.sum ./

# Download dependencies
RUN --mount=type=cache,id=discboeing-gomodcache,target=/go/pkg/mod \
    --mount=type=cache,id=discboeing-gobuildcache,target=/root/.cache/go-build \
    go mod download

# Agent API binary builder
FROM agent-go-deps AS agent-go-builder

# Copy modelsdev source (required for compilation, not just module resolution)
COPY controlsocket/ /controlsocket/
COPY modelsdev/ /modelsdev/

# Copy agent-go source
COPY agent-go/ ./

# Build the agent-go binary as discboeing-agent-api and the sudo policy gate.
# Use mcp_go_client_oauth build tag to enable OAuth support for MCP tools
RUN --mount=type=cache,id=discboeing-gomodcache,target=/go/pkg/mod \
    --mount=type=cache,id=discboeing-gobuildcache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -tags mcp_go_client_oauth -ldflags="-s -w" -o /discboeing-agent-api ./cmd/agent-api \
    && CGO_ENABLED=0 go build -ldflags="-s -w" -o /discboeing-sudo-gate ./cmd/sudo-gate

# Shared Ubuntu runtime base
FROM ubuntu:24.04 AS runtime-base

ARG UBUNTU_MIRROR
ARG UBUNTU_PORTS_MIRROR

COPY --chmod=755 container-assets/configure-ubuntu-mirrors.sh /usr/local/bin/configure-ubuntu-mirrors

# Label for image identification and cleanup
LABEL io.discboeing.sandbox-image=true

# Tell systemd it's running inside a container
ENV container=docker

# Install shared apt packages first for better layer caching
# Keep repo COPY steps in later stages so source changes do not invalidate this layer
# systemd + dbus: init system for managing services (PID 1)
# git is needed for workspace cloning
# socat is needed for vsock forwarding in VZ VMs
# nodejs is needed for JavaScript CLIs and pnpm
# pnpm is needed for package management
# direnv is needed for project-local environment activation
# xz-utils is needed to unpack the Nix binary tarball
# docker.io provides dockerd daemon and docker CLI (runs inside container with privileged mode)
# docker-buildx is needed for multi-arch builds and advanced build features
# docker-compose-v2 provides the Docker Compose v2 CLI plugin
# iptables and iproute2 are needed by dockerd and runtime diagnostics for network management
RUN configure-ubuntu-mirrors "${UBUNTU_MIRROR}" "${UBUNTU_PORTS_MIRROR}" \
    && apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    dbus \
    direnv \
    docker-buildx \
    docker-compose-v2 \
    docker.io \
    git \
    iproute2 \
    iptables \
    jq \
    less \
    libnss3-tools \
    openssh-client \
    openssh-sftp-server \
    psmisc \
    poppler-utils \
    ripgrep \
    shellcheck \
    python3 \
    python-is-python3 \
    python3-pip \
    python3-requests \
    python3-venv \
    socat \
    sqlite3 \
    sudo \
    systemd \
    systemd-sysv \
    unzip \
    vim \
    xz-utils \
    && curl -fsSL https://deb.nodesource.com/setup_25.x | bash - \
    && sed -i 's|http://|https://|g' /etc/apt/sources.list.d/nodesource.list 2>/dev/null || true \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends gh nodejs \
    # Install code-server
    && curl -fsSL https://code-server.dev/install.sh | sh \
    # Seed bundled code-server extensions
    && mkdir -p /opt/discboeing/code-server-defaults/extensions \
    && code-server --install-extension vscodevim.vim --extensions-dir /opt/discboeing/code-server-defaults/extensions \
    && code-server --install-extension golang.go --extensions-dir /opt/discboeing/code-server-defaults/extensions \
    && code-server --install-extension rust-lang.rust-analyzer --extensions-dir /opt/discboeing/code-server-defaults/extensions \
    && code-server --install-extension ms-python.python --extensions-dir /opt/discboeing/code-server-defaults/extensions \
    && code-server --install-extension svelte.svelte-vscode --extensions-dir /opt/discboeing/code-server-defaults/extensions \
    && rm -f /opt/discboeing/code-server-defaults/extensions/extensions.json \
    # Install pnpm
    && npm install -g pnpm \
    # Install latest stable Go
    && GO_VERSION=$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1) \
    && curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz" | tar -C /usr/local -xz \
    # Install uv (Python package installer) to /usr/local/bin
    && curl -LsSf https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/usr/local/bin sh \
    # Install Bun runtime to /usr/local
    && curl -fsSL https://bun.sh/install | BUN_INSTALL=/usr/local bash \
    && rm -rf /var/lib/apt/lists/* /root/.npm \
    # Disable Docker's apt auto-clean so downloaded .deb files persist in /var/cache/apt/archives.
    # This allows apt package downloads to be cached across sessions via cache volume mounts.
    # All image-time apt installs are already complete, so this only affects runtime installs.
    && rm -f /etc/apt/apt.conf.d/docker-clean

# Create discboeing user (UID 1000)
# Handle case where UID 1000 might already be taken by another user
RUN (useradd -m -s /bin/bash -u 1000 discboeing 2>/dev/null \
    || (userdel -r $(getent passwd 1000 | cut -d: -f1) 2>/dev/null; useradd -m -s /bin/bash -u 1000 discboeing) \
    || useradd -m -s /bin/bash discboeing) \
    && usermod -aG systemd-journal discboeing

# Install Nix in multi-user mode. The installer cannot start systemd during
# image builds, so copy the generated units explicitly. The installed /nix tree
# is staged outside /nix so a runtime /nix cache mount can be populated from it.
RUN curl -fsSL https://nixos.org/nix/install | sh -s -- --daemon --yes --no-channel-add \
    && install -D -m 0644 /nix/var/nix/profiles/default/lib/systemd/system/nix-daemon.service /etc/systemd/system/nix-daemon.service \
    && install -D -m 0644 /nix/var/nix/profiles/default/lib/systemd/system/nix-daemon.socket /etc/systemd/system/nix-daemon.socket \
    && install -D -m 0644 /nix/var/nix/profiles/default/lib/tmpfiles.d/nix-daemon.conf /etc/tmpfiles.d/nix-daemon.conf \
    && mv /nix /nix.staging \
    && mkdir -m 0755 /nix

RUN cat > /usr/local/bin/restore-nix-staging <<'EOF' \
    && chmod 755 /usr/local/bin/restore-nix-staging
#!/bin/sh
set -eu

if [ -x /nix/var/nix/profiles/default/bin/nix-daemon ] && [ -d /nix/store ]; then
    exit 0
fi

if [ ! -d /nix.staging ]; then
    echo "Nix staging directory not found at /nix.staging" >&2
    exit 1
fi

mkdir -p /nix

unexpected=$(find /nix -mindepth 1 \
    ! -path /nix/var \
    ! -path /nix/var/nix \
    ! -path /nix/var/nix/daemon-socket \
    ! -path '/nix/var/nix/daemon-socket/*' \
    ! -path /nix/var/nix/builds \
    ! -path '/nix/var/nix/builds/*' \
    -print -quit)
if [ -n "$unexpected" ]; then
    echo "/nix is not empty and does not contain a valid Nix installation" >&2
    echo "unexpected path: $unexpected" >&2
    exit 1
fi

echo "Restoring Nix installation from /nix.staging to /nix"
(cd /nix.staging && tar cpf - .) | (cd /nix && tar xpf -)
EOF

# Install the Discboeing sudo gate. The real sudo binary is kept in a
# root-only path; /usr/bin/sudo becomes a setuid Discboeing gate that calls the
# local agent API before exec'ing the real sudo binary.
COPY --from=agent-go-builder --chmod=4755 /discboeing-sudo-gate /tmp/discboeing-sudo-gate
RUN mkdir -p /usr/lib/discboeing /etc/discboeing \
    && dpkg-divert --rename --add /usr/bin/sudo \
    && mv /usr/bin/sudo.distrib /usr/lib/discboeing/sudo.real \
    && chown root:root /usr/lib/discboeing/sudo.real \
    && install -m 4755 /tmp/discboeing-sudo-gate /usr/bin/sudo \
    && rm -f /tmp/discboeing-sudo-gate \
    && chmod 4700 /usr/lib/discboeing/sudo.real \
    && printf '%s\n' '{"realSudo":"/usr/lib/discboeing/sudo.real","agentAPIURL":"http://127.0.0.1:3002/sudo/authorize"}' > /etc/discboeing/sudo-gate.json \
    && chown root:root /etc/discboeing/sudo-gate.json \
    && chmod 400 /etc/discboeing/sudo-gate.json \
    && printf '%s\n' \
        'Defaults env_keep += "DISCBOEING_SUDO_RUNTIME DISCBOEING_SUDO_TOKEN DISCBOEING_SUDO_CREDENTIAL_ID DISCBOEING_SUDO_USE_ID DISCBOEING_SUDO_TOOL_CALL_ID DISCBOEING_SUDO_COMMAND DISCBOEING_SECRET"' \
        'discboeing ALL=(ALL) NOPASSWD:SETENV: ALL' \
        > /etc/sudoers.d/discboeing-gated \
    && chmod 440 /etc/sudoers.d/discboeing-gated

# Install rustup for discboeing user (Rust toolchain manager)
# Must be done after user creation so rust tools are owned by discboeing
# Install rustup without any toolchains (users can install toolchains on demand with rustup install)
RUN su - discboeing -c 'curl --proto "=https" --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain none'

# Configure npm global directory in /home/discboeing/.npm-global
# This allows npm install -g to work without root for the discboeing user
# Also add ~/.local/bin so uv-installed Python executables are on PATH by default
# Environment is set system-wide via /etc/profile.d so both root and discboeing can use it
RUN mkdir -p /home/discboeing/.npm-global/bin /home/discboeing/.local/bin \
    && chown -R discboeing:discboeing /home/discboeing/.npm-global /home/discboeing/.local \
    && printf '%s\n' \
    '# User-local executables and npm global packages' \
    'export NPM_CONFIG_PREFIX="/home/discboeing/.npm-global"' \
    'export PATH="/home/discboeing/.local/bin:/home/discboeing/.npm-global/bin:$PATH"' \
    > /etc/profile.d/npm-global.sh \
    && chmod 644 /etc/profile.d/npm-global.sh


# Create directory structure per filesystem design
# /.data      - persistent storage (Docker volume or VZ disk)
# /.workspace - base workspace (read-only)
RUN mkdir -p /.data /.workspace /opt/discboeing/bin /opt/discboeing/scripts \
    && chown discboeing:discboeing /.data /opt/discboeing/scripts

# Add discboeing binaries, Nix, user-local bin, and npm global bin to PATH
# Also set NPM_CONFIG_PREFIX for non-login shell contexts
# Set PNPM_HOME to use persistent storage for pnpm cache/store
# Add Rust cargo bin for rustc and cargo
# Claude CLI is installed to /usr/local/bin (already in default PATH)
ENV NPM_CONFIG_PREFIX="/home/discboeing/.npm-global"
ENV PNPM_HOME="/.data/pnpm"
ENV NIX_REMOTE=daemon
ENV PATH="/home/discboeing/.cargo/bin:/nix/var/nix/profiles/default/bin:/usr/local/go/bin:/home/discboeing/.local/bin:/home/discboeing/.npm-global/bin:/opt/discboeing/bin:${PATH}"
ENV WORKSPACE_PATH=/home/discboeing/workspace

WORKDIR /home/discboeing

EXPOSE 3002

# systemd as PID 1 — manages discboeing services (setup, proxy, dockerd, agent-api)
# SIGRTMIN+3 tells systemd to shut down cleanly (used by docker stop)
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]

# Shared graphical runtime base
FROM runtime-base AS runtime-gui-base

# Install graphical packages: virtual X11 display, VNC, window manager, browser.
# Ubuntu's chromium package is a Snap stub and the Launchpad-hosted Chromium
# PPAs are slow/unreliable. Debian publishes real chromium .deb packages for
# both amd64 and arm64, so use Debian only for chromium and keep Ubuntu as the
# source for all other packages.
RUN apt-get update && apt-get install -y --no-install-recommends \
    debian-archive-keyring \
    menu \
    openbox \
    pcmanfm \
    python3-xdg \
    python3-websockify \
    scrot \
    x11vnc \
    x11-xserver-utils \
    xdotool \
    xserver-xorg-core \
    xserver-xorg-video-dummy \
    xterm \
    xvfb \
    && printf '%s\n' \
    'Types: deb' \
    'URIs: https://deb.debian.org/debian' \
    'Suites: bookworm' \
    'Components: main' \
    'Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg' \
    '' \
    'Types: deb' \
    'URIs: https://security.debian.org/debian-security' \
    'Suites: bookworm-security' \
    'Components: main' \
    'Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg' \
    > /etc/apt/sources.list.d/debian-chromium.sources \
    && printf '%s\n' \
    'Package: *' \
    'Pin: release n=bookworm' \
    'Pin-Priority: 100' \
    '' \
    'Package: *' \
    'Pin: release n=bookworm-security' \
    'Pin-Priority: 100' \
    '' \
    'Package: chromium chromium-common chromium-sandbox' \
    'Pin: release n=bookworm' \
    'Pin-Priority: 990' \
    '' \
    'Package: chromium chromium-common chromium-sandbox' \
    'Pin: release n=bookworm-security' \
    'Pin-Priority: 990' \
    > /etc/apt/preferences.d/debian-chromium \
    && apt-get update \
    && apt-get install -y --no-install-recommends chromium \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb

# Configure Openbox to autostart PCManFM in desktop mode (renders desktop icons)
# Configure libfm to launch executable .desktop files without the "Execute File" prompt
RUN mkdir -p /home/discboeing/.config/openbox /home/discboeing/.config/libfm \
    && printf '%s\n' \
    '# Launch PCManFM in desktop mode to render desktop icons' \
    'pcmanfm --desktop &' \
    > /home/discboeing/.config/openbox/autostart \
    && printf '%s\n' \
    '[config]' \
    'single_click=0' \
    'use_trash=1' \
    'confirm_del=1' \
    'confirm_trash=1' \
    'quick_exec=1' \
    > /home/discboeing/.config/libfm/libfm.conf \
    && chown -R discboeing:discboeing /home/discboeing/.config

# Create desktop shortcuts for Chromium and XTerm
RUN mkdir -p /home/discboeing/Desktop \
    && printf '%s\n' \
    '[Desktop Entry]' \
    'Type=Application' \
    'Name=Chromium' \
    'Exec=chromium' \
    'Icon=chromium' \
    'Terminal=false' \
    'Categories=Network;WebBrowser;' \
    > /home/discboeing/Desktop/chromium.desktop \
    && printf '%s\n' \
    '[Desktop Entry]' \
    'Type=Application' \
    'Name=XTerm' \
    'Exec=xterm' \
    'Icon=xterm-color' \
    'Terminal=false' \
    'Categories=System;TerminalEmulator;' \
    > /home/discboeing/Desktop/xterm.desktop \
    && chmod 755 /home/discboeing/Desktop/*.desktop \
    && chown -R discboeing:discboeing /home/discboeing/Desktop

ENV DISPLAY=:0

# Desktop access is served through the localhost-bound websockify proxy socket.
EXPOSE 6080

# Browser harness package builder
FROM runtime-base AS browser-harness-builder

ARG BROWSER_HARNESS_REPO=https://github.com/browser-use/browser-harness.git
ARG BROWSER_HARNESS_REF=main

RUN git clone --depth 1 --branch "${BROWSER_HARNESS_REF}" "${BROWSER_HARNESS_REPO}" /tmp/browser-harness \
    && mkdir -p /opt/browser-harness-skills/browser-harness \
    && cp -a /tmp/browser-harness/. /opt/browser-harness-skills/browser-harness/ \
    && uv venv /opt/browser-harness \
    && uv pip install --python /opt/browser-harness/bin/python /tmp/browser-harness \
    && ln -s /opt/browser-harness/bin/browser-harness /usr/local/bin/browser-harness \
    && rm -rf /tmp/browser-harness /root/.cache/uv

# Runtime overlay with frequently-changing binaries and container assets
FROM scratch AS runtime-overlay

# Copy binaries to /opt/discboeing/bin
COPY --from=agent-go-builder --chmod=755 /discboeing-agent-api /opt/discboeing/bin/discboeing-agent-api
COPY --from=proxy-builder --chmod=755 /proxy /opt/discboeing/bin/proxy
COPY --chmod=755 sandbox-init/discboeing-sandbox-init.sh /opt/discboeing/bin/discboeing-sandbox-init
COPY --from=vsock-port-proxy-builder --chmod=755 /discboeing-vsock-port-proxy /opt/discboeing/bin/discboeing-vsock-port-proxy

# Copy browser-harness runtime and expose it at /usr/local/bin/browser-harness
COPY --from=browser-harness-builder /opt/browser-harness /opt/browser-harness
COPY --from=browser-harness-builder /usr/local/bin/browser-harness /usr/local/bin/browser-harness
COPY --from=browser-harness-builder /opt/browser-harness-skills/ /opt/discboeing/skills/
COPY container-assets/discboeing/skills/ /opt/discboeing/skills/

# Docker wrapper: injects --output type=docker for build commands so remote
# buildx builders always load images into the local daemon.
COPY --chmod=755 container-assets/docker-wrapper.sh /usr/local/bin/docker
COPY --chmod=755 container-assets/discboeing-session-env.sh /usr/local/bin/discboeing-session-env
COPY --chmod=755 container-assets/discboeing-vnc-websockify /usr/local/bin/discboeing-vnc-websockify

# Copy systemd service files and setup helper for container service management
COPY --chmod=755 container-assets/configure-container-systemd.sh /opt/discboeing/bin/configure-container-systemd
COPY container-assets/systemd/ /etc/systemd/system/
COPY container-assets/xorg-dummy.conf /etc/X11/xorg-dummy.conf

# Copy code-server default profile templates
COPY --chown=1000:1000 container-assets/code-server/ /opt/discboeing/code-server-defaults/

# Copy container-specific Discboeing docs.
COPY --chown=1000:1000 container-assets/docs.txt /discboeing/docs.txt

# Minimal runtime without graphical tools
FROM runtime-base AS runtime-shell

COPY --from=runtime-overlay / /

RUN ! grep -R /opt/discboeing/bin/restore-nix-staging /etc/systemd/system \
    && grep -R /usr/local/bin/restore-nix-staging /etc/systemd/system

# Configure systemd for container environment
# Disable docker.service so it only starts via docker.socket activation
# (the Ubuntu docker.io package preset enables it by default)
RUN configure-container-systemd shell

# Full runtime with graphical desktop tools (X11, VNC, browser)
FROM runtime-gui-base AS runtime

COPY --from=runtime-overlay / /

RUN ! grep -R /opt/discboeing/bin/restore-nix-staging /etc/systemd/system \
    && grep -R /usr/local/bin/restore-nix-staging /etc/systemd/system

# Configure systemd for container environment
# Disable docker.service so it only starts via docker.socket activation
# (the Ubuntu docker.io package preset enables it by default)
RUN configure-container-systemd gui

# VZ/WSL root filesystem builder with systemd and Docker
# Build with: docker build --target vz-image -t discboeing-vz .
# Then extract /vmlinuz and /discboeing-rootfs.squashfs with docker cp from a
# temporary container. The watcher uses this flow so local Windows/WSL builds
# do not rely on docker build --output extraction.
# This creates a minimal systemd-based system with Docker daemon for macOS Virtualization.framework
# This stage is completely independent from the runtime image
FROM ubuntu:24.04 AS vz-rootfs-builder

ARG UBUNTU_MIRROR
ARG UBUNTU_PORTS_MIRROR

COPY --chmod=755 container-assets/configure-ubuntu-mirrors.sh /usr/local/bin/configure-ubuntu-mirrors

# Docker image to preload into the VM at build time (pulled via crane as OCI tarball)
# Defaults to the main tag of the discboeing runtime image
ARG PRELOAD_IMAGE=ghcr.io/boeing-platform/discboeing:main

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install kernel, systemd, Docker, and minimal tools
# Use a specific stable kernel version with virtio drivers built-in
RUN configure-ubuntu-mirrors "${UBUNTU_MIRROR}" "${UBUNTU_PORTS_MIRROR}" \
    && apt-get update && apt-get install -y --no-install-recommends \
    # Kernel with virtio support built-in (no modules needed)
    # Using specific version to avoid metapackage dependency issues
    linux-image-6.8.0-31-generic \
    linux-modules-6.8.0-31-generic \
    # systemd as init system with network support
    systemd \
    systemd-sysv \
    systemd-resolved \
    systemd-timesyncd \
    # Docker daemon and dependencies
    docker.io \
    iptables \
    isc-dhcp-client \
    # Minimal essential tools
    ca-certificates \
    curl \
    socat \
    # e2fsprogs for mkfs.ext4 to format data disk
    e2fsprogs \
    # udev for device enumeration
    udev \
    && rm -rf /var/lib/apt/lists/*

# Pull the preload image as an OCI tarball using crane
# crane is a standalone tool from go-containerregistry that doesn't need Docker daemon
# TARGETARCH is automatically set by Docker buildx (amd64 or arm64)
ARG TARGETARCH
RUN set -ex \
    # Install crane from go-containerregistry releases with checksum verification
    && CRANE_VERSION="v0.20.7" \
    # Map Docker TARGETARCH to crane release filename arch
    && if [ "${TARGETARCH}" = "amd64" ]; then \
    CRANE_ARCH="x86_64"; \
    CRANE_SHA256="8ef3564d264e6b5ca93f7b7f5652704c4dd29d33935aff6947dd5adefd05953e"; \
    else \
    CRANE_ARCH="${TARGETARCH}"; \
    CRANE_SHA256="b04ee6e4904d9219c76383f5b73521a63f69ecc93c0b1840846eebfd071a6355"; \
    fi \
    && curl -fsSL -o /tmp/crane.tar.gz \
    "https://github.com/google/go-containerregistry/releases/download/${CRANE_VERSION}/go-containerregistry_Linux_${CRANE_ARCH}.tar.gz" \
    && echo "${CRANE_SHA256}  /tmp/crane.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/crane.tar.gz -C /usr/local/bin crane \
    && chmod +x /usr/local/bin/crane \
    && rm -f /tmp/crane.tar.gz \
    # Pull the image as an OCI tarball for the target architecture
    && echo "Pulling ${PRELOAD_IMAGE} for linux/${TARGETARCH}..." \
    && crane pull --platform "linux/${TARGETARCH}" "${PRELOAD_IMAGE}" /preload-image.tar \
    && echo "Preload image saved to /preload-image.tar" \
    # Save the image reference for the boot-time load script
    && echo "${PRELOAD_IMAGE}" > /preload-image-tag \
    # Clean up crane binary (not needed at runtime)
    && rm -f /usr/local/bin/crane

# Create /var skeleton for first-boot initialization
# This is copied to /var after the data disk is mounted
RUN cp -a /var /var.skel

# Copy shared guest assets (systemd units, scripts, network config, fstab, WSL config)
COPY vm-assets/fstab /etc/fstab
COPY vm-assets/wsl/wsl.conf /etc/wsl.conf
COPY --from=gvforwarder-builder /gvforwarder /usr/local/bin/gvforwarder
COPY vm-assets/systemd/docker-vsock-proxy.service /etc/systemd/system/
COPY vm-assets/systemd/gvforwarder.service /etc/systemd/system/
COPY vm-assets/systemd/init-var.service /etc/systemd/system/
COPY vm-assets/systemd/mount-home.service /etc/systemd/system/
COPY vm-assets/systemd/preload-image.service /etc/systemd/system/
COPY vm-assets/systemd/docker.service.d/ /etc/systemd/system/docker.service.d/
COPY vm-assets/systemd/containerd.service.d/ /etc/systemd/system/containerd.service.d/
COPY vm-assets/systemd/systemd-networkd.service.d/ /etc/systemd/system/systemd-networkd.service.d/
COPY vm-assets/systemd/systemd-networkd-wait-online.service.d/ /etc/systemd/system/systemd-networkd-wait-online.service.d/
COPY vm-assets/systemd/systemd-timesyncd.service.d/ /etc/systemd/system/systemd-timesyncd.service.d/
COPY vm-assets/systemd/systemd-resolved.service.d/ /etc/systemd/system/systemd-resolved.service.d/
COPY vm-assets/network/20-dhcp.network /etc/systemd/network/
COPY --chmod=755 vm-assets/scripts/check-wsl-role.sh /usr/local/bin/
COPY --chmod=755 vm-assets/scripts/init-var.sh /usr/local/bin/
COPY --chmod=755 vm-assets/scripts/mount-home.sh /usr/local/bin/
COPY --chmod=755 vm-assets/scripts/preload-image.sh /usr/local/bin/

# Configure systemd for VM environment
RUN set -ex \
    # Disable unnecessary systemd services.
    && systemctl mask \
    getty@.service \
    serial-getty@.service \
    # Enable the network stack in the shared image; WSL role-aware drop-ins
    # skip these units there while VZ keeps using them.
    && systemctl enable \
    systemd-networkd \
    systemd-resolved \
    systemd-timesyncd \
    fstrim.timer \
    # Enable /var initialization and home mount services
    && systemctl enable init-var.service \
    && systemctl enable mount-home.service \
    # Enable Docker service, vsock proxy, and preloaded image loader
    && systemctl enable docker \
    && systemctl enable gvforwarder \
    && systemctl enable docker-vsock-proxy \
    && systemctl enable preload-image

# Create discboeing user (UID 1000)
RUN useradd -m -s /bin/bash -u 1000 discboeing || \
    (userdel -r $(getent passwd 1000 | cut -d: -f1) 2>/dev/null; useradd -m -s /bin/bash -u 1000 discboeing)

# Create minimal directory structure for VM
# /Users is for macOS host home directory VirtioFS mounts (root is read-only squashfs)
RUN mkdir -p /.data /.workspace /Users \
    && chown discboeing:discboeing /.data

# VZ/WSL image artifact builder
FROM ubuntu:24.04 AS vz-image-builder

ARG UBUNTU_MIRROR
ARG UBUNTU_PORTS_MIRROR

COPY --chmod=755 container-assets/configure-ubuntu-mirrors.sh /usr/local/bin/configure-ubuntu-mirrors

# Install tools for image creation and kernel extraction
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && configure-ubuntu-mirrors "${UBUNTU_MIRROR}" "${UBUNTU_PORTS_MIRROR}" \
    && apt-get update && apt-get install -y --no-install-recommends \
    squashfs-tools \
    zstd \
    && rm -rf /var/lib/apt/lists/*

# Copy the rootfs from builder
COPY --from=vz-rootfs-builder / /rootfs

# Extract kernel from /rootfs/boot (no initrd needed)
RUN set -ex \
    && cd /rootfs/boot \
    # Find the kernel (vmlinuz-*)
    && KERNEL=$(ls -1 vmlinuz-* | head -1) \
    && KERNEL_VERSION=$(echo $KERNEL | sed 's/vmlinuz-//') \
    && echo "Found kernel: $KERNEL (version: $KERNEL_VERSION)" \
    # Copy kernel to root for extraction
    && cp "$KERNEL" /vmlinuz \
    # Save kernel version
    && echo "$KERNEL_VERSION" > /kernel-version

# Prepare rootfs for VM use
RUN set -ex \
    # Create essential mount points
    && mkdir -p /rootfs/proc /rootfs/sys /rootfs/dev /rootfs/run /rootfs/tmp \
    # VZ uses systemd-resolved's stub listener here. WSL rewrites
    # /etc/resolv.conf to /mnt/wsl/resolv.conf at runtime.
    && rm -f /rootfs/etc/resolv.conf \
    && ln -s /run/systemd/resolve/stub-resolv.conf /rootfs/etc/resolv.conf \
    # Clean up /boot to save space (kernel/initrd already extracted)
    && rm -rf /rootfs/boot/*

# Create SquashFS image with zstd compression for Apple VZ and a tar.zst
# archive for managed WSL imports.
# SquashFS is built into the kernel - no initrd needed!
# Boot with: root=/dev/vda rootfstype=squashfs ro
RUN set -ex \
    && ROOTFS_SIZE_MB=$(du -sm /rootfs | cut -f1) \
    && echo "Rootfs size: ${ROOTFS_SIZE_MB}MB" \
    && echo "Creating SquashFS image with zstd compression..." \
    && mksquashfs /rootfs /rootfs.squashfs \
    -comp zstd \
    -Xcompression-level 19 \
    -noappend \
    && SQUASHFS_SIZE_MB=$(du -m /rootfs.squashfs | cut -f1) \
    && RATIO=$((100 - (SQUASHFS_SIZE_MB * 100 / ROOTFS_SIZE_MB))) \
    && echo "SquashFS image: ${SQUASHFS_SIZE_MB}MB (${RATIO}% reduction)" \
    && echo "Creating WSL rootfs archive with zstd compression..." \
    && tar --numeric-owner -C /rootfs -cf - . | zstd -T0 -19 -o /discboeing-rootfs.tar.zst \
    && ROOTFS_TAR_SIZE_MB=$(du -m /discboeing-rootfs.tar.zst | cut -f1) \
    && TAR_RATIO=$((100 - (ROOTFS_TAR_SIZE_MB * 100 / ROOTFS_SIZE_MB))) \
    && echo "WSL rootfs archive: ${ROOTFS_TAR_SIZE_MB}MB (${TAR_RATIO}% reduction)"

# VZ output with kernel and SquashFS root filesystem
# This target is published as the macOS VZ guest image.
FROM scratch AS vz-image
COPY --from=vz-image-builder /vmlinuz /vmlinuz
COPY --from=vz-image-builder /kernel-version /kernel-version
COPY --from=vz-image-builder /rootfs.squashfs /discboeing-rootfs.squashfs

# WSL output with rootfs archive
# This target is published as the Windows WSL guest image.
FROM scratch AS wsl-image
COPY --from=vz-image-builder /discboeing-rootfs.tar.zst /discboeing-rootfs.tar.zst

# Build the Microsoft WSL2 kernel from the release source ref selected by CI.
# The GitHub releases currently publish source archives rather than prebuilt
# kernels, so the HCS guest artifact image builds the kernel for each target
# platform.
FROM ubuntu:24.04 AS wsl-kernel-builder

ARG UBUNTU_MIRROR
ARG UBUNTU_PORTS_MIRROR
ARG TARGETARCH
ARG WSL_KERNEL_REF=linux-msft-wsl-6.18.26.3

COPY --chmod=755 container-assets/configure-ubuntu-mirrors.sh /usr/local/bin/configure-ubuntu-mirrors

RUN configure-ubuntu-mirrors "${UBUNTU_MIRROR}" "${UBUNTU_PORTS_MIRROR}" \
    && apt-get update && apt-get install -y --no-install-recommends \
    bc \
    bison \
    build-essential \
    ca-certificates \
    curl \
    dwarves \
    flex \
    git \
    libelf-dev \
    libssl-dev \
    python3 \
    && rm -rf /var/lib/apt/lists/*

RUN --mount=type=cache,id=discboeing-wsl-kernel-git,target=/root/.cache/git \
    set -ex \
    && git clone --depth 1 --branch "${WSL_KERNEL_REF}" https://github.com/microsoft/WSL2-Linux-Kernel.git /kernel \
    && cd /kernel \
    && if [ "${TARGETARCH}" = "arm64" ]; then KERNEL_ARCH="arm64"; KERNEL_IMAGE="arch/arm64/boot/Image"; else KERNEL_ARCH="x86"; KERNEL_IMAGE="arch/x86/boot/bzImage"; fi \
    && make ARCH="${KERNEL_ARCH}" KCONFIG_CONFIG=Microsoft/config-wsl olddefconfig \
    && make -j"$(nproc)" ARCH="${KERNEL_ARCH}" KCONFIG_CONFIG=Microsoft/config-wsl \
    && cp "${KERNEL_IMAGE}" /wsl-kernel \
    && make -s ARCH="${KERNEL_ARCH}" KCONFIG_CONFIG=Microsoft/config-wsl kernelrelease > /kernel-version \
    && echo "${WSL_KERNEL_REF}" > /wsl-kernel-ref

# Build the Windows HCS launcher binary.
FROM mcr.microsoft.com/dotnet/sdk:8.0 AS hcs-launcher-builder

ARG TARGETARCH

WORKDIR /src/hcs
COPY hcs/ ./

RUN set -ex \
    && if [ "${TARGETARCH}" = "arm64" ]; then RID="win-arm64"; else RID="win-x64"; fi \
    && dotnet publish HcsLinuxVmLauncher.csproj \
    --configuration Release \
    --runtime "${RID}" \
    --self-contained true \
    -p:PublishSingleFile=true \
    -p:PublishTrimmed=false \
    -o /out \
    && cp /out/HcsLinuxVmLauncher.exe /HcsLinuxVmLauncher.exe

# Convert the shared SquashFS root filesystem into a fixed VHD. HCS virtual
# disk attachments require VHD/VHDX inputs; the guest still mounts the SquashFS
# image at byte zero with root=/dev/sda rootfstype=squashfs.
FROM ubuntu:24.04 AS hcs-image-builder

RUN apt-get update && apt-get install -y --no-install-recommends python3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=vz-image-builder /rootfs.squashfs /discboeing-rootfs.squashfs

RUN python3 - <<'PY'
import hashlib
import os
import struct
import uuid

raw_path = "/discboeing-rootfs.squashfs"
vhd_path = "/discboeing-rootfs.vhd"
sector = 512

raw = open(raw_path, "rb").read()
virtual_size = ((len(raw) + sector - 1) // sector) * sector
padded = raw + b"\0" * (virtual_size - len(raw))

def chs(size):
    total_sectors = size // sector
    if total_sectors > 65535 * 16 * 255:
        total_sectors = 65535 * 16 * 255
    if total_sectors >= 65535 * 16 * 63:
        sectors = 255
        heads = 16
        cylinders = total_sectors // (heads * sectors)
    else:
        sectors = 17
        cylinders = total_sectors // sectors
        heads = (cylinders + 1023) // 1024
        if heads < 4:
            heads = 4
        if cylinders >= 1024 or heads > 16:
            sectors = 31
            heads = 16
            cylinders = total_sectors // (heads * sectors)
        if cylinders >= 1024:
            sectors = 63
            heads = 16
            cylinders = total_sectors // (heads * sectors)
    return min(cylinders, 65535), heads, sectors

cylinders, heads, sectors = chs(virtual_size)
disk_id = bytearray(hashlib.sha256(padded).digest()[:16])
disk_id[6] = (disk_id[6] & 0x0F) | 0x40
disk_id[8] = (disk_id[8] & 0x3F) | 0x80

footer = bytearray(512)
struct.pack_into(">8sIIQI4sI4sQQHBBI16sB427s", footer, 0,
    b"conectix",      # cookie
    0x00000002,       # features: no features enabled
    0x00010000,       # file format version
    0xFFFFFFFFFFFFFFFF, # data offset for fixed disks
    int(os.environ.get("SOURCE_DATE_EPOCH", "946684800")) - 946684800,
    b"dcbo",          # creator application
    0x00010000,       # creator version
    b"Wi2k",          # creator host OS
    virtual_size,
    virtual_size,
    cylinders,
    heads,
    sectors,
    2,                # fixed hard disk
    bytes(disk_id),
    0,                # saved state
    b"\0" * 427,
)
struct.pack_into(">I", footer, 64, 0)
checksum = (~sum(footer) & 0xFFFFFFFF)
struct.pack_into(">I", footer, 64, checksum)

with open(vhd_path, "wb") as out:
    out.write(padded)
    out.write(footer)

print(f"Created fixed VHD {vhd_path}: raw={len(raw)} padded={virtual_size} uuid={uuid.UUID(bytes=bytes(disk_id))}")
PY

# HCS output with root VHD, WSL2 kernel, host launcher, host gvproxy, and guest
# gvforwarder. This target is published as the Windows HCS guest image.
FROM scratch AS hcs-image
COPY --from=hcs-image-builder /discboeing-rootfs.vhd /discboeing-rootfs.vhd
COPY --from=wsl-kernel-builder /wsl-kernel /wsl-kernel
COPY --from=wsl-kernel-builder /kernel-version /kernel-version
COPY --from=wsl-kernel-builder /wsl-kernel-ref /wsl-kernel-ref
COPY --from=hcs-launcher-builder /HcsLinuxVmLauncher.exe /HcsLinuxVmLauncher.exe
COPY --from=gvforwarder-builder /gvproxy.exe /gvproxy.exe
COPY --from=gvforwarder-builder /gvforwarder /gvforwarder

# Default target: runtime image
FROM runtime
