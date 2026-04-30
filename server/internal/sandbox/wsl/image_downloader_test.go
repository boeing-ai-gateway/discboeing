package wsl

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImageDownloaderCheckCache(t *testing.T) {
	tempDir := t.TempDir()
	downloader := NewImageDownloader(ImageDownloadConfig{
		ImageRef: "ghcr.io/obot-platform/discobot-wsl:test",
		DataDir:  tempDir,
	})

	digest := downloader.computeDigest()
	cacheDir := filepath.Join(tempDir, "images", digest)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, rootfsArchiveName), []byte("zstd"), 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "manifest.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}

	artifact, ok, err := downloader.checkCache()
	if err != nil {
		t.Fatalf("checkCache() error = %v", err)
	}
	if !ok {
		t.Fatal("checkCache() ok = false, want true")
	}
	if artifact.RootfsArchive != filepath.Join(cacheDir, rootfsArchiveName) {
		t.Fatalf("unexpected rootfs path %q", artifact.RootfsArchive)
	}
}

func TestImageDownloaderExtractFiles(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	payload := []byte("archive-bytes")
	if err := tw.WriteHeader(&tar.Header{Name: "nested/" + rootfsArchiveName, Mode: 0644, Size: int64(len(payload))}); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	destDir := t.TempDir()
	found := false
	downloader := NewImageDownloader(ImageDownloadConfig{})
	if err := downloader.extractFiles(bytes.NewReader(buf.Bytes()), destDir, &found); err != nil {
		t.Fatalf("extractFiles() error = %v", err)
	}
	if !found {
		t.Fatal("extractFiles() found = false, want true")
	}
	got, err := os.ReadFile(filepath.Join(destDir, rootfsArchiveName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("extractFiles() wrote %q, want %q", string(got), string(payload))
	}
}

func TestImageDownloaderEnsureRootfsUsesLocalArchive(t *testing.T) {
	tempDir := t.TempDir()
	rootfsPath := filepath.Join(tempDir, rootfsArchiveName)
	if err := os.WriteFile(rootfsPath, []byte("local-rootfs"), 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}

	downloader := NewImageDownloader(ImageDownloadConfig{
		DataDir:            tempDir,
		LocalRootfsArchive: rootfsPath,
	})

	artifact, err := downloader.EnsureRootfs(context.Background())
	if err != nil {
		t.Fatalf("EnsureRootfs() error = %v", err)
	}
	if artifact.RootfsArchive != rootfsPath {
		t.Fatalf("EnsureRootfs() rootfs = %q, want %q", artifact.RootfsArchive, rootfsPath)
	}
}

func TestImageDownloaderEnsureRootfsUsesLocalArchiveReportsProgress(t *testing.T) {
	tempDir := t.TempDir()
	rootfsPath := filepath.Join(tempDir, rootfsArchiveName)
	if err := os.WriteFile(rootfsPath, []byte("local-rootfs"), 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}

	downloader := NewImageDownloader(ImageDownloadConfig{
		DataDir:            tempDir,
		LocalRootfsArchive: rootfsPath,
	})

	var operations []string
	_, err := downloader.EnsureRootfsWithProgress(context.Background(), func(progress ImageDownloadProgress) {
		operations = append(operations, progress.CurrentOperation)
	})
	if err != nil {
		t.Fatalf("EnsureRootfsWithProgress() error = %v", err)
	}
	if len(operations) != 1 || operations[0] != "Using local WSL rootfs archive" {
		t.Fatalf("EnsureRootfsWithProgress() operations = %#v, want local archive progress", operations)
	}
}

func TestImageDownloaderEnsureRootfsUsesCacheReportsProgress(t *testing.T) {
	tempDir := t.TempDir()
	downloader := NewImageDownloader(ImageDownloadConfig{
		ImageRef: "ghcr.io/obot-platform/discobot-wsl:test",
		DataDir:  tempDir,
	})

	digest := downloader.computeDigest()
	cacheDir := filepath.Join(tempDir, "images", digest)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, rootfsArchiveName), []byte("zstd"), 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "manifest.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}

	var operations []string
	artifact, err := downloader.EnsureRootfsWithProgress(context.Background(), func(progress ImageDownloadProgress) {
		operations = append(operations, progress.CurrentOperation)
	})
	if err != nil {
		t.Fatalf("EnsureRootfsWithProgress() error = %v", err)
	}
	if artifact.RootfsArchive != filepath.Join(cacheDir, rootfsArchiveName) {
		t.Fatalf("EnsureRootfsWithProgress() rootfs = %q, want %q", artifact.RootfsArchive, filepath.Join(cacheDir, rootfsArchiveName))
	}
	if len(operations) != 1 || operations[0] != "Using cached WSL rootfs artifact" {
		t.Fatalf("EnsureRootfsWithProgress() operations = %#v, want cached artifact progress", operations)
	}
}

func TestImageDownloaderEnsureRootfsRejectsEmptyLocalArchive(t *testing.T) {
	tempDir := t.TempDir()
	rootfsPath := filepath.Join(tempDir, rootfsArchiveName)
	if err := os.WriteFile(rootfsPath, nil, 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}

	downloader := NewImageDownloader(ImageDownloadConfig{
		DataDir:            tempDir,
		LocalRootfsArchive: rootfsPath,
	})

	if _, err := downloader.EnsureRootfs(context.Background()); err == nil {
		t.Fatal("EnsureRootfs() error = nil, want non-nil")
	}
}
