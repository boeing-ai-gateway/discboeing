# Init Process Design

Discboeing sandbox containers now use systemd as PID 1. Sandbox initialization is a
Bash setup script installed at `/opt/discboeing/bin/discboeing-sandbox-init` and run
by `discboeing-sandbox-init.service`.

## Responsibilities

The script is intentionally limited to setup work:

- copy or refresh the persistent base home at `/.data/discboeing`
- mount OverlayFS over `/home/discboeing`
- create `/home/discboeing/workspace`
- mount cache directories from `/.data/cache`
- write proxy config and initialize proxy CA certificates
- write proxy and agent environment files under `/run/discboeing`
- write Docker daemon configuration
- notify systemd readiness and exit

Systemd owns process supervision, shutdown, and service ordering. The agent API,
proxy, Docker, VS Code, and desktop services run as separate units.

## Service Contract

```ini
Type=notify
NotifyAccess=all
RemainAfterExit=yes
ExecStart=/opt/discboeing/bin/discboeing-sandbox-init setup
```

`DISCBOEING_SESSION_ID` is required. `AGENT_USER` defaults to `discboeing`.
