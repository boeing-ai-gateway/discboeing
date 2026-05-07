package sessionconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverInstructions_SingleCLAUDEMD(t *testing.T) {
	dir := t.TempDir()

	// Create a CLAUDE.md at the project root.
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "Project instructions here.")
	// Create .git to mark project root.
	mkdirAll(t, filepath.Join(dir, ".git"))

	entries, err := discoverInstructions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Content != "Project instructions here." {
		t.Errorf("content = %q, want %q", entries[0].Content, "Project instructions here.")
	}
	if entries[0].Description != "project instructions, checked into the codebase" {
		t.Errorf("description = %q", entries[0].Description)
	}
}

func TestDiscoverInstructions_PrefersFirstMatchingFilePerDirectory(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))

	writeFile(t, filepath.Join(root, "CLAUDE.md"), "Root CLAUDE.md")
	writeFile(t, filepath.Join(root, "AGENTS.md"), "Root AGENTS.md")

	mkdirAll(t, filepath.Join(root, ".claude"))
	writeFile(t, filepath.Join(root, ".claude", "CLAUDE.md"), "Dot-claude CLAUDE.md")

	entries, err := discoverInstructions(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Content != "Root AGENTS.md" {
		t.Fatalf("content = %q, want %q", entries[0].Content, "Root AGENTS.md")
	}
	if entries[0].Description != "agent instructions, checked into the codebase" {
		t.Errorf("AGENTS.md description = %q", entries[0].Description)
	}
}

func TestDiscoverInstructions_GEMINIMDProviderFallback(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	writeFile(t, filepath.Join(root, "GEMINI.md"), "Gemini instructions")

	entries, err := discoverInstructions(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "GEMINI.md" {
		t.Fatalf("path = %q, want GEMINI.md", entries[0].Path)
	}
	if entries[0].Content != "Gemini instructions" {
		t.Fatalf("content = %q, want Gemini instructions", entries[0].Content)
	}
}

func TestDiscoverInstructions_AGENTSPrecedesProviderFallbacks(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	writeFile(t, filepath.Join(root, "AGENTS.md"), "Agents instructions")
	writeFile(t, filepath.Join(root, "GEMINI.md"), "Gemini instructions")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "Claude instructions")

	entries, err := discoverInstructions(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "AGENTS.md" {
		t.Fatalf("path = %q, want AGENTS.md", entries[0].Path)
	}
	if entries[0].Content != "Agents instructions" {
		t.Fatalf("content = %q, want Agents instructions", entries[0].Content)
	}
}

func TestDiscoverInstructions_WalkUpDirectory(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "Root instructions")

	subdir := filepath.Join(root, "src", "deep")
	mkdirAll(t, subdir)
	writeFile(t, filepath.Join(subdir, "CLAUDE.md"), "Deep instructions")
	writeFile(t, filepath.Join(subdir, "AGENTS.md"), "Deep agents")

	entries, err := discoverInstructions(subdir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Closest directory comes first, and AGENTS.md wins within that directory.
	if entries[0].Content != "Deep agents" {
		t.Errorf("first entry = %q, want %q", entries[0].Content, "Deep agents")
	}
	if entries[1].Content != "Root instructions" {
		t.Errorf("second entry = %q, want %q", entries[1].Content, "Root instructions")
	}
}

func TestDiscoverInstructions_Rules(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))

	rulesDir := filepath.Join(root, ".discobot", "rules")
	mkdirAll(t, rulesDir)
	writeFile(t, filepath.Join(rulesDir, "formatting.md"), "Use tabs.")
	writeFile(t, filepath.Join(rulesDir, "testing.md"), "Write tests.")

	entries, err := discoverInstructions(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 rule entries, got %d", len(entries))
	}

	// Rules should be sorted alphabetically.
	if entries[0].Content != "Use tabs." {
		t.Errorf("first rule content = %q", entries[0].Content)
	}
	if entries[0].Path != ".discobot/rules/formatting.md" {
		t.Errorf("first rule path = %q", entries[0].Path)
	}
	if entries[0].Description != "project rule" {
		t.Errorf("first rule description = %q", entries[0].Description)
	}
	if entries[1].Content != "Write tests." {
		t.Errorf("second rule content = %q", entries[1].Content)
	}
}

func TestDiscoverInstructions_SystemFiles(t *testing.T) {
	root := t.TempDir()
	systemRoot := t.TempDir()
	originalRoots := discobotSystemRoots
	discobotSystemRoots = []string{systemRoot}
	t.Cleanup(func() { discobotSystemRoots = originalRoots })
	mkdirAll(t, filepath.Join(root, ".git"))

	writeFile(t, filepath.Join(systemRoot, "CLAUDE.md"), "System instructions")
	rulesDir := filepath.Join(systemRoot, "rules")
	mkdirAll(t, rulesDir)
	writeFile(t, filepath.Join(rulesDir, "policy.md"), "System rule")

	entries, err := discoverInstructions(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Path != filepath.Join(systemRoot, "CLAUDE.md") {
		t.Fatalf("first path = %q", entries[0].Path)
	}
	if entries[0].Description != "system-level instructions" {
		t.Fatalf("first description = %q", entries[0].Description)
	}
	if entries[1].Path != filepath.Join(systemRoot, "rules", "policy.md") {
		t.Fatalf("second path = %q", entries[1].Path)
	}
	if entries[1].Description != "system rule" {
		t.Fatalf("second description = %q", entries[1].Description)
	}
}

func TestInstructionDisplayPath(t *testing.T) {
	t.Run("relative prefix uses slash path", func(t *testing.T) {
		got := instructionDisplayPath(".discobot/rules", "policy.md")
		if got != ".discobot/rules/policy.md" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("windows absolute prefix keeps windows separators", func(t *testing.T) {
		got := instructionDisplayPath(`C:\discobot\rules`, "policy.md")
		if got != `C:\discobot\rules\policy.md` {
			t.Fatalf("got %q", got)
		}
	})
}

func TestDiscoverInstructions_NoFiles(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, ".git"))

	entries, err := discoverInstructions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestFindProjectRoot_WithGit(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	subdir := filepath.Join(root, "a", "b", "c")
	mkdirAll(t, subdir)

	got := findProjectRoot(subdir)
	if got != root {
		t.Errorf("findProjectRoot(%s) = %s, want %s", subdir, got, root)
	}
}

func TestFindProjectRoot_NoGit(t *testing.T) {
	dir := t.TempDir()
	got := findProjectRoot(dir)
	if got != dir {
		t.Errorf("findProjectRoot(%s) = %s, want %s (self)", dir, got, dir)
	}
}

// helpers

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
