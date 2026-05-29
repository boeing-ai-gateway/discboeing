package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/processes"
	"github.com/obot-platform/discobot/agent-go/internal/workspaceenv"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/promptqueue"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
	"github.com/obot-platform/discobot/agent-go/tools"
)

const (
	// InlineOutputMaxLines is the max lines to inline in LLM failure messages.
	InlineOutputMaxLines = 200
	// InlineOutputMaxBytes is the max bytes to inline in LLM failure messages.
	InlineOutputMaxBytes = 5 * 1024
	// HookOutputInlineMaxBytes is the max bytes to inline in the hook output viewer.
	HookOutputInlineMaxBytes = 200 * 1024
	// TruncatedOutputTailLines is the number of trailing lines to show for large hook output.
	TruncatedOutputTailLines = 15
	maxHookRetries           = 3
)

// ErrHookPaused is returned when a caller tries to execute a paused hook.
var ErrHookPaused = errors.New("hooks are paused")

// Conversation starts and resumes hook failure follow-up turns.
type Conversation interface {
	Chat(threadID string, req agent.PromptRequest) (string, error)
	Resume(threadID string, req agent.PromptRequest) (string, error)
	HasInterruptedTurn(threadID string) (bool, error)
}

// AIHookAgent runs prompts for AI-powered hooks.
type AIHookAgent interface {
	tools.PromptTaskAgent
}

// AIHookEvaluator runs one-off AI judgments for AI hook responses.
type AIHookEvaluator interface {
	CompleteText(ctx context.Context, model string, messages []message.Message, maxTokens *int) (string, error)
}

// HookFailureMessageMetadata carries structured hook-failure details for UI rendering.
type HookFailureMessageMetadata struct {
	Kind            string   `json:"kind"`
	HookName        string   `json:"hookName"`
	ExitCode        int      `json:"exitCode"`
	Pattern         string   `json:"pattern,omitempty"`
	HookPath        string   `json:"hookPath,omitempty"`
	Files           []string `json:"files,omitempty"`
	ExtraFileCount  int      `json:"extraFileCount,omitempty"`
	Output          string   `json:"output,omitempty"`
	OutputPath      string   `json:"outputPath,omitempty"`
	OutputTail      string   `json:"outputTail,omitempty"`
	OutputTruncated bool     `json:"outputTruncated,omitempty"`
}

// FileHookEvalResult is the result of evaluating file hooks after a completion.
type FileHookEvalResult struct {
	Evaluated      bool
	ShouldReprompt bool
	LLMMessage     string
	FailedResult   *HookResult
	HookFailure    *HookFailureMessageMetadata
}

// HookRunResult is the result of a manual hook run.
type HookRunResult struct {
	Result HookResult
	Eval   FileHookEvalResult
}

type startupHookRun struct {
	hook Hook
	env  map[string]string
}

// Manager orchestrates hook discovery, execution, and status tracking.
type Manager struct {
	workspaceRoot string
	sessionID     string
	hooksDataDir  string
	processes     *processes.Manager

	mu              sync.Mutex
	sessionHooks    []Hook
	fileHooks       []Hook
	preCommitHooks  []Hook
	initialized     bool
	chunkEmitter    func(message.MessageChunk)
	envSnapshot     func() map[string]string
	startupHookEnv  func(Hook) map[string]string
	conversations   Conversation
	promptQueue     *promptqueue.Manager
	aiHookAgent     AIHookAgent
	aiHookEvaluator AIHookEvaluator
	threadPhase     func(threadID string) string

	hookRetryCount     map[string]int
	hookNotificationTo map[string]string
}

// SetThreadPhaseLookup configures how phase-gated hooks read thread phase.
func (m *Manager) SetThreadPhaseLookup(fn func(threadID string) string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.threadPhase = fn
}

func (m *Manager) currentThreadPhase(threadID string) string {
	m.mu.Lock()
	fn := m.threadPhase
	m.mu.Unlock()
	if fn == nil {
		return ""
	}
	return strings.TrimSpace(strings.ToLower(fn(threadID)))
}

// HasPhaseHooks reports whether any hook is gated by a thread phase.
func (m *Manager) HasPhaseHooks() bool {
	m.mu.Lock()
	m.reloadHooks()
	defer m.mu.Unlock()
	for _, hook := range append(append([]Hook{}, m.fileHooks...), m.sessionHooks...) {
		if strings.TrimSpace(hook.Phase) != "" {
			return true
		}
	}
	return false
}

// NewManager creates a new HookManager.
func NewManager(workspaceRoot, sessionID string, processManager ...*processes.Manager) *Manager {
	var procMgr *processes.Manager
	if len(processManager) > 0 && processManager[0] != nil {
		procMgr = processManager[0]
	} else {
		procMgr = processes.NewManager(workspaceRoot)
	}
	return &Manager{
		workspaceRoot:      workspaceRoot,
		sessionID:          sessionID,
		hooksDataDir:       GetHooksDataDir(sessionID),
		processes:          procMgr,
		hookRetryCount:     make(map[string]int),
		hookNotificationTo: make(map[string]string),
	}
}

// SetAIHookAgent configures the agent used to run AI-powered hooks.
func (m *Manager) SetAIHookAgent(aiHookAgent AIHookAgent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aiHookAgent = aiHookAgent
}

// SetAIHookEvaluator configures the direct model caller used to judge AI hook
// responses.
func (m *Manager) SetAIHookEvaluator(evaluator AIHookEvaluator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aiHookEvaluator = evaluator
}

// SetRepromptRunner configures how failed hook notifications start follow-up
// prompts.
func (m *Manager) SetRepromptRunner(conversations Conversation, promptQueue *promptqueue.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conversations = conversations
	m.promptQueue = promptQueue
}

// SetEnvSnapshot sets an optional function that returns request-scoped
// environment variables to inject into executed hooks.
func (m *Manager) SetEnvSnapshot(fn func() map[string]string) {
	m.mu.Lock()
	m.envSnapshot = fn
	m.mu.Unlock()
}

