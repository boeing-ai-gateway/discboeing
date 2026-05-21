package wsl

import "testing"

func TestResolveBridgeInfo(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		wantHost string
		wantErr  bool
	}{
		{
			name:     "tcp explicit port",
			port:     23755,
			wantHost: "tcp://127.0.0.1:23755",
		},
		{
			name:     "tcp random port placeholder",
			port:     0,
			wantHost: "",
		},
		{
			name:    "invalid port",
			port:    70000,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveBridgeInfo(tt.port)
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
		})
	}
}

func TestBridgeTCPPingURL(t *testing.T) {
	if got := bridgeTCPPingURL(23755); got != "http://127.0.0.1:23755/_ping" {
		t.Fatalf("bridgeTCPPingURL() = %q", got)
	}
	if got := bridgeTCPPingURL(0); got != "" {
		t.Fatalf("bridgeTCPPingURL(0) = %q, want empty string", got)
	}
}
