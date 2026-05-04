//go:build !windows

package virtualdisk

import "fmt"

func CreateDynamicVHDX(path string, sizeBytes uint64) error {
	return fmt.Errorf("CreateDynamicVHDX is only supported on Windows")
}
