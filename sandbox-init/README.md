# Discboeing Sandbox Init

`discboeing-sandbox-init` is a small Bash setup script that runs as a systemd
oneshot-style service inside Discboeing sandbox containers. Systemd is PID 1; the
script prepares the filesystem and runtime environment, sends `READY=1`, and
exits.

## Responsibilities

- Copy `/home/discboeing` into persistent storage at `/.data/discboeing` on first
  start, then sync only new image-provided files on later starts.
- Mount an OverlayFS view of `/.data/discboeing` at `/home/discboeing` using the
  current `DISCBOEING_SESSION_ID` as the writable layer key.
- Ensure `/home/discboeing/workspace` exists and is owned by the sandbox user.
- Mount configured cache directories from `/.data/cache` when a cache volume is
  present.
- Write the built-in proxy configuration to `/.data/proxy/config.yaml`, run
  proxy certificate setup, and generate environment files for dependent systemd
  services.
- Write Docker daemon configuration with an MTU derived from `eth0`.
- Configure Git safe directories and localhost/proxy shell defaults.

## Usage

The systemd unit invokes the script as:

```bash
/opt/discboeing/bin/discboeing-sandbox-init setup
```

Required environment:

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DISCBOEING_SESSION_ID` | Yes | - | Session identifier used for the OverlayFS writable layer. |
| `AGENT_USER` | No | `discboeing` | User that owns the sandbox home and agent runtime. |
| `CACHE_ENABLED` | No | `true` | Set to `false` to skip cache bind mounts. |

## Filesystem Layout

```text
/.data/
├── discboeing/                 # Persistent lower home copied from image
├── .overlayfs/{sessionID}/   # OverlayFS upper/work dirs per session
├── cache/                    # Optional project cache volume
├── docker/                   # Docker daemon data root
└── proxy/                    # Proxy config, certs, and recordings
```

`/tmp` is handled separately by `tmp.mount` and
`discboeing-persistent-tmp-prepare.service` before this setup service runs.

## Building

The Docker build copies the script directly into the runtime overlay:

```dockerfile
COPY --chmod=755 sandbox-init/discboeing-sandbox-init.sh \
  /opt/discboeing/bin/discboeing-sandbox-init
```

There is no separate Go binary for sandbox init.
