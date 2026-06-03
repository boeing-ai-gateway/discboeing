# AGENTS.md

Guidance for coding agents working on this repository.

## Project summary

This is a .NET 8 console app that launches a Linux utility VM through Windows Host Compute System APIs. It generates HCS JSON for Linux direct boot with WSL kernel assets, attaches a read-only root VHDX and read/write data VHDX, optionally attaches an HCN ICS/NAT endpoint, and can open a host Hyper-V socket listener mapped from a Linux VSOCK port.

## Important runtime constraints

- The project can compile on Linux or Windows.
- Non-dry-run execution must happen on Windows.
- HCS/HCN runtime behavior cannot be validated on Linux because it depends on:
  - `ComputeCore.dll`
  - `ComputeNetwork.dll`
  - Windows HCS/HCN services
  - AF_HYPERV sockets
  - Windows VHDX/VM access behavior
- Running actual launch commands may require an elevated Windows terminal.
- Keep `ShouldTerminateOnLastHandleClosed = true` unless the VM lifetime model is intentionally changed.

## Build and validation commands

If `dotnet` is already on PATH:

```bash
dotnet build
dotnet format --verify-no-changes
```

In this Discobot Linux sandbox, the SDK may be installed under `~/.dotnet`; use:

```bash
PATH="$HOME/.dotnet:$PATH" dotnet build
PATH="$HOME/.dotnet:$PATH" dotnet format --verify-no-changes
```

Dry-run commands are safe on Linux and validate CLI parsing plus JSON generation:

```bash
PATH="$HOME/.dotnet:$PATH" dotnet run -- --root 'C:\vm\root.vhdx' --data 'C:\vm\data.vhdx' --network none --dry-run
PATH="$HOME/.dotnet:$PATH" dotnet run -- --root 'C:\vm\root.vhdx' --data 'C:\vm\data.vhdx' --network hcn-nat --dry-run
```

There is currently no unit test project. `dotnet test` only builds the app until tests are added.

## Test disk generation

No test VM disks are checked in. Generate small fixed VHD smoke-test disks with:

```bash
scripts/create-test-vhds.sh artifacts/test-disks
```

The generated root disk has a static `/init` and an ext4 filesystem directly on `/dev/sda`; the data disk is ext4 on `/dev/sdb`. Copy the VHDs to a Windows host and launch with:

```powershell
dotnet run -- --root C:\vm\hcs-test-root.vhd --data C:\vm\hcs-test-data.vhd --root-device /dev/sda --no-initrd --append-kernel-cmdline "init=/init" --network none
```

After stopping the VM, inspect the data VHD for `boot-ok.txt`.

## Windows integration test checklist

See `WINDOWS-TESTING.md` for the detailed handoff checklist, expected outcomes, troubleshooting, and resume notes.

Run these only on a Windows host with .NET 8 SDK, Virtual Machine Platform/Hyper-V support, WSL kernel assets or explicit kernel/initrd paths, and suitable VHDX files:

```powershell
dotnet build
dotnet run -- --root C:\vm\root.vhdx --data C:\vm\data.vhdx --dry-run
dotnet run -- --root C:\vm\root.vhdx --data C:\vm\data.vhdx --network none
dotnet run -- --root C:\vm\root.vhdx --data C:\vm\data.vhdx --network hcn-nat
dotnet run -- --root C:\vm\root.vhdx --data C:\vm\data.vhdx --listen-vsock --echo-vsock
```

## Code organization

- `ProgramEntry.cs`: CLI flow, launch orchestration, cancellation, cleanup.
- `CliOptions.cs`: argument parsing/defaults/validation.
- `Hcs/`: HCS JSON and lifecycle wrappers.
- `Hcn/`: HCN NAT JSON and endpoint lifecycle.
- `HvSocket/`: optional host AF_HYPERV listener.
- `NativeMethods.cs`: native P/Invoke declarations.
- `HvSocketPorts.cs`: Linux VSOCK port to Hyper-V socket service GUID mapping.
- `Networking/Ipv4.cs`: IPv4/CIDR parsing helpers.

## Change guidance

- Keep Windows-only native calls behind dry-run or `OperatingSystem.IsWindows()` paths where practical.
- Prefer preserving JSON fields already mirrored from WSL/HCS examples unless changing behavior intentionally.
- Keep root disk read-only and data disk read/write unless requirements change.
- Avoid deleting or relaxing `HcsGrantVmAccess`/`HcsRevokeVmAccess` unless a replacement access model is added.
- Treat HCN/NAT changes as integration-sensitive; validate generated JSON with `--dry-run` and real behavior on Windows.
- If tests are added, prioritize pure logic first: CLI parsing, IPv4/CIDR parsing, VSOCK GUID mapping, and HCS/HCN JSON shape tests.
