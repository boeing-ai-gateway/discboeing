package docker

import "testing"

func TestIsLocalHost(t *testing.T) {
	for _, host := range []string{"", "unix:///var/run/docker.sock", "npipe:////./pipe/docker_engine", "fd://"} {
		if !IsLocalHost(host) {
			t.Fatalf("host %q should be local", host)
		}
	}
	for _, host := range []string{"tcp://docker.example.com:2376", "ssh://docker.example.com", "http://docker.example.com:2375", "https://docker.example.com:2376"} {
		if IsLocalHost(host) {
			t.Fatalf("host %q should be remote", host)
		}
	}
}
