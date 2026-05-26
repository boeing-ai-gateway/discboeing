package vm

import (
	"context"
	"fmt"
	"log"

	containerTypes "github.com/docker/docker/api/types/container"

	"github.com/obot-platform/discobot/server/internal/sandbox/docker"
)

// StartProxyContainer returns a post-VM setup hook that creates and starts the
// VSOCK port proxy container inside the VM. The proxy watches Docker events for
// containers with published ports and creates VSOCK listeners that forward those
// ports to the host.
func StartProxyContainer(sandboxImage string) func(context.Context, string, *docker.Provider) error {
	return func(ctx context.Context, projectID string, dockerProv *docker.Provider) error {
		return startProxyContainer(ctx, projectID, dockerProv, sandboxImage)
	}
}

func startProxyContainer(ctx context.Context, projectID string, dockerProv *docker.Provider, sandboxImage string) error {
	cli := dockerProv.Client()
	suffix := projectID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	name := fmt.Sprintf("discobot-proxy-%s", suffix)

	existing, err := cli.ContainerInspect(ctx, name)
	if err == nil {
		needsRecreate := existing.Config.Image != sandboxImage || !existing.HostConfig.Privileged

		if existing.State.Running && !needsRecreate {
			log.Printf("Proxy container %s already running for project %s", name, projectID)
			return nil
		}
		if needsRecreate {
			log.Printf("Proxy container %s has stale config, recreating", name)
		}
		_ = cli.ContainerRemove(ctx, existing.ID, containerTypes.RemoveOptions{Force: true})
	}

	if err := dockerProv.EnsureImage(ctx); err != nil {
		return fmt.Errorf("failed to ensure sandbox image: %w", err)
	}

	containerConfig := &containerTypes.Config{
		Image: sandboxImage,
		Cmd:   []string{"/opt/discobot/bin/discobot-sandbox-init", "proxy"},
		Labels: map[string]string{
			"discobot.proxy":      "true",
			"discobot.project.id": projectID,
		},
	}

	hostConfig := &containerTypes.HostConfig{
		NetworkMode: "host",
		IpcMode:     "host",
		Privileged:  true,
		Binds:       []string{"/var/run/docker.sock:/var/run/docker.sock"},
		RestartPolicy: containerTypes.RestartPolicy{
			Name: containerTypes.RestartPolicyAlways,
		},
	}
	hostConfig.Ulimits = []*containerTypes.Ulimit{{
		Name: "nofile",
		Soft: 1048576,
		Hard: 1048576,
	}}

	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		return fmt.Errorf("failed to create proxy container: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, containerTypes.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start proxy container: %w", err)
	}

	log.Printf("Started proxy container %s (%s) for project %s", name, resp.ID[:12], projectID)
	return nil
}
