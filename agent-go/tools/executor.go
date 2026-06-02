// Package tools provides a concrete implementation of thread.ToolExecutor
// that executes all built-in tools natively in Go.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/helperbin"
	"github.com/obot-platform/discobot/agent-go/internal/workspaceenv"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

const (
	maxOutputLines = 2000
	maxOutputBytes = 50 * 1024
)

// fileRecord stores the mtime and size of a file at the time it was last read
// via the Read tool. It is used to enforce the read-before-write invariant.
type fileRecord struct {
	modTime time.Time
	size    int64
}

// Executor implements thread.ToolExecutor with native Go tool implementations.
type Executor struct {
	cwd             string // workspace root for file and shell operations
	dataDir         string // root for persistent data (bash logs, spill files, etc.); separate from cwd
	threadsDir      string // root for per-thread runtime data; defaults to {dataDir}/threads
	defaultThreadID string

	// cwdMu guards currentCwd, the fallback working directory used by callers
	// that do not provide a thread-scoped ToolContext (mainly tests and legacy
	// compatibility wrappers). Real agent turns should use
	// ToolContext.CurrentWorkingDirectory instead.
	cwdMu      sync.Mutex
	currentCwd string

	// fileReadsMu guards fileReads, which records the mtime+size of every file
	// read via the Read tool. Write and Edit consult this to enforce
	// read-before-write: an existing file may not be overwritten unless the
	// executor has a matching record for it.
	fileReadsMu sync.RWMutex
	fileReads   map[string]fileRecord // keyed by absolute path

	// envLookup is an optional secondary source for environment variable
	// lookups (e.g. per-request credentials). It is consulted first; os.Getenv
	// is the fallback.
	envLookup func(key string) string

	// envSnapshot is an optional source of the full current request-scoped
	// environment (e.g. credentials visible to subprocesses). Bash uses it
	// to merge request-scoped variables into its process environment.
	envSnapshot func() map[string]string

	envForThread func(threadID string) map[string]string

	credentialUseAuthorizer func(ctx context.Context, currentProviderID, toolCallID, command, description string, uses []CredentialUseBinding) error
	credentialUseEnv        func(uses []CredentialUseBinding) (map[string]string, error)
}

type CredentialUseBinding struct {
	CredentialID string `json:"credentialId"`
	UseID        string `json:"useId"`
	EnvVar       string `json:"envVar"`
}

// New creates an Executor rooted at cwd.
// dataDir is the root for persistent storage (bash logs, spill files, etc.);
// it defaults to the user's home directory if empty.
func New(cwd, dataDir, threadID string) *Executor {
	if dataDir == "" {
		dataDir, _ = os.UserHomeDir()
	}
	return &Executor{
		cwd:             cwd,
		dataDir:         dataDir,
		threadsDir:      filepath.Join(dataDir, "threads"),
		defaultThreadID: threadID,
		currentCwd:      cwd,
		fileReads:       make(map[string]fileRecord),
	}
}

// SetThreadsDir overrides the default per-thread runtime root.
func (e *Executor) SetThreadsDir(dir string) {
	if strings.TrimSpace(dir) == "" {
		e.threadsDir = filepath.Join(e.dataDir, "threads")
		return
	}
	e.threadsDir = dir
}

// recordFileRead saves the mtime and size of a file after a successful Read.
func (e *Executor) recordFileRead(absPath string, info os.FileInfo) {
	e.fileReadsMu.Lock()
	defer e.fileReadsMu.Unlock()
	e.fileReads[absPath] = fileRecord{modTime: info.ModTime(), size: info.Size()}
}

// recordFileWritten updates the stored record for a file after a successful
// Write or Edit, so subsequent writes don't require a re-read.
func (e *Executor) recordFileWritten(absPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		return
	}
	e.recordFileRead(absPath, info)
}

// checkWriteAllowed returns nil when it is safe to write to absPath:
//   - the file does not exist yet (new file creation is always permitted), or
//   - the file was previously read via the Read tool AND its mtime+size still
//     match the recorded snapshot (the file has not changed underneath us).
//
// displayPath is the user-facing path used in error messages.
func (e *Executor) checkWriteAllowed(absPath, displayPath string) error {
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return nil // new file — no prior read required
	}
	if err != nil {
		return err
	}
	return e.checkRecordedRead(info, absPath, displayPath)
}

