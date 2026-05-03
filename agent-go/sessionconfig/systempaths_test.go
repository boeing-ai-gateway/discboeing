package sessionconfig

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	originalRoots := discobotSystemRoots
	discobotSystemRoots = nil
	code := m.Run()
	discobotSystemRoots = originalRoots
	os.Exit(code)
}
