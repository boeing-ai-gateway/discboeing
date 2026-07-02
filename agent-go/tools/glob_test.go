package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkfile is a helper that creates a file (and any needed parent dirs) with the given content.
func mkfile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGlob_SkipsNodeModules verifies that node_modules directories (including
// those reached via symlinks) are excluded from glob results. This prevents
// thousands of package files from polluting results for patterns like **/*.ts.
func TestGlob_SkipsNodeModules(t *testing.T) {
	cwd := t.TempDir()

	// Create a real source file.
	if err := os.MkdirAll(filepath.Join(cwd, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "src", "app.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a node_modules directory with a matching file inside.
	if err := os.MkdirAll(filepath.Join(cwd, "node_modules", "some-pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "node_modules", "some-pkg", "index.ts"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Glob", map[string]any{
		"path":    cwd,
		"pattern": "**/*.ts",
	})
	if !ok {
		t.Fatalf("expected successful glob result, got error: %s", out)
	}

	if !strings.Contains(out, "src/app.ts") {
		t.Fatalf("expected source file to match, got: %q", out)
	}
	if strings.Contains(out, "node_modules") {
		t.Fatalf("expected node_modules to be excluded, got: %q", out)
	}
}

func TestGlob_DoubleStarMatchesRecursively(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "level1", "level2"), 0o755); err != nil {
		t.Fatal(err)
	}

	topLevelMatch := filepath.Join(cwd, "sandbox-root.txt")
	if err := os.WriteFile(topLevelMatch, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	nestedMatch := filepath.Join(cwd, "level1", "level2", "sandbox-nested.txt")
	if err := os.WriteFile(nestedMatch, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	nonMatch := filepath.Join(cwd, "level1", "level2", "regular.txt")
	if err := os.WriteFile(nonMatch, []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Glob", map[string]any{
		"path":    cwd,
		"pattern": "**/*sandbox*",
	})
	if !ok {
		t.Fatalf("expected successful glob result, got error: %s", out)
	}

	if !strings.Contains(out, "sandbox-root.txt") {
		t.Fatalf("expected top-level sandbox file to match, got: %q", out)
	}
	if !strings.Contains(out, "level1/level2/sandbox-nested.txt") {
		t.Fatalf("expected nested sandbox file to match recursively, got: %q", out)
	}
	if strings.Contains(out, "regular.txt") {
		t.Fatalf("expected non-matching file to be excluded, got: %q", out)
	}
}

// TestGlob_HiddenDirViaPattern verifies that a pattern explicitly rooted at a
// hidden directory (e.g. ".discboeing/**/*") finds files inside that directory.
// Regression: the walk was skipping all hidden directories unconditionally,
// so ".discboeing" was never descended into even when named explicitly in the pattern.
func TestGlob_HiddenDirViaPattern(t *testing.T) {
	cwd := t.TempDir()
	mkfile(t, cwd, ".discboeing/hooks/01-setup.sh", "#!/bin/sh")
	mkfile(t, cwd, ".discboeing/services/ui.sh", "#!/bin/sh")
	mkfile(t, cwd, "src/main.go", "package main")

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Glob", map[string]any{
		"pattern": ".discboeing/**/*",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}

	if !strings.Contains(out, "hooks/01-setup.sh") {
		t.Fatalf("expected .discboeing/hooks/01-setup.sh in results, got: %q", out)
	}
	if !strings.Contains(out, "services/ui.sh") {
		t.Fatalf("expected .discboeing/services/ui.sh in results, got: %q", out)
	}
	if strings.Contains(out, "src/main.go") {
		t.Fatalf("expected non-.discboeing file to be excluded, got: %q", out)
	}
}

// TestGlob_HiddenDirViaPath verifies that setting path to a hidden directory
// and using a wildcard pattern finds files inside it.
// Regression: when the walk root was itself a hidden directory, the skip
// condition fired on the root entry and aborted the entire walk immediately.
func TestGlob_HiddenDirViaPath(t *testing.T) {
	cwd := t.TempDir()
	mkfile(t, cwd, ".discboeing/hooks/01-setup.sh", "#!/bin/sh")
	mkfile(t, cwd, ".discboeing/services/ui.sh", "#!/bin/sh")

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Glob", map[string]any{
		"path":    ".discboeing",
		"pattern": "**/*",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}

	if !strings.Contains(out, "hooks/01-setup.sh") {
		t.Fatalf("expected hooks/01-setup.sh in results, got: %q", out)
	}
	if !strings.Contains(out, "services/ui.sh") {
		t.Fatalf("expected services/ui.sh in results, got: %q", out)
	}
}

// TestGlob_HiddenSubdirsStillSkipped verifies that hidden subdirectories are
// still skipped when they are not explicitly targeted by the pattern or path.
// For example, a pattern like "**/*.go" should not descend into ".cache".
func TestGlob_HiddenSubdirsStillSkipped(t *testing.T) {
	cwd := t.TempDir()
	mkfile(t, cwd, "src/main.go", "package main")
	mkfile(t, cwd, ".cache/build/main.go", "package main") // should be skipped

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Glob", map[string]any{
		"pattern": "**/*.go",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}

	if !strings.Contains(out, "src/main.go") {
		t.Fatalf("expected src/main.go in results, got: %q", out)
	}
	if strings.Contains(out, ".cache") {
		t.Fatalf("expected .cache to be skipped, got: %q", out)
	}
}
