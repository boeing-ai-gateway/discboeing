//go:build windows

package wsl

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"golang.org/x/sys/windows"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
)

const (
	defaultProbeTimeout          = 5 * time.Second
	defaultReadyTimeout          = 30 * time.Second
	defaultReadyPollDelay        = 500 * time.Millisecond
	defaultTempCleanupRetryDelay = 250 * time.Millisecond
	defaultTempCleanupMaxRetries = 20
	rootfsArchiveName            = "discobot-rootfs.tar.zst"
	staleRootfsTempFileMaxAge    = 10 * time.Minute
	ext4VolumeLabelMaxLength     = 16
	discobotWSLEnvPath           = "etc/default/discobot-wsl"
)

var (
	removePath       = os.Remove
	sleep            = time.Sleep
	runCommandOutput = func(ctx context.Context, name string, args ...string) (string, error) {
		output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		return decodeCommandOutput(output), err
	}
)

// StatusDetails contains WSL-specific provider details.
type StatusDetails struct {
	DistroName        string `json:"distro_name"`
	DistroInstalled   bool   `json:"distro_installed"`
	DistroState       string `json:"distro_state,omitempty"`
	DistroVersion     int    `json:"distro_version,omitempty"`
	InstallDir        string `json:"install_dir,omitempty"`
	StateDir          string `json:"state_dir,omitempty"`
	StatePath         string `json:"state_path,omitempty"`
	VarDiskPath       string `json:"var_disk_path,omitempty"`
	VarDiskLabel      string `json:"var_disk_label,omitempty"`
	RootfsArchivePath string `json:"rootfs_archive_path,omitempty"`
	ImageRef          string `json:"image_ref,omitempty"`
}

type progressReporter struct {
	update func(progress int, currentOperation string)
}

func (r progressReporter) Update(progress int, currentOperation string) {
	if r.update != nil {
		r.update(progress, currentOperation)
	}
}

// Manager owns managed WSL distro lifecycle for the Windows sandbox backend.
type Manager struct {
	cfg        *config.Config
	state      *StateStore
	downloader *vm.ImageDownloader

	lifecycleMu sync.Mutex
	mu          sync.RWMutex
}

type managedDistro struct {
	name       string
	installDir string
}

// NewManager creates a new WSL lifecycle manager.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:   cfg,
		state: NewStateStore(cfg.WSLStateDir),
		downloader: vm.NewImageDownloader(vm.ImageDownloadConfig{
			ImageRef:                 cfg.WSLImageRef,
			DataDir:                  cfg.WSLStateDir,
			ArtifactName:             rootfsArchiveName,
			LocalArtifactPath:        cfg.WSLRootfsPath,
			ProviderName:             "WSL",
			ArtifactDescription:      "WSL rootfs artifact",
			LocalArtifactDescription: "WSL rootfs archive",
		}),
	}
}

func (m *Manager) mainDistro() managedDistro {
	return managedDistro{
		name:       strings.TrimSpace(m.cfg.WSLDistroName),
		installDir: strings.TrimSpace(m.cfg.WSLInstallDir),
	}
}

func (m *Manager) varDiskPath() string {
	if path := strings.TrimSpace(m.cfg.WSLVarDiskPath); path != "" {
		return path
	}
	return filepath.Join(m.cfg.WSLStateDir, "var.vhdx")
}

func (m *Manager) varDiskSizeGB() int {
	if m.cfg.WSLVarDiskSizeGB > 0 {
		return m.cfg.WSLVarDiskSizeGB
	}
	return 100
}

func (m *Manager) varDiskLabel() string {
	base := strings.TrimSpace(m.cfg.WSLDistroName)
	if base == "" {
		base = "discobot"
	}
	return truncateLowerDashName(sanitizeLowerDashName(base+"-var", "discobot-var"), "discobot-var", ext4VolumeLabelMaxLength)
}

func sanitizeLowerDashName(value string, fallback string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	if name == "" {
		name = fallback
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return fallback
	}
	return sanitized
}

func truncateLowerDashName(value string, fallback string, maxLength int) string {
	if maxLength <= 0 || len(value) <= maxLength {
		return value
	}
	truncated := strings.Trim(value[:maxLength], "-")
	if truncated == "" {
		return fallback
	}
	return truncated
}

// EnsureInstalled verifies that WSL tooling is available and reserves the managed distro identity.
func (m *Manager) EnsureInstalled(ctx context.Context) error {
	return m.verifyInstalledWithProgress(ctx, progressReporter{})
}

