//go:build windows

package wsl

import (
	"archive/tar"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/Microsoft/go-winio"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/sys/windows"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/windows/virtualdisk"
)

const (
	defaultProbeTimeout          = 5 * time.Second
	defaultReadyTimeout          = 30 * time.Second
	defaultReadyPollDelay        = 500 * time.Millisecond
	defaultMoveRetryDelay        = 250 * time.Millisecond
	defaultMoveMaxRetries        = 40
	defaultTempCleanupRetryDelay = 250 * time.Millisecond
	defaultTempCleanupMaxRetries = 20
	staleRootfsTempFileMaxAge    = 10 * time.Minute
	dockerSockPath               = "/var/run/docker.sock"
	bridgeLogPath                = "/tmp/discobot-docker-bridge.log"
	bridgeUnitName               = "discobot-docker-bridge"
	discobotWSLEnvPath           = "etc/default/discobot-wsl"
)

var (
	renamePath       = os.Rename
	removePath       = os.Remove
	sleep            = time.Sleep
	createDynamicVHD = virtualdisk.CreateDynamicVHDX
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).CombinedOutput()
	}
)

// RuntimeInfo contains the current runtime connection details returned by EnsureRunning.
type RuntimeInfo struct {
	DistroName        string `json:"distro_name"`
	DistroInstalled   bool   `json:"distro_installed"`
	DistroState       string `json:"distro_state,omitempty"`
	DistroVersion     int    `json:"distro_version,omitempty"`
	BridgeType        string `json:"bridge_type"`
	BridgePort        int    `json:"bridge_port,omitempty"`
	BridgePipeName    string `json:"bridge_pipe_name,omitempty"`
	BridgeDockerHost  string `json:"bridge_docker_host,omitempty"`
	InstallDir        string `json:"install_dir,omitempty"`
	StateDir          string `json:"state_dir,omitempty"`
	RootfsArchivePath string `json:"rootfs_archive_path,omitempty"`
	ImageRef          string `json:"image_ref,omitempty"`
	BridgeReady       bool   `json:"bridge_ready"`
}

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
	BridgeType        string `json:"bridge_type,omitempty"`
	BridgePort        int    `json:"bridge_port,omitempty"`
	BridgePipeName    string `json:"bridge_pipe_name,omitempty"`
	BridgeDockerHost  string `json:"bridge_docker_host,omitempty"`
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
	downloader *ImageDownloader

	lifecycleMu       sync.Mutex
	mu                sync.RWMutex
	pipeListener      net.Listener
	pipeListenerName  string
	pipeListenerClose chan struct{}
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
		downloader: NewImageDownloader(ImageDownloadConfig{
			ImageRef:           cfg.WSLImageRef,
			DataDir:            cfg.WSLStateDir,
			LocalRootfsArchive: cfg.WSLRootfsPath,
		}),
	}
}

func (m *Manager) mainDistro() managedDistro {
	return managedDistro{
		name:       strings.TrimSpace(m.cfg.WSLDistroName),
		installDir: strings.TrimSpace(m.cfg.WSLInstallDir),
	}
}

func (m *Manager) legacyDataDistro() managedDistro {
	name := strings.TrimSpace(m.cfg.WSLDistroName) + "-data"
	installDir := filepath.Join(m.cfg.WSLStateDir, "data-distro")
	return managedDistro{
		name:       name,
		installDir: installDir,
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
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(base + "-var") {
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
	name := strings.Trim(builder.String(), "-")
	if name == "" {
		return "discobot-var"
	}
	return name
}

// EnsureInstalled verifies that WSL tooling is available and reserves the managed distro identity.
func (m *Manager) EnsureInstalled(ctx context.Context) error {
	return m.ensureInstalledWithProgress(ctx, progressReporter{})
}

func (m *Manager) ensureInstalledWithProgress(ctx context.Context, progress progressReporter) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	return m.ensureInstalledLocked(ctx, progress)
}

func (m *Manager) ensureInstalledLocked(ctx context.Context, progress progressReporter) error {
	progress.Update(5, "Checking WSL availability")
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return fmt.Errorf("wsl.exe not found: %w", err)
	}
	if err := m.cleanupStaleRootfsTempFiles(); err != nil {
		log.Printf("Failed to clean stale WSL rootfs temp files in %q: %v", m.cfg.WSLStateDir, err)
	}

	progress.Update(10, "Ensuring persistent WSL /var disk exists and is attached")
	if err := m.ensureVarDiskAttached(ctx); err != nil {
		return err
	}

	progress.Update(20, "Checking managed WSL runtime image")
	if err := m.upgradeIfNeededLocked(ctx); err != nil {
		return err
	}

	progress.Update(25, "Checking managed WSL distro registration")
	_, found, err := m.probeDistro(ctx)
	if err != nil {
		return err
	}
	if !found {
		importProgress := progressReporter{
			update: func(childProgress int, currentOperation string) {
				mapped := 25 + (childProgress * 70 / 100)
				progress.Update(mapped, currentOperation)
			},
		}
		if err := m.importDistro(ctx, importProgress); err != nil {
			return err
		}
	}
	if err := m.hideWindowsTerminalWSLProfiles(); err != nil {
		log.Printf("Failed to hide managed WSL distro %q in Windows Terminal settings: %v", m.cfg.WSLDistroName, err)
	}

	progress.Update(100, "Managed WSL distro and WSL /var disk are installed")
	return nil
}

func (m *Manager) hideWindowsTerminalWSLProfiles() error {
	distroName := strings.TrimSpace(m.cfg.WSLDistroName)
	if distroName == "" {
		return nil
	}

	for _, settingsPath := range windowsTerminalSettingsPaths() {
		data, err := os.ReadFile(settingsPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read Windows Terminal settings %q: %w", settingsPath, err)
		}

		updated, changed, err := hideWindowsTerminalWSLProfilesInSettings(data, distroName, m.cfg.DesktopIconPath)
		if err != nil {
			return fmt.Errorf("update Windows Terminal settings %q: %w", settingsPath, err)
		}
		if !changed {
			continue
		}
		if err := os.WriteFile(settingsPath, updated, 0644); err != nil {
			return fmt.Errorf("write Windows Terminal settings %q: %w", settingsPath, err)
		}
	}

	return nil
}

func windowsTerminalSettingsPaths() []string {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(homeDir) != "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
	}
	if localAppData == "" {
		return nil
	}

	return []string{
		filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"),
		filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", "settings.json"),
		filepath.Join(localAppData, "Microsoft", "Windows Terminal", "settings.json"),
	}
}

func hideWindowsTerminalWSLProfilesInSettings(data []byte, distroName string, iconPath string) ([]byte, bool, error) {
	if strings.TrimSpace(distroName) == "" {
		return data, false, nil
	}

	spans, err := jsonObjectSpans(data)
	if err != nil {
		return nil, false, err
	}

	type replacement struct {
		start int
		end   int
		value []byte
	}

	var replacements []replacement
	for _, span := range spans {
		objectData := data[span.start:span.end]
		updatedObject, changed := hideWindowsTerminalWSLProfileObject(objectData, distroName, iconPath)
		if !changed {
			continue
		}
		replacements = append(replacements, replacement{
			start: span.start,
			end:   span.end,
			value: updatedObject,
		})
	}
	if len(replacements) == 0 {
		return data, false, nil
	}

	updated := append([]byte(nil), data...)
	for i := len(replacements) - 1; i >= 0; i-- {
		replacement := replacements[i]
		updated = append(updated[:replacement.start], append(replacement.value, updated[replacement.end:]...)...)
	}
	return updated, true, nil
}

