//go:build windows

package wsl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
)

const (
	wslStartupScriptEnv  = "WSL_STARTUP_SCRIPT_PATH"
	wslStartupScriptName = "discobot-wsl-startup.ps1"

	wslStartupExitOK              = 0
	wslStartupExitActionsRequired = 10
	wslStartupExitWSLUnavailable  = 42
)

type wslStartupScriptResult struct {
	Mode     string   `json:"mode,omitempty"`
	ExitCode int      `json:"exitCode,omitempty"`
	Message  string   `json:"message,omitempty"`
	Actions  []string `json:"actions,omitempty"`
	Output   string   `json:"-"`
}

type wslStartupScriptError struct {
	ExitCode int
	Code     string
	Message  string
}

func (e *wslStartupScriptError) Error() string {
	if strings.TrimSpace(e.Code) == "" {
		return e.Message
	}
	if strings.TrimSpace(e.Message) == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

type wslBootstrapRequiredError struct {
	Actions []string
	Cause   error
}

func (e *wslBootstrapRequiredError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("WSL startup bootstrap is required before continuing: %v", e.Cause)
	}
	if len(e.Actions) > 0 {
		return "WSL startup bootstrap is required before continuing; needed actions: " + strings.Join(e.Actions, ", ")
	}
	return "WSL startup bootstrap is required before continuing"
}

func (e *wslBootstrapRequiredError) Unwrap() error {
	return e.Cause
}

var (
	findWSLStartupScriptPath        = defaultWSLStartupScriptPath
	runWSLStartupPowerShell         = defaultRunWSLStartupPowerShell
	runElevatedWSLStartupPowerShell = defaultRunElevatedWSLStartupPowerShell
)

func (m *Manager) ensureHostStartupWithPowerShell(ctx context.Context, progress progressReporter) error {
	progress.Update(5, "Checking WSL host startup requirements")
	check, err := m.runWSLStartupScript(ctx, "check")
	if err != nil {
		return err
	}
	switch check.ExitCode {
	case wslStartupExitOK:
		progress.Update(100, "WSL host startup requirements are ready")
		return nil
	case wslStartupExitActionsRequired:
		progress.Update(30, wslStartupProgressMessage(check, "WSL host changes require elevation"))
	case wslStartupExitWSLUnavailable:
		return wslUnavailableStartupError(check.Message)
	default:
		return wslStartupExitError(check)
	}

	rootfsTarPath := ""
	if wslStartupNeedsRootfs(check.Actions) {
		progress.Update(35, "Preparing WSL rootfs artifact")
		rootfsTar, cleanup, err := m.prepareStartupRootfsTar(ctx, progressReporter{
			update: func(childProgress int, currentOperation string) {
				progress.Update(35+(childProgress*10/100), currentOperation)
			},
		})
		if err != nil {
			return err
		}
		rootfsTarPath = rootfsTar
		defer cleanup()
	}

	progress.Update(50, "Applying WSL host startup changes with elevation")
	execute, err := m.runElevatedWSLStartupScript(ctx, "execute", rootfsTarPath)
	if err != nil {
		return err
	}
	switch execute.ExitCode {
	case wslStartupExitOK:
		progress.Update(85, wslStartupProgressMessage(execute, "WSL host startup changes applied"))
	case wslStartupExitWSLUnavailable:
		return wslUnavailableStartupError(execute.Message)
	default:
		return wslStartupExitError(execute)
	}

	verify, err := m.runWSLStartupScript(ctx, "check")
	if err != nil {
		return err
	}
	switch verify.ExitCode {
	case wslStartupExitOK:
		progress.Update(100, "WSL host startup requirements are ready")
		return nil
	case wslStartupExitWSLUnavailable:
		return wslUnavailableStartupError(verify.Message)
	default:
		return fmt.Errorf("elevated WSL startup completed but verification still failed: %w", wslStartupExitError(verify))
	}
}

