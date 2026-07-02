# Windows Testing Handoff

Use this checklist after copying the source tree to a Windows machine. Linux validation has covered build, formatting, CLI parsing, and dry-run JSON generation only; all real HCS/HCN behavior still needs Windows validation.

## Current Linux-side status

Validated in the Discboeing Linux sandbox:

```text
dotnet build                         passes, 0 warnings/errors
dotnet format --verify-no-changes     passes
dry-run with --network none            emits HCS JSON
dry-run with test-disk options         emits root=/dev/sda ... rootwait init=/init
scripts/create-test-vhds.sh           creates fixed VHD smoke-test disks
```

Not yet validated:

- `HcsCreateComputeSystem`
- `HcsStartComputeSystem`
- `HcsModifyComputeSystem` network adapter attach
- `HcsGrantVmAccess` / `HcsRevokeVmAccess`
- HCN ICS/NAT creation and endpoint attachment
- AF_HYPERV / VSOCK host listener
- Actual Linux boot with WSL kernel assets

## Windows host prerequisites

Use a Windows x64 machine with:

- .NET 8 SDK installed.
- Virtual Machine Platform / Hyper-V support enabled.
- WSL installed, or explicit `--kernel` / `--initrd` paths available.
- Administrator/elevated PowerShell for the first integration tests.
- Enough free disk space for generated VHDs and any converted VHDX files.

Useful checks:

```powershell
dotnet --info
wsl --status
Test-Path "$env:ProgramFiles\WSL\tools\kernel"
Test-Path "$env:ProgramFiles\WSL\tools\initrd.img"
```

If the WSL kernel assets are elsewhere, pass them explicitly:

```powershell
--kernel C:\path\to\kernel --initrd C:\path\to\initrd.img
```

## Copying the source and test disks

Copy the repository to the Windows host, for example:

```text
C:\src\HcsLinuxVmLauncher
```

Generate smoke-test VHDs on Linux before copying:

```bash
scripts/create-test-vhds.sh artifacts/test-disks
```

Copy these to Windows, for example:

```text
C:\vm\hcs-test-root.vhd
C:\vm\hcs-test-data.vhd
```

The script creates fixed `.vhd` files because they are easy to generate from Linux. If HCS rejects them or you want VHDX, convert on Windows with Hyper-V PowerShell tools:

```powershell
Convert-VHD -Path C:\vm\hcs-test-root.vhd -DestinationPath C:\vm\hcs-test-root.vhdx -VHDType Dynamic
Convert-VHD -Path C:\vm\hcs-test-data.vhd -DestinationPath C:\vm\hcs-test-data.vhdx -VHDType Dynamic
```

Then use the `.vhdx` paths in the commands below.

## Step 1: Build on Windows

From the copied repo:

```powershell
cd C:\src\HcsLinuxVmLauncher
dotnet restore
dotnet build
```

Expected result:

```text
Build succeeded.
0 Warning(s)
0 Error(s)
```

If build fails, fix this before any HCS testing.

## Step 2: Dry-run on Windows

Run a smoke-test dry-run using the generated root/data disks:

```powershell
dotnet run -- `
  --root C:\vm\hcs-test-root.vhd `
  --data C:\vm\hcs-test-data.vhd `
  --root-device /dev/sda `
  --no-initrd `
  --append-kernel-cmdline "init=/init" `
  --network none `
  --dry-run
```

Expected HCS JSON details:

- `ShouldTerminateOnLastHandleClosed` is `true`.
- root attachment LUN `0` has `ReadOnly: true`.
- data attachment LUN `1` has `ReadOnly: false`.
- kernel command line includes:

```text
root=/dev/sda rootfstype=ext4 ro rootwait ... init=/init
```

## Step 3: First real boot, no network

Run elevated PowerShell.

```powershell
dotnet run -- `
  --root C:\vm\hcs-test-root.vhd `
  --data C:\vm\hcs-test-data.vhd `
  --root-device /dev/sda `
  --no-initrd `
  --append-kernel-cmdline "init=/init" `
  --network none
