// Package docker provides cache volume management for Docker containers.
package docker

import (
	"context"
	"fmt"
	"log"

	cerrdefs "github.com/containerd/errdefs"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	volumeTypes "github.com/docker/docker/api/types/volume"
)

const (
	// cacheVolumePrefix is the prefix for project-scoped cache volume names.
	cacheVolumePrefix = "discboeing-cache-"
)

// cacheVolumeName generates a cache volume name from project ID.
func cacheVolumeName(projectID string) string {
	return fmt.Sprintf("%s%s", cacheVolumePrefix, projectID)
}

// ensureCacheVolume creates the project-scoped cache volume if it doesn't exist and returns its name.
func (p *Provider) ensureCacheVolume(ctx context.Context, projectID string) (string, error) {
	volName := cacheVolumeName(projectID)

	// Try to inspect the volume first
	_, err := p.client.VolumeInspect(ctx, volName)
	if err == nil {
		// Volume already exists
		return volName, nil
	}

	// Create the volume
	_, err = p.client.VolumeCreate(ctx, volumeTypes.CreateOptions{
		Name: volName,
		Labels: map[string]string{
			"discboeing.project.id": projectID,
			"discboeing.managed":    "true",
			"discboeing.type":       "cache",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create cache volume: %w", err)
	}

	return volName, nil
}

// RemoveCacheVolume removes the project-scoped cache volume.
// This should be called when a project is deleted.
// This is exported to satisfy the cacheVolumeManager interface check in ProjectService.
func (p *Provider) RemoveCacheVolume(ctx context.Context, projectID string) error {
	volName := cacheVolumeName(projectID)

	// Force removal even if volume is in use
	if err := p.client.VolumeRemove(ctx, volName, true); err != nil {
		// Ignore "not found" errors
		if !cerrdefs.IsNotFound(err) {
			return fmt.Errorf("failed to remove cache volume: %w", err)
		}
	}

	return nil
}

// ClearCache removes the project-scoped cache volume and any managed containers
// that are currently attached to it. It does not remove any other named volumes.
func (p *Provider) ClearCache(ctx context.Context, projectID string) error {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()

	volName := cacheVolumeName(projectID)
	containers, err := p.client.ContainerList(ctx, containerTypes.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "discboeing.managed=true"),
			filters.Arg("volume", volName),
		),
	})
	if err != nil {
		return fmt.Errorf("failed to list cache containers: %w", err)
	}

	for _, ctr := range containers {
		if !containerUsesVolume(ctr, volName) {
			continue
		}

		if err := p.client.ContainerRemove(ctx, ctr.ID, containerTypes.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil && !cerrdefs.IsNotFound(err) {
			return fmt.Errorf("failed to remove cache container %s: %w", ctr.ID, err)
		}

		if sessionID := ctr.Labels["discboeing.session.id"]; sessionID != "" {
			p.clearContainerID(sessionID)
		}
		log.Printf("Removed cache-attached container %s for project %s", ctr.ID[:12], projectID)
	}

	if err := p.RemoveCacheVolume(ctx, projectID); err != nil {
		return err
	}

	return nil
}

func containerUsesVolume(container containerTypes.Summary, volumeName string) bool {
	for _, mount := range container.Mounts {
		if mount.Type == "volume" && mount.Name == volumeName {
			return true
		}
	}
	return false
}

// ListCacheVolumes returns all cache volumes, optionally filtered by project ID.
func (p *Provider) ListCacheVolumes(ctx context.Context, projectID string) ([]*volumeTypes.Volume, error) {
	filters := filters.NewArgs()
	filters.Add("label", "discboeing.managed=true")
	filters.Add("label", "discboeing.type=cache")

	if projectID != "" {
		filters.Add("label", fmt.Sprintf("discboeing.project.id=%s", projectID))
	}

	resp, err := p.client.VolumeList(ctx, volumeTypes.ListOptions{
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list cache volumes: %w", err)
	}

	return resp.Volumes, nil
}
