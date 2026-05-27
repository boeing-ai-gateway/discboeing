//go:build windows

package wsl

import (
	"context"
	"fmt"
)

type distroCommandRunner func(context.Context, string, ...string) (string, error)

func probeDistro(ctx context.Context, distroName string, run distroCommandRunner) (DistroInfo, bool, error) {
	return probeNamedDistro(ctx, distroName, run)
}

func probeNamedDistro(ctx context.Context, distroName string, run distroCommandRunner) (DistroInfo, bool, error) {
	output, err := run(ctx, "wsl.exe", "--list", "--verbose")
	if err != nil {
		return DistroInfo{}, false, err
	}

	distros, err := ParseDistroList(output)
	if err != nil {
		return DistroInfo{}, false, err
	}
	distro, found := FindDistro(distros, distroName)
	return distro, found, nil
}

func checkNamedDistroStillRegistered(ctx context.Context, distroName string, operation string, run distroCommandRunner) error {
	_, found, err := probeNamedDistro(ctx, distroName, run)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("managed WSL distro %q disappeared while %s", distroName, operation)
	}
	return nil
}
