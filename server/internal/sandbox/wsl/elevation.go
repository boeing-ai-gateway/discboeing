//go:build windows

package wsl

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

const (
	seeMaskNoCloseProcess = 0x00000040
	swHide                = 0
	elevatedWaitInterval  = 200
)

type shellExecuteInfo struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         windows.Handle
	LpVerb       *uint16
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	NShow        int32
	HInstApp     windows.Handle
	LpIDList     unsafe.Pointer
	LpClass      *uint16
	HkeyClass    windows.Handle
	DwHotKey     uint32
	HIcon        windows.Handle
	HProcess     windows.Handle
}

var runElevatedProgram = func(ctx context.Context, programPath string, args ...string) (uint32, error) {
	parameters := joinWindowsCommandLineArgs(args)
	verbPtr, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return 0, fmt.Errorf("encode elevated command verb: %w", err)
	}
	filePtr, err := windows.UTF16PtrFromString(programPath)
	if err != nil {
		return 0, fmt.Errorf("encode elevated command path %q: %w", programPath, err)
	}
	parametersPtr, err := windows.UTF16PtrFromString(parameters)
	if err != nil {
		return 0, fmt.Errorf("encode elevated command arguments %q: %w", parameters, err)
	}
	directoryPtr, err := windows.UTF16PtrFromString(filepath.Dir(programPath))
	if err != nil {
		return 0, fmt.Errorf("encode elevated command directory %q: %w", filepath.Dir(programPath), err)
	}

	info := shellExecuteInfo{
		CbSize:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		FMask:        seeMaskNoCloseProcess,
		LpVerb:       verbPtr,
		LpFile:       filePtr,
		LpParameters: parametersPtr,
		LpDirectory:  directoryPtr,
		NShow:        swHide,
	}

	if err := callShellExecuteEx(&info); err != nil {
		return 0, fmt.Errorf("launch elevated command %q: %w", programPath, err)
	}
	if info.HProcess == 0 {
		return 0, fmt.Errorf("launch elevated command %q: missing process handle", programPath)
	}
	defer func() {
		_ = windows.CloseHandle(info.HProcess)
	}()

	if err := waitForElevatedProcess(ctx, info.HProcess); err != nil {
		return 0, fmt.Errorf("wait for elevated command %q: %w", programPath, err)
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.HProcess, &exitCode); err != nil {
		return 0, fmt.Errorf("get elevated command exit code: %w", err)
	}
	return exitCode, nil
}

func callShellExecuteEx(info *shellExecuteInfo) error {
	r1, _, err := shellExecuteExW.Call(uintptr(unsafe.Pointer(info)))
	if r1 != 0 {
		return nil
	}
	if err != nil && err != windows.ERROR_SUCCESS {
		return err
	}
	return windows.GetLastError()
}

func waitForElevatedProcess(ctx context.Context, process windows.Handle) error {
	for {
		status, err := windows.WaitForSingleObject(process, elevatedWaitInterval)
		if err != nil {
			return err
		}
		switch status {
		case windows.WAIT_OBJECT_0:
			return nil
		case uint32(windows.WAIT_TIMEOUT):
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		default:
			return fmt.Errorf("unexpected wait status %d", status)
		}
	}
}

func joinWindowsCommandLineArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteWindowsCommandLineArg(arg))
	}
	return strings.Join(quoted, " ")
}

func quoteWindowsCommandLineArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\n\v\"") {
		return arg
	}

	var builder strings.Builder
	builder.WriteByte('"')
	backslashes := 0
	for _, r := range arg {
		switch r {
		case '\\':
			backslashes++
		case '"':
			builder.WriteString(strings.Repeat(`\`, backslashes*2+1))
			builder.WriteRune('"')
			backslashes = 0
		default:
			if backslashes > 0 {
				builder.WriteString(strings.Repeat(`\`, backslashes))
				backslashes = 0
			}
			builder.WriteRune(r)
		}
	}
	if backslashes > 0 {
		builder.WriteString(strings.Repeat(`\`, backslashes*2))
	}
	builder.WriteByte('"')
	return builder.String()
}
