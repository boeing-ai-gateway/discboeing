package wsl

import (
	"fmt"
	"path"
	"strings"
)

// TranslatePath converts a Windows absolute path into the corresponding WSL path
// used for bind mounts. Paths that are already Unix-style absolute paths are
// returned unchanged.
func TranslatePath(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", nil
	}

	if strings.HasPrefix(source, "/") {
		return path.Clean(source), nil
	}

	if strings.HasPrefix(source, `\\?\`) {
		return "", fmt.Errorf("windows device paths are not supported: %q", source)
	}
	if strings.HasPrefix(source, `\\`) || strings.HasPrefix(source, `//`) {
		return "", fmt.Errorf("UNC paths are not supported: %q", source)
	}
	if !isWindowsDrivePath(source) {
		return "", fmt.Errorf("path must be an absolute Windows path or Unix path: %q", source)
	}

	drive := strings.ToLower(source[:1])
	remainder := strings.ReplaceAll(source[2:], `\`, "/")
	remainder = strings.TrimPrefix(remainder, "/")

	translated := path.Clean("/mnt/" + drive + "/" + remainder)
	if translated == "." {
		translated = "/mnt/" + drive
	}
	return translated, nil
}

func isWindowsDrivePath(source string) bool {
	if len(source) < 3 {
		return false
	}
	drive := source[0]
	return ((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) &&
		source[1] == ':' &&
		(source[2] == '\\' || source[2] == '/')
}