func (m *Manager) verifyInstalledWithProgress(ctx context.Context, progress progressReporter) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	return m.verifyInstalledLocked(ctx, progress)
}

func (m *Manager) verifyInstalledLocked(ctx context.Context, progress progressReporter) error {
	progress.Update(5, "Checking WSL availability")
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return fmt.Errorf("wsl.exe not found: %w", err)
	}
	if err := m.cleanupStaleRootfsTempFiles(); err != nil {
		log.Printf("Failed to clean stale WSL rootfs temp files in %q: %v", m.cfg.WSLStateDir, err)
	}

	progress.Update(25, "Verifying managed WSL distro registration")
	_, found, err := m.probeDistro(ctx)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("managed WSL distro %q is not installed", m.cfg.WSLDistroName)
	}
	if err := m.hideWindowsTerminalWSLProfiles(); err != nil {
		log.Printf("Failed to hide managed WSL distro %q in Windows Terminal settings: %v", m.cfg.WSLDistroName, err)
	}

	progress.Update(100, "Managed WSL distro registration verified")
	return nil
}

func (m *Manager) ensureMainDistroReady(ctx context.Context, progress progressReporter) (DistroInfo, error) {
	progress.Update(40, "Checking managed WSL distro state")
	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return DistroInfo{}, err
	}
	if !found {
		return DistroInfo{}, fmt.Errorf("managed WSL distro %q is not installed", m.cfg.WSLDistroName)
	}

	if err := m.waitForMainDistroReadiness(ctx, distro, progress); err != nil {
		if !shouldRecoverBrokenDistro(err) {
			return DistroInfo{}, err
		}
		if err := m.recoverBrokenMainDistro(ctx, progress, err); err != nil {
			return DistroInfo{}, err
		}

		progress.Update(40, "Rechecking managed WSL distro state")
		distro, found, err = m.probeDistro(ctx)
		if err != nil {
			return DistroInfo{}, err
		}
		if !found {
			return DistroInfo{}, fmt.Errorf("managed WSL distro %q is not installed after recovery", m.cfg.WSLDistroName)
		}

		if err := m.waitForMainDistroReadiness(ctx, distro, progress); err != nil {
			return DistroInfo{}, err
		}
	}

	distro, found, err = m.probeDistro(ctx)
	if err != nil {
		return DistroInfo{}, err
	}
	if !found {
		return DistroInfo{}, fmt.Errorf("managed WSL distro %q disappeared after startup", m.cfg.WSLDistroName)
	}
	return distro, nil
}

func (m *Manager) waitForMainDistroReadiness(ctx context.Context, distro DistroInfo, progress progressReporter) error {
	if !isRunnableDistroState(distro.State) {
		progress.Update(50, "Waiting for managed WSL distro import to settle")
		var err error
		distro, err = m.waitForNamedDistroRunnableState(ctx, m.mainDistro().name)
		if err != nil {
			return err
		}
	}
	if isStoppedDistroState(distro.State) {
		progress.Update(50, "Starting managed WSL distro")
		if err := m.startDistro(ctx); err != nil {
			return err
		}
	}

	progress.Update(65, "Waiting for systemd readiness")
	if err := m.waitForSystemdReady(ctx); err != nil {
		return err
	}
	progress.Update(72, "Waiting for /var runtime paths")
	if err := m.waitForVarReady(ctx); err != nil {
		return err
	}
	progress.Update(80, "Waiting for docker.service readiness")
	if err := m.waitForDockerReady(ctx); err != nil {
		return err
	}
	return nil
}

// Stop terminates the managed distro if it is currently running.
func (m *Manager) Stop(ctx context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if isStoppedDistroState(distro.State) {
		return nil
	}

	if _, err := m.runCommand(ctx, "wsl.exe", "--terminate", m.cfg.WSLDistroName); err != nil {
		return fmt.Errorf("terminate managed WSL distro %q: %w", m.cfg.WSLDistroName, err)
	}
	return nil
}

func (m *Manager) probeDistro(ctx context.Context) (DistroInfo, bool, error) {
	return m.probeNamedDistro(ctx, m.mainDistro().name)
}

func (m *Manager) probeNamedDistro(ctx context.Context, distroName string) (DistroInfo, bool, error) {
	output, err := m.runCommand(ctx, "wsl.exe", "--list", "--verbose")
	if err != nil {
		return DistroInfo{}, false, err
	}

	distros, err := ParseDistroList(output)
	if err != nil {
		return DistroInfo{}, false, err
	}
	distro, found := FindDistro(distros, distroName)
	return distro, found, nil
}

