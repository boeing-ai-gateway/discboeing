package assets

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestScriptsSelectsCommitVariant(t *testing.T) {
	local := Scripts("/Users/me/project")
	remote := Scripts("https://github.com/boeing-ai-gateway/discboeing.git")

	localCommit := findScript(t, local, "discboeing-commit")
	remoteCommit := findScript(t, remote, "discboeing-commit")

	if !strings.Contains(string(localCommit.Content), "Discboeing commit context") {
		t.Fatalf("local commit script did not contain local flow")
	}
	if !strings.Contains(string(remoteCommit.Content), "Discboeing remote commit context") {
		t.Fatalf("remote commit script did not contain remote flow")
	}
	if findScript(t, remote, "discboeing-rebase").Name != "discboeing-rebase" {
		t.Fatalf("expected rebase script")
	}
}

func TestInstallSystemScripts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "discboeing-commit-remote"), []byte("stale"), 0o755); err != nil {
		t.Fatalf("write stale remote script: %v", err)
	}
	if err := InstallSystemScripts(dir, "https://github.com/boeing-ai-gateway/discboeing.git"); err != nil {
		t.Fatal(err)
	}

	commitPath := filepath.Join(dir, "discboeing-commit")
	content, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatalf("read commit script: %v", err)
	}
	if !strings.Contains(string(content), "Discboeing remote commit context") {
		t.Fatalf("installed commit script did not contain remote flow")
	}
	if info, err := os.Stat(commitPath); err != nil {
		t.Fatalf("stat commit script: %v", err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("commit script mode = %v; want 0755", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(dir, "discboeing-commit-remote")); !os.IsNotExist(err) {
		t.Fatalf("remote variant should not be installed separately, err=%v", err)
	}
}

func findScript(t *testing.T, scripts []ScriptFile, name string) ScriptFile {
	t.Helper()
	for _, script := range scripts {
		if script.Name == name {
			return script
		}
	}
	t.Fatalf("script %q not found in %#v", name, scripts)
	return ScriptFile{}
}
