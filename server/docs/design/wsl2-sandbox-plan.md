# WSL2 Sandbox Implementation Plan

This document is the working implementation plan for the Windows sandbox backend.
It is intended to stay up to date as implementation progresses so future sessions can
resume work without reconstructing the design from scratch.

## Document Status

- Owner: Discobot sandbox runtime work
- Status: planned
- Last updated: 2026-04-03
- Related docs:
  - `server/docs/design/sandbox.md`
  - `server/internal/sandbox/runtime.go`
  - `server/internal/sandbox/docker/provider.go`
  - `server/internal/sandbox/vm/provider.go`
  - `server/internal/sandbox/vz/vz_docker.go`
  - `server/internal/sandbox/vz/image_downloader.go`

## Progress Tracker

### Completed

- [x] Investigated current Linux Docker and macOS VZ sandbox implementations
- [x] Chosen high-level Windows direction: managed WSL2 distro with Docker inside
- [x] Chosen image strategy: reuse the same OCI image family used for VZ, with an additional WSL rootfs tar artifact
- [x] Chosen provider strategy: implement a dedicated `wsl.Provider` that wraps `docker.Provider`
- [x] Chosen path strategy: translate Windows bind source paths into WSL-visible paths before sandbox creation
- [x] Chosen lifecycle strategy: one managed shared WSL distro per user install, not one distro per project
- [x] Added Windows default provider selection (`wsl`) in `server/internal/sandbox/manager.go`
- [x] Added Windows-specific provider registration in `server/cmd/server/provider_windows.go`
- [x] Split Linux provider bootstrap into `server/cmd/server/provider_linux.go`
- [x] Added WSL config fields in `server/internal/config/config.go`
- [x] Added Phase 1 WSL manager skeleton in `server/internal/sandbox/wsl/manager.go`
- [x] Added Phase 1 WSL provider skeleton in `server/internal/sandbox/wsl/provider.go`
- [x] Added initial WSL provider status reporting for UI and diagnostics
- [x] Added `server/internal/sandbox/wsl/path.go` for Windows-to-WSL bind path translation
- [x] Added path translation unit tests in `server/internal/sandbox/wsl/path_test.go`
- [x] Added `server/internal/sandbox/wsl/bridge.go` for bridge host and pipe resolution
- [x] Added bridge resolution unit tests in `server/internal/sandbox/wsl/bridge_test.go`
- [x] Wired the WSL provider to prepare translated create options and to construct a future inner `docker.Provider` from resolved bridge settings
- [x] Added `server/internal/sandbox/wsl/state.go` for persisted runtime metadata
- [x] Added runtime state unit tests in `server/internal/sandbox/wsl/state_test.go`
- [x] Updated the WSL manager to read persisted TCP bridge port metadata from runtime state
- [x] Added `server/internal/sandbox/wsl/distro.go` for parsing `wsl.exe --list --verbose` output
- [x] Added distro parsing unit tests in `server/internal/sandbox/wsl/distro_test.go`
- [x] Updated the WSL manager to detect whether the managed distro is missing, stopped, or running before bridge startup exists
- [x] Updated `EnsureRunning()` to start a stopped managed distro and wait for `systemd` and `docker.service` readiness
- [x] Added `server/internal/sandbox/wsl/image_downloader.go` for OCI-backed WSL rootfs artifact download and caching
- [x] Added image downloader unit tests in `server/internal/sandbox/wsl/image_downloader_test.go`
- [x] Updated the WSL manager to import a missing distro with `wsl.exe --import` using the cached rootfs artifact
- [x] Added initial TCP bridge startup and readiness probing for Windows-to-WSL Docker access
- [x] Updated WSL runtime state persistence to remember dynamically assigned TCP bridge ports
- [x] Added shared host-runtime idle shutdown abstraction and wired WSL to use `WSLIdleTimeout` for managed distro shutdown

### In Progress

- [ ] Harden the bridge transport from prototype loopback TCP to the final supported bridge strategy
- [x] Add named-pipe bridge support

### Not Started

- [ ] Add `server/internal/sandbox/wsl/upgrade.go`
- [ ] Extend build pipeline to emit `discobot-rootfs.tar.zst`
- [ ] Add Windows integration tests for distro bootstrap and Docker connectivity