func (m *Manager) decompressRootfsArchive(rootfsArchivePath string) (string, func(), error) {
	if err := os.MkdirAll(m.cfg.WSLStateDir, 0755); err != nil {
		return "", nil, fmt.Errorf("create WSL state dir: %w", err)
	}

	src, err := os.Open(rootfsArchivePath)
	if err != nil {
		return "", nil, fmt.Errorf("open rootfs archive %q: %w", rootfsArchivePath, err)
	}
	defer src.Close()

	decoder, err := zstd.NewReader(src)
	if err != nil {
		return "", nil, fmt.Errorf("open zstd decoder for %q: %w", rootfsArchivePath, err)
	}
	defer decoder.Close()

	tempFile, err := os.CreateTemp(m.cfg.WSLStateDir, "discobot-rootfs-*.tar")
	if err != nil {
		return "", nil, fmt.Errorf("create temp rootfs tar: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := func() {
		if err := cleanupTempRootfsFile(tempPath); err != nil {
			log.Printf("Failed to remove temp WSL rootfs tar %q: %v", tempPath, err)
		}
	}

	if _, err := io.Copy(tempFile, decoder); err != nil {
		tempFile.Close()
		cleanup()
		return "", nil, fmt.Errorf("decompress rootfs archive %q: %w", rootfsArchivePath, err)
	}
	if err := tempFile.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close temp rootfs tar %q: %w", tempPath, err)
	}
	return tempPath, cleanup, nil
}

func (m *Manager) prepareImportRootfsTar(rootfsArchivePath string) (string, func(), error) {
	baseTarPath, baseCleanup, err := m.decompressRootfsArchive(rootfsArchivePath)
	if err != nil {
		return "", nil, err
	}

	importTarPath, importCleanup, err := customizeImportRootfsTar(baseTarPath, m.cfg.WSLStateDir, m.buildDiscobotWSLEnvFile())
	if err != nil {
		baseCleanup()
		return "", nil, err
	}

	return importTarPath, func() {
		importCleanup()
		baseCleanup()
	}, nil
}

func (m *Manager) buildDiscobotWSLEnvFile() string {
	return strings.Join([]string{
		"DISCOBOT_GUEST_PLATFORM=" + quoteShellEnvValue("wsl"),
		"DISCOBOT_VAR_DISK_LABEL=" + quoteShellEnvValue(m.varDiskLabel()),
		"",
	}, "\n")
}

func customizeImportRootfsTar(sourceTarPath string, outputDir string, envFileContents string) (string, func(), error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", nil, fmt.Errorf("create WSL state dir: %w", err)
	}

	src, err := os.Open(sourceTarPath)
	if err != nil {
		return "", nil, fmt.Errorf("open temp rootfs tar %q: %w", sourceTarPath, err)
	}
	defer src.Close()

	dst, err := os.CreateTemp(outputDir, "discobot-rootfs-import-*.tar")
	if err != nil {
		return "", nil, fmt.Errorf("create customized rootfs tar: %w", err)
	}
	dstPath := dst.Name()
	cleanup := func() {
		if err := cleanupTempRootfsFile(dstPath); err != nil {
			log.Printf("Failed to remove temp WSL import rootfs tar %q: %v", dstPath, err)
		}
	}

	tr := tar.NewReader(src)
	tw := tar.NewWriter(dst)
	fail := func(err error) (string, func(), error) {
		_ = tw.Close()
		_ = dst.Close()
		cleanup()
		return "", nil, err
	}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fail(fmt.Errorf("read temp rootfs tar %q: %w", sourceTarPath, err))
		}
		if normalizeTarPath(hdr.Name) == discobotWSLEnvPath {
			continue
		}

		headerCopy := *hdr
		if err := tw.WriteHeader(&headerCopy); err != nil {
			return fail(fmt.Errorf("write customized rootfs header %q: %w", headerCopy.Name, err))
		}
		if hdr.Size == 0 {
			continue
		}
		if _, err := io.CopyN(tw, tr, hdr.Size); err != nil {
			return fail(fmt.Errorf("copy customized rootfs entry %q: %w", headerCopy.Name, err))
		}
	}

	envBytes := []byte(envFileContents)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "./" + discobotWSLEnvPath,
		Mode:     0644,
		Size:     int64(len(envBytes)),
		Typeflag: tar.TypeReg,
		ModTime:  time.Unix(0, 0),
	}); err != nil {
		return fail(fmt.Errorf("write customized rootfs header %q: %w", discobotWSLEnvPath, err))
	}
	if _, err := tw.Write(envBytes); err != nil {
		return fail(fmt.Errorf("write customized rootfs contents %q: %w", discobotWSLEnvPath, err))
	}
	if err := tw.Close(); err != nil {
		_ = dst.Close()
		cleanup()
		return "", nil, fmt.Errorf("close customized rootfs tar writer: %w", err)
	}
	if err := dst.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close customized rootfs tar %q: %w", dstPath, err)
	}

	return dstPath, cleanup, nil
}

