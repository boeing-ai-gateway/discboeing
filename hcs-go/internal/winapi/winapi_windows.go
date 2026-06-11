//go:build windows

package winapi

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

const Infinite = 0xffffffff

var (
	kernel32       = windows.NewLazySystemDLL("kernel32.dll")
	computeCore    = windows.NewLazySystemDLL("ComputeCore.dll")
	computeNetwork = windows.NewLazySystemDLL("ComputeNetwork.dll")
	ws2            = windows.NewLazySystemDLL("Ws2_32.dll")

	procLocalFree = kernel32.NewProc("LocalFree")

	ProcHcsCreateOperation            = computeCore.NewProc("HcsCreateOperation")
	ProcHcsCloseOperation             = computeCore.NewProc("HcsCloseOperation")
	ProcHcsCreateComputeSystem        = computeCore.NewProc("HcsCreateComputeSystem")
	ProcHcsCloseComputeSystem         = computeCore.NewProc("HcsCloseComputeSystem")
	ProcHcsStartComputeSystem         = computeCore.NewProc("HcsStartComputeSystem")
	ProcHcsTerminateComputeSystem     = computeCore.NewProc("HcsTerminateComputeSystem")
	ProcHcsModifyComputeSystem        = computeCore.NewProc("HcsModifyComputeSystem")
	ProcHcsGetComputeSystemProperties = computeCore.NewProc("HcsGetComputeSystemProperties")
	ProcHcsWaitForOperationResult     = computeCore.NewProc("HcsWaitForOperationResult")
	ProcHcsGrantVMAccess              = computeCore.NewProc("HcsGrantVmAccess")
	ProcHcsRevokeVMAccess             = computeCore.NewProc("HcsRevokeVmAccess")
	ProcHcnCreateNetwork              = computeNetwork.NewProc("HcnCreateNetwork")
	ProcHcnOpenNetwork                = computeNetwork.NewProc("HcnOpenNetwork")
	ProcHcnCloseNetwork               = computeNetwork.NewProc("HcnCloseNetwork")
	ProcHcnDeleteNetwork              = computeNetwork.NewProc("HcnDeleteNetwork")
	ProcHcnCreateEndpoint             = computeNetwork.NewProc("HcnCreateEndpoint")
	ProcHcnCloseEndpoint              = computeNetwork.NewProc("HcnCloseEndpoint")
	ProcHcnDeleteEndpoint             = computeNetwork.NewProc("HcnDeleteEndpoint")
	ProcHcnQueryEndpointProperties    = computeNetwork.NewProc("HcnQueryEndpointProperties")
	ProcWSAStartup                    = ws2.NewProc("WSAStartup")
	ProcWSACleanup                    = ws2.NewProc("WSACleanup")
	ProcWSAGetLastError               = ws2.NewProc("WSAGetLastError")
	ProcWSASocketW                    = ws2.NewProc("WSASocketW")
	ProcBind                          = ws2.NewProc("bind")
	ProcListen                        = ws2.NewProc("listen")
	ProcIoctlSocket                   = ws2.NewProc("ioctlsocket")
	ProcAccept                        = ws2.NewProc("accept")
	ProcRecv                          = ws2.NewProc("recv")
	ProcSend                          = ws2.NewProc("send")
	ProcCloseSocket                   = ws2.NewProc("closesocket")
)

func GUIDFromUUID(id uuid.UUID) windows.GUID {
	guid := windows.GUID{
		Data1: binary.BigEndian.Uint32(id[0:4]),
		Data2: binary.BigEndian.Uint16(id[4:6]),
		Data3: binary.BigEndian.Uint16(id[6:8]),
	}
	copy(guid.Data4[:], id[8:16])
	return guid
}

func UTF16Ptr(value string) (*uint16, error) {
	return windows.UTF16PtrFromString(value)
}

func OptionalUTF16Ptr(value string) (uintptr, error) {
	if value == "" {
		return 0, nil
	}
	ptr, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return 0, err
	}
	return uintptr(unsafe.Pointer(ptr)), nil
}

func ConsumeNativeString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	defer procLocalFree.Call(ptr)
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr)))
}

func HRESULTError(operation string, hr uintptr, detail string) error {
	if detail != "" {
		return fmt.Errorf("%s failed with HRESULT 0x%08x: %s", operation, uint32(hr), detail)
	}
	return fmt.Errorf("%s failed with HRESULT 0x%08x", operation, uint32(hr))
}

func Failed(hr uintptr) bool {
	return int32(uint32(hr)) < 0
}