func (m *Manager) checkHostStartupReadyWithPowerShell(ctx context.Context, progress progressReporter) error {
	progress.Update(5, "Checking WSL host startup requirements")
	check, err := m.runWSLStartupScript(ctx, "check")
	if err != nil {
		return err
	}
	switch check.ExitCode {
	case wslStartupExitOK:
		progress.Update(100, "WSL host startup requirements are ready")
		return nil
	case wslStartupExitActionsRequired:
		return &wslBootstrapRequiredError{
			Actions: append([]string(nil), check.Actions...),
			Cause:   wslStartupExitError(check),
		}
	case wslStartupExitWSLUnavailable:
		return wslUnavailableStartupError(check.Message)
	default:
		return wslStartupExitError(check)
	}
}

func wslStartupProgressMessage(result wslStartupScriptResult, fallback string) string {
	if strings.TrimSpace(result.Message) != "" {
		return result.Message
	}
	if len(result.Actions) > 0 {
		return "WSL host startup actions needed: " + strings.Join(result.Actions, ", ")
	}
	return fallback
}

func wslStartupNeedsRootfs(actions []string) bool {
	return slices.Contains(actions, "import-distro") || slices.Contains(actions, "upgrade-distro")
}

func (m *Manager) prepareStartupRootfsTar(ctx context.Context, progress progressReporter) (string, func(), error) {
	artifact, err := m.downloader.EnsureArtifactWithProgress(ctx, func(update vm.ImageDownloadProgress) {
		if strings.TrimSpace(update.CurrentOperation) == "" {
			return
		}
		progress.Update(40, update.CurrentOperation)
	})
	if err != nil {
		return "", nil, fmt.Errorf("prepare WSL rootfs artifact: %w", err)
	}

	rootfsTarPath, cleanup, err := m.prepareImportRootfsTar(artifact.ArtifactPath)
	if err != nil {
		return "", nil, err
	}
	return rootfsTarPath, func() {
		if cleanup != nil {
			cleanup()
		}
		if err := m.cleanupStaleRootfsTempFiles(); err != nil {
			log.Printf("Failed to clean stale WSL rootfs temp files in %q: %v", m.cfg.WSLStateDir, err)
		}
	}, nil
}

func wslUnavailableStartupError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "WSL is not installed or is not enabled. Install WSL with `wsl.exe --install`, restart Windows if prompted, then restart Discobot."
	}
	return &wslStartupScriptError{
		ExitCode: wslStartupExitWSLUnavailable,
		Code:     "wsl_not_installed",
		Message:  message,
	}
}

func wslStartupExitError(result wslStartupScriptResult) error {
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = strings.TrimSpace(result.Output)
	}
	if message == "" {
		message = fmt.Sprintf("WSL startup script exited with code %d", result.ExitCode)
	}
	return &wslStartupScriptError{
		ExitCode: result.ExitCode,
		Code:     "wsl_startup_failed",
		Message:  message,
	}
}

func (m *Manager) runWSLStartupScript(ctx context.Context, mode string) (wslStartupScriptResult, error) {
	scriptPath, args, err := m.wslStartupScriptCommand(mode, "")
	if err != nil {
		return wslStartupScriptResult{}, err
	}
	return runWSLStartupPowerShell(ctx, scriptPath, args...)
}

func (m *Manager) runElevatedWSLStartupScript(ctx context.Context, mode string, rootfsTarPath string) (wslStartupScriptResult, error) {
	resultFile, err := os.CreateTemp("", "discobot-wsl-startup-result-*.json")
	if err != nil {
		return wslStartupScriptResult{}, fmt.Errorf("create elevated WSL startup result file: %w", err)
	}
	resultPath := resultFile.Name()
	if err := resultFile.Close(); err != nil {
		_ = os.Remove(resultPath)
		return wslStartupScriptResult{}, fmt.Errorf("close elevated WSL startup result file %q: %w", resultPath, err)
	}
	defer os.Remove(resultPath)

	scriptPath, args, err := m.wslStartupScriptCommand(mode, resultPath)
	if err != nil {
		return wslStartupScriptResult{}, err
	}
	if strings.TrimSpace(rootfsTarPath) != "" {
		args = append(args, "-RootfsArchivePath", rootfsTarPath)
	}
	return runElevatedWSLStartupPowerShell(ctx, scriptPath, resultPath, args...)
}

