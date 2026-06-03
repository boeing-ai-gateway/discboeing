# HCS Linux VM Launcher

A small .NET 8 console launcher that creates a Windows Host Compute System (HCS) Linux utility VM with:

- the WSL2 Linux kernel path by default (`%ProgramFiles%\WSL\tools\kernel`)
- optional WSL2 initrd path by default (`%ProgramFiles%\WSL\tools\initrd.img`)
- one read-only root VHDX attached at SCSI LUN 0
- one read/write data VHDX attached at SCSI LUN 1
- Hyper-V socket / Linux VSOCK access using the WSL service GUID template
- optional HCN ICS/NAT networking
- VM lifetime tied to the launcher process via `ShouldTerminateOnLastHandleClosed = true`

## Prerequisites

Run this on Windows with:

- .NET 8 SDK
- Hyper-V / Virtual Machine Platform support enabled
- WSL installed so the default kernel/initrd paths exist, or pass `--kernel` and `--initrd`
- permission to use HCS/HCN APIs; running from an elevated terminal is often required
- bootable Linux VHDX files compatible with direct Linux kernel boot and Hyper-V devices

The guest root filesystem must contain the init/userspace you expect to run. The WSL kernel provides Hyper-V drivers, but it does not make an arbitrary root disk bootable by itself.

## Project layout

```text
Program.cs                         top-level entry point
ProgramEntry.cs                    CLI flow, launch sequence, cleanup
CliOptions.cs                      argument parsing and validation
Hcs/HcsConfigurationFactory.cs     HCS compute-system JSON generation
Hcs/HcsComputeSystem.cs            HCS create/start/modify/terminate wrapper
Hcs/HcsOperation.cs                HCS async operation helper
Hcs/VmAccessGrant.cs               HcsGrantVmAccess/HcsRevokeVmAccess wrapper
Hcn/HcnConfigurationFactory.cs     HCN network, endpoint, and NIC JSON generation
Hcn/HcnNatAttachment.cs            HCN NAT network/endpoint lifecycle
HvSocket/HvSocketServer.cs         optional host AF_HYPERV listener
HvSocketPorts.cs                   Linux VSOCK port to Hyper-V service GUID mapping
NativeMethods.cs                   ComputeCore/ComputeNetwork/Winsock P/Invoke declarations
scripts/create-test-vhds.sh        creates tiny fixed VHDs for Windows smoke testing
```

## Build

```powershell
dotnet build
```

## Development validation

The pure C# pieces can be built and dry-run on Linux or Windows:

```bash
dotnet build
dotnet format --verify-no-changes
dotnet run -- --root 'C:\vm\root.vhdx' --data 'C:\vm\data.vhdx' --network none --dry-run
dotnet run -- --root 'C:\vm\root.vhdx' --data 'C:\vm\data.vhdx' --network hcn-nat --dry-run
```

There is not currently a unit test project. Good future unit-test targets are CLI parsing, IPv4/CIDR parsing, VSOCK GUID mapping, and HCS/HCN JSON generation.

Actual launch testing must run on Windows because HCS, HCN, and AF_HYPERV are Windows-only APIs.

For the full Windows handoff checklist, including expected results, troubleshooting, and what to capture for the next development session, see [WINDOWS-TESTING.md](WINDOWS-TESTING.md).

## Test disks

No VM disk images are checked into this repository. For smoke testing, generate tiny fixed VHD files with:

```bash
scripts/create-test-vhds.sh artifacts/test-disks
```

This creates:

```text
artifacts/test-disks/hcs-test-root.vhd
artifacts/test-disks/hcs-test-data.vhd
```

The root VHD contains an ext4 filesystem directly on `/dev/sda` and a static `/init` program. The init program mounts the data disk from `/dev/sdb`, writes `/data/boot-ok.txt`, and then sleeps forever. The data disk is an empty ext4 filesystem. The generated kernel command line includes `rootwait` so the kernel waits for the SCSI root disk to appear.

