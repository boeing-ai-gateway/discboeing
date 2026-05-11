//go:build !darwin

// Package vz provides a macOS Virtualization.framework-based implementation of the sandbox.Provider interface.
// This stub file is used on non-darwin platforms where the vz library is not available.
package vz

import (
	"fmt"
	"runtime"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
)

// NewProvider returns an error on non-darwin platforms.
func NewProvider(_ *config.Config, _ *vm.Config, _ vm.SessionProjectResolver, _ vm.ProviderResourceResolver, _ vm.SystemManager) (*vm.Provider, error) {
	return nil, fmt.Errorf("vz sandbox provider is only available on macOS (darwin), current platform: %s", runtime.GOOS)
}
