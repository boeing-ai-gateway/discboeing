package wsl

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
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

func TestImageDownloaderDownloadCachesRootfsArtifact(t *testing.T) {
	image := testImageWithLayer(t, map[string][]byte{
		"usr/share/discobot/" + rootfsArchiveName: []byte("downloaded-rootfs"),
		"ignored.txt": []byte("ignored"),
	})
	restore := replaceImageDescriptor(t, image, nil)
	defer restore()

	tempDir := t.TempDir()
	downloader := NewImageDownloader(ImageDownloadConfig{
		ImageRef: "example.com/discobot/wsl:test",
		DataDir:  tempDir,
	})

	var operations []string
	artifact, err := downloader.download(context.Background(), func(progress ImageDownloadProgress) {
		operations = append(operations, progress.CurrentOperation)
	})
	if err != nil {
		t.Fatalf("download() error = %v", err)
	}

	wantDigest := computeImageRefDigest("example.com/discobot/wsl:test")
	wantCacheDir := filepath.Join(tempDir, "images", wantDigest)
	if artifact.Digest != wantDigest {
		t.Fatalf("download() digest = %q, want %q", artifact.Digest, wantDigest)
	}
	if artifact.RootfsArchive != filepath.Join(wantCacheDir, rootfsArchiveName) {
		t.Fatalf("download() rootfs = %q, want cached rootfs", artifact.RootfsArchive)
	}
	if artifact.ManifestPath != filepath.Join(wantCacheDir, "manifest.json") {
		t.Fatalf("download() manifest = %q, want cached manifest", artifact.ManifestPath)
	}
	rootfsBytes, err := os.ReadFile(artifact.RootfsArchive)
	if err != nil {
		t.Fatalf("ReadFile(rootfs) error = %v", err)
	}
	if string(rootfsBytes) != "downloaded-rootfs" {
		t.Fatalf("download() rootfs bytes = %q, want downloaded-rootfs", string(rootfsBytes))
	}
	manifestBytes, err := os.ReadFile(artifact.ManifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error = %v", err)
	}
	if !bytes.Contains(manifestBytes, []byte(`"artifact": "`+rootfsArchiveName+`"`)) {
		t.Fatalf("manifest %s did not record artifact name: %s", artifact.ManifestPath, string(manifestBytes))
	}

	wantOperations := []string{
		"Resolving WSL runtime image",
		"Fetching WSL runtime image metadata",
		"Extracting WSL rootfs artifact from image layer 1/1",
		"Writing WSL runtime image metadata",
		"Finalizing cached WSL rootfs artifact",
	}
	if len(operations) != len(wantOperations) {
		t.Fatalf("download() operations = %#v, want %#v", operations, wantOperations)
	}
	for i := range wantOperations {
		if operations[i] != wantOperations[i] {
			t.Fatalf("download() operations = %#v, want %#v", operations, wantOperations)
		}
	}
}

func TestImageDownloaderDownloadFailsWhenRootfsMissing(t *testing.T) {
	image := testImageWithLayer(t, map[string][]byte{
		"not-rootfs.txt": []byte("not the artifact"),
	})
	restore := replaceImageDescriptor(t, image, nil)
	defer restore()

	downloader := NewImageDownloader(ImageDownloadConfig{
		ImageRef: "example.com/discobot/wsl:test",
		DataDir:  t.TempDir(),
	})

	if _, err := downloader.download(context.Background(), nil); err == nil {
		t.Fatal("download() error = nil, want missing rootfs error")
	}
}

func TestImageDownloaderDownloadRejectsInvalidImageReference(t *testing.T) {
	downloader := NewImageDownloader(ImageDownloadConfig{
		ImageRef: "not a valid image ref",
		DataDir:  t.TempDir(),
	})

	if _, err := downloader.download(context.Background(), nil); err == nil {
		t.Fatal("download() error = nil, want invalid image reference error")
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

type fakeImageDescriptor struct {
	image v1.Image
	err   error
}

func (d fakeImageDescriptor) Image() (v1.Image, error) {
	return d.image, d.err
}

func replaceImageDescriptor(t *testing.T, image v1.Image, err error) func() {
	t.Helper()
	original := getImageDescriptor
	getImageDescriptor = func(context.Context, name.Reference, v1.Platform) (imageDescriptor, error) {
		return fakeImageDescriptor{image: image, err: err}, nil
	}
	return func() {
		getImageDescriptor = original
	}
}

func testImageWithLayer(t *testing.T, files map[string][]byte) v1.Image {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, contents := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(contents))}); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", name, err)
		}
		if _, err := tw.Write(contents); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	if err != nil {
		t.Fatalf("LayerFromReader() error = %v", err)
	}
	image, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatalf("AppendLayers() error = %v", err)
	}
	return image
}
