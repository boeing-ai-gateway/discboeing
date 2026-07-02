package sessionconfig

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	originalRoots := discboeingSystemRoots
	discboeingSystemRoots = nil
	code := m.Run()
	discboeingSystemRoots = originalRoots
	os.Exit(code)
}