```

Expected behavior:

- The launcher creates and starts the HCS VM.
- It prints that the VM is running.
- The test `/init` writes `/data/boot-ok.txt` and sleeps forever.
- Press Ctrl+C after waiting 10-30 seconds.
- The launcher terminates the VM and exits.

Success criteria:

- Launcher exits cleanly after Ctrl+C.
- Data disk contains `boot-ok.txt`.

Inspect the data disk using whichever Windows disk-inspection method you prefer. If Windows cannot mount ext4 directly, inspect it from WSL or another Linux machine after copying it back. From Linux, the generated VHD is a fixed VHD with a 512-byte footer, so the ext4 payload starts at byte 0.

Example Linux inspection after copying `hcs-test-data.vhd` back:

```bash
mkdir -p /tmp/hcs-data
sudo mount -o loop,ro ./hcs-test-data.vhd /tmp/hcs-data
cat /tmp/hcs-data/boot-ok.txt
sudo umount /tmp/hcs-data
```

If you cannot use sudo/loop mounts, use `debugfs` on Linux:

```bash
debugfs -R 'cat /boot-ok.txt' ./hcs-test-data.vhd
```

## Step 4: If no-network boot fails

Capture:

- Full command line.
- Full console output.
- HCS HRESULT and result/error JSON printed by the launcher.
- Windows build/version:

```powershell
winver
[System.Environment]::OSVersion.Version
```

Useful diagnostics to try if available:

```powershell
hcsdiag list
Get-WinEvent -LogName Microsoft-Windows-Hyper-V-Compute-Admin -MaxEvents 50 | Format-List
Get-WinEvent -LogName Microsoft-Windows-Hyper-V-Worker-Admin -MaxEvents 50 | Format-List
```

Likely failure areas and next fixes:

| Symptom | Likely area | Next action |
| --- | --- | --- |
| VM creation rejects JSON | HCS schema mismatch | Capture HCS result JSON; compare generated JSON to WSL schema/source examples. |
| VM creation rejects `.vhd` | Disk format support | Convert VHD to VHDX and retry. |
| Kernel starts but cannot mount root | Kernel driver/filesystem availability | Try WSL `initrd.img`; consider root as `/dev/sda` vs `/dev/sda1`; capture console. |
| VM starts but no `boot-ok.txt` | `/init` did not run or data disk name differs | Add console capture; try `/dev/sdb`, `/dev/sda` assumptions; inspect kernel logs. |
| Process exit leaves VM alive | lifetime/handle behavior | Check `ShouldTerminateOnLastHandleClosed`, explicit terminate path, and HCS handles. |

## Step 5: HCN NAT test

Only run this after the no-network boot test works.

```powershell
dotnet run -- `
  --root C:\vm\hcs-test-root.vhd `
  --data C:\vm\hcs-test-data.vhd `
  --root-device /dev/sda `
  --no-initrd `
  --append-kernel-cmdline "init=/init" `
  --network hcn-nat `
  --nat-subnet 172.31.240.0/20 `
  --nat-gateway 172.31.240.1
```

Expected behavior:

- HCN network creation succeeds or opens an existing network with the chosen ID.
- HCN endpoint creation succeeds.
- `HcsModifyComputeSystem` attaches the endpoint as a NIC.
- Launcher prints endpoint ID, MAC, and possibly IPv4/gateway.

This smoke `/init` does not configure networking inside Linux. For end-to-end guest networking, add DHCP/static network setup to the test init or test with a fuller root filesystem.

If HCN NAT fails, capture:

```powershell
Get-WinEvent -LogName Microsoft-Windows-Host-Network-Service-Admin -MaxEvents 50 | Format-List
```

If available, also capture HNS state:

```powershell
Get-HnsNetwork | ConvertTo-Json -Depth 20
Get-HnsEndpoint | ConvertTo-Json -Depth 20
```

Likely NAT follow-ups:

- Try `--nat-disable-dhcp`.
- Try a different `--nat-subnet` that does not conflict with existing host/VPN networks.
- Verify whether the host Windows build accepts the endpoint and adapter modify JSON as generated.

## Step 6: Hyper-V socket / VSOCK test

Only run this after basic boot works.

Host side:

```powershell
dotnet run -- `
  --root C:\vm\hcs-test-root.vhd `
  --data C:\vm\hcs-test-data.vhd `
  --root-device /dev/sda `
  --no-initrd `
  --append-kernel-cmdline "init=/init" `
  --network none `
  --listen-vsock `
  --echo-vsock `
  --vsock-port 5000
```

The current smoke-test `/init` does not initiate a VSOCK connection. To test VSOCK, either:

- use a fuller guest root filesystem with a VSOCK client tool, or
- extend `scripts/create-test-vhds.sh` to compile a small static guest VSOCK client into `/init`.

Expected host service GUID for port `5000`:

```text
00001388-facb-11e6-bd58-64006a7986d3
```

## Step 7: Additional development tasks after first Windows run

Prioritize based on observed failures:

1. Add console/dmesg capture from the configured `hvc0` named pipe so boot failures are visible.
2. Decide whether the test disk should be VHDX instead of fixed VHD.
3. Add an optional guest test mode that attempts DHCP/static IP configuration and VSOCK connection.
4. Add unit tests for pure logic:
   - `HvSocketPorts.PortToServiceId`
   - IPv4/CIDR parsing
   - CLI parsing/validation
   - HCS JSON generation
   - HCN JSON generation
5. Add stricter analyzer settings once runtime behavior stabilizes.

## Resume prompt for the next session

When resuming on Windows, provide:

- Windows version/build.
- Whether the repo built on Windows.
- Exact command run.
- Full launcher output.
- Whether `boot-ok.txt` appeared on the data disk.
- Any HCS/HCN HRESULT and error JSON.
- Whether VHD or VHDX was used.