## Goals

On Windows, Discobot should:

- run sandboxes inside a managed WSL2 runtime
- use Docker inside the managed WSL distro for session isolation
- preserve the existing Discobot model of one container per session
- reuse the same image build pipeline as VZ
- manage distro install, startup, shutdown, and upgrades
- translate Windows host paths into WSL-visible bind mount paths

## Non-Goals

The first implementation does not need to:

- create one WSL distro per project
- support arbitrary UNC path bind mounts
- expose raw Docker TCP without an authenticated bridge
- redesign the session/container abstraction
- replace the existing Linux Docker or macOS VZ backends

## Current Runtime Model in Discobot

### Linux

Linux uses the plain Docker provider directly:

- provider registration: `server/cmd/server/provider_linux.go`
- implementation: `server/internal/sandbox/docker/provider.go`

This gives Discobot one Docker container per session and direct access to the host Docker daemon.

### macOS

macOS uses a hybrid VZ + Docker provider:

- VZ registration: `server/cmd/server/provider_darwin.go`
- VZ provider entry point: `server/internal/sandbox/vz/vz_docker.go`
- generic VM provider: `server/internal/sandbox/vm/provider.go`
- VZ VM manager: `server/internal/sandbox/vz/vz_vm_manager.go`

This creates one VM per project and one Docker container per session inside that VM.
Docker is reached through a VSOCK bridge.

### Implication for Windows

Windows should follow the same broad shape as macOS:

- outer runtime boundary managed by Discobot
- Docker daemon running inside that boundary
- one session container per session

But unlike VZ, Windows should use one shared managed WSL distro instead of per-project VMs.

## Recommended Windows Architecture

```text
Windows host
  ├── Discobot server
  ├── WSL manager
  └── Managed WSL distro: discobot
        ├── systemd
        ├── dockerd
        ├── discobot docker bridge
        └── session containers
```

## Key Design Decisions

### 1. One shared managed distro

Use one WSL distro per user install.

Reasoning:

- simpler install and upgrade path
- simpler Docker access model
- simpler diagnostics and recovery
- avoids forcing WSL into the current per-project VM abstraction
- session isolation still comes from inner Docker containers

### 2. Dedicated `wsl.Provider`

Do not force WSL into `vm.ProjectVMManager`.

Instead, add a dedicated provider that:

- ensures the managed distro is installed and running
- ensures Docker is reachable
- translates bind source paths
- delegates sandbox operations to an inner `docker.Provider`

Expected package layout:

```text
server/internal/sandbox/wsl/
  manager.go
  provider.go
  bridge.go
  path.go
  state.go
  image_downloader.go
  upgrade.go
```

### 3. Reuse the VZ image family

Keep one OCI image family for guest runtime assets.

Current VZ image contents include:

- `vmlinuz`
- `discobot-rootfs.squashfs`

The Windows/WSL plan is to extend the image to also include:

- `discobot-rootfs.tar.zst`
- `image-manifest.json`
- optional `rootfs-filelist.txt`

That keeps the guest userspace aligned across Apple and Windows.

### 4. Keep Docker on a Unix socket inside WSL

Inside WSL, Docker should remain on:

- `unix:///var/run/docker.sock`

Discobot on Windows should talk to Docker through a Discobot-controlled bridge, not by exposing raw Docker widely.

### 5. Translate host paths before bind mounting

Any host-originated path passed as a Docker bind source must be translated into a WSL-visible path first.

Examples:

- `C:\Users\me\repo` -> `/mnt/c/Users/me/repo`
- `D:\code\proj` -> `/mnt/d/code/proj`

## Provider Design

## `wsl.Provider`

Expected structure:

```go
type Provider struct {
    cfg            *config.Config
    manager        *Manager
    dockerProvider *docker.Provider
}
```

Responsibilities:

- call `EnsureRunning()` before sandbox operations
- translate `CreateOptions.WorkspacePath` into a WSL path
- preserve `CreateOptions.WorkspaceSource` as the original Windows path or git URL
- construct or reuse an inner `docker.Provider` using the WSL bridge transport
- delegate `Create`, `Start`, `Stop`, `Remove`, `Get`, `List`, `Exec`, `Attach`, `ExecStream`, `HTTPClient`, and `Watch`

