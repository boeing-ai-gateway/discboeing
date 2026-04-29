//go:build windows

package cli

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestEscWatchInputMode_PreservesProcessedInput(t *testing.T) {
	oldMode := uint32(
		windows.ENABLE_PROCESSED_INPUT |
			windows.ENABLE_LINE_INPUT |
			windows.ENABLE_ECHO_INPUT |
			windows.ENABLE_WINDOW_INPUT,
	)

	mode := escWatchInputMode(oldMode)

	if mode&windows.ENABLE_LINE_INPUT != 0 {
		t.Fatal("expected ENABLE_LINE_INPUT to be cleared")
	}
	if mode&windows.ENABLE_ECHO_INPUT != 0 {
		t.Fatal("expected ENABLE_ECHO_INPUT to be cleared")
	}
	if mode&windows.ENABLE_PROCESSED_INPUT == 0 {
		t.Fatal("expected ENABLE_PROCESSED_INPUT to remain enabled for Ctrl+C")
	}
	if mode&windows.ENABLE_WINDOW_INPUT == 0 {
		t.Fatal("expected unrelated input mode bits to be preserved")
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_INPUT == 0 {
		t.Fatal("expected ENABLE_VIRTUAL_TERMINAL_INPUT to be enabled")
	}
}
