package vm

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// DownloadState represents the current state of an image download process.
type DownloadState int

const (
	DownloadStateNotStarted DownloadState = iota
	DownloadStateDownloading
	DownloadStateExtracting
	DownloadStateReady
	DownloadStateFailed
)

func (s DownloadState) String() string {
	switch s {
	case DownloadStateNotStarted:
		return "not_started"
	case DownloadStateDownloading:
		return "downloading"
	case DownloadStateExtracting:
		return "extracting"
	case DownloadStateReady:
		return "ready"
	case DownloadStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// DownloadProgress tracks image download progress.
type DownloadProgress struct {
	State           DownloadState `json:"state"`
	BytesDownloaded int64         `json:"bytes_downloaded"`
	TotalBytes      int64         `json:"total_bytes"`
	CurrentLayer    string        `json:"current_layer"`
	Error           string        `json:"error,omitempty"`
	StartedAt       time.Time     `json:"started_at"`
	CompletedAt     time.Time     `json:"completed_at,omitzero"`
}

// ImageDescriptor is the subset of a registry descriptor needed by ImageDownloader.
type ImageDescriptor interface {
	Image() (v1.Image, error)
}

// ImageDescriptorGetter resolves an OCI image descriptor for a platform.
type ImageDescriptorGetter func(context.Context, name.Reference, v1.Platform) (ImageDescriptor, error)

// ImageArtifactSpec describes one artifact to extract from an OCI image.
type ImageArtifactSpec struct {
	Name string
}

// ImageDownloadConfig configures download of one or more named artifacts from an OCI image.
type ImageDownloadConfig struct {
	ImageRef                 string
	DataDir                  string
	ArtifactName             string
	Artifacts                []ImageArtifactSpec
	LocalArtifactPath        string
	ProviderName             string
	ArtifactDescription      string
	LocalArtifactDescription string
	PostProcess              func(map[string]string) error
	GetDescriptor            ImageDescriptorGetter
}

// ImageArtifact describes a cached VM runtime artifact.
type ImageArtifact struct {
	Digest          string
	ArtifactPath    string
	ManifestPath    string
	ImageRef        string
	DownloadedAtUTC time.Time
}

// ImageDownloadProgress reports high-level image download progress.
type ImageDownloadProgress struct {
	CurrentOperation string
}

// ImageDownloader downloads and caches VM runtime artifacts from an OCI image.
type ImageDownloader struct {
	cfg ImageDownloadConfig

	state      DownloadState
	stateMu    sync.RWMutex
	progress   DownloadProgress
	progressMu sync.RWMutex
	doneCh     chan struct{}
	doneOnce   sync.Once

	artifactPaths map[string]string
}

func NewImageDownloader(cfg ImageDownloadConfig) *ImageDownloader {
	if cfg.ProviderName == "" {
		cfg.ProviderName = "VM"
	}
	if cfg.ArtifactName != "" && len(cfg.Artifacts) == 0 {
		cfg.Artifacts = []ImageArtifactSpec{{Name: cfg.ArtifactName}}
	}
	if cfg.ArtifactDescription == "" {
		cfg.ArtifactDescription = cfg.ProviderName + " artifact"
	}
	if cfg.LocalArtifactDescription == "" {
		cfg.LocalArtifactDescription = cfg.ArtifactDescription
	}
	if cfg.GetDescriptor == nil {
		cfg.GetDescriptor = func(ctx context.Context, ref name.Reference, platform v1.Platform) (ImageDescriptor, error) {
			return remote.Get(ref, remote.WithContext(ctx), remote.WithPlatform(platform))
		}
	}
	return &ImageDownloader{
		cfg:           cfg,
		state:         DownloadStateNotStarted,
		doneCh:        make(chan struct{}),
		artifactPaths: make(map[string]string),
		progress: DownloadProgress{
			State: DownloadStateNotStarted,
		},
	}
}

func (d *ImageDownloader) Start(ctx context.Context) error {
	d.updateState(DownloadStateDownloading)
	d.updateProgress(func(p *DownloadProgress) {
		p.State = DownloadStateDownloading
		p.StartedAt = time.Now()
	})

	if paths, cached := d.checkCache(); cached {
		log.Printf("%s artifacts already cached: %v", d.cfg.ProviderName, paths)
		d.storeArtifactPaths(paths)
		d.updateState(DownloadStateReady)
		d.updateProgress(func(p *DownloadProgress) {
			p.State = DownloadStateReady
			p.CompletedAt = time.Now()
		})
		d.closeDone()
		return nil
	}

	paths, err := d.downloadArtifacts(ctx, nil)
	if err != nil {
		d.updateState(DownloadStateFailed)
		d.updateProgress(func(p *DownloadProgress) {
			p.State = DownloadStateFailed
			p.Error = err.Error()
		})
		d.closeDone()
		return err
	}

	d.storeArtifactPaths(paths)
	d.updateState(DownloadStateReady)
	d.updateProgress(func(p *DownloadProgress) {
		p.State = DownloadStateReady
		p.CompletedAt = time.Now()
	})
	d.closeDone()
	return nil
}

func (d *ImageDownloader) Status() DownloadProgress {
	d.progressMu.RLock()
	defer d.progressMu.RUnlock()
	return d.progress
}

func (d *ImageDownloader) Wait(ctx context.Context) error {
	select {
	case <-d.doneCh:
		d.stateMu.RLock()
		state := d.state
		d.stateMu.RUnlock()
		if state == DownloadStateFailed {
			d.progressMu.RLock()
			err := d.progress.Error
			d.progressMu.RUnlock()
			return fmt.Errorf("download failed: %s", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *ImageDownloader) RecordError(err error) {
	d.updateState(DownloadStateFailed)
	d.updateProgress(func(p *DownloadProgress) {
		p.State = DownloadStateFailed
		p.Error = err.Error()
		if p.CompletedAt.IsZero() {
			p.CompletedAt = time.Now()
		}
	})
	d.closeDone()
}

func (d *ImageDownloader) EnsureArtifact(ctx context.Context) (*ImageArtifact, error) {
	return d.EnsureArtifactWithProgress(ctx, nil)
}

func (d *ImageDownloader) EnsureArtifactWithProgress(ctx context.Context, report func(ImageDownloadProgress)) (*ImageArtifact, error) {
	if strings.TrimSpace(d.cfg.LocalArtifactPath) != "" {
		if report != nil {
			report(ImageDownloadProgress{CurrentOperation: fmt.Sprintf("Using local %s", d.cfg.LocalArtifactDescription)})
		}
		return d.localArtifact()
	}
	if artifact, ok, err := d.CheckCache(); err != nil {
		return nil, err
	} else if ok {
		if report != nil {
			report(ImageDownloadProgress{CurrentOperation: fmt.Sprintf("Using cached %s", d.cfg.ArtifactDescription)})
		}
		return artifact, nil
	}
	return d.Download(ctx, report)
}

func (d *ImageDownloader) localArtifact() (*ImageArtifact, error) {
	path := strings.TrimSpace(d.cfg.LocalArtifactPath)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat local %s artifact %q: %w", d.cfg.ProviderName, path, err)
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("local %s artifact %q is empty", d.cfg.ProviderName, path)
	}
	return &ImageArtifact{
		Digest:          ComputeShortDigest(path),
		ArtifactPath:    path,
		ImageRef:        path,
		DownloadedAtUTC: info.ModTime().UTC(),
	}, nil
}

func (d *ImageDownloader) CheckCache() (*ImageArtifact, bool, error) {
	artifacts := d.artifacts()
	if len(artifacts) != 1 {
		return nil, false, fmt.Errorf("CheckCache requires exactly one configured artifact")
	}
	paths, ok := d.checkCache()
	if !ok {
		return nil, false, nil
	}
	digest := d.ComputeDigest()
	cacheDir := filepath.Join(d.cfg.DataDir, "images", digest)
	manifestPath := filepath.Join(cacheDir, "manifest.json")
	manifestInfo, err := os.Stat(manifestPath)
	if err != nil {
		return nil, false, nil
	}
	return &ImageArtifact{
		Digest:          digest,
		ArtifactPath:    paths[artifacts[0].Name],
		ManifestPath:    manifestPath,
		ImageRef:        d.cfg.ImageRef,
		DownloadedAtUTC: manifestInfo.ModTime().UTC(),
	}, true, nil
}

func (d *ImageDownloader) checkCache() (map[string]string, bool) {
	digest := d.ComputeDigest()
	cacheDir := filepath.Join(d.cfg.DataDir, "images", digest)
	manifestPath := filepath.Join(cacheDir, "manifest.json")
	manifestInfo, manifestErr := os.Stat(manifestPath)
	if manifestErr != nil || manifestInfo.Size() == 0 {
		return nil, false
	}

	paths := make(map[string]string)
	for _, artifact := range d.artifacts() {
		artifactPath := filepath.Join(cacheDir, artifact.Name)
		artifactInfo, artifactErr := os.Stat(artifactPath)
		if artifactErr != nil || artifactInfo.Size() == 0 {
			return nil, false
		}
		paths[artifact.Name] = artifactPath
	}
	return paths, true
}

func (d *ImageDownloader) Download(ctx context.Context, report func(ImageDownloadProgress)) (*ImageArtifact, error) {
	artifacts := d.artifacts()
	if len(artifacts) != 1 {
		return nil, fmt.Errorf("Download requires exactly one configured artifact")
	}
	paths, err := d.downloadArtifacts(ctx, report)
	if err != nil {
		return nil, err
	}
	digest := d.ComputeDigest()
	cacheDir := filepath.Join(d.cfg.DataDir, "images", digest)
	return &ImageArtifact{
		Digest:          digest,
		ArtifactPath:    paths[artifacts[0].Name],
		ManifestPath:    filepath.Join(cacheDir, "manifest.json"),
		ImageRef:        d.cfg.ImageRef,
		DownloadedAtUTC: time.Now().UTC(),
	}, nil
}

func (d *ImageDownloader) downloadArtifacts(ctx context.Context, report func(ImageDownloadProgress)) (map[string]string, error) {
	artifacts := d.artifacts()
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("at least one artifact is required")
	}
	if report != nil {
		report(ImageDownloadProgress{CurrentOperation: fmt.Sprintf("Resolving %s runtime image", d.cfg.ProviderName)})
	}
	ref, err := name.ParseReference(d.cfg.ImageRef)
	if err != nil {
		return nil, fmt.Errorf("invalid %s image reference %s: %w", d.cfg.ProviderName, d.cfg.ImageRef, err)
	}

	platform := v1.Platform{OS: "linux", Architecture: runtime.GOARCH}
	if report != nil {
		report(ImageDownloadProgress{CurrentOperation: fmt.Sprintf("Fetching %s runtime image metadata", d.cfg.ProviderName)})
	}
	desc, err := d.cfg.GetDescriptor(ctx, ref, platform)
	if err != nil {
		return nil, fmt.Errorf("fetch %s image descriptor: %w", d.cfg.ProviderName, err)
	}

	img, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("resolve %s image: %w", d.cfg.ProviderName, err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("get %s image manifest: %w", d.cfg.ProviderName, err)
	}

	var totalBytes int64
	for _, layer := range manifest.Layers {
		totalBytes += layer.Size
	}
	d.updateProgress(func(p *DownloadProgress) { p.TotalBytes = totalBytes })

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("get %s image layers: %w", d.cfg.ProviderName, err)
	}

	digest := d.ComputeDigest()
	cacheDir := filepath.Join(d.cfg.DataDir, "images", digest)
	tempDir := cacheDir + ".tmp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("create %s image temp dir: %w", d.cfg.ProviderName, err)
	}
	defer os.RemoveAll(tempDir)

	d.updateState(DownloadStateExtracting)
	d.updateProgress(func(p *DownloadProgress) { p.State = DownloadStateExtracting })

	found := make(map[string]bool, len(artifacts))
	var bytesDownloaded int64
	for i, layer := range layers {
		layerDigest, err := layer.Digest()
		if err != nil {
			return nil, fmt.Errorf("failed to get layer digest: %w", err)
		}
		d.updateProgress(func(p *DownloadProgress) { p.CurrentLayer = layerDigest.String() })
		if report != nil {
			report(ImageDownloadProgress{
				CurrentOperation: fmt.Sprintf("Extracting %s from image layer %d/%d", d.cfg.ArtifactDescription, i+1, len(layers)),
			})
		}
		uncompressed, err := layer.Uncompressed()
		if err != nil {
			return nil, fmt.Errorf("open %s image layer: %w", d.cfg.ProviderName, err)
		}
		if err := d.ExtractFiles(uncompressed, tempDir, found); err != nil {
			uncompressed.Close()
			return nil, fmt.Errorf("extract %s image layer: %w", d.cfg.ProviderName, err)
		}
		if err := uncompressed.Close(); err != nil {
			return nil, fmt.Errorf("close %s image layer: %w", d.cfg.ProviderName, err)
		}
		size, _ := layer.Size()
		bytesDownloaded += size
		d.updateProgress(func(p *DownloadProgress) { p.BytesDownloaded = bytesDownloaded })
	}

	for _, artifact := range artifacts {
		if !found[artifact.Name] {
			return nil, fmt.Errorf("artifact (%s) not found in image", artifact.Name)
		}
	}

	paths := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		paths[artifact.Name] = filepath.Join(tempDir, artifact.Name)
	}
	if d.cfg.PostProcess != nil {
		if err := d.cfg.PostProcess(paths); err != nil {
			return nil, err
		}
	}

	metadata := map[string]any{
		"image_ref":   d.cfg.ImageRef,
		"digest":      digest,
		"pulled_at":   time.Now().UTC().Format(time.RFC3339),
		"total_bytes": totalBytes,
	}
	if len(artifacts) == 1 {
		metadata["artifact"] = artifacts[0].Name
	} else {
		names := make([]string, 0, len(artifacts))
		for _, artifact := range artifacts {
			names = append(names, artifact.Name)
		}
		metadata["artifacts"] = names
	}
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal %s image metadata: %w", d.cfg.ProviderName, err)
	}
	metadataJSON = append(metadataJSON, '\n')
	if report != nil {
		report(ImageDownloadProgress{CurrentOperation: fmt.Sprintf("Writing %s runtime image metadata", d.cfg.ProviderName)})
	}
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), metadataJSON, 0644); err != nil {
		return nil, fmt.Errorf("write %s image metadata: %w", d.cfg.ProviderName, err)
	}

	if report != nil {
		report(ImageDownloadProgress{CurrentOperation: fmt.Sprintf("Finalizing cached %s", d.cfg.ArtifactDescription)})
	}
	if err := os.RemoveAll(cacheDir); err != nil {
		return nil, fmt.Errorf("remove existing %s image cache: %w", d.cfg.ProviderName, err)
	}
	if err := os.Rename(tempDir, cacheDir); err != nil {
		return nil, fmt.Errorf("finalize %s image cache: %w", d.cfg.ProviderName, err)
	}

	finalPaths := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		finalPaths[artifact.Name] = filepath.Join(cacheDir, artifact.Name)
	}
	return finalPaths, nil
}