func (m *Manager) cleanupStaleRootfsTempFiles() error {
	stateDir := strings.TrimSpace(m.cfg.WSLStateDir)
	if stateDir == "" {
		return nil
	}

	entries, err := os.ReadDir(stateDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read WSL state dir %q: %w", stateDir, err)
	}

	cutoff := time.Now().Add(-staleRootfsTempFileMaxAge)
	var cleanupErrs []error
	for _, entry := range entries {
		if entry.IsDir() || !isRootfsTempTarName(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("stat %q: %w", filepath.Join(stateDir, entry.Name()), err))
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		tempPath := filepath.Join(stateDir, entry.Name())
		if err := cleanupTempRootfsFile(tempPath); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("remove stale rootfs temp file %q: %w", tempPath, err))
		}
	}
	return errors.Join(cleanupErrs...)
}

func cleanupTempRootfsFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	var lastErr error
	for attempt := range defaultTempCleanupMaxRetries {
		err := removePath(path)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		lastErr = err
		if !isRetryableCleanupError(err) || attempt == defaultTempCleanupMaxRetries-1 {
			return err
		}
		sleep(defaultTempCleanupRetryDelay)
	}
	return lastErr
}

func isRootfsTempTarName(name string) bool {
	return strings.HasSuffix(name, ".tar") &&
		(strings.HasPrefix(name, "discobot-rootfs-") || strings.HasPrefix(name, "discobot-rootfs-import-"))
}

func isRetryableCleanupError(err error) bool {
	return isWindowsAccessOrSharingError(err)
}

func isWindowsAccessOrSharingError(err error) bool {
	var errno windows.Errno
	if errors.As(err, &errno) {
		return errno == windows.ERROR_ACCESS_DENIED || errno == windows.ERROR_SHARING_VIOLATION
	}
	return false
}

func normalizeTarPath(name string) string {
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	return path.Clean(name)
}

func quoteShellEnvValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func (m *Manager) startDistro(ctx context.Context) error {
	return m.startNamedDistro(ctx, m.mainDistro().name)
}

func (m *Manager) startNamedDistro(ctx context.Context, distroName string) error {
	_, err := m.runCommand(ctx, "wsl.exe", "-d", distroName, "--", "true")
	if err != nil {
		return fmt.Errorf("start managed WSL distro %q: %w", distroName, err)
	}
	return nil
}

func (m *Manager) waitForSystemdReady(ctx context.Context) error {
	return m.waitForSystemdReadyInDistro(ctx, m.mainDistro().name)
}

func (m *Manager) waitForNamedDistroRunnableState(ctx context.Context, distroName string) (DistroInfo, error) {
	var readyDistro DistroInfo
	if err := m.waitForCommandSuccess(ctx, "wait for managed WSL distro to become runnable", func(ctx context.Context) error {
		distro, found, err := m.probeNamedDistro(ctx, distroName)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("managed WSL distro %q disappeared while waiting to become runnable", distroName)
		}
		if isRunnableDistroState(distro.State) {
			readyDistro = distro
			return nil
		}
		return fmt.Errorf("managed WSL distro %q is still %s", distroName, distro.State)
	}); err != nil {
		return DistroInfo{}, err
	}
	return readyDistro, nil
}

func (m *Manager) waitForSystemdReadyInDistro(ctx context.Context, distroName string) error {
	return m.waitForCommandSuccessUntilCanceled(ctx, "wait for systemd readiness", func(ctx context.Context) error {
		args := []string{"-d", distroName, "--", "systemctl", "is-system-running"}
		output, err := runCommandOutput(ctx, "wsl.exe", args...)
		state := strings.TrimSpace(output)
		if state == "running" || state == "degraded" {
			return nil
		}
		if stopErr := m.checkNamedDistroStillRegistered(ctx, distroName, "waiting for systemd readiness"); stopErr != nil {
			return stopErr
		}
		if err != nil {
			if state == "" {
				return fmt.Errorf("wsl.exe %s: %w", strings.Join(args, " "), err)
			}
			return fmt.Errorf("wsl.exe %s: %w: %s", strings.Join(args, " "), err, state)
		}
		return fmt.Errorf("systemd state is %q", state)
	})
}

