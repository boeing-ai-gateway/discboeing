package wsl

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// DistroInfo describes one entry from `wsl.exe --list --verbose`.
type DistroInfo struct {
	Name      string
	State     string
	Version   int
	IsDefault bool
}

// ParseDistroList parses `wsl.exe --list --verbose` output.
func ParseDistroList(output string) ([]DistroInfo, error) {
	var distros []DistroInfo

	for rawLine := range strings.SplitSeq(output, "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(rawLine, "\ufeff"))
		if line == "" {
			continue
		}

		upper := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(line, "*")))
		if strings.HasPrefix(upper, "NAME") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		version, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			continue
		}

		state := fields[len(fields)-2]
		nameFields := fields[:len(fields)-2]
		isDefault := len(nameFields) > 0 && nameFields[0] == "*"
		if isDefault {
			nameFields = nameFields[1:]
		}
		if len(nameFields) == 0 {
			return nil, fmt.Errorf("invalid WSL distro line %q", rawLine)
		}

		distros = append(distros, DistroInfo{
			Name:      strings.Join(nameFields, " "),
			State:     state,
			Version:   version,
			IsDefault: isDefault,
		})
	}

	return distros, nil
}

func FindDistro(distros []DistroInfo, name string) (DistroInfo, bool) {
	name = strings.TrimSpace(name)
	for _, distro := range distros {
		if strings.EqualFold(strings.TrimSpace(distro.Name), name) {
			return distro, true
		}
	}
	return DistroInfo{}, false
}

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