func hideWindowsTerminalWSLProfileObject(objectData []byte, distroName string, iconPath string) ([]byte, bool) {
	properties, err := topLevelJSONStringProperties(objectData)
	if err != nil {
		return objectData, false
	}
	if !strings.EqualFold(properties["name"], distroName) {
		return objectData, false
	}
	if properties["source"] != "Microsoft.WSL" && properties["source"] != "Windows.Terminal.Wsl" {
		return objectData, false
	}

	objectText := string(objectData)
	changed := false
	hiddenPattern := regexp.MustCompile(`(?m)("hidden"\s*:\s*)(true|false)`)
	if hiddenPattern.MatchString(objectText) {
		updated := hiddenPattern.ReplaceAllString(objectText, `${1}true`)
		if updated != objectText {
			objectText = updated
			changed = true
		}
	} else {
		updated := insertJSONProperty(objectText, "hidden", "true")
		if updated != objectText {
			objectText = updated
			changed = true
		}
	}

	if strings.TrimSpace(iconPath) != "" {
		updated := upsertJSONStringProperty(objectText, "icon", iconPath)
		if updated != objectText {
			objectText = updated
			changed = true
		}
	}

	return []byte(objectText), changed
}

func upsertJSONStringProperty(objectText string, propertyName string, value string) string {
	propertyPattern := regexp.MustCompile(`(?m)("` + regexp.QuoteMeta(propertyName) + `"\s*:\s*)("(?:\\.|[^"\\])*")`)
	if loc := propertyPattern.FindStringSubmatchIndex(objectText); loc != nil {
		return objectText[:loc[4]] + strconv.Quote(value) + objectText[loc[5]:]
	}
	return insertJSONProperty(objectText, propertyName, strconv.Quote(value))
}

func insertJSONProperty(objectText string, propertyName string, rawValue string) string {
	propertyIndent := "\t"
	if matches := regexp.MustCompile(`(?m)^([ \t]+)"[^"]+"\s*:`).FindStringSubmatch(objectText); len(matches) == 2 {
		propertyIndent = matches[1]
	}

	property := strconv.Quote(propertyName) + ": " + rawValue + ","
	rest := objectText[1:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
		return "{" + "\r\n" + propertyIndent + property + "\r\n" + rest
	}
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
		return "{" + "\n" + propertyIndent + property + "\n" + rest
	}

	return "{" + property + " " + strings.TrimLeft(rest, " \t")
}

