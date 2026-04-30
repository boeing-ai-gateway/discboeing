//go:build windows

package wsl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const wslElevationHelperBaseName = "discobot-wsl-helper"

var (
	osExecutablePath = os.Executable
	osGetwdPath      = os.Getwd
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

var runElevatedWSLHelper = func(ctx context.Context, helperPath string, args ...string) (string, error) {
	resultFile, err := os.CreateTemp("", "discobot-wsl-helper-result-*.log")
	if err != nil {
		return "", fmt.Errorf("create elevated helper result log: %w", err)
	}
	resultPath := resultFile.Name()
	if err := resultFile.Close(); err != nil {
		_ = os.Remove(resultPath)
		return "", fmt.Errorf("close elevated helper result log %q: %w", resultPath, err)
	}
	defer os.Remove(resultPath)

	parameters := joinWindowsCommandLineArgs(append([]string{"--result-file", resultPath}, args...))
	verbPtr, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return "", fmt.Errorf("encode elevated helper verb: %w", err)
	}
	filePtr, err := windows.UTF16PtrFromString(helperPath)
	if err != nil {
		return "", fmt.Errorf("encode elevated helper path %q: %w", helperPath, err)
	}
	parametersPtr, err := windows.UTF16PtrFromString(parameters)
	if err != nil {
		return "", fmt.Errorf("encode elevated helper arguments %q: %w", parameters, err)
	}
	directoryPtr, err := windows.UTF16PtrFromString(filepath.Dir(helperPath))
	if err != nil {
		return "", fmt.Errorf("encode elevated helper directory %q: %w", filepath.Dir(helperPath), err)
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
		return "", fmt.Errorf("launch elevated helper %q: %w", helperPath, err)
	}
	if info.HProcess == 0 {
		return "", fmt.Errorf("launch elevated helper %q: missing process handle", helperPath)
	}
	defer windows.CloseHandle(info.HProcess)

	if err := waitForElevatedProcess(ctx, info.HProcess); err != nil {
		return "", fmt.Errorf("wait for elevated helper %q: %w", helperPath, err)
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.HProcess, &exitCode); err != nil {
		return "", fmt.Errorf("get elevated helper exit code: %w", err)
	}

	resultBytes, readErr := os.ReadFile(resultPath)
	resultText := strings.TrimSpace(string(resultBytes))
	if readErr != nil && !os.IsNotExist(readErr) {
		return "", fmt.Errorf("read elevated helper result log %q: %w", resultPath, readErr)
	}
	if exitCode != 0 {
		if resultText == "" {
			resultText = fmt.Sprintf("elevated helper exited with code %d", exitCode)
		}
		return "", fmt.Errorf("%s", resultText)
	}
	return resultText, nil
}

var findWSLElevationHelperPath = defaultWSLElevationHelperPath

func (m *Manager) runWSLElevationHelper(ctx context.Context, args ...string) (string, error) {
	helperPath, err := findWSLElevationHelperPath()
	if err != nil {
		return "", err
	}
	return runElevatedWSLHelper(ctx, helperPath, args...)
}

func defaultWSLElevationHelperPath() (string, error) {
	helperName := wslElevationHelperBinaryName()
	var (
		candidates []string
		seen       = make(map[string]struct{})
	)

	addCandidate := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	addCandidatesFromBase := func(baseDir string) {
		dir := strings.TrimSpace(baseDir)
		for depth := 0; dir != "" && depth < 5; depth++ {
			addCandidate(filepath.Join(dir, helperName))
			addCandidate(filepath.Join(dir, "src-tauri", "binaries", helperName))

			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if exePath, err := osExecutablePath(); err == nil && strings.TrimSpace(exePath) != "" {
		addCandidatesFromBase(filepath.Dir(exePath))
	}
	if cwd, err := osGetwdPath(); err == nil && strings.TrimSpace(cwd) != "" {
		addCandidatesFromBase(cwd)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			absPath, err := filepath.Abs(candidate)
			if err == nil {
				return absPath, nil
			}
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not find %s; expected it next to the Discobot server binary or in a nearby src-tauri/binaries directory", helperName)
}

func wslElevationHelperBinaryName() string {
	return wslElevationHelperBaseName + "-" + windowsTargetTriple() + ".exe"
}

func windowsTargetTriple() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64-pc-windows-msvc"
	case "arm64":
		return "aarch64-pc-windows-msvc"
	default:
		return runtime.GOARCH + "-pc-windows-msvc"
	}
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
