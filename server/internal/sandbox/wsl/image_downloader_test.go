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

	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
)

const testRootfsArchiveName = "discobot-rootfs.tar.zst"

type wslDownloaderConfig struct {
	ImageRef           string
	DataDir            string
	LocalRootfsArchive string
	Image              v1.Image
	ImageErr           error
}

func newWSLDownloader(cfg wslDownloaderConfig) *vm.ImageDownloader {
	return vm.NewImageDownloader(vm.ImageDownloadConfig{
		ImageRef:                 cfg.ImageRef,
		DataDir:                  cfg.DataDir,
		ArtifactName:             testRootfsArchiveName,
		LocalArtifactPath:        cfg.LocalRootfsArchive,
		ProviderName:             "WSL",
		ArtifactDescription:      "WSL rootfs artifact",
		LocalArtifactDescription: "WSL rootfs archive",
		GetDescriptor: func(context.Context, name.Reference, v1.Platform) (vm.ImageDescriptor, error) {
			return fakeImageDescriptor{image: cfg.Image, err: cfg.ImageErr}, nil
		},
	})
}

func TestImageDownloaderCheckCache(t *testing.T) {
	tempDir := t.TempDir()
	downloader := newWSLDownloader(wslDownloaderConfig{
		ImageRef: "ghcr.io/obot-platform/discobot-wsl:test",
		DataDir:  tempDir,
	})

	digest := downloader.ComputeDigest()
	cacheDir := filepath.Join(tempDir, "images", digest)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, testRootfsArchiveName), []byte("zstd"), 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "manifest.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}

	artifact, ok, err := downloader.CheckCache()
	if err != nil {
		t.Fatalf("CheckCache() error = %v", err)
	}
	if !ok {
		t.Fatal("CheckCache() ok = false, want true")
	}
	if artifact.ArtifactPath != filepath.Join(cacheDir, testRootfsArchiveName) {
		t.Fatalf("unexpected rootfs path %q", artifact.ArtifactPath)
	}
}

func TestImageDownloaderExtractFiles(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	payload := []byte("archive-bytes")
	if err := tw.WriteHeader(&tar.Header{Name: "nested/" + testRootfsArchiveName, Mode: 0644, Size: int64(len(payload))}); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	destDir := t.TempDir()
	found := map[string]bool{}
	downloader := newWSLDownloader(wslDownloaderConfig{})
	if err := downloader.ExtractFiles(bytes.NewReader(buf.Bytes()), destDir, found); err != nil {
		t.Fatalf("ExtractFiles() error = %v", err)
	}
	if !found[testRootfsArchiveName] {
		t.Fatal("ExtractFiles() found = false, want true")
	}
	got, err := os.ReadFile(filepath.Join(destDir, testRootfsArchiveName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("ExtractFiles() wrote %q, want %q", string(got), string(payload))
	}
}

func TestImageDownloaderEnsureArtifactUsesLocalArchive(t *testing.T) {
	tempDir := t.TempDir()
	rootfsPath := filepath.Join(tempDir, testRootfsArchiveName)
	if err := os.WriteFile(rootfsPath, []byte("local-rootfs"), 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}

	downloader := newWSLDownloader(wslDownloaderConfig{
		DataDir:            tempDir,
		LocalRootfsArchive: rootfsPath,
	})

	artifact, err := downloader.EnsureArtifact(context.Background())
	if err != nil {
		t.Fatalf("EnsureArtifact() error = %v", err)
	}
	if artifact.ArtifactPath != rootfsPath {
		t.Fatalf("EnsureArtifact() rootfs = %q, want %q", artifact.ArtifactPath, rootfsPath)
	}
}

func TestImageDownloaderEnsureArtifactUsesLocalArchiveReportsProgress(t *testing.T) {
	tempDir := t.TempDir()
	rootfsPath := filepath.Join(tempDir, testRootfsArchiveName)
	if err := os.WriteFile(rootfsPath, []byte("local-rootfs"), 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}

	downloader := newWSLDownloader(wslDownloaderConfig{
		DataDir:            tempDir,
		LocalRootfsArchive: rootfsPath,
	})

	var operations []string
	_, err := downloader.EnsureArtifactWithProgress(context.Background(), func(progress vm.ImageDownloadProgress) {
		operations = append(operations, progress.CurrentOperation)
	})
	if err != nil {
		t.Fatalf("EnsureArtifactWithProgress() error = %v", err)
	}
	if len(operations) != 1 || operations[0] != "Using local WSL rootfs archive" {
		t.Fatalf("EnsureArtifactWithProgress() operations = %#v, want local archive progress", operations)
	}
}

func TestImageDownloaderEnsureArtifactUsesCacheReportsProgress(t *testing.T) {
	tempDir := t.TempDir()
	downloader := newWSLDownloader(wslDownloaderConfig{
		ImageRef: "ghcr.io/obot-platform/discobot-wsl:test",
		DataDir:  tempDir,
	})

	digest := downloader.ComputeDigest()
	cacheDir := filepath.Join(tempDir, "images", digest)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, testRootfsArchiveName), []byte("zstd"), 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "manifest.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}

	var operations []string
	artifact, err := downloader.EnsureArtifactWithProgress(context.Background(), func(progress vm.ImageDownloadProgress) {
		operations = append(operations, progress.CurrentOperation)
	})
	if err != nil {
		t.Fatalf("EnsureArtifactWithProgress() error = %v", err)
	}
	if artifact.ArtifactPath != filepath.Join(cacheDir, testRootfsArchiveName) {
		t.Fatalf("EnsureArtifactWithProgress() rootfs = %q, want %q", artifact.ArtifactPath, filepath.Join(cacheDir, testRootfsArchiveName))
	}
	if len(operations) != 1 || operations[0] != "Using cached WSL rootfs artifact" {
		t.Fatalf("EnsureArtifactWithProgress() operations = %#v, want cached artifact progress", operations)
	}
}