Copy those two VHD files to a Windows test host, for example `C:\vm`, and launch with:

```powershell
dotnet run -- `
  --root C:\vm\hcs-test-root.vhd `
  --data C:\vm\hcs-test-data.vhd `
  --root-device /dev/sda `
  --no-initrd `
  --append-kernel-cmdline "init=/init" `
  --network none
```

Stop the launcher after the VM has had time to boot, then inspect `hcs-test-data.vhd` for `boot-ok.txt`. That confirms the kernel booted, mounted the read-only root disk, mounted the read/write data disk, and executed userspace.

The generated files use fixed VHD rather than VHDX because fixed VHDs are easy to create from Linux without Hyper-V tooling. The launcher accepts either as long as Windows HCS can attach the virtual disk.

## Planned disk and resource behavior

HCS `VirtualDisk` attachments should be treated as VHD/VHDX inputs, not arbitrary raw files. If the desired root filesystem is a raw image such as SquashFS, wrap it as a fixed VHD instead of passing the raw file directly. A fixed VHD is the raw bytes plus a 512-byte VHD footer, so a SquashFS image placed at byte zero can still be mounted by Linux as `/dev/sda` with `rootfstype=squashfs`. Pad the raw image to a 512-byte boundary before appending the fixed-VHD footer.

Planned launcher behavior for VM resources and the writable data disk:

- CPU count is configurable. The default should be the host logical processor count.
- RAM is configurable in GiB. The default should be 50% of host physical memory.
- Hyper-V dynamic memory / memory ballooning, or the closest HCS equivalent, must remain enabled.
- Data disk size is configurable.
- Data disks should be sparse/thin-provisioned VHDX files that support guest TRIM/discard.
- If the requested data disk does not exist, create it at the requested size.
- If the existing data disk is smaller than the requested size, expand it to the requested size.
- If the existing data disk is larger than the requested size, do not shrink it and do not fail; print a warning and continue.
- Keep the root disk read-only unless the VM lifetime/storage model is intentionally changed.

## Dry run

Print the generated HCS and HCN JSON without calling Windows APIs:

```powershell
dotnet run -- --root C:\vm\root.vhdx --data C:\vm\data.vhdx --dry-run
```

## Launch without networking

```powershell
dotnet run -- `
  --root C:\vm\root.vhdx `
  --data C:\vm\data.vhdx `
  --network none
```

The root disk is attached read-only. The data disk is attached read/write.

## Launch with HCN NAT

```powershell
dotnet run -- `
  --root C:\vm\root.vhdx `
  --data C:\vm\data.vhdx `
  --network hcn-nat `
  --nat-subnet 172.31.240.0/20 `
  --nat-gateway 172.31.240.1
```

Request a specific endpoint address:

```powershell
dotnet run -- `
  --root C:\vm\root.vhdx `
  --data C:\vm\data.vhdx `
  --nat-vm-ip 172.31.240.10
```

NAT caveats:

- The launcher creates an HCN `ICS` network with `EnableDns`, `EnableNonPersistent`, and optionally `EnableDhcp`.
- The VM gets an HCN endpoint and a synthetic NIC, but the guest still needs Linux networking configured. Use DHCP in the guest, or configure the requested static address manually.
- Windows HCN behavior varies by build. If DHCP creation fails, try `--nat-disable-dhcp` and configure the guest statically.
- The launcher deletes the endpoint on exit and deletes the network when it created that network for this run.

## Hyper-V socket / VSOCK

The launcher maps Linux VSOCK ports to Hyper-V socket service GUIDs with WSL's standard template:

```text
00000000-facb-11e6-bd58-64006a7986d3
```

The first DWORD is replaced with the VSOCK port. For example, port `5000` becomes:

```powershell
dotnet run -- --root C:\vm\root.vhdx --data C:\vm\data.vhdx --vsock-port 5000 --dry-run
```

To accept a guest-to-host Hyper-V socket connection and print received bytes:

```powershell
dotnet run -- `
  --root C:\vm\root.vhdx `
  --data C:\vm\data.vhdx `
  --listen-vsock `
  --echo-vsock