// SetStartupHookEnv sets extra environment that is only injected into session
// hook executions that use visibleStartupHookEnv. Runtime file/pre-commit hooks
// intentionally do not inherit this bootstrap authority.
func (m *Manager) SetStartupHookEnv(fn func(Hook) map[string]string) {
	m.mu.Lock()
	m.startupHookEnv = fn
	m.mu.Unlock()
}

// SetExecutionPaused controls whether runtime hooks execute. When paused,
// hooks do not run or report failures back to the LLM.
func (m *Manager) SetExecutionPaused(paused bool) error {
	status := LoadStatus(m.hooksDataDir)
	changed := status.ExecutionPaused != paused
	status.ExecutionPaused = paused
	if !changed {
		return nil
	}
	if err := SaveStatus(m.hooksDataDir, status); err != nil {
		return err
	}
	m.emitStatusChunk(m.GetStatus())
	if !paused && status.LastThreadID != "" {
		go m.scheduleEvaluation(status.LastThreadID, m.threadModel(status.LastThreadID), true)
	}
	return nil
}

// ExecutionPaused returns whether runtime hook execution is currently paused.
func (m *Manager) ExecutionPaused() bool {
	return LoadStatus(m.hooksDataDir).ExecutionPaused
}

// SetHookExecutionPaused controls whether one hook executes. When paused, that
// hook does not run or report failures back to the LLM.
func (m *Manager) SetHookExecutionPaused(hookID string, paused bool) error {
	hook, ok := m.findHook(hookID)
	if !ok {
		return fmt.Errorf("hook %q not found", hookID)
	}
	if err := SetHookExecutionPaused(m.hooksDataDir, hook, paused); err != nil {
		return err
	}
	if !paused {
		status := LoadStatus(m.hooksDataDir)
		if status.ExecutionPaused {
			status.ExecutionPaused = false
			if err := SaveStatus(m.hooksDataDir, status); err != nil {
				return err
			}
		}
	}
	status := m.GetStatus()
	m.emitStatusChunk(status)
	if !paused && status.LastThreadID != "" {
		go m.scheduleEvaluation(status.LastThreadID, m.threadModel(status.LastThreadID), true)
	}
	return nil
}

func (m *Manager) hookExecutionPaused(hookID string) bool {
	status := LoadStatus(m.hooksDataDir)
	if status.ExecutionPaused {
		return true
	}
	hookStatus, ok := status.Hooks[hookID]
	return ok && hookStatus.ExecutionPaused
}

func (m *Manager) setLastEvaluationThread(threadID string) {
	if strings.TrimSpace(threadID) == "" {
		return
	}
	status := LoadStatus(m.hooksDataDir)
	if status.LastThreadID == threadID {
		return
	}
	status.LastThreadID = threadID
	if err := SaveStatus(m.hooksDataDir, status); err != nil {
		log.Printf("hooks: failed to record last hook thread: %v", err)
	}
}

func (m *Manager) visibleStartupHookEnv(hook Hook) map[string]string {
	env := m.visibleEnvSnapshot()
	m.mu.Lock()
	fn := m.startupHookEnv
	m.mu.Unlock()
	if fn == nil {
		return env
	}
	maps.Copy(env, fn(hook))
	return env
}

func (m *Manager) visibleEnvSnapshot() map[string]string {
	env := workspaceenv.FileSnapshot(m.workspaceRoot)
	if env == nil {
		env = map[string]string{}
	}
	m.mu.Lock()
	fn := m.envSnapshot
	m.mu.Unlock()
	if fn == nil {
		return env
	}
	maps.Copy(env, fn())
	return env
}

// SetChunkEmitter configures how hook status updates are emitted as message chunks.
func (m *Manager) SetChunkEmitter(fn func(message.MessageChunk)) {
	m.mu.Lock()
	m.chunkEmitter = fn
	m.mu.Unlock()
}

func (m *Manager) statusChunk(status StatusFile) (message.MessageChunk, error) {
	data, err := json.Marshal(status)
	if err != nil {
		return nil, err
	}
	return message.DataChunk{
		DataType: "hooks-status",
		Data:     data,
	}, nil
}

func (m *Manager) emitStatusChunk(status StatusFile) {
	m.mu.Lock()
	emitter := m.chunkEmitter
	m.mu.Unlock()
	if emitter == nil {
		return
	}

	chunk, err := m.statusChunk(status)
	if err != nil {
		log.Printf("hooks: failed to marshal status chunk: %v", err)
		return
	}
	emitter(chunk)
}

func (m *Manager) emitCurrentStatusChunk() {
	m.emitStatusChunk(LoadStatus(m.hooksDataDir))
}

// Init discovers hooks and installs pre-commit hooks if needed.
func (m *Manager) Init() error {
	m.mu.Lock()
	if m.initialized {
		m.mu.Unlock()
		return nil
	}

	if err := m.loadHooks(); err != nil {
		m.mu.Unlock()
		return err
	}

	fileHookIDs := make([]string, 0, len(m.fileHooks))
	for _, hook := range m.fileHooks {
		fileHookIDs = append(fileHookIDs, hook.ID)
	}
	if err := RecoverInterruptedHooks(m.hooksDataDir, fileHookIDs); err != nil {
		m.mu.Unlock()
		return err
	}

	m.initialized = true
	m.mu.Unlock()
	return nil
}