## `wsl.Manager`

Suggested responsibilities:

- check WSL availability
- install distro if missing
- start distro if stopped
- verify systemd and Docker readiness
- manage installed image metadata
- decide whether upgrade is required
- terminate or uninstall distro
- report provider status

Suggested methods:

```go
type Manager interface {
    EnsureInstalled(ctx context.Context) error
    EnsureRunning(ctx context.Context) (*RuntimeInfo, error)
    Stop(ctx context.Context) error
    UpgradeIfNeeded(ctx context.Context) error
    Uninstall(ctx context.Context) error
    Status() sandbox.ProviderStatus
}
```

## Provider Registration

### New server bootstrap split

Replace the current non-darwin catch-all provider registration with OS-specific registration:

- `provider_darwin.go`
- `provider_linux.go`
- `provider_windows.go`

### Default provider rules

Update platform default provider selection to:

- darwin -> `vz`
- windows -> `wsl`
- linux -> `docker`

The existing provider proxy in `server/cmd/server/main.go` can continue to route by workspace provider.

## Config Changes

Add Windows-specific config fields in `server/internal/config/config.go`.

Proposed fields:

```go
WSLDistroName      string
WSLInstallDir      string
WSLStateDir        string
WSLImageRef        string
WSLBridgeType      string // named_pipe|tcp
WSLBridgePort      int    // 0=random
WSLIdleTimeout     time.Duration
WSLUpgradeStrategy string // inplace
```

Suggested defaults:

- `WSLDistroName=discobot`
- `WSLImageRef=DefaultVZImage()` initially
- `WSLBridgeType=tcp` initially while named-pipe transport is still pending
- install/state under `%LOCALAPPDATA%`

## Distro Lifecycle

## Install flow

1. Verify `wsl.exe` exists.
2. Verify WSL2 is enabled and available.
3. Check whether the managed distro exists.
4. If missing:
   - download `discobot-rootfs.tar.zst`
   - decompress to tar
   - run `wsl.exe --import <name> <install-dir> <rootfs.tar> --version 2`
5. Run first-boot provisioning.
6. Write installed digest metadata.

## Start flow

1. Ensure installed.
2. Start the distro with `wsl.exe -d <name> -- true`.
3. Wait for systemd readiness.
4. Wait for `docker.service` readiness.
5. Wait for the Docker bridge to be reachable.
6. Return runtime connection info.

## Stop flow

Use:

- `wsl.exe --terminate <name>`

## Uninstall flow

Use:

- `wsl.exe --unregister <name>`

Then delete Discobot-managed Windows-side state.

## Distro Contents

The managed rootfs should include:

- systemd-enabled base runtime
- Docker daemon
- Discobot Docker bridge service or helper
- Discobot upgrade helper
- basic diagnostics tooling needed for support and recovery

Required config should include `/etc/wsl.conf` with systemd enabled.

## Docker Bridge Design

## Recommended transport

Preferred:

- Windows named pipe exposed by Discobot

Example endpoint:

- `\\.\pipe\discobot-docker`

### Why named pipe

- Windows-native local IPC
- no network listener required
- better ACL story than TCP
- no TLS required for same-machine transport

## Acceptable fallback for v1

If named pipe integration is too much for the first version:

- use loopback-only TCP on `127.0.0.1`
- choose a random port
- require a bearer token
- never expose raw Docker unauthenticated

## Security requirements

The security boundary is authorization, not just encryption.

Minimum rules:

- Docker inside WSL stays on `/var/run/docker.sock`
- raw Docker API must not be exposed broadly
- Discobot bridge must only accept local clients
- bridge access must be restricted to the current user or Discobot-owned process
- if TCP is used, require a random bearer token and bind only to `127.0.0.1`

## Path Translation Rules

Add a reusable path translator, likely in `server/internal/sandbox/wsl/path.go`.

Suggested API:

```go
type HostPathTranslator interface {
    TranslatePath(hostPath string) (guestPath string, error)
}
```

### Supported in first version

