# Proxy Implementation Summary

Sandbox init writes a built-in proxy configuration to
`/.data/proxy/config.yaml` during startup. The config enables Docker registry
caching by default and stores proxy data under `/.data/proxy` and
`/.data/cache/proxy`.

The setup script then runs:

```bash
/opt/discboeing/bin/proxy init-certs -config /.data/proxy/config.yaml -user "$AGENT_USER"
```

It also writes proxy environment variables to:

- `/run/discboeing/proxy-env` for services such as Docker
- `/run/discboeing/agent-env` for `discboeing-agent-api`
- `/etc/profile.d/discboeing-proxy.sh` for login shells

The proxy daemon itself is managed by `discboeing-proxy.service`; sandbox init does
not keep the proxy process alive.