func (m *Manager) wslStartupScriptCommand(mode string, resultFile string) (string, []string, error) {
	scriptPath, err := findWSLStartupScriptPath()
	if err != nil {
		return "", nil, err
	}
	args := []string{
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-Mode", mode,
		"-VarDiskPath", m.varDiskPath(),
		"-VarDiskSizeGB", fmt.Sprintf("%d", m.varDiskSizeGB()),
		"-VarDiskLabel", m.varDiskLabel(),
		"-DistroName", m.mainDistro().name,
		"-InstallDir", m.mainDistro().installDir,
		"-StatePath", m.state.Path(),
		"-ImageRef", m.configuredRootfsSourceRef(),
	}
	if strings.TrimSpace(resultFile) != "" {
		args = append(args, "-ResultFile", resultFile)
	}
	return scriptPath, args, nil
}

func defaultRunWSLStartupPowerShell(ctx context.Context, _ string, args ...string) (wslStartupScriptResult, error) {
	resultFile, err := os.CreateTemp("", "discobot-wsl-startup-check-*.json")
	if err != nil {
		return wslStartupScriptResult{}, fmt.Errorf("create WSL startup check result file: %w", err)
	}
	resultPath := resultFile.Name()
	if err := resultFile.Close(); err != nil {
		_ = os.Remove(resultPath)
		return wslStartupScriptResult{}, fmt.Errorf("close WSL startup check result file %q: %w", resultPath, err)
	}
	defer os.Remove(resultPath)

	args = append(args, "-ResultFile", resultPath)
	output, runErr := exec.CommandContext(ctx, "powershell.exe", args...).CombinedOutput()
	return readWSLStartupScriptResult(resultPath, output, runErr)
}

func defaultRunElevatedWSLStartupPowerShell(ctx context.Context, _ string, resultPath string, args ...string) (wslStartupScriptResult, error) {
	exitCode, err := runElevatedProgram(ctx, "powershell.exe", args...)
	if err != nil {
		return wslStartupScriptResult{}, err
	}
	return readWSLStartupScriptResultForExitCode(resultPath, nil, int(exitCode))
}

func readWSLStartupScriptResult(resultPath string, output []byte, runErr error) (wslStartupScriptResult, error) {
	exitCode := wslStartupExitOK
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return wslStartupScriptResult{}, runErr
		}
		exitCode = exitErr.ExitCode()
	}
	return readWSLStartupScriptResultForExitCode(resultPath, output, exitCode)
}

func readWSLStartupScriptResultForExitCode(resultPath string, output []byte, exitCode int) (wslStartupScriptResult, error) {
	result := wslStartupScriptResult{
		ExitCode: exitCode,
		Output:   strings.TrimSpace(string(output)),
	}
	if raw, err := os.ReadFile(resultPath); err == nil && len(raw) > 0 {
		if jsonErr := json.Unmarshal(stripUTF8BOM(raw), &result); jsonErr != nil {
			return wslStartupScriptResult{}, fmt.Errorf("parse WSL startup script result %q: %w", resultPath, jsonErr)
		}
		result.ExitCode = exitCode
		result.Output = strings.TrimSpace(string(output))
	} else if err != nil && !os.IsNotExist(err) {
		return wslStartupScriptResult{}, fmt.Errorf("read WSL startup script result %q: %w", resultPath, err)
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = result.Output
	}
	return result, nil
}

func stripUTF8BOM(raw []byte) []byte {
	if len(raw) >= 3 && raw[0] == 0xef && raw[1] == 0xbb && raw[2] == 0xbf {
		return raw[3:]
	}
	return raw
}

func defaultWSLStartupScriptPath() (string, error) {
	var candidates []string
	seen := make(map[string]struct{})
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
		for depth := 0; dir != "" && depth < 6; depth++ {
			addCandidate(filepath.Join(dir, "wsl", wslStartupScriptName))
			addCandidate(filepath.Join(dir, "resources", "wsl", wslStartupScriptName))
			addCandidate(filepath.Join(dir, "src-tauri", "resources", "wsl", wslStartupScriptName))

			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	addCandidate(os.Getenv(wslStartupScriptEnv))
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
	return "", fmt.Errorf("could not find WSL startup script %q; expected it in the bundled wsl resources directory", wslStartupScriptName)
}
