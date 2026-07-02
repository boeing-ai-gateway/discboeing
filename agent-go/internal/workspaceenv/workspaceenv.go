package workspaceenv

import (
	"bufio"
	"errors"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const RelativePath = ".discboeing/env"

// ProcessSnapshot returns the current process environment.
func ProcessSnapshot() map[string]string {
	env := make(map[string]string, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		env[key] = value
	}
	return env
}

// MergeProcessSnapshot returns the current process environment merged with any
// explicit overrides.
func MergeProcessSnapshot(overrides map[string]string) map[string]string {
	env := ProcessSnapshot()
	maps.Copy(env, overrides)
	return env
}

// List returns environment entries in KEY=VALUE form sorted by key.
func List(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

// FileSnapshot returns the latest parsed values from workspace-local
// .discboeing/env. Invalid lines are ignored with a warning.
func FileSnapshot(workspaceRoot string) map[string]string {
	if workspaceRoot == "" {
		return nil
	}

	envPath := filepath.Join(workspaceRoot, RelativePath)
	file, err := os.Open(envPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("warn: workspace env: read %s: %v", envPath, err)
		}
		return nil
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		log.Printf("warn: workspace env: stat %s: %v", envPath, err)
		return nil
	}
	if info.IsDir() {
		log.Printf("warn: workspace env: %s is a directory", envPath)
		return nil
	}

	env := map[string]string{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if key, value, ok := parseLine(envPath, lineNumber, line); ok {
			env[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("warn: workspace env: scan %s: %v", envPath, err)
	}
	return env
}

func parseLine(envPath string, lineNumber int, line string) (string, string, bool) {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	if after, ok := strings.CutPrefix(trimmed, "export "); ok {
		trimmed = strings.TrimLeftFunc(after, unicode.IsSpace)
	}

	key, value, ok := strings.Cut(trimmed, "=")
	if !ok {
		logInvalidLine(envPath, lineNumber)
		return "", "", false
	}

	key = strings.TrimRightFunc(key, unicode.IsSpace)
	value = strings.TrimLeftFunc(value, unicode.IsSpace)
	if !validKey(key) {
		logInvalidLine(envPath, lineNumber)
		return "", "", false
	}

	normalized, ok := normalizeValue(value)
	if !ok {
		logInvalidLine(envPath, lineNumber)
		return "", "", false
	}
	return key, normalized, true
}

func validKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func normalizeValue(value string) (string, bool) {
	if len(value) < 2 {
		switch value {
		case "\"", "'":
			return "", false
		default:
			return value, true
		}
	}

	if strings.HasPrefix(value, "\"") {
		if !strings.HasSuffix(value, "\"") {
			return "", false
		}
		return strings.TrimSuffix(strings.TrimPrefix(value, "\""), "\""), true
	}
	if strings.HasPrefix(value, "'") {
		if !strings.HasSuffix(value, "'") {
			return "", false
		}
		return strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'"), true
	}
	return value, true
}

func logInvalidLine(envPath string, lineNumber int) {
	log.Printf("warn: workspace env: ignoring invalid env line %d in %s", lineNumber, envPath)
}
