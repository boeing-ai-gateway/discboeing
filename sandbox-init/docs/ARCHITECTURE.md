# Sandbox Init Architecture

`sandbox-init/discboeing-sandbox-init.sh` is the sandbox setup entrypoint copied to
`/opt/discboeing/bin/discboeing-sandbox-init` in the runtime image. It is a Bash
script run by `discboeing-sandbox-init.service`; systemd remains PID 1 and owns
process supervision for proxy, Docker, agent API, VS Code, and desktop services.

## Startup Contract

The service calls:

```bash
/opt/discboeing/bin/discboeing-sandbox-init setup
```

The script requires `DISCBOEING_SESSION_ID` and defaults `AGENT_USER` to
`discboeing`. It writes `/run/discboeing/proxy-env` and
`/run/discboeing/agent-env` for dependent services, then notifies systemd with
`READY=1` before exiting.

## Setup Flow

1. Normalize `/etc/hosts` so `localhost` resolves consistently to IPv4.
2. Enable TCP MTU probing for nested Docker environments.
3. Configure Git `safe.directory` entries for image, persistent, staging, and
   mounted workspace paths.
4. Create or refresh the persistent base home at `/.data/discboeing`.
5. Remove obsolete bundled Discboeing command/skill files from the persistent
   home.
6. Create OverlayFS `upper` and `work` directories under
   `/.data/.overlayfs/$DISCBOEING_SESSION_ID` and mount the overlay at
   `/home/discboeing`.
7. Ensure `/home/discboeing/workspace` exists and has sandbox-user ownership.
8. Bind-mount cache paths from `/.data/cache` unless disabled.
9. Write the default proxy config, initialize proxy CA certificates, and write
   proxy shell/systemd environment.
10. Write Docker daemon config, including derived MTU and containerd snapshotter
    enablement.
11. Remove stale Docker buildx default-builder pointers.
12. Write the agent API environment and notify readiness.

## Filesystem Model

```text
/.data/discboeing                    persistent lower home
/.data/.overlayfs/<session>/upper  session writable layer
/.data/.overlayfs/<session>/work   OverlayFS workdir
/home/discboeing                     mounted OverlayFS view
/.data/cache                       optional bind-mount source for caches
/.data/proxy                       proxy config, certs, and recordings
```

The lower home survives image upgrades. New image-provided files are copied into
it with no overwrite, so user/session state is preserved while new defaults can
appear.

## Service Integration

`discboeing-sandbox-init.service` runs after local filesystems and tmpfiles setup,
and before the proxy, Docker daemon, and agent API services. The unit uses
`Type=notify`, `NotifyAccess=all`, and `RemainAfterExit=yes` so dependent units
can wait for setup completion without keeping the setup script alive.
