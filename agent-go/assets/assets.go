package assets

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed scripts/*
var assets embed.FS

// ScriptFile is an embedded executable script.
type ScriptFile struct {
	Name    string
	Content []byte
}

// InstallSystemScripts writes built-in Discobot scripts into dir. The commit
// script is selected to match local-folder or remote-git workspace sessions.
func InstallSystemScripts(dir, workspaceSource string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("scripts dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create scripts dir %s: %w", dir, err)
	}
	if err := os.Remove(filepath.Join(dir, "discobot-commit-remote")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale remote commit script: %w", err)
	}

	for _, script := range Scripts(workspaceSource) {
		path := filepath.Join(dir, script.Name)
		if err := os.WriteFile(path, script.Content, 0o755); err != nil {
			return fmt.Errorf("write script %s: %w", path, err)
		}
	}
	return nil
}

// Scripts returns built-in Discobot scripts from memory.
func Scripts(workspaceSource string) []ScriptFile {
	commitPath := "scripts/discobot-commit"
	if isGitURL(workspaceSource) {
		commitPath = "scripts/discobot-commit-remote"
	}

	var scripts []ScriptFile
	for _, path := range []string{commitPath, "scripts/discobot-rebase"} {
		content, err := assets.ReadFile(path)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if name == "discobot-commit-remote" {
			name = "discobot-commit"
		}
		scripts = append(scripts, ScriptFile{Name: name, Content: content})
	}
	return scripts
}

func isGitURL(source string) bool {
	source = strings.TrimSpace(source)
	return strings.HasPrefix(source, "git://") ||
		strings.HasPrefix(source, "ssh://") ||
		strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "git@") ||
		strings.HasSuffix(source, ".git")
}
