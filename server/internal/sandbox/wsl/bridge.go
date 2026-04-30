package wsl

import (
	"fmt"
	"strings"
)

const (
	BridgeTypeNamedPipe = "named_pipe"
	BridgeTypeTCP       = "tcp"
)

// BridgeInfo contains the resolved client-side connection details for the WSL
// Docker bridge.
type BridgeInfo struct {
	Type       string
	DockerHost string
	PipeName   string
	Port       int
}

// ResolveBridgeInfo validates bridge settings and derives the Docker host that a
// Windows-side Docker client should use to reach the bridge once it is running.
func ResolveBridgeInfo(bridgeType, distroName string, port int) (BridgeInfo, error) {
	bridgeType = strings.ToLower(strings.TrimSpace(bridgeType))
	switch bridgeType {
	case "", BridgeTypeNamedPipe:
		pipeName := bridgePipeName(distroName)
		return BridgeInfo{
			Type:       BridgeTypeNamedPipe,
			PipeName:   pipeName,
			DockerHost: bridgePipeDockerHost(pipeName),
		}, nil
	case BridgeTypeTCP:
		if port < 0 || port > 65535 {
			return BridgeInfo{}, fmt.Errorf("invalid WSL bridge port %d", port)
		}
		info := BridgeInfo{
			Type: BridgeTypeTCP,
			Port: port,
		}
		if port > 0 {
			info.DockerHost = fmt.Sprintf("tcp://127.0.0.1:%d", port)
		}
		return info, nil
	default:
		return BridgeInfo{}, fmt.Errorf("unsupported WSL bridge type %q", bridgeType)
	}
}

func bridgePipeDockerHost(pipeName string) string {
	return "npipe:////./pipe/" + pipeName
}

func bridgePipePath(pipeName string) string {
	return `\\.\pipe\` + pipeName
}

func bridgeTCPPingURL(port int) string {
	if port <= 0 || port > 65535 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d/_ping", port)
}

func bridgePipeName(distroName string) string {
	name := strings.ToLower(strings.TrimSpace(distroName))
	if name == "" {
		name = "discobot"
	}

	var b strings.Builder
	lastDash := false
	for _, r := range name {
		keep := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if keep {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	sanitized := strings.Trim(b.String(), "-")
	if sanitized == "" {
		sanitized = "discobot"
	}
	if !strings.HasSuffix(sanitized, "-docker") {
		sanitized += "-docker"
	}
	return sanitized
}
