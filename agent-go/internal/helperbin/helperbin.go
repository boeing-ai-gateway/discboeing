package helperbin

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Dir() string {
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home != "" {
		return filepath.Join(home, ".discobot", "bin")
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".discobot", "bin")
	}
	return filepath.Join(home, ".discobot", "bin")
}

func ScriptPath(name string) string {
	return ScriptPathForOS(runtime.GOOS, name)
}

func ScriptPathForOS(goos, name string) string {
	if goos == "windows" && filepath.Ext(name) == "" {
		name += ".ps1"
	}
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
