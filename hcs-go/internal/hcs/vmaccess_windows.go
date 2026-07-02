//go:build windows

package hcs

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/cli"
	"github.com/boeing-ai-gateway/discboeing/hcs-go/internal/winapi"
)

type VMAccessGrant struct {
	vmID  string
	files []string
}

func GrantVMAccess(options cli.Options) (*VMAccessGrant, error) {
	grant := &VMAccessGrant{vmID: options.VMID.String()}
	seen := map[string]bool{}
	for _, file := range options.FilesNeedingVMAccess() {
		key := strings.ToLower(file)
		if seen[key] {
			continue
		}
		seen[key] = true
		vmIDPtr, err := winapi.UTF16Ptr(options.VMID.String())
		if err != nil {
			return nil, err
		}
		filePtr, err := winapi.UTF16Ptr(file)
		if err != nil {
			return nil, err
		}
		hr, _, _ := winapi.ProcHcsGrantVMAccess.Call(uintptr(unsafe.Pointer(vmIDPtr)), uintptr(unsafe.Pointer(filePtr)))
		if winapi.Failed(hr) {
			grant.Close()
			return nil, winapi.HRESULTError("HcsGrantVmAccess", hr, file)
		}
		grant.files = append(grant.files, file)
	}
	return grant, nil
}

func (g *VMAccessGrant) Close() error {
	for i := len(g.files) - 1; i >= 0; i-- {
		file := g.files[i]
		vmIDPtr, err := winapi.UTF16Ptr(g.vmID)
		if err != nil {
			fmt.Printf("Warning: HcsRevokeVmAccess failed for '%s': %v\n", file, err)
			continue
		}
		filePtr, err := winapi.UTF16Ptr(file)
		if err != nil {
			fmt.Printf("Warning: HcsRevokeVmAccess failed for '%s': %v\n", file, err)
			continue
		}
		hr, _, _ := winapi.ProcHcsRevokeVMAccess.Call(uintptr(unsafe.Pointer(vmIDPtr)), uintptr(unsafe.Pointer(filePtr)))
		if winapi.Failed(hr) {
			fmt.Printf("Warning: HcsRevokeVmAccess failed for '%s' with HRESULT 0x%08x.\n", file, uint32(hr))
		}
	}
	g.files = nil
	return nil
}
