# HCS Linux VM Launcher (Go port)

This is a pure Go port of `../hcs`. It keeps the same core launcher shape:

- generate HCS compute-system JSON for Linux direct kernel boot
- attach read-only root and read/write data VHD/VHDX disks
- optionally add Plan9/9P shares
- optionally attach HCN ICS/NAT networking
- optionally run gvproxy user-mode networking over Hyper-V sockets
- keep VM lifetime tied to the launcher process

The native HCS, HCN, and AF_HYPERV pieces are Windows-only and live behind
`//go:build windows` files. Non-Windows hosts can still build and test the
shared code and use `--dry-run` to inspect generated JSON.

## Validate on Linux

```bash
go test ./...
GOOS=windows GOARCH=amd64 go test -exec=true ./...
go run ./cmd/hcs-linux-vm-launcher \
  --root 'C:\vm\root.vhdx' \
  --data 'C:\vm\data.vhdx' \
  --network hcn-nat \
  --dry-run
```

`GOOS=windows ... -exec=true` compiles the Windows-specific code from Linux
without trying to execute the produced Windows test binaries.

## Run on Windows

```powershell
go run ./cmd/hcs-linux-vm-launcher `
  --root C:\vm\root.vhdx `
  --data C:\vm\data.vhdx `
  --network hcn-nat
```

Most launch options match the C# launcher. Use `--help` for the current list.
Actual VM launch validation still needs a Windows host with Hyper-V / Virtual
Machine Platform, WSL kernel assets, and suitable VHD/VHDX files.
