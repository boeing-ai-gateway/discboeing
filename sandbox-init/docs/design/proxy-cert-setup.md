# Proxy CA Certificate Setup

During sandbox startup, `discobot-sandbox-init` delegates CA generation and trust
installation to the proxy binary:

```bash
/opt/discobot/bin/proxy init-certs -config /.data/proxy/config.yaml -user discobot
```

The proxy owns certificate generation and installs trust for the system store and
the runtime user's NSS database. Sandbox init only ensures the proxy config is
written first and logs a warning if certificate setup fails.

The generated CA lives under `/.data/proxy/certs`, matching the built-in proxy
configuration written by the setup script.
