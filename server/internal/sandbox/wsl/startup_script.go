//go:build windows

package wsl

import (
	"bytes"
	"context"
	"crypto/sha256"
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
	code := strings.TrimSpace(e.Code)
	message := strings.TrimSpace(e.Message)
	if code == "" {
		return message
	}
	if message == "" {
		return code
	}
	return code + ": " + message
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
	resultPath, cleanup, err := createTempWSLStartupResultFile("discobot-wsl-startup-result-*.json", "elevated WSL startup result")
	if err != nil {
		return wslStartupScriptResult{}, err
	}
	defer cleanup()

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
	mainDistro := m.mainDistro()
	args := []string{
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-Mode", mode,
		"-VarDiskPath", m.varDiskPath(),
		"-VarDiskSizeGB", fmt.Sprintf("%d", m.varDiskSizeGB()),
		"-VarDiskLabel", m.varDiskLabel(),
		"-RuntimeID", m.runtimeID,
		"-DistroName", mainDistro.name,
		"-InstallDir", mainDistro.installDir,
		"-StatePath", m.state.Path(),
		"-ImageRef", m.configuredRootfsSourceRef(),
	}
	if strings.TrimSpace(resultFile) != "" {
		args = append(args, "-ResultFile", resultFile)
	}
	return scriptPath, args, nil
}

func defaultRunWSLStartupPowerShell(ctx context.Context, _ string, args ...string) (wslStartupScriptResult, error) {
	resultPath, cleanup, err := createTempWSLStartupResultFile("discobot-wsl-startup-check-*.json", "WSL startup check result")
	if err != nil {
		return wslStartupScriptResult{}, err
	}
	defer cleanup()

	args = append(args, "-ResultFile", resultPath)
	output, runErr := exec.CommandContext(ctx, "powershell.exe", args...).CombinedOutput()
	return readWSLStartupScriptResult(resultPath, output, runErr)
}

func createTempWSLStartupResultFile(pattern string, description string) (string, func(), error) {
	resultFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, fmt.Errorf("create %s file: %w", description, err)
	}
	resultPath := resultFile.Name()
	if err := resultFile.Close(); err != nil {
		_ = os.Remove(resultPath)
		return "", nil, fmt.Errorf("close %s file %q: %w", description, resultPath, err)
	}
	return resultPath, func() {
		_ = os.Remove(resultPath)
	}, nil
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
	outputText := strings.TrimSpace(decodeCommandOutput(output))
	result := wslStartupScriptResult{
		ExitCode: exitCode,
		Output:   outputText,
	}
	if raw, err := os.ReadFile(resultPath); err == nil && len(raw) > 0 {
		if jsonErr := json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}), &result); jsonErr != nil {
			return wslStartupScriptResult{}, fmt.Errorf("parse WSL startup script result %q: %w", resultPath, jsonErr)
		}
		result.ExitCode = exitCode
		result.Output = outputText
	} else if err != nil && !os.IsNotExist(err) {
		return wslStartupScriptResult{}, fmt.Errorf("read WSL startup script result %q: %w", resultPath, err)
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = result.Output
	}
	return result, nil
}

func defaultWSLStartupScriptPath() (string, error) {
	if scriptPath := strings.TrimSpace(os.Getenv(wslStartupScriptEnv)); scriptPath != "" {
		if info, err := os.Stat(scriptPath); err == nil && !info.IsDir() {
			absPath, err := filepath.Abs(scriptPath)
			if err == nil {
				return absPath, nil
			}
			return scriptPath, nil
		}
	}
	return stageEmbeddedWSLStartupScript()
}

func stageEmbeddedWSLStartupScript() (string, error) {
	if len(embeddedWSLStartupScript) == 0 {
		return "", fmt.Errorf("embedded WSL startup script %q is empty", wslStartupScriptName)
	}

	sum := sha256.Sum256(embeddedWSLStartupScript)
	scriptDir, err := wslStartupScriptCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(scriptDir, 0700); err != nil {
		return "", fmt.Errorf("create embedded WSL startup script cache directory %q: %w", scriptDir, err)
	}

	scriptPath := filepath.Join(scriptDir, fmt.Sprintf("discobot-wsl-startup-%x.ps1", sum[:8]))
	if current, err := os.ReadFile(scriptPath); err == nil && bytes.Equal(current, embeddedWSLStartupScript) {
		return scriptPath, nil
	}

	tempFile, err := os.CreateTemp(scriptDir, "discobot-wsl-startup-*.ps1")
	if err != nil {
		return "", fmt.Errorf("create embedded WSL startup script temp file: %w", err)
	}
	tempPath := tempFile.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(embeddedWSLStartupScript); err != nil {
		_ = tempFile.Close()
		return "", fmt.Errorf("write embedded WSL startup script temp file %q: %w", tempPath, err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("close embedded WSL startup script temp file %q: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, scriptPath); err != nil {
		if removeErr := os.Remove(scriptPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", fmt.Errorf("replace embedded WSL startup script %q: remove existing file: %w", scriptPath, removeErr)
		}
		if renameErr := os.Rename(tempPath, scriptPath); renameErr != nil {
			return "", fmt.Errorf("stage embedded WSL startup script %q: %w", scriptPath, renameErr)
		}
	}
	removeTemp = false
	return scriptPath, nil
}

func wslStartupScriptCacheDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheDir) == "" {
		cacheDir = os.TempDir()
	}
	if strings.TrimSpace(cacheDir) == "" {
		if err != nil {
			return "", fmt.Errorf("resolve embedded WSL startup script cache directory: %w", err)
		}
		return "", fmt.Errorf("resolve embedded WSL startup script cache directory: no cache or temp directory")
	}
	return filepath.Join(cacheDir, "Discobot", "wsl"), nil
}