- absolute drive-letter paths
- normalized local filesystem paths
- Discobot-managed workspace paths on local drives

### Rejected in first version

- UNC/network shares
- `\\wsl$\...` paths
- ambiguous or non-absolute inputs

### Field rules

- `WorkspacePath`: translated WSL-visible path
- `WorkspaceSource`: original Windows path or git URL

That preserves existing semantics while making bind mounts work from the WSL daemon's perspective.

## HTTP and Port Access

The initial implementation should try to preserve the existing Docker provider model where session containers publish port 3002 to localhost.

Expected first attempt:

- Docker inside WSL publishes `127.0.0.1:<random-port>`
- Windows host accesses that forwarded localhost port
- `docker.Provider` continues to resolve published ports normally

If Windows-to-WSL localhost forwarding is not reliable enough, add a WSL-specific `HTTPClient()` override later.

## Upgrade Strategy

## Strong recommendation

Use in-place distro upgrades rather than unregister/reimport.

### Why

Reimporting would risk losing:

- Docker images
- cache volumes
- session data
- logs

### Upgrade flow

1. Compare installed digest to desired digest from OCI metadata.
2. If changed:
   - pause or drain new sandbox creation
   - run an in-distro upgrade helper
   - untar the new rootfs over `/`
   - preserve mutable directories such as:
     - `/var/lib/docker`
     - `/var/lib/discobot`
     - `/var/log`
     - possibly `/etc/discobot`
   - remove deleted files using `rootfs-filelist.txt`
3. Terminate distro.
4. Restart distro.
5. Verify Docker bridge health.
6. Mark new digest as installed.

## Build Pipeline Work

Extend the existing OCI runtime image build so it emits:

- `discobot-rootfs.squashfs` for VZ
- `discobot-rootfs.tar.zst` for WSL
- metadata files used for install and upgrade

The root filesystem should be defined once and exported in multiple formats.

## Status Reporting

Implement `sandbox.StatusProvider` on the WSL provider or manager.

Suggested states:

- `not_available`
- `not_installed`
- `installing`
- `upgrading`
- `starting`
- `ready`
- `failed`

Suggested details payload:

- distro name
- installed digest
- desired digest
- bridge type
- bridge path or port
- Docker health state
- install or upgrade progress if active

## Reconciliation Model

Keep two separate reconciliation layers:

### Distro reconciliation

Handled by WSL manager:

- compare installed distro image digest to desired digest
- perform distro upgrade if needed

### Session sandbox reconciliation

Handled by existing sandbox service logic:

- compare session container image identity to `SANDBOX_IMAGE`
- recreate outdated session containers as needed

Do not conflate distro image updates with container image updates.

## Failure Handling

### WSL unavailable

Provider state should be `not_available` with a clear diagnostic.

### Distro install failure

Persist enough state to show where install failed and allow retry.

### Docker unhealthy inside distro

Recovery sequence:

1. retry Docker health probe
2. restart distro
3. if still unhealthy, surface error and logs

### Unsupported bind source path

Fail sandbox creation with a path translation error before attempting Docker create.

## Testing Plan

## Unit tests

Add focused tests for:

- path normalization and translation
- unsupported path rejection
- WSL state transitions
- digest comparison and upgrade decisions
- provider behavior that rewrites `WorkspacePath` and preserves `WorkspaceSource`

## Integration tests

Later Windows-only integration coverage should verify:

- distro bootstrap
- bridge connectivity
- Docker ping
- session container create/start
- bind mounting translated workspace paths

## Implementation Phases

### Phase 1: bootstrap

- add config fields
- add provider registration for Windows
- add WSL manager skeleton
- add status reporting
- add distro install/start/stop plumbing

### Phase 2: Docker connectivity

- implement bridge transport
- wire inner `docker.Provider`
- validate Docker ping and image checks

### Phase 3: sandbox operations

- support container create/start/stop/remove/list
- support translated workspace bind mounts
- validate health checks and port access

### Phase 4: upgrades and reconciliation

- add OCI metadata parsing
- add rootfs download/extract flow
- add in-place upgrade
- integrate startup reconciliation

### Phase 5: hardening

- expand diagnostics
- tighten auth and ACL behavior
- add recovery tooling
- add Windows integration coverage