func (d *ImageDownloader) ExtractFiles(r io.Reader, destDir string, found map[string]bool) error {
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
		for _, artifact := range d.artifacts() {
			if header.Name != artifact.Name && !strings.HasSuffix(header.Name, "/"+artifact.Name) {
				continue
			}
			if err := writeArchiveFile(tr, filepath.Join(destDir, artifact.Name), header.Mode); err != nil {
				return err
			}
			found[artifact.Name] = true
			break
		}
	}
}

func (d *ImageDownloader) ComputeDigest() string {
	return ComputeShortDigest(d.cfg.ImageRef)
}

func ComputeShortDigest(value string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(value))
	return fmt.Sprintf("sha256-%x", h.Sum(nil))[:19]
}

func (d *ImageDownloader) GetArtifactPath(name string) (string, bool) {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()
	if d.state != DownloadStateReady {
		return "", false
	}
	path, ok := d.artifactPaths[name]
	return path, ok
}

func (d *ImageDownloader) GetArtifactPaths() (map[string]string, bool) {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()
	if d.state != DownloadStateReady {
		return nil, false
	}
	paths := make(map[string]string, len(d.artifactPaths))
	for name, path := range d.artifactPaths {
		paths[name] = path
	}
	return paths, true
}

func (d *ImageDownloader) storeArtifactPaths(paths map[string]string) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	d.artifactPaths = make(map[string]string, len(paths))
	for name, path := range paths {
		d.artifactPaths[name] = path
	}
}

func (d *ImageDownloader) updateState(state DownloadState) {
	d.stateMu.Lock()
	d.state = state
	d.stateMu.Unlock()
}

func (d *ImageDownloader) updateProgress(fn func(*DownloadProgress)) {
	d.progressMu.Lock()
	fn(&d.progress)
	d.progressMu.Unlock()
}

func (d *ImageDownloader) closeDone() {
	d.doneOnce.Do(func() { close(d.doneCh) })
}

func (d *ImageDownloader) artifacts() []ImageArtifactSpec {
	if len(d.cfg.Artifacts) > 0 {
		return d.cfg.Artifacts
	}
	if d.cfg.ArtifactName != "" {
		return []ImageArtifactSpec{{Name: d.cfg.ArtifactName}}
	}
	return nil
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
