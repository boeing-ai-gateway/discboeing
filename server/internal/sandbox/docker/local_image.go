package docker

import (
	"context"
	"fmt"
	"io"
	"log"

	dockerclient "github.com/docker/docker/client"
)

const dockerLoadTaskID = "docker-load"

// EnsureLocalImageLoaded copies a local sandbox image from the host Docker daemon
// into a target Docker daemon when the configured image cannot be pulled from a
// registry. Non-local images are ignored.
func EnsureLocalImageLoaded(
	ctx context.Context,
	hostClient *dockerclient.Client,
	targetClient *dockerclient.Client,
	image string,
	systemManager SystemManager,
) error {
	if !IsLocalImage(image) {
		return nil
	}
	if hostClient == nil {
		return fmt.Errorf("host docker client is required for local image %s", shortDockerImageRef(image))
	}
	if targetClient == nil {
		return fmt.Errorf("target docker client is required for local image %s", shortDockerImageRef(image))
	}

	inspect, err := targetClient.ImageInspect(ctx, image)
	if err == nil {
		log.Printf("Local image %s already exists in target Docker (ID: %s)", shortDockerImageRef(image), shortDockerImageRef(inspect.ID))
		return nil
	}
	log.Printf("Local image %s not found in target Docker, will load from host: %v", shortDockerImageRef(image), err)

	inspect, err = hostClient.ImageInspect(ctx, image)
	if err != nil {
		return fmt.Errorf("image %s not found on host docker: %w", shortDockerImageRef(image), err)
	}

	imageSize := inspect.Size
	log.Printf("Loading local image %s (%d MB) from host Docker into target Docker...", shortDockerImageRef(image), imageSize/(1024*1024))

	if systemManager != nil {
		systemManager.RegisterTask(dockerLoadTaskID, fmt.Sprintf("Loading runtime image: %s", shortDockerImageRef(image)))
		systemManager.StartTask(dockerLoadTaskID)
	}

	reader, err := hostClient.ImageSave(ctx, []string{image})
	if err != nil {
		if systemManager != nil {
			systemManager.FailTask(dockerLoadTaskID, err)
		}
		return fmt.Errorf("failed to export image from host: %w", err)
	}
	defer reader.Close()

	progress := &imageTransferProgressReader{
		reader:       reader,
		total:        imageSize,
		logEvery:     100 * 1024 * 1024,
		label:        shortDockerImageRef(image),
		systemMgr:    systemManager,
		systemTaskID: dockerLoadTaskID,
	}

	resp, err := targetClient.ImageLoad(ctx, progress, dockerclient.ImageLoadWithQuiet(true))
	if err != nil {
		if systemManager != nil {
			systemManager.FailTask(dockerLoadTaskID, err)
		}
		return fmt.Errorf("failed to load image into target docker: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if systemManager != nil {
		systemManager.CompleteTask(dockerLoadTaskID)
	}

	log.Printf("Successfully loaded local image %s into target Docker", shortDockerImageRef(image))
	return nil
}

func shortDockerImageRef(image string) string {
	if len(image) <= 19 {
		return image
	}
	return image[:19]
}

type imageTransferProgressReader struct {
	reader       io.Reader
	total        int64
	read         int64
	logEvery     int64
	lastLog      int64
	label        string
	systemMgr    SystemManager
	systemTaskID string
}

func (r *imageTransferProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)

	if r.read-r.lastLog >= r.logEvery {
		pct := float64(r.read) / float64(r.total) * 100
		log.Printf("Image transfer %s: %.1f%% (%d/%d MB)", r.label, pct, r.read/(1024*1024), r.total/(1024*1024))
		r.lastLog = r.read

		if r.systemMgr != nil {
			r.systemMgr.UpdateTaskBytes(r.systemTaskID, r.read, r.total)
		}
	}

	return n, err
}
