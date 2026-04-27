package wsl

import (
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
		isDefault := false
		if len(nameFields) > 0 && nameFields[0] == "*" {
			isDefault = true
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
	for _, distro := range distros {
		if strings.EqualFold(strings.TrimSpace(distro.Name), strings.TrimSpace(name)) {
			return distro, true
		}
	}
	return DistroInfo{}, false
}
