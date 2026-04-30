package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesResultFileOnFailure(t *testing.T) {
	resultFile := filepath.Join(t.TempDir(), "result.txt")

	exitCode := run([]string{"--result-file", resultFile, "unknown-command"})
	if exitCode == 0 {
		t.Fatal("run() exitCode = 0, want non-zero")
	}

	contents, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(contents), "unknown command") {
		t.Fatalf("result file = %q, want unknown command message", string(contents))
	}
}