func (e *Executor) checkRecordedRead(info os.FileInfo, absPath, displayPath string) error {
	e.fileReadsMu.RLock()
	rec, ok := e.fileReads[absPath]
	e.fileReadsMu.RUnlock()

	if !ok {
		return fmt.Errorf("you must read %q before writing it", displayPath)
	}
	if rec.modTime != info.ModTime() || rec.size != info.Size() {
		return fmt.Errorf("%q has changed since it was last read — re-read it before writing", displayPath)
	}
	return nil
}

// SetEnvLookup sets an optional function used to look up environment variables
// from a secondary source (e.g. per-request credentials). It is consulted
// before os.Getenv so that request-scoped values take precedence.
func (e *Executor) SetEnvLookup(fn func(key string) string) {
	e.envLookup = fn
}

// SetEnvSnapshot sets an optional function that returns the full current
// request-scoped environment. Bash uses it to merge request-scoped variables
// into command executions.
func (e *Executor) SetEnvSnapshot(fn func() map[string]string) {
	e.envSnapshot = fn
}

func (e *Executor) SetEnvForThread(fn func(threadID string) map[string]string) {
	e.envForThread = fn
}

func (e *Executor) SetCredentialUseAuthorizer(fn func(ctx context.Context, currentProviderID, toolCallID, command, description string, uses []CredentialUseBinding) error) {
	e.credentialUseAuthorizer = fn
}

func (e *Executor) SetCredentialUseEnv(fn func(uses []CredentialUseBinding) (map[string]string, error)) {
	e.credentialUseEnv = fn
}

func (e *Executor) authorizeCredentialUses(ctx context.Context, currentProviderID, toolCallID, command, description string, uses []CredentialUseBinding) error {
	if len(uses) == 0 || e.credentialUseAuthorizer == nil {
		return nil
	}
	return e.credentialUseAuthorizer(ctx, currentProviderID, toolCallID, command, description, uses)
}

func (e *Executor) envForCredentialUses(uses []CredentialUseBinding) (map[string]string, error) {
	if len(uses) == 0 {
		return nil, nil
	}
	if e.credentialUseEnv == nil {
		return nil, fmt.Errorf("credential environment resolver is not configured")
	}
	return e.credentialUseEnv(uses)
}

// getenv returns the value of the environment variable named by key.
// It consults e.envLookup first (if set), then the latest workspace-aware
// environment snapshot.
func (e *Executor) getenv(key string) string {
	if e.envLookup != nil {
		if v := e.envLookup(key); v != "" {
			return v
		}
	}
	return e.currentEnv()[key]
}

func (e *Executor) currentEnv() map[string]string {
	env := workspaceenv.ProcessSnapshot()
	if e.envSnapshot != nil {
		maps.Copy(env, e.envSnapshot())
	}
	env["PATH"] = helperbin.PrependToPath(env["PATH"])
	return env
}

func (e *Executor) bashEnv() []string {
	return workspaceenv.List(e.currentEnv())
}

func contextThreadID(toolCtx *thread.ToolContext, fallback string) string {
	if toolCtx != nil && toolCtx.ThreadID != "" {
		return toolCtx.ThreadID
	}
	if fallback != "" {
		return fallback
	}
	return "default"
}

func (e *Executor) threadDataDir(toolCtx *thread.ToolContext) string {
	return filepath.Join(e.threadsDir, contextThreadID(toolCtx, e.defaultThreadID))
}

// Execute dispatches to the appropriate tool handler and enforces output size limits.
func (e *Executor) Execute(ctx context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	result, err := e.dispatch(ctx, toolCtx, call)
	if err != nil {
		return result, err
	}
	return e.limitOutput(toolCtx, call, result), nil
}