func TestImageDownloaderDownloadCachesRootfsArtifact(t *testing.T) {
	image := testImageWithLayer(t, map[string][]byte{
		"usr/share/discobot/" + testRootfsArchiveName: []byte("downloaded-rootfs"),
		"ignored.txt": []byte("ignored"),
	})

	tempDir := t.TempDir()
	downloader := newWSLDownloader(wslDownloaderConfig{
		ImageRef: "example.com/discobot/wsl:test",
		DataDir:  tempDir,
		Image:    image,
	})

	var operations []string
	artifact, err := downloader.Download(context.Background(), func(progress vm.ImageDownloadProgress) {
		operations = append(operations, progress.CurrentOperation)
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	wantDigest := vm.ComputeShortDigest("example.com/discobot/wsl:test")
	wantCacheDir := filepath.Join(tempDir, "images", wantDigest)
	if artifact.Digest != wantDigest {
		t.Fatalf("Download() digest = %q, want %q", artifact.Digest, wantDigest)
	}
	if artifact.ArtifactPath != filepath.Join(wantCacheDir, testRootfsArchiveName) {
		t.Fatalf("Download() rootfs = %q, want cached rootfs", artifact.ArtifactPath)
	}
	if artifact.ManifestPath != filepath.Join(wantCacheDir, "manifest.json") {
		t.Fatalf("Download() manifest = %q, want cached manifest", artifact.ManifestPath)
	}
	rootfsBytes, err := os.ReadFile(artifact.ArtifactPath)
	if err != nil {
		t.Fatalf("ReadFile(rootfs) error = %v", err)
	}
	if string(rootfsBytes) != "downloaded-rootfs" {
		t.Fatalf("Download() rootfs bytes = %q, want downloaded-rootfs", string(rootfsBytes))
	}
	manifestBytes, err := os.ReadFile(artifact.ManifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error = %v", err)
	}
	if !bytes.Contains(manifestBytes, []byte(`"artifact": "`+testRootfsArchiveName+`"`)) {
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
		t.Fatalf("Download() operations = %#v, want %#v", operations, wantOperations)
	}
	for i := range wantOperations {
		if operations[i] != wantOperations[i] {
			t.Fatalf("Download() operations = %#v, want %#v", operations, wantOperations)
		}
	}
}

func TestImageDownloaderDownloadFailsWhenRootfsMissing(t *testing.T) {
	image := testImageWithLayer(t, map[string][]byte{
		"not-rootfs.txt": []byte("not the artifact"),
	})

	downloader := newWSLDownloader(wslDownloaderConfig{
		ImageRef: "example.com/discobot/wsl:test",
		DataDir:  t.TempDir(),
		Image:    image,
	})

	if _, err := downloader.Download(context.Background(), nil); err == nil {
		t.Fatal("Download() error = nil, want missing rootfs error")
	}
}

func TestImageDownloaderDownloadRejectsInvalidImageReference(t *testing.T) {
	downloader := newWSLDownloader(wslDownloaderConfig{
		ImageRef: "not a valid image ref",
		DataDir:  t.TempDir(),
	})

	if _, err := downloader.Download(context.Background(), nil); err == nil {
		t.Fatal("Download() error = nil, want invalid image reference error")
	}
}

func TestImageDownloaderEnsureArtifactRejectsEmptyLocalArchive(t *testing.T) {
	tempDir := t.TempDir()
	rootfsPath := filepath.Join(tempDir, testRootfsArchiveName)
	if err := os.WriteFile(rootfsPath, nil, 0644); err != nil {
		t.Fatalf("WriteFile(rootfs) error = %v", err)
	}

	downloader := newWSLDownloader(wslDownloaderConfig{
		DataDir:            tempDir,
		LocalRootfsArchive: rootfsPath,
	})

	if _, err := downloader.EnsureArtifact(context.Background()); err == nil {
		t.Fatal("EnsureArtifact() error = nil, want non-nil")
	}
}

type fakeImageDescriptor struct {
	image v1.Image
	err   error
}

func (d fakeImageDescriptor) Image() (v1.Image, error) {
	return d.image, d.err
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
