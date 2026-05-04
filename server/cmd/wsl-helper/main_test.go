package main

import (
	"context"
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

func TestRunCreateVHDUsesNativeVirtualDiskAPI(t *testing.T) {
	originalCreateDynamicVHD := createDynamicVHD
	t.Cleanup(func() {
		createDynamicVHD = originalCreateDynamicVHD
	})

	called := false
	createDynamicVHD = func(path string, sizeBytes uint64) error {
		called = true
		if path != `C:\temp\var.vhdx` {
			t.Fatalf("createDynamicVHD() path = %q, want %q", path, `C:\temp\var.vhdx`)
		}
		if sizeBytes != 100*1024*1024*1024 {
			t.Fatalf("createDynamicVHD() sizeBytes = %d, want %d", sizeBytes, uint64(100*1024*1024*1024))
		}
		return nil
	}

	err := runCreateVHD(context.Background(), []string{"--path", `C:\temp\var.vhdx`, "--size-gb", "100"})
	if err != nil {
		t.Fatalf("runCreateVHD() error = %v", err)
	}
	if !called {
		t.Fatal("runCreateVHD() did not invoke createDynamicVHD")
	}
}
