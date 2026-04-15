package helperbin

import (
	"os"
	"path/filepath"
	"strings"
)

func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".discobot", "bin")
	}
	return filepath.Join(home, ".discobot", "bin")
}

func ScriptPath(name string) string {
	return filepath.Join(Dir(), name)
}

func PrependToPath(pathValue string) string {
	dir := Dir()
	if dir == "" {
		return pathValue
	}
	parts := filepath.SplitList(pathValue)
	filtered := make([]string, 0, len(parts)+1)
	filtered = append(filtered, dir)
	for _, part := range parts {
		if strings.TrimSpace(part) == "" || part == dir {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, string(os.PathListSeparator))
}
