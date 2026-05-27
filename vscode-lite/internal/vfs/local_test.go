package vfs

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLocalVFSRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	vfs, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	unsafePaths := []string{"/etc/passwd", "../outside", "a/../b", "a//b", "./a"}
	for _, path := range unsafePaths {
		if _, err := vfs.Read(context.Background(), path); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
}

func TestLocalVFSReadWriteList(t *testing.T) {
	root := t.TempDir()
	vfs, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Write(context.Background(), "src/main.go", []byte("package main\n")); err != nil {
		t.Fatal(err)
	}
	read, err := vfs.Read(context.Background(), "src/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if read.Content != "package main\n" {
		t.Fatalf("unexpected content %q", read.Content)
	}
	list, err := vfs.List(context.Background(), "src", ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Entries) != 1 || list.Entries[0].Path != "src/main.go" {
		t.Fatalf("unexpected list: %#v", list.Entries)
	}
}

func TestLocalVFSRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges vary on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	vfs, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vfs.Read(context.Background(), "outside/secret.txt"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}