func topLevelJSONStringProperties(objectData []byte) (map[string]string, error) {
	properties := make(map[string]string)
	if len(objectData) < 2 || objectData[0] != '{' {
		return properties, fmt.Errorf("object does not start with '{'")
	}

	depth := 0
	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(objectData); i++ {
		ch := objectData[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if inLineComment {
			if ch == '\n' || ch == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(objectData) && objectData[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(objectData) {
			switch objectData[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}

		switch ch {
		case '"':
			if depth == 1 {
				key, next, ok := consumeJSONStringProperty(objectData, i)
				if ok {
					properties[key] = next.value
					i = next.index
					continue
				}
			}
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}

	return properties, nil
}

type consumedJSONStringProperty struct {
	index int
	value string
}

func consumeJSONStringProperty(data []byte, start int) (string, consumedJSONStringProperty, bool) {
	key, keyEnd, ok := consumeJSONStringToken(data, start)
	if !ok {
		return "", consumedJSONStringProperty{}, false
	}
	index := skipJSONWhitespaceAndComments(data, keyEnd)
	if index >= len(data) || data[index] != ':' {
		return "", consumedJSONStringProperty{}, false
	}
	index = skipJSONWhitespaceAndComments(data, index+1)
	if index >= len(data) || data[index] != '"' {
		return "", consumedJSONStringProperty{}, false
	}
	value, valueEnd, ok := consumeJSONStringToken(data, index)
	if !ok {
		return "", consumedJSONStringProperty{}, false
	}
	return key, consumedJSONStringProperty{index: valueEnd - 1, value: value}, true
}

func consumeJSONStringToken(data []byte, start int) (string, int, bool) {
	if start >= len(data) || data[start] != '"' {
		return "", 0, false
	}

	var builder strings.Builder
	escaped := false
	for i := start + 1; i < len(data); i++ {
		ch := data[i]
		if escaped {
			builder.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return builder.String(), i + 1, true
		}
		builder.WriteByte(ch)
	}

	return "", 0, false
}

func skipJSONWhitespaceAndComments(data []byte, start int) int {
	inLineComment := false
	inBlockComment := false

	for i := start; i < len(data); i++ {
		if inLineComment {
			if data[i] == '\n' || data[i] == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if data[i] == '/' && i+1 < len(data) {
			switch data[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return i
		}
	}

	return len(data)
}

type objectSpan struct {
	start int
	end   int
}

func jsonObjectSpans(data []byte) ([]objectSpan, error) {
	var spans []objectSpan
	var stack []int

	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(data); i++ {
		ch := data[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if inLineComment {
			if ch == '\n' || ch == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(data) && data[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(data) {
			switch data[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			stack = append(stack, i)
			continue
		}
		if ch != '}' {
			continue
		}
		if len(stack) == 0 {
			return nil, fmt.Errorf("unbalanced Windows Terminal settings object braces")
		}
		start := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		spans = append(spans, objectSpan{start: start, end: i + 1})
	}

	if inString || inBlockComment || len(stack) != 0 {
		return nil, fmt.Errorf("unterminated Windows Terminal settings content")
	}

	return spans, nil
}

// EnsureRunning ensures the managed distro exists, starts it if needed, and waits
// for basic in-guest readiness before returning runtime connection details.
func (m *Manager) EnsureRunning(ctx context.Context) (*RuntimeInfo, error) {
	return m.ensureRunningWithProgress(ctx, progressReporter{})
}

func (m *Manager) ensureRunningWithProgress(ctx context.Context, progress progressReporter) (*RuntimeInfo, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	progress.Update(5, "Ensuring managed WSL distro is installed")
	if err := m.ensureInstalledLocked(ctx, progressReporter{}); err != nil {
		return nil, err
	}

	progress.Update(15, "Ensuring persistent WSL /var disk exists and is attached")
	if err := m.ensureVarDiskAttached(ctx); err != nil {
		return nil, err
	}

	distro, err := m.ensureMainDistroReady(ctx, progress)
	if err != nil {
		return nil, err
	}

	progress.Update(85, "Cleaning up legacy WSL /var storage state")
	if err := m.cleanupLegacyDataDistro(ctx); err != nil {
		return nil, err
	}

	progress.Update(90, "Resolving WSL Docker bridge configuration")
	bridgeInfo, err := m.resolveBridgeInfo()
	if err != nil {
		return nil, err
	}
	progress.Update(95, "Ensuring WSL Docker bridge is ready")
	bridgeInfo, bridgeReady, err := m.ensureBridgeReady(ctx, bridgeInfo)
	if err != nil {
		if !shouldRecoverBrokenDistro(err) {
			return nil, err
		}
		if err := m.recoverBrokenMainDistro(ctx, progress, err); err != nil {
			return nil, err
		}
		progress.Update(40, "Rechecking managed WSL distro state")
		distro, err = m.ensureMainDistroReady(ctx, progress)
		if err != nil {
			return nil, err
		}
		progress.Update(90, "Resolving WSL Docker bridge configuration")
		bridgeInfo, err = m.resolveBridgeInfo()
		if err != nil {
			return nil, err
		}
		progress.Update(95, "Ensuring WSL Docker bridge is ready")
		bridgeInfo, bridgeReady, err = m.ensureBridgeReady(ctx, bridgeInfo)
		if err != nil {
			return nil, err
		}
	}
	progress.Update(100, "Managed WSL distro and Docker bridge are ready")

	return &RuntimeInfo{
		DistroName:        m.cfg.WSLDistroName,
		DistroInstalled:   true,
		DistroState:       distro.State,
		DistroVersion:     distro.Version,
		BridgeType:        bridgeInfo.Type,
		BridgePort:        bridgeInfo.Port,
		BridgePipeName:    bridgeInfo.PipeName,
		BridgeDockerHost:  bridgeInfo.DockerHost,
		InstallDir:        m.cfg.WSLInstallDir,
		StateDir:          m.cfg.WSLStateDir,
		RootfsArchivePath: strings.TrimSpace(m.cfg.WSLRootfsPath),
		ImageRef:          m.cfg.WSLImageRef,
		BridgeReady:       bridgeReady,
	}, nil
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
	if !strings.EqualFold(distro.State, "Stopped") && !strings.EqualFold(distro.State, "Running") {
		progress.Update(50, "Waiting for managed WSL distro import to settle")
		var err error
		distro, err = m.waitForNamedDistroRunnableState(ctx, m.mainDistro().name)
		if err != nil {
			return err
		}
	}
	if strings.EqualFold(distro.State, "Stopped") {
		progress.Update(50, "Starting managed WSL distro")
		if err := m.startDistro(ctx); err != nil {
			return err
		}
	}
	if strings.EqualFold(distro.State, "Stopped") || strings.EqualFold(distro.State, "Running") {
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
	if strings.EqualFold(distro.State, "Stopped") {
		m.stopNamedPipeBridgeLocked()
		return nil
	}

	if _, err := m.runCommand(ctx, "wsl.exe", "--terminate", m.cfg.WSLDistroName); err != nil {
		return fmt.Errorf("terminate managed WSL distro %q: %w", m.cfg.WSLDistroName, err)
	}
	m.stopNamedPipeBridgeLocked()
	return nil
}

// UpgradeIfNeeded reconciles the managed WSL runtime image with the configured rootfs source.
func (m *Manager) UpgradeIfNeeded(ctx context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	return m.upgradeIfNeededLocked(ctx)
}

func (m *Manager) upgradeIfNeededLocked(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.state.Load()
	if err != nil {
		return err
	}

	_, found, err := m.probeDistro(ctx)
	if err != nil {
		return err
	}
	if !found {
		return m.importDistro(ctx, progressReporter{})
	}

	if strings.EqualFold(state.DistroName, m.cfg.WSLDistroName) &&
		strings.TrimSpace(state.ImageRef) != "" &&
		strings.TrimSpace(state.ImageRef) != m.configuredRootfsSourceRef() {
		if err := m.ensureVarDiskAttached(ctx); err != nil {
			return err
		}
		if err := m.stopMainDistro(ctx); err != nil {
			return err
		}
		m.stopNamedPipeBridgeLocked()
		if err := m.unregisterMainDistro(ctx); err != nil {
			return err
		}
		if err := m.removeMainInstallDir(); err != nil {
			return err
		}
		if err := m.state.Clear(); err != nil {
			return err
		}
		return m.importDistro(ctx, progressReporter{})
	}

	if state == (RuntimeState{}) || !strings.EqualFold(state.DistroName, m.cfg.WSLDistroName) || strings.TrimSpace(state.ImageRef) == "" {
		bridgeInfo, err := m.resolveBridgeInfo()
		if err != nil {
			return err
		}
		return m.state.Save(RuntimeState{
			DistroName: m.cfg.WSLDistroName,
			BridgeType: bridgeInfo.Type,
			BridgePort: bridgeInfo.Port,
			ImageRef:   m.configuredRootfsSourceRef(),
		})
	}

	return nil
}

// Uninstall removes the managed distro registration, install directory, and runtime state.
func (m *Manager) Uninstall(ctx context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, found, err := m.probeDistro(ctx); err != nil {
		return err
	} else if found {
		m.stopNamedPipeBridgeLocked()
		if err := m.unregisterMainDistro(ctx); err != nil {
			return err
		}
	}
	if _, found, err := m.probeLegacyDataDistro(ctx); err != nil {
		return err
	} else if found {
		if err := m.unregisterLegacyDataDistro(ctx); err != nil {
			return err
		}
	}
	if err := m.unmountVarDisk(ctx); err != nil {
		return err
	}

	if err := m.removeMainInstallDir(); err != nil {
		return err
	}
	if err := m.removeLegacyDataInstallDir(); err != nil {
		return err
	}
	if err := m.removeVarDiskFile(); err != nil {
		return err
	}
	if err := m.state.Clear(); err != nil {
		return err
	}
	return nil
}

// Status returns the current provider status for UI and diagnostics.
func (m *Manager) Status() sandbox.ProviderStatus {
	details := StatusDetails{
		DistroName:        m.cfg.WSLDistroName,
		InstallDir:        m.cfg.WSLInstallDir,
		StateDir:          m.cfg.WSLStateDir,
		StatePath:         m.state.Path(),
		VarDiskPath:       m.varDiskPath(),
		VarDiskLabel:      m.varDiskLabel(),
		RootfsArchivePath: strings.TrimSpace(m.cfg.WSLRootfsPath),
		ImageRef:          m.cfg.WSLImageRef,
		BridgeType:        m.cfg.WSLBridgeType,
		BridgePort:        m.cfg.WSLBridgePort,
	}

	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return sandbox.ProviderStatus{Available: false, State: "not_available", Message: "wsl.exe is not available on PATH", Details: details}
	}
	if strings.TrimSpace(m.cfg.WSLDistroName) == "" {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: "WSL_DISTRO_NAME is empty", Details: details}
	}

	bridgeInfo, err := m.resolveBridgeInfo()
	if err != nil {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: err.Error(), Details: details}
	}
	details.BridgeType = bridgeInfo.Type
	details.BridgePort = bridgeInfo.Port
	details.BridgePipeName = bridgeInfo.PipeName
	details.BridgeDockerHost = bridgeInfo.DockerHost

	ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
	defer cancel()

	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return sandbox.ProviderStatus{Available: false, State: "failed", Message: err.Error(), Details: details}
	}
	if !found {
		return sandbox.ProviderStatus{Available: true, State: "not_installed", Message: fmt.Sprintf("managed WSL distro %q is not installed yet", m.cfg.WSLDistroName), Details: details}
	}

	details.DistroInstalled = true
	details.DistroState = distro.State
	details.DistroVersion = distro.Version

	bridgeReady, err := m.probeBridgeReady(ctx, bridgeInfo)
	if err == nil && bridgeReady {
		return sandbox.ProviderStatus{
			Available: true,
			State:     "ready",
			Message:   "managed WSL distro and Docker bridge are ready",
			Details:   details,
		}
	}

	message := "managed WSL distro is running; Docker bridge will be started on demand"
	state := "starting"
	if strings.EqualFold(distro.State, "Stopped") {
		message = "managed WSL distro is installed but currently stopped; it will be started on demand"
		state = "stopped"
	} else if strings.EqualFold(distro.State, "Installing") {
		message = "managed WSL distro import is still being finalized"
		state = "starting"
	} else if !strings.EqualFold(distro.State, "Running") {
		message = fmt.Sprintf("managed WSL distro is currently %s", distro.State)
		state = "starting"
	} else if strings.EqualFold(bridgeInfo.Type, BridgeTypeNamedPipe) {
		message = "managed WSL distro is running; named-pipe Docker bridge will be started on demand"
	} else if strings.EqualFold(bridgeInfo.Type, BridgeTypeTCP) {
		message = "managed WSL distro is running; TCP Docker bridge will be started on demand"
	}

	return sandbox.ProviderStatus{Available: true, State: state, Message: message, Details: details}
}

func (m *Manager) resolveBridgeInfo() (BridgeInfo, error) {
	port := m.cfg.WSLBridgePort
	if port == 0 && strings.EqualFold(m.cfg.WSLBridgeType, BridgeTypeTCP) {
		state, err := m.state.Load()
		if err != nil {
			return BridgeInfo{}, err
		}
		if strings.EqualFold(state.BridgeType, BridgeTypeTCP) && state.BridgePort > 0 {
			port = state.BridgePort
		}
	}
	return ResolveBridgeInfo(m.cfg.WSLBridgeType, m.cfg.WSLDistroName, port)
}

func (m *Manager) ensureBridgeReady(ctx context.Context, bridgeInfo BridgeInfo) (BridgeInfo, bool, error) {
	switch bridgeInfo.Type {
	case BridgeTypeNamedPipe:
		ready, err := m.probeBridgeReady(ctx, bridgeInfo)
		if err == nil && ready {
			return bridgeInfo, true, nil
		}
		if err := m.startNamedPipeBridge(ctx, bridgeInfo.PipeName); err != nil {
			return BridgeInfo{}, false, err
		}
		if err := m.waitForNamedPipeBridgeReady(ctx, bridgeInfo); err != nil {
			return BridgeInfo{}, false, err
		}
		if err := m.saveBridgeRuntimeState(bridgeInfo); err != nil {
			return BridgeInfo{}, false, err
		}
		return bridgeInfo, true, nil
	case BridgeTypeTCP:
		if bridgeInfo.Port == 0 {
			port, err := allocateLoopbackPort()
			if err != nil {
				return BridgeInfo{}, false, err
			}
			bridgeInfo.Port = port
			bridgeInfo.DockerHost = fmt.Sprintf("tcp://127.0.0.1:%d", port)
		}

		ready, err := m.probeBridgeReady(ctx, bridgeInfo)
		if err == nil && ready {
			return bridgeInfo, true, nil
		}

		if err := m.startTCPBridge(ctx, bridgeInfo.Port); err != nil {
			return BridgeInfo{}, false, err
		}
		if err := m.waitForTCPBridgeReady(ctx, bridgeInfo); err != nil {
			return BridgeInfo{}, false, err
		}
		if err := m.saveBridgeRuntimeState(bridgeInfo); err != nil {
			return BridgeInfo{}, false, err
		}
		return bridgeInfo, true, nil
	default:
		return BridgeInfo{}, false, fmt.Errorf("unsupported WSL bridge type %q", bridgeInfo.Type)
	}
}

func (m *Manager) probeDistro(ctx context.Context) (DistroInfo, bool, error) {
	return m.probeNamedDistro(ctx, m.mainDistro().name)
}

func (m *Manager) probeLegacyDataDistro(ctx context.Context) (DistroInfo, bool, error) {
	return m.probeNamedDistro(ctx, m.legacyDataDistro().name)
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

func (m *Manager) importDistro(ctx context.Context, progress progressReporter) error {
	return m.importNamedDistro(ctx, m.mainDistro(), "managed WSL distro", progress, true)
}

func (m *Manager) importNamedDistro(ctx context.Context, distro managedDistro, description string, progress progressReporter, saveRuntimeState bool) error {
	progress.Update(30, "Preparing WSL rootfs artifact")
	artifact, err := m.downloader.EnsureRootfsWithProgress(ctx, func(update ImageDownloadProgress) {
		if strings.TrimSpace(update.CurrentOperation) == "" {
			return
		}
		progress.Update(30, update.CurrentOperation)
	})
	if err != nil {
		return fmt.Errorf("prepare WSL rootfs artifact: %w", err)
	}

	progress.Update(45, "Preparing "+description+" install directory")
	if err := prepareInstallDirForImport(distro.installDir, distro.name); err != nil {
		return err
	}

	progress.Update(60, "Customizing WSL rootfs archive")
	rootfsTarPath, cleanup, err := m.prepareImportRootfsTar(artifact.RootfsArchive)
	if err != nil {
		return err
	}
	defer cleanup()

	progress.Update(75, "Importing "+description)
	if _, err := m.runCommand(ctx, "wsl.exe", "--import", distro.name, distro.installDir, rootfsTarPath, "--version", "2"); err != nil {
		return fmt.Errorf("import %s %q: %w", description, distro.name, err)
	}
	if !saveRuntimeState {
		progress.Update(100, "Managed WSL distro import completed")
		return nil
	}

	progress.Update(90, "Saving managed WSL runtime metadata")
	bridgeInfo, err := m.resolveBridgeInfo()
	if err != nil {
		return err
	}
	if err := m.state.Save(RuntimeState{
		DistroName: m.cfg.WSLDistroName,
		BridgeType: bridgeInfo.Type,
		BridgePort: bridgeInfo.Port,
		ImageRef:   m.configuredRootfsSourceRef(),
	}); err != nil {
		return err
	}
	progress.Update(100, "Managed WSL distro import completed")
	return nil
}

func (m *Manager) prepareInstallDirForImport() error {
	mainDistro := m.mainDistro()
	return prepareInstallDirForImport(mainDistro.installDir, mainDistro.name)
}

func prepareInstallDirForImport(installDir string, distroName string) error {
	if err := os.MkdirAll(filepath.Dir(installDir), 0755); err != nil {
		return fmt.Errorf("create WSL install parent dir: %w", err)
	}

	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return fmt.Errorf("WSL install dir is empty")
	}

	if _, err := os.Stat(installDir); err == nil {
		backupDir := fmt.Sprintf("%s.stale-%d", installDir, time.Now().UTC().UnixNano())
		if err := moveInstallDirAsideWithRetry(installDir, backupDir); err != nil {
			return err
		}
		log.Printf(
			"Moved stale WSL install dir %q to %q before re-importing unregistered distro %q",
			installDir,
			backupDir,
			distroName,
		)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat WSL install dir %q: %w", installDir, err)
	}

	return nil
}

func moveInstallDirAsideWithRetry(installDir string, backupDir string) error {
	for attempt := 1; ; attempt++ {
		if err := renamePath(installDir, backupDir); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			if !isRetryableInstallDirMoveError(err) || attempt >= defaultMoveMaxRetries {
				if isRetryableInstallDirMoveError(err) {
					return fmt.Errorf(
						"move stale WSL install dir %q to %q after %d attempts: %w; WSL may still be releasing files in %q. Try running `wsl.exe --shutdown` and then restart Discobot",
						installDir,
						backupDir,
						attempt,
						err,
						installDir,
					)
				}
				return fmt.Errorf("move stale WSL install dir %q to %q: %w", installDir, backupDir, err)
			}

			log.Printf(
				"Retrying move of stale WSL install dir %q to %q after Windows filesystem error (%d/%d): %v",
				installDir,
				backupDir,
				attempt,
				defaultMoveMaxRetries,
				err,
			)
			sleep(defaultMoveRetryDelay)
			continue
		}

		return nil
	}
}

func isRetryableInstallDirMoveError(err error) bool {
	if os.IsPermission(err) {
		return true
	}

	var errno windows.Errno
	if errors.As(err, &errno) {
		return errno == windows.ERROR_ACCESS_DENIED || errno == windows.ERROR_SHARING_VIOLATION
	}

	return false
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
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = tw.Close()
			_ = dst.Close()
			cleanup()
			return "", nil, fmt.Errorf("read temp rootfs tar %q: %w", sourceTarPath, err)
		}
		if normalizeTarPath(hdr.Name) == discobotWSLEnvPath {
			continue
		}

		headerCopy := *hdr
		if err := tw.WriteHeader(&headerCopy); err != nil {
			_ = tw.Close()
			_ = dst.Close()
			cleanup()
			return "", nil, fmt.Errorf("write customized rootfs header %q: %w", headerCopy.Name, err)
		}
		if hdr.Size == 0 {
			continue
		}
		if _, err := io.CopyN(tw, tr, hdr.Size); err != nil {
			_ = tw.Close()
			_ = dst.Close()
			cleanup()
			return "", nil, fmt.Errorf("copy customized rootfs entry %q: %w", headerCopy.Name, err)
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
		_ = tw.Close()
		_ = dst.Close()
		cleanup()
		return "", nil, fmt.Errorf("write customized rootfs header %q: %w", discobotWSLEnvPath, err)
	}
	if _, err := tw.Write(envBytes); err != nil {
		_ = tw.Close()
		_ = dst.Close()
		cleanup()
		return "", nil, fmt.Errorf("write customized rootfs contents %q: %w", discobotWSLEnvPath, err)
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
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		var errno windows.Errno
		if errors.As(pathErr.Err, &errno) {
			return errno == windows.ERROR_ACCESS_DENIED || errno == windows.ERROR_SHARING_VIOLATION
		}
	}

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
		if strings.EqualFold(distro.State, "Stopped") || strings.EqualFold(distro.State, "Running") {
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
		state := strings.TrimSpace(decodeCommandOutput(output))
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
	return m.waitForCommandSuccessUntilCanceled(ctx, "wait for docker.service readiness", func(ctx context.Context) error {
		output, err := m.runInDistro(ctx, "systemctl", "is-active", "docker.service")
		if err != nil {
			if stopErr := m.checkNamedDistroStillRegistered(ctx, m.mainDistro().name, "waiting for docker.service readiness"); stopErr != nil {
				return stopErr
			}
			return err
		}
		if strings.TrimSpace(output) != "active" {
			if stopErr := m.checkNamedDistroStillRegistered(ctx, m.mainDistro().name, "waiting for docker.service readiness"); stopErr != nil {
				return stopErr
			}
			return fmt.Errorf("docker.service state is %q", strings.TrimSpace(output))
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

func (m *Manager) waitForTCPBridgeReady(ctx context.Context, bridgeInfo BridgeInfo) error {
	return m.waitForCommandSuccessUntilCanceled(ctx, "wait for WSL Docker bridge readiness", func(ctx context.Context) error {
		ready, err := m.probeBridgeReady(ctx, bridgeInfo)
		if err != nil {
			if stopErr := m.checkNamedDistroUnexpectedState(ctx, m.mainDistro().name, "waiting for WSL Docker bridge readiness"); stopErr != nil {
				return stopErr
			}
			return err
		}
		if !ready {
			if stopErr := m.checkNamedDistroUnexpectedState(ctx, m.mainDistro().name, "waiting for WSL Docker bridge readiness"); stopErr != nil {
				return stopErr
			}
			return fmt.Errorf("docker bridge is not responding on %s", bridgeInfo.DockerHost)
		}
		return nil
	})
}

func (m *Manager) waitForNamedPipeBridgeReady(ctx context.Context, bridgeInfo BridgeInfo) error {
	return m.waitForCommandSuccessUntilCanceled(ctx, "wait for WSL named-pipe Docker bridge readiness", func(ctx context.Context) error {
		ready, err := m.probeBridgeReady(ctx, bridgeInfo)
		if err != nil {
			if stopErr := m.checkNamedDistroUnexpectedState(ctx, m.mainDistro().name, "waiting for WSL named-pipe Docker bridge readiness"); stopErr != nil {
				return stopErr
			}
			return err
		}
		if !ready {
			if stopErr := m.checkNamedDistroUnexpectedState(ctx, m.mainDistro().name, "waiting for WSL named-pipe Docker bridge readiness"); stopErr != nil {
				return stopErr
			}
			return fmt.Errorf("docker bridge is not responding on %s", bridgeInfo.DockerHost)
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
		if shouldStopRetryingWaitError(lastErr) {
			return lastErr
		}

		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("%s: %w (last error: %v)", description, deadlineCtx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func (m *Manager) runInDistro(ctx context.Context, args ...string) (string, error) {
	return m.runInNamedDistro(ctx, m.mainDistro().name, args...)
}

func (m *Manager) runInSystem(ctx context.Context, args ...string) (string, error) {
	base := []string{"--system", "-u", "root", "--"}
	base = append(base, args...)
	return m.runCommand(ctx, "wsl.exe", base...)
}

func (m *Manager) runInNamedDistro(ctx context.Context, distroName string, args ...string) (string, error) {
	base := []string{"-d", distroName, "--"}
	base = append(base, args...)
	return m.runCommand(ctx, "wsl.exe", base...)
}

func (m *Manager) startTCPBridge(ctx context.Context, port int) error {
	if port <= 0 {
		return fmt.Errorf("tcp bridge port must be greater than zero")
	}

	bridgeCommand := fmt.Sprintf(
		"exec socat TCP-LISTEN:%d,bind=0.0.0.0,reuseaddr,fork UNIX-CONNECT:%s >>%s 2>&1",
		port,
		dockerSockPath,
		bridgeLogPath,
	)
	command := fmt.Sprintf(
		"command -v socat >/dev/null 2>&1 || { echo 'socat is required for WSL TCP bridge startup' >&2; exit 127; }; "+
			"( ss -ltnH '( sport = :%d )' 2>/dev/null || netstat -ltn 2>/dev/null ) | grep -q ':%d ' || "+
			"systemctl stop %s.service >/dev/null 2>&1 || true; "+
			"systemctl reset-failed %s.service >/dev/null 2>&1 || true; "+
			"systemd-run --unit=%s --property=Restart=always --property=RestartSec=1 --service-type=simple /bin/sh -lc %s",
		port,
		port,
		bridgeUnitName,
		bridgeUnitName,
		bridgeUnitName,
		quoteShellEnvValue(bridgeCommand),
	)
	if _, err := m.runInDistro(ctx, "sh", "-lc", command); err != nil {
		return fmt.Errorf("start WSL TCP Docker bridge on port %d: %w", port, err)
	}
	return nil
}

func (m *Manager) startNamedPipeBridge(_ context.Context, pipeName string) error {
	if strings.TrimSpace(pipeName) == "" {
		return fmt.Errorf("named-pipe bridge pipe name is empty")
	}

	pipePath := bridgePipePath(pipeName)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pipeListener != nil && m.pipeListenerName == pipeName {
		return nil
	}
	if m.pipeListener != nil {
		m.stopNamedPipeBridgeLocked()
	}

	listener, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		return fmt.Errorf("listen on WSL named pipe %q: %w", pipePath, err)
	}

	closeCh := make(chan struct{})
	m.pipeListener = listener
	m.pipeListenerName = pipeName
	m.pipeListenerClose = closeCh

	go m.serveNamedPipeBridge(listener, closeCh)
	return nil
}

func (m *Manager) probeBridgeReady(ctx context.Context, bridgeInfo BridgeInfo) (bool, error) {
	switch bridgeInfo.Type {
	case BridgeTypeNamedPipe:
		if bridgeInfo.PipeName == "" {
			return false, nil
		}
		return m.probeNamedPipeBridgeReady(ctx, bridgeInfo.PipeName)
	case BridgeTypeTCP:
		if bridgeInfo.DockerHost == "" || bridgeInfo.Port <= 0 {
			return false, nil
		}

		pingURL := bridgeTCPPingURL(bridgeInfo.Port)
		if pingURL == "" {
			return false, nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			return false, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK, nil
	default:
		return false, fmt.Errorf("unsupported WSL bridge type %q", bridgeInfo.Type)
	}
}

func (m *Manager) saveBridgeRuntimeState(bridgeInfo BridgeInfo) error {
	return m.state.Save(RuntimeState{
		DistroName: m.cfg.WSLDistroName,
		BridgeType: bridgeInfo.Type,
		BridgePort: bridgeInfo.Port,
		ImageRef:   m.configuredRootfsSourceRef(),
	})
}

func (m *Manager) clearBridgeRuntimeState() error {
	return m.state.Clear()
}

func (m *Manager) configuredRootfsSourceRef() string {
	if rootfsPath := strings.TrimSpace(m.cfg.WSLRootfsPath); rootfsPath != "" {
		return rootfsPath
	}
	return strings.TrimSpace(m.cfg.WSLImageRef)
}

func (m *Manager) ensureVarDiskAttached(ctx context.Context) error {
	err := m.ensureVarDiskAttachedOnce(ctx)
	if err == nil {
		return nil
	}
	if !shouldRecoverBrokenDistro(err) {
		return err
	}
	if err := m.recoverBrokenMainDistro(ctx, progressReporter{}, err); err != nil {
		return err
	}
	return m.ensureVarDiskAttachedOnce(ctx)
}

func (m *Manager) ensureVarDiskAttachedOnce(ctx context.Context) error {
	if err := m.ensureVarDiskFile(ctx); err != nil {
		return err
	}

	if _, found, err := m.findVarDiskDeviceByLabel(ctx); err != nil {
		return err
	} else if found {
		return nil
	}

	before, err := m.listDiskDevices(ctx)
	if err != nil {
		return err
	}

	alreadyAttached, err := m.attachVarDiskBareWithStatus(ctx)
	if err != nil {
		return err
	}
	if alreadyAttached {
		recovered, err := m.recoverVarDiskAttachment(ctx)
		if err != nil {
			return err
		}
		if !recovered {
			return fmt.Errorf("attach WSL /var disk %q: label %q is still unavailable after attach", m.varDiskPath(), m.varDiskLabel())
		}
		before, err = m.listDiskDevices(ctx)
		if err != nil {
			return err
		}
		if _, err := m.attachVarDiskBareWithStatus(ctx); err != nil {
			return err
		}
	}

	_, found, err := m.findVarDiskDeviceByLabel(ctx)
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	devicePath, err := m.waitForNewDiskDevice(ctx, before)
	if err != nil {
		return fmt.Errorf("wait for attached WSL /var disk %q device: %w", m.varDiskPath(), err)
	}

	label, err := m.readDiskLabel(ctx, devicePath)
	if err != nil {
		return err
	}
	switch label {
	case m.varDiskLabel():
		return nil
	case "":
		return m.initializeAttachedVarDisk(ctx, devicePath)
	default:
		return fmt.Errorf("attached WSL /var disk %q appeared as %q with unexpected label %q", m.varDiskPath(), devicePath, label)
	}
}

func (m *Manager) ensureVarDiskFile(ctx context.Context) error {
	varDiskPath := m.varDiskPath()
	if err := os.MkdirAll(filepath.Dir(varDiskPath), 0755); err != nil {
		return fmt.Errorf("create WSL /var disk directory %q: %w", filepath.Dir(varDiskPath), err)
	}
	if _, err := os.Stat(varDiskPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat WSL /var disk %q: %w", varDiskPath, err)
	}
	if err := createDynamicVHD(varDiskPath, uint64(m.varDiskSizeGB())*1024*1024*1024); err != nil {
		if shouldElevateVarDiskOperation(err) {
			if _, helperErr := m.runWSLElevationHelper(ctx, "create-vhd", "--path", varDiskPath, "--size-gb", strconv.Itoa(m.varDiskSizeGB())); helperErr != nil {
				return fmt.Errorf("create WSL /var disk %q with elevated helper: %w", varDiskPath, helperErr)
			}
			return nil
		}
		return fmt.Errorf("create WSL /var disk %q: %w", varDiskPath, err)
	}
	return nil
}

func (m *Manager) attachVarDiskBare(ctx context.Context) error {
	_, err := m.attachVarDiskBareWithStatus(ctx)
	return err
}

func (m *Manager) attachVarDiskBareWithStatus(ctx context.Context) (bool, error) {
	if _, err := m.runCommand(ctx, "wsl.exe", "--mount", "--vhd", m.varDiskPath(), "--bare"); err != nil {
		if isAlreadyMountedVarDiskError(err) {
			return true, nil
		}
		if shouldElevateVarDiskOperation(err) {
			if _, helperErr := m.runWSLElevationHelper(ctx, "mount-vhd-bare", "--path", m.varDiskPath()); helperErr != nil {
				if isAlreadyMountedVarDiskError(helperErr) {
					return true, nil
				}
				return false, fmt.Errorf("attach WSL /var disk %q in bare mode with elevated helper: %w", m.varDiskPath(), helperErr)
			}
			return false, nil
		}
		return false, fmt.Errorf("attach WSL /var disk %q in bare mode: %w", m.varDiskPath(), err)
	}
	return false, nil
}

func (m *Manager) unmountVarDisk(ctx context.Context) error {
	if _, err := m.runCommand(ctx, "wsl.exe", "--unmount", m.varDiskPath()); err != nil {
		if isAlreadyUnmountedVarDiskError(err) {
			return nil
		}
		if shouldElevateVarDiskOperation(err) {
			if _, helperErr := m.runWSLElevationHelper(ctx, "unmount-vhd", "--path", m.varDiskPath()); helperErr != nil {
				return fmt.Errorf("unmount WSL /var disk %q with elevated helper: %w", m.varDiskPath(), helperErr)
			}
			return nil
		}
		return fmt.Errorf("unmount WSL /var disk %q: %w", m.varDiskPath(), err)
	}
	return nil
}

func (m *Manager) initializeAttachedVarDisk(ctx context.Context, devicePath string) error {
	if _, err := m.runInSystem(ctx, "mkfs.ext4", "-F", "-L", m.varDiskLabel(), devicePath); err != nil {
		return fmt.Errorf("format WSL /var disk %q as ext4 on %q: %w", m.varDiskPath(), devicePath, err)
	}
	label, err := m.readDiskLabel(ctx, devicePath)
	if err != nil {
		return err
	}
	if label != m.varDiskLabel() {
		return fmt.Errorf("formatted WSL /var disk %q on %q but label is %q", m.varDiskPath(), devicePath, label)
	}
	return nil
}

func (m *Manager) listDiskDevices(ctx context.Context) ([]string, error) {
	output, err := m.runInSystem(ctx, "lsblk", "-dn", "-o", "NAME,TYPE")
	if err != nil {
		return nil, fmt.Errorf("list WSL block devices: %w", err)
	}
	var devices []string
	for rawLine := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(rawLine)
		if len(fields) != 2 || fields[1] != "disk" {
			continue
		}
		devices = append(devices, "/dev/"+fields[0])
	}
	return devices, nil
}

func (m *Manager) findVarDiskDeviceByLabel(ctx context.Context) (string, bool, error) {
	output, err := m.runInSystem(ctx, "blkid", "-o", "device", "-t", "LABEL="+m.varDiskLabel())
	if err != nil {
		if isNoBlkidMatchError(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("find WSL /var disk by label %q: %w", m.varDiskLabel(), err)
	}
	for rawLine := range strings.SplitSeq(output, "\n") {
		devicePath := strings.TrimSpace(rawLine)
		if devicePath != "" {
			return devicePath, true, nil
		}
	}
	return "", false, nil
}

func (m *Manager) readDiskLabel(ctx context.Context, devicePath string) (string, error) {
	output, err := m.runInSystem(ctx, "blkid", "-o", "value", "-s", "LABEL", devicePath)
	if err != nil {
		if isNoBlkidMatchError(err) {
			return "", nil
		}
		return "", fmt.Errorf("read WSL disk label for %q: %w", devicePath, err)
	}
	return strings.TrimSpace(output), nil
}

func (m *Manager) waitForNewDiskDevice(ctx context.Context, before []string) (string, error) {
	beforeSet := make(map[string]struct{}, len(before))
	for _, device := range before {
		beforeSet[device] = struct{}{}
	}

	var newDevice string
	if err := m.waitForCommandSuccess(ctx, "wait for attached WSL /var disk device", func(ctx context.Context) error {
		current, err := m.listDiskDevices(ctx)
		if err != nil {
			return err
		}
		var newDevices []string
		for _, device := range current {
			if _, ok := beforeSet[device]; !ok {
				newDevices = append(newDevices, device)
			}
		}
		switch len(newDevices) {
		case 0:
			return fmt.Errorf("no new disk device detected")
		case 1:
			newDevice = newDevices[0]
			return nil
		default:
			return fmt.Errorf("multiple new disk devices detected: %v", newDevices)
		}
	}); err != nil {
		return "", err
	}
	return newDevice, nil
}

func (m *Manager) stopMainDistro(ctx context.Context) error {
	distro, found, err := m.probeDistro(ctx)
	if err != nil {
		return err
	}
	if !found || strings.EqualFold(distro.State, "Stopped") {
		return nil
	}
	if _, err := m.runCommand(ctx, "wsl.exe", "--terminate", m.mainDistro().name); err != nil {
		return fmt.Errorf("terminate managed WSL distro %q: %w", m.mainDistro().name, err)
	}
	return nil
}

func shouldRecoverBrokenDistro(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "wsl/service/e_unexpected") ||
		strings.Contains(message, "catastrophic failure") ||
		(strings.Contains(message, "managed wsl distro") && strings.Contains(message, "stopped while")) ||
		(strings.Contains(message, "managed wsl distro") && strings.Contains(message, "disappeared while"))
}

func shouldStopRetryingWaitError(err error) bool {
	return shouldRecoverBrokenDistro(err)
}

func (m *Manager) checkNamedDistroUnexpectedState(ctx context.Context, distroName string, operation string) error {
	distro, found, err := m.probeNamedDistro(ctx, distroName)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("managed WSL distro %q disappeared while %s", distroName, operation)
	}
	if strings.EqualFold(distro.State, "Stopped") {
		return fmt.Errorf("managed WSL distro %q stopped while %s", distroName, operation)
	}
	return nil
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

func isUnformattedVarDiskMountError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "incorrect function") ||
		strings.Contains(message, "the parameter is incorrect") ||
		strings.Contains(message, "failed to mount: invalid argument") ||
		strings.Contains(message, "disk was attached but failed to mount") ||
		strings.Contains(message, "invalid argument") ||
		strings.Contains(message, "no such file or directory")
}

func isAlreadyMountedVarDiskError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already mounted") ||
		strings.Contains(message, "already attached")
}

func isAlreadyUnmountedVarDiskError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not mounted") ||
		strings.Contains(message, "not attached") ||
		strings.Contains(message, "cannot find the path specified")
}

func isStaleVarDiskUnmountError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "failed to detach") ||
		strings.Contains(message, "invalid argument")
}

func isNoBlkidMatchError(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "exit status 2")
}

func isDistroNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "there is no distribution with the supplied name") ||
		strings.Contains(message, "wsl_e_distro_not_found")
}

func shouldElevateVarDiskOperation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "do not have the required permission") ||
		strings.Contains(message, "authorization policy") ||
		strings.Contains(message, "administrator privileges") ||
		strings.Contains(message, "requires elevation") ||
		strings.Contains(message, "requested operation requires elevation") ||
		strings.Contains(message, "access is denied")
}

func (m *Manager) recoverVarDiskAttachment(ctx context.Context) (bool, error) {
	devicePath, found, err := m.findVarDiskDeviceByLabel(ctx)
	if err != nil {
		return false, err
	}

	if found {
		log.Printf(
			"Recovering WSL /var disk attachment for %q: label %q is currently visible on %s",
			m.varDiskPath(),
			m.varDiskLabel(),
			devicePath,
		)
	} else {
		log.Printf(
			"Recovering WSL /var disk attachment for %q: WSL reported the disk as already attached but label %q is unavailable",
			m.varDiskPath(),
			m.varDiskLabel(),
		)
	}

	if err := m.unmountVarDisk(ctx); err == nil {
		return true, nil
	} else if !isStaleVarDiskUnmountError(err) {
		return false, fmt.Errorf("detach WSL /var disk %q during attachment recovery: %w", m.varDiskPath(), err)
	}

	m.mu.Lock()
	m.stopNamedPipeBridgeLocked()
	m.mu.Unlock()

	if _, err := m.runCommand(ctx, "wsl.exe", "--shutdown"); err != nil {
		return false, fmt.Errorf("shutdown WSL to recover /var disk attachment %q: %w", m.varDiskPath(), err)
	}
	return true, nil
}

func (m *Manager) recoverBrokenMainDistro(ctx context.Context, progress progressReporter, cause error) error {
	log.Printf("Recovering broken managed WSL distro %q after startup failure: %v", m.mainDistro().name, cause)
	progress.Update(45, "Recovering broken managed WSL distro")

	if err := m.stopMainDistro(ctx); err != nil {
		log.Printf("Failed to terminate broken managed WSL distro %q before recovery: %v", m.mainDistro().name, err)
	}

	m.mu.Lock()
	m.stopNamedPipeBridgeLocked()
	m.mu.Unlock()

	if err := m.unregisterMainDistro(ctx); err != nil {
		return err
	}
	if err := m.removeMainInstallDir(); err != nil {
		return err
	}
	if err := m.state.Clear(); err != nil {
		return err
	}

	recoveryProgress := progressReporter{
		update: func(childProgress int, currentOperation string) {
			mapped := 50 + (childProgress * 35 / 100)
			progress.Update(mapped, currentOperation)
		},
	}
	return m.importDistro(ctx, recoveryProgress)
}

func (m *Manager) cleanupLegacyDataDistro(ctx context.Context) error {
	if _, found, err := m.probeLegacyDataDistro(ctx); err != nil {
		return err
	} else if found {
		if err := m.unregisterLegacyDataDistro(ctx); err != nil {
			return err
		}
	}
	return m.removeLegacyDataInstallDir()
}

func (m *Manager) unregisterMainDistro(ctx context.Context) error {
	return m.unregisterNamedDistro(ctx, m.mainDistro().name, "managed WSL distro")
}

func (m *Manager) unregisterLegacyDataDistro(ctx context.Context) error {
	return m.unregisterNamedDistro(ctx, m.legacyDataDistro().name, "legacy managed WSL data distro")
}

func (m *Manager) unregisterNamedDistro(ctx context.Context, distroName string, description string) error {
	if _, err := m.runCommand(ctx, "wsl.exe", "--unregister", distroName); err != nil {
		if isDistroNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("unregister %s %q: %w", description, distroName, err)
	}
	return nil
}

func (m *Manager) removeMainInstallDir() error {
	return removeInstallDir(m.mainDistro().installDir)
}

func (m *Manager) removeLegacyDataInstallDir() error {
	return removeInstallDir(m.legacyDataDistro().installDir)
}

func (m *Manager) removeVarDiskFile() error {
	varDiskPath := strings.TrimSpace(m.varDiskPath())
	if varDiskPath == "" {
		return nil
	}
	if err := os.Remove(varDiskPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove WSL /var disk %q: %w", varDiskPath, err)
	}
	return nil
}

func removeInstallDir(installDir string) error {
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return nil
	}
	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("remove WSL install dir %q: %w", installDir, err)
	}
	return nil
}

func (m *Manager) stopNamedPipeBridgeLocked() {
	if m.pipeListener != nil {
		_ = m.pipeListener.Close()
		m.pipeListener = nil
	}
	if m.pipeListenerClose != nil {
		close(m.pipeListenerClose)
		m.pipeListenerClose = nil
	}
	m.pipeListenerName = ""
}

func (m *Manager) serveNamedPipeBridge(listener net.Listener, closeCh <-chan struct{}) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-closeCh:
				return
			default:
			}
			return
		}

		go m.handleNamedPipeBridgeConn(conn)
	}
}