// dispatch routes a tool call to its handler.
func (e *Executor) dispatch(ctx context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	switch call.ToolName {
	case "Bash", "PowerShell":
		return e.executeBash(ctx, toolCtx, call)
	case "Read":
		return e.executeRead(toolCtx, call)
	case "Write":
		return e.executeWrite(call)
	case "Edit":
		return e.executeEdit(call)
	case "apply_patch":
		return e.executeApplyPatch(call)
	case "Glob":
		return e.executeGlob(call)
	case "Grep":
		return e.executeGrep(ctx, call)
	case "WebFetch":
		return e.executeWebFetch(ctx, toolCtx, call)
	case "WebSearch":
		return e.executeWebSearch(ctx, call)
	case "AskUserQuestion":
		return e.executeAskUserQuestion(call)
	case "RequestUserCredential":
		return e.executeRequestUserCredential(call)
	case "RequestCommitPull":
		return e.executeRequestCommitPull(toolCtx, call)
	case "Task", "Agent":
		return e.executeTask(ctx, toolCtx, call)
	case "TodoWrite":
		return e.executeTodoWrite(ctx, toolCtx, call)
	case "TaskOutput":
		return e.executeTaskOutput(toolCtx, call)
	case "TaskStop":
		return e.executeTaskStop(call)
	case "Skill":
		return e.executeSkill(ctx, call)
	case "ReadyForReview":
		return e.executeReadyForReview(toolCtx, call)
	default:
		return textResult(call, fmt.Sprintf("unknown tool: %s", call.ToolName)), nil
	}
}

func (e *Executor) executeReadyForReview(toolCtx *thread.ToolContext, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	if toolCtx == nil || toolCtx.SetThreadPhase == nil {
		return errResult(call, "thread phase updates are unavailable"), nil
	}
	if err := toolCtx.SetThreadPhase("review"); err != nil {
		return errResult(call, err.Error()), nil
	}
	return textResult(call, "Thread phase set to review."), nil
}

// limitOutput checks whether a successful TextOutput exceeds the model-facing
// inline limits.
// If it does, the full content is written to a file and the inline value is
// replaced with a short preview plus a path to the full output.
func (e *Executor) limitOutput(toolCtx *thread.ToolContext, call message.ToolCallPart, result thread.ToolExecuteResult) thread.ToolExecuteResult {
	if call.ToolName == "Read" {
		return result
	}
	to, ok := result.Result.Output.(message.TextOutput)
	if !ok {
		return result
	}
	preview, truncated := truncateTextOutput(to.Value)
	if !truncated {
		return result
	}

	outPath, writeErr := e.spillToFile(toolCtx, call, to.Value)

	var text string
	if writeErr != nil {
		text = fmt.Sprintf(
			"%s\n\nThe tool call succeeded but the output was truncated. Could not write the full output to a file: %v",
			preview, writeErr,
		)
	} else {
		text = fmt.Sprintf(
			"%s\n\nThe tool call succeeded but the output was truncated. Full output saved to: %s\nUse Grep to search the full content or Read with offset/limit to view specific sections.",
			preview, outPath,
		)
	}

	result.Result.Output = message.TextOutput{Value: text}
	return result
}

// spillToFile writes text to {threadsDir}/{threadID}/output/{toolCallID}.txt
// and returns the absolute path.
func (e *Executor) spillToFile(toolCtx *thread.ToolContext, call message.ToolCallPart, text string) (string, error) {
	dir := filepath.Join(e.threadDataDir(toolCtx), "output")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, call.ToolCallID+".txt")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Continue resumes a previously paused or asynchronous tool execution using
// executor-owned continuation metadata.
func (e *Executor) Continue(ctx context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart, continuation json.RawMessage, req *api.AnswerQuestionRequest) (thread.ToolExecuteResult, error) {
	if len(continuation) > 0 {
		switch call.ToolName {
		case "Task", "Agent":
			return e.continueTask(ctx, toolCtx, call, continuation, req)
		default:
			return thread.ToolExecuteResult{
				Result: errorResult(call, fmt.Sprintf("continuation for %s lost after crash", call.ToolName)),
			}, nil
		}
	}

	if req == nil {
		return thread.ToolExecuteResult{}, fmt.Errorf("Continue requires either continuation state or an answer for tool %s", call.ToolName)
	}

	switch call.ToolName {
	case "AskUserQuestion":
		result, err := e.resolveAskUserQuestion(call, *req)
		if err != nil {
			return thread.ToolExecuteResult{}, err
		}
		return thread.ToolExecuteResult{Result: result}, nil
	case "RequestUserCredential":
		result, err := e.resolveRequestUserCredential(call, *req)
		if err != nil {
			return thread.ToolExecuteResult{}, err
		}
		return thread.ToolExecuteResult{Result: result}, nil
	case "RequestCommitPull":
		result, err := e.resolveRequestCommitPull(call, *req)
		if err != nil {
			return thread.ToolExecuteResult{}, err
		}
		return thread.ToolExecuteResult{Result: result}, nil
	default:
		return thread.ToolExecuteResult{}, fmt.Errorf("Continue not supported for tool %s", call.ToolName)
	}
}

