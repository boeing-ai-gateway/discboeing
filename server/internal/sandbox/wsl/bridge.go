package wsl

import "fmt"

const (
	BridgeTypeTCP = "tcp"
)

// BridgeInfo contains the resolved client-side connection details for the WSL
// Docker bridge.
type BridgeInfo struct {
	Type       string
	DockerHost string
	Port       int
}

// ResolveBridgeInfo validates bridge settings and derives the Docker host that a
// Windows-side Docker client should use to reach the bridge once it is running.
func ResolveBridgeInfo(port int) (BridgeInfo, error) {
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
}

func bridgeTCPPingURL(port int) string {
	if port <= 0 || port > 65535 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d/_ping", port)
}