// loadHooks discovers and categorizes hooks. Must be called with m.mu held.
func (m *Manager) loadHooks() error {
	hooksDir := filepath.Join(m.workspaceRoot, HooksDir)
	allHooks, err := DiscoverHooks(hooksDir)
	if err != nil {
		return fmt.Errorf("discover hooks: %w", err)
	}

	m.fileHooks = nil
	m.sessionHooks = nil
	m.preCommitHooks = nil

	for _, hook := range allHooks {
		switch hook.Type {
		case HookTypeSession:
			m.sessionHooks = append(m.sessionHooks, hook)
		case HookTypeFile:
			m.fileHooks = append(m.fileHooks, hook)
		case HookTypePreCommit:
			m.preCommitHooks = append(m.preCommitHooks, hook)
		}
	}

	if len(m.preCommitHooks) > 0 {
		if err := InstallPreCommitHook(m.workspaceRoot, m.preCommitHooks, m.sessionID); err != nil {
			log.Printf("Warning: failed to install pre-commit hook: %v", err)
		}
	}

	return nil
}

// RunSessionHooks runs startup hooks discovered from .discobot/hooks and
// returns a wait function for any background hooks. Blocking hooks gate
// configure-time startup; non-blocking hooks continue in the background after
// the runtime becomes ready.
func (m *Manager) RunSessionHooks(progress func(string)) func() {
	if m == nil {
		return func() {}
	}

	m.mu.Lock()
	m.reloadHooks()
	sessionHooks := make([]Hook, len(m.sessionHooks))
	copy(sessionHooks, m.sessionHooks)
	m.mu.Unlock()

	if len(sessionHooks) == 0 {
		return func() {}
	}

	var blockingHooks, backgroundHooks []startupHookRun
	for _, hook := range sessionHooks {
		run := startupHookRun{
			hook: hook,
			env:  m.visibleStartupHookEnv(hook),
		}
		if hook.Blocking {
			blockingHooks = append(blockingHooks, run)
		} else {
			backgroundHooks = append(backgroundHooks, run)
		}
	}

	if len(blockingHooks) > 0 {
		m.runStartupHookGroup(blockingHooks, "blocking", progress)
	}
	if len(backgroundHooks) > 0 {
		var wg sync.WaitGroup
		wg.Go(func() {
			m.runStartupHookGroup(backgroundHooks, "background", nil)
		})
		return wg.Wait
	}
	return func() {}
}

func (m *Manager) runStartupHookGroup(sessionHooks []startupHookRun, group string, progress func(string)) {
	if progress != nil {
		progress(fmt.Sprintf("running %d %s session hook(s)", len(sessionHooks), group))
	}
	for _, run := range sessionHooks {
		if progress != nil {
			progress(fmt.Sprintf("running session hook %q", run.hook.Name))
		}
		m.runStartupHook(run)
	}
}

func (m *Manager) runStartupHook(run startupHookRun) {
	hook := run.hook
	if m.hookExecutionPaused(hook.ID) {
		return
	}

	outputPath := GetHookOutputPath(m.hooksDataDir, hook.ID)
	_ = SetHookRunning(m.hooksDataDir, hook)
	m.emitCurrentStatusChunk()

	runCutoff := time.Now()
	result := m.runHook(hook, runHookOptions{
		env:        run.env,
		outputPath: outputPath,
	})

	_ = UpdateHookStatus(m.hooksDataDir, result, outputPath, runCutoff)
	_ = UpdateLastEvaluatedAt(m.hooksDataDir)
	m.emitCurrentStatusChunk()
}

// reloadHooks re-discovers hooks from disk. Must be called with m.mu held.
func (m *Manager) reloadHooks() {
	if err := m.loadHooks(); err != nil {
		log.Printf("Warning: failed to reload hooks: %v", err)
	}
}

// checkAndReloadHooks checks if hook files have changed and reloads if needed.
// Must be called with m.mu held.
func (m *Manager) checkAndReloadHooks() {
	markerPath := filepath.Join(m.hooksDataDir, ".last-eval")
	markerInfo, err := os.Stat(markerPath)
	if err != nil {
		// No marker yet (first eval) — hooks are already fresh from init()
		return
	}
	markerMtime := markerInfo.ModTime()

	hooksDir := filepath.Join(m.workspaceRoot, HooksDir)

	// Check directory mtime
	dirInfo, err := os.Stat(hooksDir)
	if err != nil {
		return
	}
	if dirInfo.ModTime().After(markerMtime) {
		m.reloadHooks()
		return
	}

	// Check individual file mtimes
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(markerMtime) {
			m.reloadHooks()
			return
		}
	}
}

// HasFileHooks returns whether any file hooks are configured.
func (m *Manager) HasFileHooks() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.fileHooks) > 0
}

// GetStatus returns the current hook status.
func (m *Manager) GetStatus() StatusFile {
	m.mu.Lock()
	m.reloadHooks()
	knownHooks := append([]Hook{}, m.sessionHooks...)
	knownHooks = append(knownHooks, m.fileHooks...)
	knownHooks = append(knownHooks, m.preCommitHooks...)
	m.mu.Unlock()
	status := LoadStatus(m.hooksDataDir)
	for _, hook := range knownHooks {
		if _, ok := status.Hooks[hook.ID]; ok {
			continue
		}
		status.Hooks[hook.ID] = HookRunStatus{
			HookID:     hook.ID,
			HookName:   hook.Name,
			Type:       string(hook.Type),
			LastResult: "pending",
			OutputPath: GetHookOutputPath(m.hooksDataDir, hook.ID),
		}
	}
	return status
}

// HookOutput contains hook log metadata and inline output when available.
type HookOutput struct {
	Output         string
	SizeBytes      int64
	DisplayedBytes int64
	TooLarge       bool
}

// GetHookOutput returns the output log metadata for a hook.
func (m *Manager) GetHookOutput(hookID string) (*HookOutput, error) {
	outputPath := GetHookOutputPath(m.hooksDataDir, hookID)
	info, err := os.Stat(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &HookOutput{}, nil
		}
		return nil, err
	}

	result := &HookOutput{
		SizeBytes: info.Size(),
	}
	data, err := readHookOutputTail(outputPath, result.SizeBytes, HookOutputInlineMaxBytes)
	if err != nil {
		return nil, err
	}
	result.DisplayedBytes = int64(len(data))
	result.TooLarge = result.SizeBytes > HookOutputInlineMaxBytes
	result.Output = string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
	return result, nil
}