## Immediate Next Tasks

When implementation resumes, start with these tasks in order:

1. Decide whether the first live bridge implementation will use named pipe or authenticated loopback TCP.
2. Start the Windows-side Docker bridge process and connect the resolved bridge host to the live runtime.
3. Persist live bridge metadata updates through `server/internal/sandbox/wsl/state.go` during bridge startup/shutdown.
4. Add first-boot provisioning for any guest-side setup that cannot be baked fully into the rootfs image.
5. Extend the runtime image pipeline to emit the WSL rootfs tar artifact.

## Implementation Notes

### 2026-04-03 — Phase 2 import/bootstrap scaffolding landed

Implemented the missing-distro bootstrap path:

- added `server/internal/sandbox/wsl/image_downloader.go` to download and cache `discobot-rootfs.tar.zst` from the shared OCI runtime image
- added `server/internal/sandbox/wsl/image_downloader_test.go` for cache and extraction coverage
- updated `server/internal/sandbox/wsl/manager.go` so `EnsureInstalled()` imports a missing distro automatically with `wsl.exe --import`
- updated `server/internal/sandbox/wsl/manager.go` to decompress the cached rootfs archive into a temporary tar before import and to persist basic image/runtime metadata after import

Current limitation: first-boot provisioning and Windows-side Docker bridge startup are still not implemented, so a freshly imported distro still relies on the guest image being preconfigured for systemd and Docker.

### 2026-04-03 — Phase 2 lifecycle controls landed

Implemented the first real managed-runtime teardown and upgrade controls:

- updated `server/internal/sandbox/wsl/manager.go` so `Stop()` now terminates a running managed distro with `wsl.exe --terminate`
- updated `server/internal/sandbox/wsl/manager.go` so `Uninstall()` now unregisters the managed distro, removes the install directory, and clears persisted runtime state
- updated `server/internal/sandbox/wsl/manager.go` so `UpgradeIfNeeded()` now supports the current `inplace` strategy by reinstalling when the persisted `ImageRef` differs from the configured WSL image
- kept upgrade detection conservative by keying off persisted runtime state, avoiding destructive reinstall when the existing distro has no recorded image metadata yet

Current limitation: upgrade handling is still coarse-grained reinstall logic, with no in-guest migration path, no named-pipe bridge lifecycle, and no Windows integration coverage yet.

### 2026-04-03 — Phase 2 named-pipe bridge support landed

Implemented the first working Windows named-pipe bridge path:

- updated `server/internal/sandbox/wsl/manager.go` so `EnsureRunning()` now starts and waits for a named-pipe Docker bridge when `WSL_BRIDGE_TYPE=named_pipe`
- added an in-process Windows named-pipe listener backed by `go-winio`
- wired each named-pipe client connection to a `wsl.exe ... socat STDIO UNIX-CONNECT:/var/run/docker.sock` bridge inside the managed distro
- updated readiness probing so named-pipe bridges are validated with a Docker `/_ping` request over the pipe before the provider reports `BridgeReady=true`
- added bridge helper coverage for named-pipe path/host derivation in `server/internal/sandbox/wsl/bridge_test.go`

Current limitation: the named-pipe bridge is process-local and connection-proxy based, so bridge hardening, shutdown polish, and Windows integration coverage are still pending.

### 2026-04-03 — Shared runtime-idle shutdown abstraction landed

Implemented a shared host-runtime idle monitor so VM-backed and WSL-backed runtimes
can reuse the same watch/stop behavior without forcing WSL into the per-project VM model:

- added `server/internal/sandbox/idle_runtime_monitor.go` with documented `IdleRuntimeController` and `IdleRuntimeMonitor` interfaces/types
- updated `server/internal/sandbox/vm/provider.go` so VZ project-VM idle shutdown now uses the shared monitor instead of an inline VM-specific loop
- updated `server/internal/sandbox/wsl/provider.go` so `WSLIdleTimeout` now participates in real managed-distro shutdown
- kept the shared abstraction runtime-scoped rather than session-scoped, leaving `server/internal/sandbox/runtime.go` focused on session sandbox lifecycle

