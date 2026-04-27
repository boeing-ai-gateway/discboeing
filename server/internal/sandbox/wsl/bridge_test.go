package wsl

import "testing"

func TestResolveBridgeInfo(t *testing.T) {
	tests := []struct {
		name       string
		bridgeType string
		distroName string
		port       int
		wantHost   string
		wantPipe   string
		wantErr    bool
	}{
		{
			name:       "named pipe default",
			bridgeType: BridgeTypeNamedPipe,
			distroName: "discobot",
			wantHost:   "npipe:////./pipe/discobot-docker",
			wantPipe:   "discobot-docker",
		},
		{
			name:       "named pipe sanitizes distro name",
			bridgeType: BridgeTypeNamedPipe,
			distroName: "Discobot WSL 2",
			wantHost:   "npipe:////./pipe/discobot-wsl-2-docker",
			wantPipe:   "discobot-wsl-2-docker",
		},
		{
			name:       "tcp explicit port",
			bridgeType: BridgeTypeTCP,
			port:       23755,
			wantHost:   "tcp://127.0.0.1:23755",
		},
		{
			name:       "tcp random port placeholder",
			bridgeType: BridgeTypeTCP,
			port:       0,
			wantHost:   "",
		},
		{
			name:       "invalid bridge type",
			bridgeType: "vsock",
			wantErr:    true,
		},
		{
			name:       "invalid port",
			bridgeType: BridgeTypeTCP,
			port:       70000,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveBridgeInfo(tt.bridgeType, tt.distroName, tt.port)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResolveBridgeInfo() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveBridgeInfo() error = %v", err)
			}
			if got.DockerHost != tt.wantHost {
				t.Fatalf("ResolveBridgeInfo() DockerHost = %q, want %q", got.DockerHost, tt.wantHost)
			}
			if got.PipeName != tt.wantPipe {
				t.Fatalf("ResolveBridgeInfo() PipeName = %q, want %q", got.PipeName, tt.wantPipe)
			}
		})
	}
}

func TestBridgePipeHelpers(t *testing.T) {
	pipeName := "discobot-docker"
	if got := bridgePipeDockerHost(pipeName); got != "npipe:////./pipe/discobot-docker" {
		t.Fatalf("bridgePipeDockerHost() = %q", got)
	}
	if got := bridgePipePath(pipeName); got != `\\.\pipe\discobot-docker` {
		t.Fatalf("bridgePipePath() = %q", got)
	}
}
