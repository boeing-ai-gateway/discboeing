//go:build windows

package cli

import (
	"context"
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// watchEscDuringTurn monitors stdin for ESC while an agent turn is streaming.
//
// On Windows we disable line input so ReadFile returns each keystroke as it
// arrives, while keeping processed input enabled so Ctrl+C still follows the
// normal SIGINT path. Any byte other than ESC is discarded so it does not show
// up at the next prompt.
func watchEscDuringTurn(ctx context.Context, cancel context.CancelFunc) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return
	}

	handle := windows.Handle(fd)
	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		return
	}

	mode := escWatchInputMode(oldMode)
	if err := windows.SetConsoleMode(handle, mode); err != nil {
		// ENABLE_VIRTUAL_TERMINAL_INPUT is not available everywhere. Retry with
		// the minimal mode change needed to read single keystrokes.
		mode &^= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
		if err := windows.SetConsoleMode(handle, mode); err != nil {
			return
		}
	}
	defer windows.SetConsoleMode(handle, oldMode) //nolint:errcheck

	buf := make([]byte, 1)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		event, err := windows.WaitForSingleObject(handle, 20)
		if err != nil {
			return
		}
		if event == uint32(windows.WAIT_TIMEOUT) {
			continue
		}
		if event != windows.WAIT_OBJECT_0 {
			return
		}

		for {
			var read uint32
			if err := windows.ReadFile(handle, buf, &read, nil); err != nil {
				return
			}
			if read == 0 {
				break
			}
			if buf[0] == 0x1b { // ESC
				cancel()
				return
			}

			event, err = windows.WaitForSingleObject(handle, 0)
			if err != nil || event != windows.WAIT_OBJECT_0 {
				break
			}
		}
	}
}

func escWatchInputMode(oldMode uint32) uint32 {
	mode := oldMode &^ (windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT)
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	return mode
}
