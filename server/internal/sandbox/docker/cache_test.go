package docker

import (
	"testing"

	containerTypes "github.com/docker/docker/api/types/container"
)

func TestContainerUsesVolume(t *testing.T) {
	t.Parallel()

	container := containerTypes.Summary{
		Mounts: []containerTypes.MountPoint{
			{Type: "bind", Name: "", Destination: "/.workspace"},
			{Type: "volume", Name: "discboeing-cache-project-a", Destination: "/.data/cache"},
			{Type: "volume", Name: "discboeing-data-session-1", Destination: "/.data"},
		},
	}

	if !containerUsesVolume(container, "discboeing-cache-project-a") {
		t.Fatal("expected exact cache volume match")
	}
	if containerUsesVolume(container, "discboeing-cache-project-b") {
		t.Fatal("did not expect different cache volume to match")
	}
}