// GetHookOutputDownload returns the full output log bytes for a hook.
func (m *Manager) GetHookOutputDownload(hookID string) ([]byte, error) {
	outputPath := GetHookOutputPath(m.hooksDataDir, hookID)
	data, err := os.ReadFile(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte{}, nil
		}
		return nil, err
	}
	return data, nil
}

func readHookOutputTail(outputPath string, fileSize, maxBytes int64) ([]byte, error) {
	file, err := os.Open(outputPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	start := int64(0)
	if fileSize > maxBytes {
		start = fileSize - maxBytes
	}
	if _, err := file.Seek(start, 0); err != nil {
		return nil, err
	}

	return io.ReadAll(file)
}

// RerunHook manually reruns a file hook against current dirty files or a
// session hook against the current workspace.
func (m *Manager) RerunHook(hookID string) (*HookRunResult, error) {
	hook, ok := m.findHook(hookID)
	if !ok {
		return nil, nil
	}
	if m.hookExecutionPaused(hook.ID) {
		return nil, ErrHookPaused
	}
	if hook.Type == HookTypeFile {
		return m.rerunFileHook(hook), nil
	}
	if hook.Type == HookTypeSession {
		return m.rerunSessionHook(hook), nil
	}
	return nil, nil
}

func (m *Manager) findHook(hookID string) (Hook, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reloadHooks()
	for i := range m.fileHooks {
		if m.fileHooks[i].ID == hookID {
			return m.fileHooks[i], true
		}
	}
	for i := range m.sessionHooks {
		if m.sessionHooks[i].ID == hookID {
			return m.sessionHooks[i], true
		}
	}
	for i := range m.preCommitHooks {
		if m.preCommitHooks[i].ID == hookID {
			return m.preCommitHooks[i], true
		}
	}
	return Hook{}, false
}

func (m *Manager) rerunFileHook(hook Hook) *HookRunResult {
	if hook.Pattern == "" {
		return nil
	}

	allDirty := filterIgnoredFiles(DirtyFiles(m.workspaceRoot), m.globalIgnorePatterns())
	matching := matchHookFiles(allDirty, hook)
	if len(matching) == 0 {
		matching = allDirty // run even with no matches
	}

	outputPath := GetHookOutputPath(m.hooksDataDir, hook.ID)
	_ = SetHookRunning(m.hooksDataDir, hook)
	m.emitCurrentStatusChunk()

	runCutoff := time.Now()
	result := m.runHook(hook, runHookOptions{
		env:          m.visibleEnvSnapshot(),
		changedFiles: matching,
		outputPath:   outputPath,
	})

	eval := FileHookEvalResult{}
	if !result.Success {
		eval = buildHookFailureEvalResult(result, matching, outputPath, m.workspaceRoot)
	}

	_ = UpdateHookStatus(m.hooksDataDir, result, outputPath, runCutoff)

	if result.Success {
		_ = RemovePendingHook(m.hooksDataDir, hook.ID)
	}

	_ = UpdateLastEvaluatedAt(m.hooksDataDir)
	m.emitCurrentStatusChunk()

	return &HookRunResult{Result: result, Eval: eval}
}

func (m *Manager) rerunSessionHook(hook Hook) *HookRunResult {
	outputPath := GetHookOutputPath(m.hooksDataDir, hook.ID)
	_ = SetHookRunning(m.hooksDataDir, hook)
	m.emitCurrentStatusChunk()

	runCutoff := time.Now()
	result := m.runHook(hook, runHookOptions{
		env:        m.visibleStartupHookEnv(hook),
		outputPath: outputPath,
	})

	_ = UpdateHookStatus(m.hooksDataDir, result, outputPath, runCutoff)
	_ = UpdateLastEvaluatedAt(m.hooksDataDir)
	m.emitCurrentStatusChunk()

	return &HookRunResult{Result: result}
}

type runHookOptions struct {
	env          map[string]string
	changedFiles []string
	outputPath   string
	model        string
}

func (m *Manager) runHook(hook Hook, opts runHookOptions) HookResult {
	if hook.Engine == HookEngineAI {
		return m.runAIHook(hook, opts)
	}
	return ExecuteHook(hook, ExecuteOptions{
		Cwd:          m.workspaceRoot,
		Env:          opts.env,
		ChangedFiles: opts.changedFiles,
		SessionID:    m.sessionID,
		OutputPath:   opts.outputPath,
		Processes:    m.processes,
	})
}

func (m *Manager) runAIHook(hook Hook, opts runHookOptions) HookResult {
	start := time.Now()
	m.mu.Lock()
	aiHookAgent := m.aiHookAgent
	aiHookEvaluator := m.aiHookEvaluator
	m.mu.Unlock()

	if aiHookAgent == nil {
		result := HookResult{
			Success:    false,
			ExitCode:   127,
			Output:     "AI hook runner unavailable",
			Hook:       hook,
			DurationMs: time.Since(start).Milliseconds(),
		}
		writeOutputFile(opts.outputPath, result.Output)
		return result
	}

	threadID := aiHookThreadID(m.sessionID, hook.ID)
	followUp := aiHookThreadExists(aiHookAgent, threadID)
	taskResult, err := tools.RunPromptTask(context.Background(), aiHookAgent, tools.PromptTaskRequest{
		ThreadID:       threadID,
		Type:           "hook",
		Name:           "Hook: " + hook.Name,
		CWD:            m.workspaceRoot,
		Description:    hook.Description,
		Prompt:         formatAIHookPrompt(threadID, hook, opts.changedFiles, m.workspaceRoot, followUp),
		Model:          opts.model,
		SubagentType:   hook.Subagent,
		ParentThreadID: m.sessionID,
	})
	output := taskResult.Output
	if err != nil {
		if strings.TrimSpace(output) != "" {
			output += "\n\n"
		}
		output += "AI hook failed: " + err.Error()
	} else if taskResult.Status != "completed" {
		if strings.TrimSpace(output) != "" {
			output += "\n\n"
		}
		output += "AI hook did not complete: " + taskResult.Status
	}

	eval := aiHookEvaluation{Success: false, NotifyLLM: true}
	if err == nil && taskResult.Status == "completed" {
		eval = m.evaluateAIHookResponse(aiHookEvaluator, hook, output, opts.model)
	}
	exitCode := 0
	if !eval.Success {
		exitCode = 1
	}

	result := HookResult{
		Success:    eval.Success,
		NotifyLLM:  &eval.NotifyLLM,
		ExitCode:   exitCode,
		Output:     output,
		Hook:       hook,
		DurationMs: time.Since(start).Milliseconds(),
	}
	writeOutputFile(opts.outputPath, output)
	return result
}

func aiHookThreadExists(aiHookAgent AIHookAgent, threadID string) bool {
	if aiHookAgent == nil {
		return false
	}
	if _, err := aiHookAgent.GetThreadInfo(threadID); err != nil {
		return false
	}
	return true
}

func aiHookThreadID(sessionID, hookID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return "hook-" + hookID
	}
	return "hook-" + sessionID + "-" + hookID
}

