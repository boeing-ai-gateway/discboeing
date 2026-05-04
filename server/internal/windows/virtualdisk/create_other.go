//go:build !windows

package virtualdisk

import "fmt"

func CreateDynamicVHDX(_ string, _ uint64) error {
	return fmt.Errorf("create dynamic VHDX is only supported on Windows")
}
