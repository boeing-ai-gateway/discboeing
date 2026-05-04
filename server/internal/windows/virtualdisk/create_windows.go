//go:build windows

package virtualdisk

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	createVirtualDiskVersion1 = 1
	createVirtualDiskFlagNone = 0
	virtualDiskAccessNone     = 0
	virtualStorageTypeVHDX    = 3
	minimumVirtualDiskSize    = 3 * 1024 * 1024
	defaultBlockSizeInBytes   = 0
	defaultSectorSizeInBytes  = 0
)

var (
	virtdiskDLL           = windows.NewLazySystemDLL("virtdisk.dll")
	createVirtualDiskProc = virtdiskDLL.NewProc("CreateVirtualDisk")

	virtualStorageVendorMicrosoft = windows.GUID{
		Data1: 0xec984aec,
		Data2: 0xa0f9,
		Data3: 0x47e9,
		Data4: [8]byte{0x90, 0x1f, 0x71, 0x41, 0x5a, 0x66, 0x34, 0x5b},
	}
)

type virtualStorageType struct {
	DeviceID uint32
	VendorID windows.GUID
}

type createVirtualDiskParametersVersion1 struct {
	UniqueID          windows.GUID
	MaximumSize       uint64
	BlockSizeInBytes  uint32
	SectorSizeInBytes uint32
	ParentPath        *uint16
	SourcePath        *uint16
}

type createVirtualDiskParameters struct {
	Version  uint32
	Version1 createVirtualDiskParametersVersion1
}

// CreateDynamicVHDX creates a sparse VHDX file without depending on Hyper-V
// PowerShell modules. The caller must provide a size that is valid for the
// Windows virtual disk API.
func CreateDynamicVHDX(path string, sizeBytes uint64) error {
	if sizeBytes < minimumVirtualDiskSize {
		return fmt.Errorf("virtual disk size must be at least %d bytes", minimumVirtualDiskSize)
	}
	if sizeBytes%512 != 0 {
		return fmt.Errorf("virtual disk size must be a multiple of 512 bytes")
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode virtual disk path %q: %w", path, err)
	}

	storageType := virtualStorageType{
		DeviceID: virtualStorageTypeVHDX,
		VendorID: virtualStorageVendorMicrosoft,
	}
	params := createVirtualDiskParameters{
		Version: createVirtualDiskVersion1,
		Version1: createVirtualDiskParametersVersion1{
			MaximumSize:       sizeBytes,
			BlockSizeInBytes:  defaultBlockSizeInBytes,
			SectorSizeInBytes: defaultSectorSizeInBytes,
		},
	}

	var handle windows.Handle
	result, _, _ := createVirtualDiskProc.Call(
		uintptr(unsafe.Pointer(&storageType)),
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(virtualDiskAccessNone),
		0,
		uintptr(createVirtualDiskFlagNone),
		0,
		uintptr(unsafe.Pointer(&params)),
		0,
		uintptr(unsafe.Pointer(&handle)),
	)
	if handle != 0 {
		defer func() {
			_ = windows.CloseHandle(handle)
		}()
	}
	if result != 0 {
		return fmt.Errorf("CreateVirtualDisk(%q): %w", path, windows.Errno(result))
	}
	return nil
}
