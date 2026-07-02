// Package vsockproxy forwards VM-hosted Docker container published ports over VSOCK.
package vsockproxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const logPrefix = "discboeing-vsock-port-proxy"

// Run watches Docker events for managed containers with published ports and
// creates socat VSOCK listeners that forward those ports to the host.
func Run() error {
	fmt.Printf("%s: starting VSOCK port proxy\n", logPrefix)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := client.NewClientWithOpts(
		client.WithHost("unix:///var/run/docker.sock"),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	if err := waitForDockerReady(ctx, cli); err != nil {
		return fmt.Errorf("docker not ready: %w", err)
	}

	fmt.Printf("%s: connected to Docker\n", logPrefix)

	mu := &sync.Mutex{}
	socatProcs := make(map[string][]*exec.Cmd)

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", "discboeing.managed=true"),
			filters.Arg("status", "running"),
		),
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		ports := extractPublishedPorts(c.Ports)
		if len(ports) > 0 {
			startSocatForContainer(mu, socatProcs, c.ID, ports)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		fmt.Printf("%s: shutting down\n", logPrefix)
		cancel()
	}()

	watchDockerEvents(ctx, cli, mu, socatProcs)

	mu.Lock()
	for containerID, cmds := range socatProcs {
		for _, cmd := range cmds {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
		delete(socatProcs, containerID)
	}
	mu.Unlock()

	fmt.Printf("%s: stopped\n", logPrefix)
	return nil
}

func waitForDockerReady(ctx context.Context, cli *client.Client) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for docker")
		}
		_, err := cli.Ping(ctx)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

func extractPublishedPorts(ports []container.Port) []int {
	seen := make(map[int]bool)
	var result []int
	for _, p := range ports {
		if p.PublicPort > 0 && !seen[int(p.PublicPort)] {
			seen[int(p.PublicPort)] = true
			result = append(result, int(p.PublicPort))
		}
	}
	return result
}

func extractPublishedPortsFromInspect(info container.InspectResponse) []int {
	seen := make(map[int]bool)
	var result []int
	for _, bindings := range info.NetworkSettings.Ports {
		for _, b := range bindings {
			port, err := strconv.Atoi(b.HostPort)
			if err == nil && port > 0 && !seen[port] {
				seen[port] = true
				result = append(result, port)
			}
		}
	}
	return result
}

func startSocatForContainer(mu *sync.Mutex, socatProcs map[string][]*exec.Cmd, containerID string, ports []int) {
	mu.Lock()
	defer mu.Unlock()

	if cmds, exists := socatProcs[containerID]; exists {
		for _, cmd := range cmds {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
	}

	var cmds []*exec.Cmd
	for _, port := range ports {
		cmd := exec.Command("socat",
			"-b131072",
			fmt.Sprintf("VSOCK-LISTEN:%d,reuseaddr,fork,shut-down", port),
			fmt.Sprintf("TCP:localhost:%d,shut-down", port),
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to start socat for port %d: %v\n", logPrefix, port, err)
			continue
		}

		fmt.Printf("%s: forwarding VSOCK port %d -> localhost:%d (container %s)\n", logPrefix, port, port, shortContainerID(containerID))
		cmds = append(cmds, cmd)

		go func(c *exec.Cmd) {
			_ = c.Wait()
		}(cmd)
	}

	socatProcs[containerID] = cmds
}

func stopSocatForContainer(mu *sync.Mutex, socatProcs map[string][]*exec.Cmd, containerID string) {
	mu.Lock()
	defer mu.Unlock()

	cmds, exists := socatProcs[containerID]
	if !exists {
		return
	}

	for _, cmd := range cmds {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	delete(socatProcs, containerID)
	fmt.Printf("%s: stopped forwarding for container %s\n", logPrefix, shortContainerID(containerID))
}

func watchDockerEvents(ctx context.Context, cli *client.Client, mu *sync.Mutex, socatProcs map[string][]*exec.Cmd) {
	filterArgs := filters.NewArgs(
		filters.Arg("type", string(events.ContainerEventType)),
		filters.Arg("event", "start"),
		filters.Arg("event", "die"),
		filters.Arg("event", "stop"),
		filters.Arg("event", "destroy"),
	)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgCh, errCh := cli.Events(ctx, events.ListOptions{
			Filters: filterArgs,
		})

		done := processEvents(ctx, cli, mu, socatProcs, msgCh, errCh)
		if !done {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			fmt.Printf("%s: reconnecting to Docker events...\n", logPrefix)
		}
	}
}

func processEvents(ctx context.Context, cli *client.Client, mu *sync.Mutex, socatProcs map[string][]*exec.Cmd, msgCh <-chan events.Message, errCh <-chan error) bool {
	for {
		select {
		case <-ctx.Done():
			return false

		case err := <-errCh:
			if err == nil {
				return true
			}
			if ctx.Err() != nil {
				return false
			}
			fmt.Fprintf(os.Stderr, "%s: docker events error: %v, reconnecting...\n", logPrefix, err)
			return true

		case msg := <-msgCh:
			containerID := msg.Actor.ID
			if containerID == "" {
				continue
			}
			if msg.Actor.Attributes["discboeing.managed"] != "true" {
				continue
			}

			switch msg.Action {
			case "start":
				info, err := cli.ContainerInspect(ctx, containerID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: failed to inspect container %s: %v\n", logPrefix, shortContainerID(containerID), err)
					continue
				}

				ports := extractPublishedPortsFromInspect(info)
				if len(ports) > 0 {
					startSocatForContainer(mu, socatProcs, containerID, ports)
				}

			case "die", "stop", "destroy":
				stopSocatForContainer(mu, socatProcs, containerID)
			}
		}
	}
}

func shortContainerID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
