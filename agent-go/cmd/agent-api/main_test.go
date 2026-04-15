package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandaloneApplyPatchInput_FromStdin(t *testing.T) {
	got, err := standaloneApplyPatchInput(nil, strings.NewReader("*** Begin Patch\n*** End Patch"))
	if err != nil {
		t.Fatalf("standaloneApplyPatchInput returned error: %v", err)
	}
	if got != "*** Begin Patch\n*** End Patch" {
		t.Fatalf("unexpected stdin patch: %q", got)
	}
}

func TestStandaloneApplyPatchInput_SingleArg(t *testing.T) {
	got, err := standaloneApplyPatchInput([]string{"*** Begin Patch\n*** End Patch"}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("standaloneApplyPatchInput returned error: %v", err)
	}
	if got != "*** Begin Patch\n*** End Patch" {
		t.Fatalf("unexpected arg patch: %q", got)
	}
}

func TestRunStandaloneApplyPatch(t *testing.T) {
	cwd := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	t.Setenv("DISCOBOT_DATA_DIR", t.TempDir())
	t.Setenv("DISCOBOT_THREADS_DIR", filepath.Join(t.TempDir(), "threads"))

	var stdout, stderr bytes.Buffer
	code := runStandaloneApplyPatch(nil, strings.NewReader(`*** Begin Patch
*** Add File: shim.txt
+hello
*** End Patch`), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "A shim.txt") {
		t.Fatalf("expected add summary, got stdout=%q", stdout.String())
	}

	data, err := os.ReadFile(filepath.Join(cwd, "shim.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}
