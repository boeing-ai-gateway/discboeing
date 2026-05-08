package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeResultPath(t *testing.T) {
	t.Run("empty becomes dot", func(t *testing.T) {
		if got := normalizeResultPath(""); got != "." {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("dot stays dot", func(t *testing.T) {
		if got := normalizeResultPath("."); got != "." {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("windows separators become slashes", func(t *testing.T) {
		if got := normalizeResultPath(`artifacts\browser\sha256\shot.png`); got != "artifacts/browser/sha256/shot.png" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestTildePathsResolveUnderHome(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	homeRoot := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(homeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeRoot)

	write, fileErr := WriteFile("~/.discobot/editor/control.json", `{"type":"openFile"}`, "utf8", workspaceRoot)
	if fileErr != nil {
		t.Fatalf("WriteFile() error = %v", fileErr)
	}
	if write.Path != "~/.discobot/editor/control.json" {
		t.Fatalf("WriteFile() path = %q", write.Path)
	}

	homeFile := filepath.Join(homeRoot, ".discobot", "editor", "control.json")
	if _, err := os.Stat(homeFile); err != nil {
		t.Fatalf("expected home file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, ".discobot", "editor", "control.json")); !os.IsNotExist(err) {
		t.Fatalf("expected workspace file to be absent, got err %v", err)
	}

	read, fileErr := ReadFile("~/.discobot/editor/control.json", workspaceRoot)
	if fileErr != nil {
		t.Fatalf("ReadFile() error = %v", fileErr)
	}
	if read.Path != "~/.discobot/editor/control.json" {
		t.Fatalf("ReadFile() path = %q", read.Path)
	}
	if read.Content != `{"type":"openFile"}` {
		t.Fatalf("ReadFile() content = %q", read.Content)
	}

	list, fileErr := ListDirectory("~/.discobot/editor", workspaceRoot, true)
	if fileErr != nil {
		t.Fatalf("ListDirectory() error = %v", fileErr)
	}
	if list.Path != "~/.discobot/editor" {
		t.Fatalf("ListDirectory() path = %q", list.Path)
	}
	if len(list.Entries) != 1 || list.Entries[0].Name != "control.json" {
		t.Fatalf("ListDirectory() entries = %#v", list.Entries)
	}
}

func TestTildePathsCannotEscapeHome(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	homeRoot := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(homeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeRoot)

	if _, err := ValidatePath("~/../escape", workspaceRoot); err == nil {
		t.Fatal("ValidatePath() expected traversal error")
	}
}

func TestRelativePathsStillResolveUnderWorkspace(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	homeRoot := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(homeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", homeRoot)

	write, fileErr := WriteFile("notes/todo.txt", "ship it", "utf8", workspaceRoot)
	if fileErr != nil {
		t.Fatalf("WriteFile() error = %v", fileErr)
	}
	if write.Path != "notes/todo.txt" {
		t.Fatalf("WriteFile() path = %q", write.Path)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, "notes", "todo.txt")); err != nil {
		t.Fatalf("expected workspace file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeRoot, "notes", "todo.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected home file to be absent, got err %v", err)
	}
}

func TestRootOperationsRejectSymlinkEscapes(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	outsideRoot := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	outsideFile := filepath.Join(outsideRoot, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(workspaceRoot, "secret-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(workspaceRoot, "outside-link")); err != nil {
		t.Fatal(err)
	}

	if _, fileErr := ReadFile("secret-link", workspaceRoot); fileErr == nil {
		t.Fatal("ReadFile() expected symlink escape error")
	}
	if _, fileErr := ListDirectory("outside-link", workspaceRoot, true); fileErr == nil {
		t.Fatal("ListDirectory() expected symlink escape error")
	}
	if _, fileErr := WriteFile("secret-link", "changed", "utf8", workspaceRoot); fileErr == nil {
		t.Fatal("WriteFile() expected symlink escape error")
	}

	content, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "secret" {
		t.Fatalf("outside file content = %q", content)
	}
}