func (m *Manager) waitForDockerReady(ctx context.Context) error {
	distroName := m.mainDistro().name
	return m.waitForCommandSuccessUntilCanceled(ctx, "wait for docker.service readiness", func(ctx context.Context) error {
		output, err := m.runInNamedDistro(ctx, distroName, "systemctl", "is-active", "docker.service")
		state := strings.TrimSpace(output)
		if err != nil {
			if stopErr := m.checkNamedDistroStillRegistered(ctx, distroName, "waiting for docker.service readiness"); stopErr != nil {
				return stopErr
			}
			return err
		}
		if state != "active" {
			if stopErr := m.checkNamedDistroStillRegistered(ctx, distroName, "waiting for docker.service readiness"); stopErr != nil {
				return stopErr
			}
			return fmt.Errorf("docker.service state is %q", state)
		}
		return nil
	})
}

func (m *Manager) waitForVarReady(ctx context.Context) error {
	return m.waitForVarReadyInDistro(ctx, m.mainDistro().name)
}

func (m *Manager) waitForVarReadyInDistro(ctx context.Context, distroName string) error {
	return m.waitForCommandSuccessUntilCanceled(ctx, "wait for /var readiness", func(ctx context.Context) error {
		if _, err := m.runInNamedDistro(ctx, distroName, "mountpoint", "-q", "/var"); err != nil {
			if stopErr := m.checkNamedDistroStillRegistered(ctx, distroName, "waiting for /var readiness"); stopErr != nil {
				return stopErr
			}
			return err
		}
		return nil
	})
}

func (m *Manager) waitForCommandSuccess(ctx context.Context, description string, fn func(context.Context) error) error {
	return m.waitForCommandSuccessWithFallbackTimeout(ctx, description, defaultReadyTimeout, fn)
}

func (m *Manager) waitForCommandSuccessUntilCanceled(ctx context.Context, description string, fn func(context.Context) error) error {
	return m.waitForCommandSuccessWithFallbackTimeout(ctx, description, 0, fn)
}

func (m *Manager) waitForCommandSuccessWithFallbackTimeout(ctx context.Context, description string, fallbackTimeout time.Duration, fn func(context.Context) error) error {
	deadlineCtx := ctx
	if fallbackTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			deadlineCtx, cancel = context.WithTimeout(ctx, fallbackTimeout)
			defer cancel()
		}
	}

	ticker := time.NewTicker(defaultReadyPollDelay)
	defer ticker.Stop()

	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(deadlineCtx, defaultProbeTimeout)
		lastErr = fn(attemptCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		if shouldRecoverBrokenDistro(lastErr) {
			return lastErr
		}

		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("%s: %w (last error: %v)", description, deadlineCtx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func (m *Manager) runInNamedDistro(ctx context.Context, distroName string, args ...string) (string, error) {
	base := []string{"-d", distroName, "--"}
	base = append(base, args...)
	return m.runCommand(ctx, "wsl.exe", base...)
}

func (m *Manager) configuredRootfsSourceRef() string {
	if rootfsPath := strings.TrimSpace(m.cfg.WSLRootfsPath); rootfsPath != "" {
		return rootfsPath
	}
	return strings.TrimSpace(m.cfg.WSLImageRef)
}

func shouldRecoverBrokenDistro(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "managed wsl distro") {
		return strings.Contains(message, "stopped while") || strings.Contains(message, "disappeared while")
	}
	return strings.Contains(message, "wsl/service/e_unexpected") ||
		strings.Contains(message, "catastrophic failure")
}

func isRunnableDistroState(state string) bool {
	return isStoppedDistroState(state) || isRunningDistroState(state)
}

func isStoppedDistroState(state string) bool {
	return strings.EqualFold(state, "Stopped")
}

func isRunningDistroState(state string) bool {
	return strings.EqualFold(state, "Running")
}

func (m *Manager) checkNamedDistroStillRegistered(ctx context.Context, distroName string, operation string) error {
	_, found, err := m.probeNamedDistro(ctx, distroName)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("managed WSL distro %q disappeared while %s", distroName, operation)
	}
	return nil
}

func (m *Manager) recoverBrokenMainDistro(_ context.Context, progress progressReporter, cause error) error {
	log.Printf("Managed WSL distro %q appears broken after startup failure: %v", m.mainDistro().name, cause)
	progress.Update(45, "WSL startup repair is required")
	return &wslBootstrapRequiredError{
		Actions: []string{"repair-distro"},
		Cause:   cause,
	}
}