const aiHookInlineDiffMaxBytes = 60 * 1024

type aiHookDiff struct {
	Full      string
	Inline    string
	Truncated bool
}

func formatAIHookPrompt(threadID string, hook Hook, changedFiles []string, workspaceRoot string, followUp bool) string {
	diff := gitDiffForHook(workspaceRoot, changedFiles)
	data := sessionconfig.AIHookPromptData{
		HookName:      hook.Name,
		Instructions:  hook.Prompt,
		Pattern:       hook.Pattern,
		ChangedFiles:  changedFiles,
		Diff:          diff.Inline,
		DiffTruncated: diff.Truncated,
		FollowUp:      followUp,
	}
	if path, err := writeAIHookContextFile(threadID, hook.ID, data, diff.Full); err != nil {
		log.Printf("hooks: warning: write AI hook context for %s: %v", hook.ID, err)
	} else {
		data.ContextFilePath = path
	}
	return sessionconfig.FormatAIHookPrompt(data)
}

func writeAIHookContextFile(threadID, hookID string, data sessionconfig.AIHookPromptData, fullDiff string) (string, error) {
	path := aiHookContextFilePath(threadID, hookID, time.Now().UTC())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data.Diff = fullDiff
	data.DiffTruncated = false
	data.ContextFilePath = ""
	content := sessionconfig.FormatAIHookContext(data)
	if err := thread.WriteFileAtomic(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func aiHookContextFilePath(threadID, hookID string, at time.Time) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(
		home,
		".discobot",
		"threads",
		threadID,
		"ai-hooks",
		hookID,
		"context-"+at.Format("20060102T150405.000000000Z")+".md",
	)
}

func gitDiffForHook(workspaceRoot string, changedFiles []string) aiHookDiff {
	args := []string{"diff", "--no-ext-diff", "--"}
	args = append(args, changedFiles...)
	diff, err := gitOutput(workspaceRoot, args...)
	if err != nil || strings.TrimSpace(diff) == "" {
		args = []string{"diff", "--no-ext-diff", "--cached", "--"}
		args = append(args, changedFiles...)
		diff, _ = gitOutput(workspaceRoot, args...)
	}
	result := aiHookDiff{
		Full:   diff,
		Inline: diff,
	}
	if len(diff) > aiHookInlineDiffMaxBytes {
		result.Inline = diff[:aiHookInlineDiffMaxBytes] + "\n[diff truncated]\n"
		result.Truncated = true
	}
	return result
}

type aiHookEvaluation struct {
	Success   bool   `json:"success"`
	NotifyLLM bool   `json:"notifyLLM"`
	Reason    string `json:"reason,omitempty"`
}

func (m *Manager) evaluateAIHookResponse(evaluator AIHookEvaluator, hook Hook, output, model string) aiHookEvaluation {
	if evaluator == nil {
		return aiHookEvaluation{Success: false, NotifyLLM: true}
	}
	maxTokens := 300
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := evaluator.CompleteText(ctx, model, []message.Message{
		{
			Role: "user",
			Parts: []message.Part{message.TextPart{Text: sessionconfig.FormatAIHookEvaluationPrompt(sessionconfig.AIHookEvaluationPromptData{
				HookName:     hook.Name,
				Instructions: hook.Prompt,
				Output:       output,
			})}},
		},
	}, &maxTokens)
	if err != nil {
		return aiHookEvaluation{Success: false, NotifyLLM: true}
	}
	eval, ok := parseAIHookEvaluation(response)
	if !ok {
		return aiHookEvaluation{Success: false, NotifyLLM: true}
	}
	return eval
}

func parseAIHookEvaluation(output string) (aiHookEvaluation, bool) {
	trimmed := strings.TrimSpace(output)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var eval aiHookEvaluation
	if err := json.Unmarshal([]byte(trimmed), &eval); err != nil {
		return aiHookEvaluation{}, false
	}
	return eval, true
}

// EvaluateFileHooks evaluates file hooks after a completion.
func (m *Manager) EvaluateFileHooks(args ...string) FileHookEvalResult {
	threadID := ""
	if len(args) > 0 {
		threadID = strings.TrimSpace(args[0])
	}
	hookModel := ""
	if len(args) > 1 {
		hookModel = strings.TrimSpace(args[1])
	}
	return m.evaluateFileHooks(threadID, hookModel, false)
}

func (m *Manager) evaluateFileHooks(threadID, hookModel string, forcePending bool) FileHookEvalResult {
	noAction := FileHookEvalResult{}
	if m.ExecutionPaused() {
		return noAction
	}
	m.mu.Lock()
	m.checkAndReloadHooks()
	fileHooks := make([]Hook, len(m.fileHooks))
	copy(fileHooks, m.fileHooks)
	m.mu.Unlock()

	if len(fileHooks) == 0 {
		return noAction
	}

	// Find files changed since marker
	globalIgnorePatterns := m.globalIgnorePatterns()
	newFiles := filterIgnoredFiles(m.findChangedFilesSinceMarker(), globalIgnorePatterns)

	addedNewPending := false
	if len(newFiles) > 0 {
		// Match against hook patterns
		var pendingIDs []string
		for _, hook := range fileHooks {
			if hook.Pattern == "" {
				continue
			}
			if len(matchHookFiles(newFiles, hook)) > 0 {
				pendingIDs = append(pendingIDs, hook.ID)
			}
		}
		if len(pendingIDs) > 0 {
			_ = AddPendingHooks(m.hooksDataDir, pendingIDs)
			m.emitCurrentStatusChunk()
			addedNewPending = true
		}
	}

	// Always advance marker
	m.touchMarker()

	// Get pending hooks
	pendingIDs := GetPendingHookIDs(m.hooksDataDir)
	if len(pendingIDs) == 0 {
		return noAction
	}

	hooksByID := make(map[string]Hook, len(fileHooks))
	for _, hook := range fileHooks {
		hooksByID[hook.ID] = hook
	}

	threadPhase := m.currentThreadPhase(threadID)
	hasPhaseEligiblePending := false
	for _, hookID := range pendingIDs {
		hook, ok := hooksByID[hookID]
		if ok && strings.TrimSpace(hook.Phase) != "" && hook.Phase == threadPhase {
			hasPhaseEligiblePending = true
			break
		}
	}

	// Skip guard: if no new files and didn't add new pending, don't re-run failed hooks
	if len(newFiles) == 0 && !addedNewPending && !forcePending && !hasPhaseEligiblePending {
		return noAction
	}

	// Get all dirty files for pattern matching
	allDirty := filterIgnoredFiles(DirtyFiles(m.workspaceRoot), globalIgnorePatterns)

	for _, hookID := range pendingIDs {
		hook, ok := hooksByID[hookID]
		if !ok {
			continue
		}
		if strings.TrimSpace(hook.Phase) != "" && hook.Phase != threadPhase {
			_ = SetHookPending(m.hooksDataDir, hook)
			m.emitCurrentStatusChunk()
			continue
		}
		if m.hookExecutionPaused(hook.ID) {
			continue
		}

		matching := matchHookFiles(allDirty, hook)
		if len(matching) == 0 {
			// Files were committed/fixed — remove from pending
			_ = RemovePendingHook(m.hooksDataDir, hook.ID)
			m.emitCurrentStatusChunk()
			continue
		}
		changedSinceHook := filterIgnoredFiles(
			m.changedFilesSinceHookMarker(hook.ID),
			globalIgnorePatterns,
		)
		reportFiles := matchHookFiles(changedSinceHook, hook)
		if len(reportFiles) == 0 && forcePending {
			reportFiles = matching
		}
		if len(reportFiles) == 0 {
			continue
		}
		if m.hookExecutionPaused(hook.ID) {
			continue
		}

		outputPath := GetHookOutputPath(m.hooksDataDir, hook.ID)
		_ = SetHookRunning(m.hooksDataDir, hook)
		m.emitCurrentStatusChunk()

		runCutoff := time.Now()
		result := m.runHook(hook, runHookOptions{
			env:          m.visibleEnvSnapshot(),
			changedFiles: reportFiles,
			outputPath:   outputPath,
			model:        hookModel,
		})

		_ = UpdateHookStatus(m.hooksDataDir, result, outputPath, runCutoff)
		m.emitCurrentStatusChunk()

		if result.Success {
			_ = RemovePendingHook(m.hooksDataDir, hook.ID)
			m.emitCurrentStatusChunk()
			continue
		}

		// Hook failed
		if m.hookExecutionPaused(hook.ID) {
			continue
		}
		return buildHookFailureEvalResult(result, reportFiles, outputPath, m.workspaceRoot)
	}

	// All pending hooks cleared
	return FileHookEvalResult{Evaluated: true}
}

// OnTurnComplete schedules post-turn hook evaluation and any needed re-prompt.
func (m *Manager) OnTurnComplete(threadID string) {
	if m == nil || !m.HasFileHooks() {
		return
	}
	m.setLastEvaluationThread(threadID)
	if m.ExecutionPaused() {
		return
	}
	go m.scheduleEvaluation(threadID, m.threadModel(threadID), false)
}

// TriggerEvaluation starts hook evaluation for a thread after external state,
// such as the thread phase, changes.
func (m *Manager) TriggerEvaluation(threadID string) {
	if m == nil || !m.HasFileHooks() {
		return
	}
	m.setLastEvaluationThread(threadID)
	go m.scheduleEvaluation(threadID, m.threadModel(threadID), true)
}

// StartFailureReprompt sends or queues a hook-failure follow-up message to the
// LLM.
func (m *Manager) StartFailureReprompt(threadID string, result FileHookEvalResult) error {
	if m.ExecutionPaused() {
		return nil
	}
	if result.FailedResult != nil && m.hookExecutionPaused(result.FailedResult.Hook.ID) {
		return nil
	}
	req := hookFailurePromptRequest(result)
	m.mu.Lock()
	conversations := m.conversations
	promptQueue := m.promptQueue
	m.mu.Unlock()
	if promptQueue != nil {
		_, err := promptQueue.StartOrQueue(threadID, req, HookFailureQueuedPrompt(result))
		if errors.Is(err, agent.ErrPendingQuestionRequiresAnswer) {
			if _, _, enqueueErr := promptQueue.Enqueue(threadID, HookFailureQueuedPrompt(result)); enqueueErr != nil {
				return enqueueErr
			}
		}
		return err
	}
	if conversations == nil {
		return errors.New("hooks: reprompt runner unavailable")
	}
	interrupted, err := conversations.HasInterruptedTurn(threadID)
	if err != nil {
		return err
	}
	if interrupted {
		_, err = conversations.Resume(threadID, req)
		return err
	}
	_, err = conversations.Chat(threadID, req)
	return err
}

// HookFailureQueuedPrompt builds the queued prompt for a hook-failure
// re-prompt.
func HookFailureQueuedPrompt(result FileHookEvalResult) promptqueue.Prompt {
	req := hookFailurePromptRequest(result)
	return promptqueue.Prompt{
		Message: message.UIMessage{
			Role:     "user",
			Parts:    req.UserParts,
			Metadata: req.Metadata,
		},
	}
}

func hookFailurePromptRequest(result FileHookEvalResult) agent.PromptRequest {
	return agent.PromptRequest{
		Metadata: func() json.RawMessage {
			if result.HookFailure == nil {
				return nil
			}
			data, err := json.Marshal(map[string]any{
				"discobot": result.HookFailure,
			})
			if err != nil {
				return nil
			}
			return data
		}(),
		UserParts: []message.UIPart{
			message.UITextPart{Text: result.LLMMessage},
		},
	}
}

func (m *Manager) scheduleEvaluation(threadID, model string, forcePending bool) {
	// 200ms grace period to let SSE flush.
	time.Sleep(200 * time.Millisecond)

	result := m.evaluateFileHooks(threadID, model, forcePending)
	m.reconcileNotificationState()
	if !result.ShouldReprompt {
		return
	}
	if m.ExecutionPaused() {
		return
	}
	if result.FailedResult != nil && m.hookExecutionPaused(result.FailedResult.Hook.ID) {
		return
	}

	hookID := ""
	if result.FailedResult != nil {
		hookID = strings.TrimSpace(result.FailedResult.Hook.ID)
	}
	if hookID == "" {
		hookID = threadID
	}

	count, shouldNotify := m.claimNotificationThread(hookID, threadID)
	if !shouldNotify {
		return
	}
	if count >= maxHookRetries {
		log.Printf("hooks: max retries (%d) reached for hook %q, not re-prompting", maxHookRetries, hookID)
		return
	}

	if err := m.StartFailureReprompt(threadID, result); err != nil {
		log.Printf("hooks: failed to start re-prompt: %v", err)
	}
}

func (m *Manager) threadModel(threadID string) string {
	m.mu.Lock()
	aiHookAgent := m.aiHookAgent
	m.mu.Unlock()
	if aiHookAgent == nil {
		return ""
	}
	info, err := aiHookAgent.GetThreadInfo(threadID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(info.Model)
}

func (m *Manager) reconcileNotificationState() {
	status := m.GetStatus()
	pending := make(map[string]struct{}, len(status.PendingHooks))
	for _, hookID := range status.PendingHooks {
		pending[hookID] = struct{}{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for hookID := range m.hookNotificationTo {
		if _, ok := pending[hookID]; !ok {
			delete(m.hookNotificationTo, hookID)
			delete(m.hookRetryCount, hookID)
		}
	}
	for hookID := range m.hookRetryCount {
		if _, ok := pending[hookID]; !ok {
			delete(m.hookRetryCount, hookID)
		}
	}
}

func (m *Manager) claimNotificationThread(hookID, threadID string) (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	owner := m.hookNotificationTo[hookID]
	if owner == "" {
		m.hookNotificationTo[hookID] = threadID
		owner = threadID
	}
	if owner != threadID {
		return 0, false
	}

	m.hookRetryCount[hookID]++
	return m.hookRetryCount[hookID], true
}

// DirtyFiles returns all dirty files in the workspace (staged, unstaged, untracked).
func DirtyFiles(workspaceRoot string) []string {
	fileSet := make(map[string]bool)

	// git diff --name-only HEAD
	if out, err := gitOutput(workspaceRoot, "diff", "--name-only", "HEAD"); err == nil {
		for _, f := range splitLines(out) {
			fileSet[f] = true
		}
	}

	// git ls-files --others --exclude-standard
	if out, err := gitOutput(workspaceRoot, "ls-files", "--others", "--exclude-standard"); err == nil {
		for _, f := range splitLines(out) {
			fileSet[f] = true
		}
	}

	result := make([]string, 0, len(fileSet))
	for f := range fileSet {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

// DirtyFilesSinceMarker returns dirty files newer than markerPath.
// If the marker does not exist, all dirty files are returned.
func DirtyFilesSinceMarker(workspaceRoot, markerPath string) []string {
	markerInfo, err := os.Stat(markerPath)
	allDirty := DirtyFiles(workspaceRoot)
	if err != nil {
		return allDirty
	}
	markerMtime := markerInfo.ModTime()

	changed := make([]string, 0, len(allDirty))
	for _, f := range allDirty {
		fullPath := filepath.Join(workspaceRoot, f)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.ModTime().After(markerMtime) {
			changed = append(changed, f)
		}
	}
	return changed
}

// findChangedFilesSinceMarker returns dirty files newer than the .last-eval marker.
func (m *Manager) findChangedFilesSinceMarker() []string {
	return DirtyFilesSinceMarker(m.workspaceRoot, filepath.Join(m.hooksDataDir, ".last-eval"))
}

func (m *Manager) changedFilesSinceHookMarker(hookID string) []string {
	return DirtyFilesSinceMarker(m.workspaceRoot, hookMarkerPath(m.hooksDataDir, hookID))
}

func (m *Manager) globalIgnorePatterns() []string {
	return readHookIgnoreFile(filepath.Join(m.workspaceRoot, HooksDir, "ignore"))
}

// touchMarker creates or updates the .last-eval marker file.
func (m *Manager) touchMarker() {
	_ = os.MkdirAll(m.hooksDataDir, 0o755)
	markerPath := filepath.Join(m.hooksDataDir, ".last-eval")

	now := time.Now()
	if err := os.Chtimes(markerPath, now, now); err != nil {
		// File doesn't exist — create it
		_ = os.WriteFile(markerPath, nil, 0o644)
	}

	_ = UpdateLastEvaluatedAt(m.hooksDataDir)
}

// matchFiles returns files that match a glob pattern.
// Supports *, **, ?, [class], and {a,b,c} brace expansion via doublestar.
func matchFiles(files []string, pattern string) []string {
	pattern = filepath.ToSlash(pattern)
	var matched []string
	for _, f := range files {
		ok, err := doublestar.Match(pattern, filepath.ToSlash(f))
		if err == nil && ok {
			matched = append(matched, f)
		}
	}
	return matched
}

func matchHookFiles(files []string, hook Hook) []string {
	matched := matchFiles(files, hook.Pattern)
	if hook.Ignore == "" {
		return matched
	}
	return filterIgnoredFiles(matched, []string{hook.Ignore})
}

func filterIgnoredFiles(files, ignorePatterns []string) []string {
	if len(files) == 0 || len(ignorePatterns) == 0 {
		return files
	}

	var filtered []string
	for _, file := range files {
		if matchesAnyPattern(file, ignorePatterns) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

func matchesAnyPattern(file string, patterns []string) bool {
	file = filepath.ToSlash(file)
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		ok, err := doublestar.Match(pattern, file)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func readHookIgnoreFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var patterns []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// buildHookFailureEvalResult builds the evaluation response for a failed hook.
func buildHookFailureEvalResult(result HookResult, matchingFiles []string, outputPath, workspaceRoot string) FileHookEvalResult {
	if result.Success {
		return FileHookEvalResult{}
	}
	notifyLLM := result.Hook.NotifyLLM
	if result.NotifyLLM != nil {
		notifyLLM = *result.NotifyLLM
	}
	if !notifyLLM {
		return FileHookEvalResult{
			Evaluated:      true,
			ShouldReprompt: false,
			FailedResult:   &result,
		}
	}

	meta := buildHookFailureMessageMetadata(result, matchingFiles, outputPath, workspaceRoot)
	msg := formatHookFailureMessage(meta)
	return FileHookEvalResult{
		Evaluated:      true,
		ShouldReprompt: true,
		LLMMessage:     msg,
		FailedResult:   &result,
		HookFailure:    &meta,
	}
}

func normalizeHookMessagePath(workspaceRoot, path string) string {
	if path == "" || !filepath.IsAbs(path) || workspaceRoot == "" {
		return path
	}

	relPath, err := filepath.Rel(workspaceRoot, path)
	if err != nil || relPath == "." || strings.HasPrefix(relPath, "..") {
		return path
	}

	return filepath.ToSlash(relPath)
}

// buildHookFailureMessageMetadata builds structured hook-failure metadata for UI rendering.
func buildHookFailureMessageMetadata(result HookResult, matchingFiles []string, outputPath, workspaceRoot string) HookFailureMessageMetadata {
	meta := HookFailureMessageMetadata{
		Kind:     "hook-failure",
		HookName: result.Hook.Name,
		ExitCode: result.ExitCode,
		Pattern:  result.Hook.Pattern,
		HookPath: normalizeHookMessagePath(workspaceRoot, result.Hook.Path),
	}

	if len(matchingFiles) > 0 {
		displayFiles := matchingFiles
		if len(displayFiles) > 20 {
			meta.ExtraFileCount = len(displayFiles) - 20
			displayFiles = displayFiles[:20]
		}
		meta.Files = append([]string(nil), displayFiles...)
	}

	output := strings.TrimSpace(result.Output)
	if output == "" {
		return meta
	}

	lines := strings.Split(output, "\n")
	if len(lines) > InlineOutputMaxLines || len(output) > InlineOutputMaxBytes {
		meta.OutputPath = outputPath
		meta.OutputTail = lastLines(output, TruncatedOutputTailLines)
		meta.OutputTruncated = true
		return meta
	}

	meta.Output = output
	return meta
}

func lastLines(output string, maxLines int) string {
	if maxLines <= 0 || output == "" {
		return ""
	}

	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}

	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

// formatHookFailureMessage builds the markdown message for LLM re-prompt.
func formatHookFailureMessage(meta HookFailureMessageMetadata) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Hook failed: %s\n\n", meta.HookName)
	fmt.Fprintf(&b, "- Exit code: `%d`\n", meta.ExitCode)
	if meta.Pattern != "" {
		fmt.Fprintf(&b, "- Pattern: `%s`\n", meta.Pattern)
	}

	if len(meta.Files) > 0 {
		filesStr := strings.Join(meta.Files, ", ")
		if meta.ExtraFileCount > 0 {
			filesStr += fmt.Sprintf(", and %d more", meta.ExtraFileCount)
		}
		fmt.Fprintf(&b, "- Files: %s\n", filesStr)
	}
	b.WriteString("\n")

	if meta.OutputTruncated && meta.OutputPath != "" {
		b.WriteString("#### Output\n\n")
		fmt.Fprintf(&b, "Output was too long to inline. Full output was written to `%s`.\n\n", meta.OutputPath)
		if meta.OutputTail != "" {
			fmt.Fprintf(&b, "Last %d lines:\n\n", TruncatedOutputTailLines)
			b.WriteString("```text\n")
			b.WriteString(meta.OutputTail)
			b.WriteString("\n```\n\n")
		}
	} else if meta.OutputTruncated && meta.OutputTail != "" {
		b.WriteString("#### Output\n\n")
		b.WriteString("```text\n")
		b.WriteString(meta.OutputTail)
		b.WriteString("\n```\n\n")
	} else if meta.Output != "" {
		b.WriteString("#### Output\n\n")
		b.WriteString("```text\n")
		b.WriteString(meta.Output)
		b.WriteString("\n```\n\n")
	}

	b.WriteString("Please fix the issues above and ensure the hook passes.")

	return b.String()
}

// gitOutput runs a git command and returns trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// splitLines splits output by newlines, filtering empty lines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