Current limitation: WSL runtime idle shutdown still depends on counting Discobot containers through the active Docker bridge and still needs Windows integration coverage.

### 2026-04-03 — Phase 2 startup readiness scaffolding landed

Implemented the first real managed-distro startup path:

- updated `server/internal/sandbox/wsl/manager.go` so `EnsureRunning()` now starts the managed distro with `wsl.exe -d <name> -- true` when it is stopped
- added polling helpers that wait for `systemd` readiness and `docker.service` readiness inside the distro
- kept bridge activation separate, so runtime startup can now prove the guest OS and Docker are up before bridge work begins
- updated status messaging so a stopped distro is treated as startable-on-demand rather than permanently blocked

Current limitation: missing distros are still not imported automatically, and bridge startup is still not implemented, so `BridgeReady` remains false.

### 2026-04-03 — Phase 2 distro probing landed

Implemented pre-start distro detection and status probing:

- added `server/internal/sandbox/wsl/distro.go` to parse `wsl.exe --list --verbose` output
- added `server/internal/sandbox/wsl/distro_test.go` for parser coverage, including names with spaces
- updated `server/internal/sandbox/wsl/manager.go` so `Status()` can distinguish missing, stopped, and running distros
- updated `server/internal/sandbox/wsl/manager.go` so `EnsureRunning()` now fails clearly when the managed distro has not been imported yet

Current limitation: the manager now knows whether the distro exists, but it still does not import or start it, verify systemd/Docker readiness, or launch the Windows-side Docker bridge.

### 2026-04-03 — Phase 2 runtime state scaffolding landed

Implemented persisted runtime metadata support:

- added `server/internal/sandbox/wsl/state.go` with atomic load/save/clear helpers
- added `server/internal/sandbox/wsl/state_test.go` to cover save, load, missing-file, and clear behavior
- updated `server/internal/sandbox/wsl/manager.go` to expose the runtime state path in status details
- updated `server/internal/sandbox/wsl/manager.go` to reuse a persisted TCP bridge port when `WSL_BRIDGE_PORT=0`

Current limitation: runtime state is now available, but nothing writes live bridge assignments yet because distro startup and bridge launch are still not implemented.

### 2026-04-03 — Phase 2 helper and delegation scaffolding landed

Implemented the first reusable Phase 2 pieces:

- added `server/internal/sandbox/wsl/path.go` with Windows-to-WSL bind path translation
- added `server/internal/sandbox/wsl/bridge.go` with named-pipe and TCP bridge host resolution
- added unit tests for both helper areas so they run outside Windows-specific builds
- updated `server/internal/sandbox/wsl/manager.go` to surface resolved bridge metadata in runtime status
- updated `server/internal/sandbox/wsl/provider.go` to translate `CreateOptions.WorkspacePath` and to build a future inner `docker.Provider` from bridge settings

Current limitation: the provider still cannot run real sandbox operations because `EnsureRunning()` does not yet install/start the distro or launch the Windows-side bridge, so `BridgeReady` remains false.

### 2026-04-03 — Phase 1 bootstrap landed

Implemented the initial Windows bootstrap surface:

- `server/internal/sandbox/manager.go` now defaults Windows to `wsl`
- `server/cmd/server/provider_windows.go` registers the WSL provider on Windows
- `server/cmd/server/provider_linux.go` now handles Linux Docker bootstrap explicitly
- `server/internal/config/config.go` now exposes WSL env/config fields
- `server/internal/sandbox/wsl/manager.go` provides Phase 1 lifecycle/status scaffolding
- `server/internal/sandbox/wsl/provider.go` provides a Phase 1 provider skeleton

Current limitations of the Phase 1 code:

- no Docker bridge yet
- no inner `docker.Provider` yet
- no distro import/start logic yet beyond `wsl.exe` presence checks
- sandbox operations intentionally return not implemented while runtime scaffolding is established

## Notes for Future Updates

When progress is made, update this file by:

- moving items between Completed, In Progress, and Not Started
- adding dated implementation notes under a new changelog section if useful
- recording deviations from the original design decision with rationale
- linking concrete implementation files as they are added

If implementation diverges from this plan, the plan should be updated immediately so it remains the source of truth.