```

The guest can connect to the host listener using Linux `AF_VSOCK` with the configured port if the guest kernel/userspace supports VSOCK.

## VM lifetime

The HCS document sets:

```json
"ShouldTerminateOnLastHandleClosed": true
```

The launcher also tries to terminate the VM during normal shutdown. If the launcher process dies unexpectedly, Windows closes the HCS handle, which should tear down the VM.

## CLI options

The launcher accepts these options:

```text
-h, --help                         print help and exit
--dry-run                          print generated HCS/HCN JSON and exit
--id <guid>                        VM ID, default generated per run
--owner <name>                     HCS document owner, default HcsLinuxVmLauncher
--root <path>                      required root VHD/VHDX path, attached read-only
--data <path>                      required data VHD/VHDX path, attached read/write

Boot assets and kernel command line:
--kernel <path>                    WSL2 kernel path, default %ProgramFiles%\WSL\tools\kernel
--initrd <path>                    initrd path, default %ProgramFiles%\WSL\tools\initrd.img
--no-initrd                        omit InitRdPath
--root-device <dev>                kernel root= value, default /dev/sda1
--root-fstype <type>               kernel rootfstype= value, default ext4
--no-root-fstype                   omit rootfstype= from the generated kernel command line
--kernel-cmdline <text>            replace generated kernel command line
--append-kernel-cmdline <text>     append extra kernel arguments; may be repeated

VM resources and console:
--memory-mb <n>                    memory size in MiB, default 50% of host memory
--ram-gb <n>                       memory size in GiB, alias --memory-gb
--memory-gb <n>                    memory size in GiB, alias --ram-gb
--processors <n>                   vCPU count, default host logical processor count
--console-pipe <name>              hvc0 named pipe, default \\.\pipe\hcs-linux-vm-<id>-hvc0

Hyper-V socket / VSOCK:
--vsock-port <n>                   VSOCK/HVSOCK port, default 5000
--listen-vsock                     open a host HVSOCK listener and print received bytes
--echo-vsock                       echo bytes received by --listen-vsock
--hvsocket-sddl <sddl>             override HVSOCK bind/connect security descriptor

Plan9/9P shares:
--share <name=host-dir>            add a read-only Plan9/9P host directory share
--share-rw <name=host-dir>         add a read/write Plan9/9P host directory share

Networking:
--network hcn-nat|none|user-vsock  network mode, default hcn-nat
                                   aliases: nat, gvproxy, slirp

HCN NAT networking:
--nat-network-id <guid>            HCN NAT network ID, default generated per run
--nat-endpoint-id <guid>           HCN NAT endpoint ID, default generated per run
--nat-name <name>                  HCN NAT network name, default derived from VM ID
--nat-subnet <cidr>                HCN NAT subnet, default 172.31.240.0/20
--nat-gateway <ip>                 HCN NAT gateway, default 172.31.240.1
--nat-vm-ip <ip>                   request a specific VM endpoint IPv4 address
--nat-disable-dhcp                 do not request HNS DHCP support on the NAT network
--nat-disable-host-port            request HNS DisableHostPort on the NAT network

User-mode VSOCK networking:
--gvproxy <path>                   gvproxy executable for --network user-vsock, default gvproxy.exe
--gvproxy-vsock-port <n>           VSOCK port for gvproxy user networking, default 1024
--usernet-ip <ip>                  static guest IP, default 192.168.127.2
--usernet-netmask <ip>             static guest netmask, default 255.255.255.0
--usernet-gateway <ip>             static guest gateway, default 192.168.127.1
--usernet-dns <ip>                 static guest DNS server, default 192.168.127.1

Access control:
--skip-grant-vm-access             skip HcsGrantVmAccess/HcsRevokeVmAccess around VM assets
```

Use `--kernel-cmdline` if your initrd/root disk needs a different boot flow than the generated `root=... ro console=hvc0` command line.
