package tools

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBuildRgArgs_UsesRegexpFlagForPattern(t *testing.T) {
	got := buildRgArgs(grepInput{
		Pattern:    "--model|--profile|--max-turns",
		OutputMode: "content",
	}, "/tmp/search")

	want := []string{"-n", "--regexp", "--model|--profile|--max-turns", "/tmp/search"}
	if !slices.Equal(got, want) {
		t.Fatalf("buildRgArgs() = %#v, want %#v", got, want)
	}
}

func TestGrep_PatternStartingWithDashWorksWithRipgrep(t *testing.T) {
	if !isRgAvailable() {
		t.Skip("rg not available")
	}

	cwd := t.TempDir()
	content := strings.Join([]string{
		"--model",
		"--profile",
		"--max-turns",
		"--subagent",
		"--new-thread",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(cwd, "flags.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Grep", map[string]any{
		"pattern":     "--model|--profile|--max-turns|--subagent|--new-thread",
		"path":        cwd,
		"output_mode": "content",
	})
	if !ok {
		t.Fatalf("expected successful grep result, got error: %s", out)
	}
	if !strings.Contains(out, "flags.txt:1:--model") {
		t.Fatalf("expected first matched flag in output, got: %q", out)
	}
	if !strings.Contains(out, "flags.txt:5:--new-thread") {
		t.Fatalf("expected last matched flag in output, got: %q", out)
	}
}

// TestGrep_HiddenDirAsRoot_PureGo verifies the pure-Go fallback searches inside
// a hidden directory when it is the explicit search path.
func TestGrep_HiddenDirAsRoot_PureGo(t *testing.T) {
	t.Setenv("DISCOBOT_NO_RIPGREP", "1")

	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".discobot", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".discobot", "hooks", "setup.sh"), []byte("# TODO: setup\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Grep", map[string]any{
		"pattern": "TODO",
		"path":    ".discobot",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "TODO") {
		t.Fatalf("expected match inside .discobot, got: %q", out)
	}
}

// TestGrep_HiddenDirAsRoot_Rg verifies the rg-based path also finds matches
// inside a hidden directory when it is the explicit search path.
func TestGrep_HiddenDirAsRoot_Rg(t *testing.T) {
	if !isRgAvailable() {
		t.Skip("rg not available")
	}

	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".discobot", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".discobot", "hooks", "setup.sh"), []byte("# TODO: setup\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Grep", map[string]any{
		"pattern": "TODO",
		"path":    ".discobot",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "TODO") {
		t.Fatalf("expected match inside .discobot, got: %q", out)
	}
}
