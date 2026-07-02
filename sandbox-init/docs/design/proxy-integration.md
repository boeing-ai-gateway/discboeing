# Proxy Integration

The sandbox setup script prepares proxy configuration and environment before
`discboeing-proxy.service`, Docker, and `discboeing-agent-api.service` start.

## Startup Flow

1. `discboeing-sandbox-init.service` runs
   `/opt/discboeing/bin/discboeing-sandbox-init setup`.
2. The script writes the built-in proxy config to `/.data/proxy/config.yaml`.
3. The script runs `proxy init-certs` to create and trust the sandbox CA.
4. The script writes `/run/discboeing/proxy-env` and `/run/discboeing/agent-env`.
5. The setup service notifies readiness and exits.
6. Systemd starts the proxy, Docker, and agent API services according to unit
   ordering.

## Environment

The generated environment sets `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`,
lowercase variants, `NO_PROXY`, `NODE_EXTRA_CA_CERTS`, and `UV_SYSTEM_CERTS=1`.

## Configuration Source

Sandbox init always writes built-in proxy defaults. It does not read proxy
configuration from the workspace during startup, avoiding trust in untrusted
workspace files before the sandbox is fully initialized.