func (m *Manager) handleNamedPipeBridgeConn(conn net.Conn) {
	defer conn.Close()

	cmd := exec.Command("wsl.exe", "-d", m.cfg.WSLDistroName, "--exec", "socat", "STDIO", "UNIX-CONNECT:"+dockerSockPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return
	}

	copyDone := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(stdin, conn)
		_ = stdin.Close()
		copyDone <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(conn, stdout)
		_ = stdout.Close()
		copyDone <- struct{}{}
	}()

	<-copyDone
	_ = conn.Close()
	<-copyDone
	_ = cmd.Wait()
}

func (m *Manager) probeNamedPipeBridgeReady(ctx context.Context, pipeName string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://discobot/_ping", nil)
	if err != nil {
		return false, err
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return winio.DialPipeContext(ctx, bridgePipePath(pipeName))
			},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

func (m *Manager) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	output, err := runCommandOutput(ctx, name, args...)
	trimmed := strings.TrimSpace(decodeCommandOutput(output))
	if err != nil {
		if trimmed == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, trimmed)
	}
	return trimmed, nil
}

func decodeCommandOutput(output []byte) string {
	if len(output) >= 2 {
		switch {
		case output[0] == 0xff && output[1] == 0xfe:
			return decodeUTF16(output[2:], binary.LittleEndian)
		case output[0] == 0xfe && output[1] == 0xff:
			return decodeUTF16(output[2:], binary.BigEndian)
		case looksLikeUTF16(output, binary.LittleEndian):
			return decodeUTF16(output, binary.LittleEndian)
		case looksLikeUTF16(output, binary.BigEndian):
			return decodeUTF16(output, binary.BigEndian)
		}
	}

	return string(output)
}

func looksLikeUTF16(output []byte, order binary.ByteOrder) bool {
	if len(output) < 4 || len(output)%2 != 0 {
		return false
	}

	zeroCount := 0
	pairs := len(output) / 2
	for i := 0; i+1 < len(output); i += 2 {
		var candidate byte
		if order == binary.LittleEndian {
			candidate = output[i+1]
		} else {
			candidate = output[i]
		}
		if candidate == 0 {
			zeroCount++
		}
	}

	return zeroCount >= pairs/2
}

func decodeUTF16(output []byte, order binary.ByteOrder) string {
	if len(output)%2 != 0 {
		output = output[:len(output)-1]
	}
	if len(output) == 0 {
		return ""
	}

	words := make([]uint16, 0, len(output)/2)
	for i := 0; i+1 < len(output); i += 2 {
		words = append(words, order.Uint16(output[i:i+2]))
	}

	return string(utf16.Decode(words))
}

func allocateLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate loopback port: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return 0, fmt.Errorf("allocate loopback port: unexpected listener addr %T", listener.Addr())
	}
	return addr.Port, nil
}
