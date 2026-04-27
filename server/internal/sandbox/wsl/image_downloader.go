package wsl

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const rootfsArchiveName = "discobot-rootfs.tar.zst"

// ImageDownloadConfig configures WSL runtime image downloads.
type ImageDownloadConfig struct {
	ImageRef string
	DataDir  string
}

// ImageArtifact describes a cached WSL runtime artifact set.
type ImageArtifact struct {
	Digest          string
	RootfsArchive   string
	ManifestPath    string
	ImageRef        string
	DownloadedAtUTC time.Time
}

// ImageDownloader downloads and caches the WSL rootfs artifact from the shared OCI image.
type ImageDownloader struct {
	cfg ImageDownloadConfig
}

func NewImageDownloader(cfg ImageDownloadConfig) *ImageDownloader {
	return &ImageDownloader{cfg: cfg}
}

func (d *ImageDownloader) EnsureRootfs(ctx context.Context) (*ImageArtifact, error) {
	if artifact, ok, err := d.checkCache(); err != nil {
		return nil, err
	} else if ok {
		return artifact, nil
	}
	return d.download(ctx)
}

func (d *ImageDownloader) checkCache() (*ImageArtifact, bool, error) {
	digest := d.computeDigest()
	cacheDir := filepath.Join(d.cfg.DataDir, "images", digest)
	rootfsPath := filepath.Join(cacheDir, rootfsArchiveName)
	manifestPath := filepath.Join(cacheDir, "manifest.json")

	rootfsInfo, rootfsErr := os.Stat(rootfsPath)
	manifestInfo, manifestErr := os.Stat(manifestPath)
	if rootfsErr != nil || manifestErr != nil || rootfsInfo.Size() == 0 || manifestInfo.Size() == 0 {
		return nil, false, nil
	}

	artifact := &ImageArtifact{
		Digest:        digest,
		RootfsArchive: rootfsPath,
		ManifestPath:  manifestPath,
		ImageRef:      d.cfg.ImageRef,
		DownloadedAtUTC: func() time.Time {
			return manifestInfo.ModTime().UTC()
		}(),
	}
	return artifact, true, nil
}

func (d *ImageDownloader) download(ctx context.Context) (*ImageArtifact, error) {
	ref, err := name.ParseReference(d.cfg.ImageRef)
	if err != nil {
		return nil, fmt.Errorf("invalid WSL image reference %s: %w", d.cfg.ImageRef, err)
	}

	platform := v1.Platform{OS: "linux", Architecture: runtime.GOARCH}
	desc, err := remote.Get(ref, remote.WithContext(ctx), remote.WithPlatform(platform))
	if err != nil {
		return nil, fmt.Errorf("fetch WSL image descriptor: %w", err)
	}

	img, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("resolve WSL image: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("get WSL image manifest: %w", err)
	}

	var totalBytes int64
	for _, layer := range manifest.Layers {
		totalBytes += layer.Size
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("get WSL image layers: %w", err)
	}

	digest := d.computeDigest()
	cacheDir := filepath.Join(d.cfg.DataDir, "images", digest)
	tempDir := cacheDir + ".tmp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("create WSL image temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	rootfsFound := false
	for _, layer := range layers {
		uncompressed, err := layer.Uncompressed()
		if err != nil {
			return nil, fmt.Errorf("open WSL image layer: %w", err)
		}
		if err := d.extractFiles(uncompressed, tempDir, &rootfsFound); err != nil {
			uncompressed.Close()
			return nil, fmt.Errorf("extract WSL image layer: %w", err)
		}
		if err := uncompressed.Close(); err != nil {
			return nil, fmt.Errorf("close WSL image layer: %w", err)
		}
	}

	if !rootfsFound {
		return nil, fmt.Errorf("rootfs archive (%s) not found in image", rootfsArchiveName)
	}

	metadata := map[string]any{
		"image_ref":   d.cfg.ImageRef,
		"digest":      digest,
		"pulled_at":   time.Now().UTC().Format(time.RFC3339),
		"total_bytes": totalBytes,
		"artifact":    rootfsArchiveName,
	}
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal WSL image metadata: %w", err)
	}
	metadataJSON = append(metadataJSON, '\n')
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), metadataJSON, 0644); err != nil {
		return nil, fmt.Errorf("write WSL image metadata: %w", err)
	}

	if err := os.RemoveAll(cacheDir); err != nil {
		return nil, fmt.Errorf("remove existing WSL image cache: %w", err)
	}
	if err := os.Rename(tempDir, cacheDir); err != nil {
		return nil, fmt.Errorf("finalize WSL image cache: %w", err)
	}

	return &ImageArtifact{
		Digest:          digest,
		RootfsArchive:   filepath.Join(cacheDir, rootfsArchiveName),
		ManifestPath:    filepath.Join(cacheDir, "manifest.json"),
		ImageRef:        d.cfg.ImageRef,
		DownloadedAtUTC: time.Now().UTC(),
	}, nil
}

func (d *ImageDownloader) extractFiles(r io.Reader, destDir string, rootfsFound *bool) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if header.Name != rootfsArchiveName && !strings.HasSuffix(header.Name, "/"+rootfsArchiveName) {
			continue
		}
		if err := writeArchiveFile(tr, filepath.Join(destDir, rootfsArchiveName), header.Mode); err != nil {
			return err
		}
		*rootfsFound = true
	}
}

func (d *ImageDownloader) computeDigest() string {
	return computeImageRefDigest(d.cfg.ImageRef)
}

func computeImageRefDigest(imageRef string) string {
	return computeShortDigest(imageRef)
}

func computeShortDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256-%x", sum[:])[:19]
}

func writeArchiveFile(r io.Reader, destPath string, mode int64) error {
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}
