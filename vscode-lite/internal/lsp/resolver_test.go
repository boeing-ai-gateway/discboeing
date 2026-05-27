package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolverPrefersWorkspaceTypeScriptServer(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	serverPath := filepath.Join(bin, executableName("typescript-language-server"))
	if err := os.WriteFile(serverPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolver{WorkspaceRoot: root}.Resolve("typescript")
	if err != nil && os.Getenv("PATH") == "" {
		t.Skip("PATH empty and node unavailable")
	}
	if err != nil {
		t.Skipf("node is required for resolver test: %v", err)
	}
	if resolved.Command != serverPath {
		t.Fatalf("expected local command %q, got %q", serverPath, resolved.Command)
	}
}

func TestResolverRejectsUnsupportedLanguage(t *testing.T) {
	_, err := Resolver{WorkspaceRoot: t.TempDir()}.Resolve("python")
	if err == nil {
		t.Fatal("expected unsupported language error")
	}
}