// ResolveAnswer is kept as a thin compatibility wrapper for callers/tests that
// still invoke the older answered-tool API directly.
func (e *Executor) ResolveAnswer(toolCtx *thread.ToolContext, call message.ToolCallPart, req api.AnswerQuestionRequest) (thread.ToolExecuteResult, error) {
	return e.Continue(context.Background(), toolCtx, call, nil, &req)
}

// ResumeAsync is kept as a thin compatibility wrapper for callers/tests that
// still invoke the older async-recovery API directly.
func (e *Executor) ResumeAsync(ctx context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart, subThreadID string, req *api.AnswerQuestionRequest) (thread.ToolExecuteResult, error) {
	return e.Continue(ctx, toolCtx, call, marshalTaskContinuation(subThreadID), req)
}

// --- helpers ---

// textResult builds a successful text tool result.
func textResult(call message.ToolCallPart, text string) thread.ToolExecuteResult {
	return thread.ToolExecuteResult{
		Result: message.ToolResultPart{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Output:     message.TextOutput{Value: text},
		},
	}
}

// errorResult builds an error text tool result.
func errorResult(call message.ToolCallPart, msg string) message.ToolResultPart {
	return message.ToolResultPart{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Output:     message.ErrorTextOutput{Value: msg},
	}
}

// errResult wraps errorResult in a ToolExecuteResult.
func errResult(call message.ToolCallPart, msg string) thread.ToolExecuteResult {
	return thread.ToolExecuteResult{Result: errorResult(call, msg)}
}

func errToolResult(call message.ToolCallPart, msg string) (thread.ToolExecuteResult, error) {
	return errResult(call, msg), nil
}

func truncateTextOutput(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	totalBytes := len(text)
	if len(lines) <= maxOutputLines && totalBytes <= maxOutputBytes {
		return text, false
	}

	out := make([]string, 0, min(len(lines), maxOutputLines))
	bytes := 0
	hitBytes := false
	for i, line := range lines {
		if i >= maxOutputLines {
			break
		}
		size := len(line)
		if i > 0 {
			size++
		}
		if bytes+size > maxOutputBytes {
			hitBytes = true
			break
		}
		out = append(out, line)
		bytes += size
	}

	removed := len(lines) - len(out)
	unit := "lines"
	if hitBytes {
		removed = totalBytes - bytes
		unit = "bytes"
	}

	return fmt.Sprintf("%s\n\n...%d %s truncated...", strings.Join(out, "\n"), removed, unit), true
}

// unmarshalInput decodes the tool call input JSON into dst.
func unmarshalInput(call message.ToolCallPart, dst any) error {
	if err := json.Unmarshal([]byte(call.Input), dst); err != nil {
		return fmt.Errorf("invalid input for %s: %w", call.ToolName, err)
	}
	return nil
}

func sameResolvedPath(targetPath, expectedPath string) bool {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	absExpected, err := filepath.Abs(expectedPath)
	if err != nil {
		return false
	}

	if targetInfo, err := os.Stat(absTarget); err == nil {
		if expectedInfo, err := os.Stat(absExpected); err == nil && os.SameFile(targetInfo, expectedInfo) {
			return true
		}
	}

	cleanTarget := filepath.Clean(absTarget)
	cleanExpected := filepath.Clean(absExpected)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(cleanTarget, cleanExpected)
	}
	return cleanTarget == cleanExpected
}

// resolvePath resolves a file path relative to cwd.
// Absolute paths are returned as-is; relative paths are joined with cwd.
func resolvePath(cwd, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if cwd == "" || path == "" {
		return path
	}
	return filepath.Join(cwd, path)
}
