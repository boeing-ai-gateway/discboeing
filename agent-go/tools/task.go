package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

// Task/Agent tool — launches a sub-agent for a complex task.
// Each task runs as an async operation with its own mini turn loop.

type taskInput struct {
	Description     string `json:"description"`
	Prompt          string `json:"prompt"`
	Resume          string `json:"resume"`
	RunInBackground bool   `json:"run_in_background"`

	// Agent-specific fields (from the Agent/Task tool schema).
	SubagentType string `json:"subagent_type"`
}

type taskContinuation struct {
	TaskID      string `json:"taskId,omitempty"`
	SubThreadID string `json:"subThreadId"`
}

// PromptTaskAgent is the subset of agent behavior needed to run a Task-style
// prompt on a managed thread.
type PromptTaskAgent interface {
	CreateThread(ctx context.Context, req agent.CreateThreadRequest) (agent.ThreadInfo, error)
	UpdateThread(ctx context.Context, threadID string, req agent.UpdateThreadRequest) (agent.ThreadInfo, error)
	GetThreadInfo(threadID string) (agent.ThreadInfo, error)
	Prompt(ctx context.Context, threadID string, req agent.PromptRequest) iter.Seq2[message.MessageChunk, error]
	Resume(ctx context.Context, threadID string, req agent.PromptRequest) (agent.ResumeResult, error)
	HasInterruptedTurn(threadID string) (bool, error)
	PendingQuestion(threadID string) (*agent.PendingQuestion, error)
	FinalResponse(threadID string) (string, error)
}

// PromptTaskRequest describes a Task-style prompt run on a caller-selected
// stable thread ID.
type PromptTaskRequest struct {
	ThreadID       string
	Type           string
	Name           string
	CWD            string
	Description    string
	Prompt         string
	Model          string
	SubagentType   string
	ParentThreadID string
	ParentTaskID   string
	Depth          int
}

// PromptTaskResult is the final status and output of a PromptTaskRequest.
type PromptTaskResult struct {
	TaskID   string
	ThreadID string
	Status   string
	Output   string
}

// taskRecord tracks an in-progress or completed Task.
type taskRecord struct {
	taskID  string
	status  string // "pending", "in_progress", "waiting_for_answer", "completed", "failed", "cancelled"
	output  string
	created time.Time

	parentThreadID     string
	parentTaskID       string
	depth              int
	subThreadID        string
	pendingApprovalID  string
	pendingQuestions   json.RawMessage
	pendingCredentials json.RawMessage

	mu     sync.Mutex
	done   chan struct{}
	cancel context.CancelFunc // non-nil for Task/Agent sub-agent tasks; called by TaskStop
}

type subagentTypeValidator interface {
	ValidateSubagentType(subagentType string) error
}

// taskStore holds all tasks for this executor instance.
type taskStore struct {
	mu    sync.Mutex
	tasks map[string]*taskRecord
}

var globalTasks = &taskStore{tasks: make(map[string]*taskRecord)}

func init() {
	agent.RegisterExternalCompletionIDProvider(taskCompletionID)
	agent.RegisterExternalCompletionCancelProvider(cancelTaskCompletion)
}

func taskCompletionID(threadID string) string {
	if IsTaskRunning(threadID) {
		return threadID
	}
	return ""
}

func cancelTaskCompletion(threadID string) (string, bool) {
	globalTasks.mu.Lock()
	rec := globalTasks.tasks[threadID]
	globalTasks.mu.Unlock()
	if rec == nil {
		return "", false
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.status != "in_progress" || rec.cancel == nil {
		return "", false
	}
	rec.status = "cancelled"
	rec.output = ""
	rec.cancel()
	return threadID, true
}

var subThreadIDSuffix = agent.GenerateID

func newSubThreadID(parentThreadID string) string {
	return fmt.Sprintf("%s.sub.%s", parentThreadID, subThreadIDSuffix())
}

func reserveSubThreadID(parentThreadID string) (string, error) {
	for range 100 {
		subThreadID := newSubThreadID(parentThreadID)
		globalTasks.mu.Lock()
		_, active := globalTasks.tasks[subThreadID]
		globalTasks.mu.Unlock()
		if active {
			continue
		}
		return subThreadID, nil
	}

	return "", fmt.Errorf("could not allocate a unique sub-thread ID")
}

func subagentDepthFromThreadID(threadID string) int {
	return strings.Count(threadID, ".sub.")
}

func normalizeSubagentType(subagentType string) string {
	if strings.TrimSpace(subagentType) == "general" {
		return "general-purpose"
	}
	return subagentType
}

func parentTaskModel(toolCtx *thread.ToolContext) string {
	if toolCtx == nil || strings.TrimSpace(toolCtx.ProviderID) == "" || strings.TrimSpace(toolCtx.ModelID) == "" {
		return ""
	}
	return strings.TrimSpace(toolCtx.ProviderID) + "/" + strings.TrimSpace(toolCtx.ModelID)
}

func taskPrompt(input taskInput) string {
	if prompt := strings.TrimSpace(input.Prompt); prompt != "" {
		return prompt
	}
	return strings.TrimSpace(input.Description)
}

func subAgentPromptRequest(toolCtx *thread.ToolContext, input taskInput, subThreadID string, depth int) agent.PromptRequest {
	return agent.PromptRequest{
		UserParts:     []message.UIPart{message.UITextPart{Text: taskPrompt(input)}},
		Model:         parentTaskModel(toolCtx),
		SubagentType:  input.SubagentType,
		ParentTaskID:  subThreadID,
		SubagentDepth: depth,
	}
}

func taskIsTerminal(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// RunPromptTask runs a prompt through the same in-memory task runner used by
// the Task/TaskOutput tools, but on a stable thread ID supplied by the caller.
// This makes the run visible to external completion tracking while still
// allowing system features such as hooks to wait synchronously for the result.
func RunPromptTask(ctx context.Context, taskAgent PromptTaskAgent, req PromptTaskRequest) (PromptTaskResult, error) {
	threadID := strings.TrimSpace(req.ThreadID)
	if threadID == "" {
		return PromptTaskResult{}, fmt.Errorf("thread ID is required")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return PromptTaskResult{}, fmt.Errorf("prompt is required")
	}
	if taskAgent == nil {
		return PromptTaskResult{}, fmt.Errorf("task agent is required")
	}
	if validator, ok := taskAgent.(subagentTypeValidator); ok {
		if err := validator.ValidateSubagentType(req.SubagentType); err != nil {
			return PromptTaskResult{}, err
		}
	}

	threadType := strings.TrimSpace(req.Type)
	if threadType == "" {
		threadType = "task"
	}
	created := time.Now()
	metadata := thread.ConfigMetadata{
		Type:           threadType,
		TaskID:         threadID,
		ParentThreadID: req.ParentThreadID,
		ParentTaskID:   req.ParentTaskID,
		SubagentType:   req.SubagentType,
		Description:    req.Description,
		Prompt:         prompt,
		StartedAt:      created.UTC(),
	}
	if req.Depth == 0 {
		req.Depth = subagentDepthFromThreadID(threadID)
	}

	rec, err := preparePromptTaskThread(ctx, taskAgent, req, metadata, created)
	if err != nil {
		return PromptTaskResult{}, err
	}

	startSubAgentRun(rec, taskAgent, threadID, agent.PromptRequest{
		UserParts:     []message.UIPart{message.UITextPart{Text: prompt}},
		Model:         req.Model,
		SubagentType:  req.SubagentType,
		ParentTaskID:  threadID,
		SubagentDepth: req.Depth,
	})

	select {
	case <-rec.done:
	case <-ctx.Done():
		rec.mu.Lock()
		cancel := rec.cancel
		rec.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return PromptTaskResult{}, ctx.Err()
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	return PromptTaskResult{
		TaskID:   rec.taskID,
		ThreadID: rec.subThreadID,
		Status:   rec.status,
		Output:   rec.output,
	}, nil
}

func preparePromptTaskThread(ctx context.Context, taskAgent PromptTaskAgent, req PromptTaskRequest, metadata thread.ConfigMetadata, created time.Time) (*taskRecord, error) {
	threadID := strings.TrimSpace(req.ThreadID)
	globalTasks.mu.Lock()
	rec := globalTasks.tasks[threadID]
	if rec == nil {
		rec = &taskRecord{
			taskID:         threadID,
			created:        created,
			parentThreadID: req.ParentThreadID,
			parentTaskID:   req.ParentTaskID,
			depth:          req.Depth,
			subThreadID:    threadID,
		}
		globalTasks.tasks[threadID] = rec
	}
	globalTasks.mu.Unlock()

	rec.mu.Lock()
	status := rec.status
	done := rec.done
	rec.mu.Unlock()
	if status == "in_progress" && done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if _, err := taskAgent.GetThreadInfo(threadID); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = taskThreadName(metadata)
		}
		if _, err := taskAgent.CreateThread(ctx, agent.CreateThreadRequest{
			ID:          threadID,
			Name:        name,
			CWD:         req.CWD,
			LastMessage: metadata.Prompt,
			Metadata:    metadata.RawMessage(),
		}); err != nil {
			globalTasks.mu.Lock()
			if globalTasks.tasks[threadID] == rec {
				delete(globalTasks.tasks, threadID)
			}
			globalTasks.mu.Unlock()
			return nil, err
		}
		return rec, nil
	}

	name := strings.TrimSpace(req.Name)
	update := agent.UpdateThreadRequest{
		LastMessage:       &metadata.Prompt,
		ClearErrorMessage: true,
		Metadata:          metadata.RawMessage(),
	}
	if name != "" {
		update.Name = &name
	}
	if cwd := strings.TrimSpace(req.CWD); cwd != "" {
		update.CWD = &cwd
	}
	if _, err := taskAgent.UpdateThread(ctx, threadID, update); err != nil {
		return nil, err
	}
	return rec, nil
}

func (e *Executor) executeTask(ctx context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	input.SubagentType = normalizeSubagentType(input.SubagentType)

	if resumeID := strings.TrimSpace(input.Resume); resumeID != "" {
		resumed, err := e.continueTask(ctx, toolCtx, call, marshalTaskContinuation(resumeID), nil)
		if err != nil {
			return thread.ToolExecuteResult{}, err
		}
		if input.RunInBackground && resumed.Async != nil {
			return backgroundTaskResult(call, resumeID, taskStatus(resumeID)), nil
		}
		return resumed, nil
	}

	prompt := taskPrompt(input)
	if prompt == "" {
		return errResult(call, "prompt or description is required for Task/Agent tool"), nil
	}

	if toolCtx == nil || toolCtx.Agent == nil {
		return errResult(call, "Task tool is not available: no sub-agent configured"), nil
	}
	if validator, ok := toolCtx.Agent.(subagentTypeValidator); ok {
		if err := validator.ValidateSubagentType(input.SubagentType); err != nil {
			return errResult(call, err.Error()), nil
		}
	}
	subAgent := toolCtx.Agent
	currentThreadID := contextThreadID(toolCtx, e.defaultThreadID)
	childDepth := toolCtx.SubagentDepth + 1
	if toolCtx.MaxSubagentDepth > 0 && childDepth > toolCtx.MaxSubagentDepth {
		return errResult(call, fmt.Sprintf("Task tool is not available: max sub-agent depth %d reached", toolCtx.MaxSubagentDepth)), nil
	}

	subThreadID, err := reserveSubThreadID(currentThreadID)
	if err != nil {
		return errResult(call, fmt.Sprintf("create sub-thread: %v", err)), nil
	}
	created := time.Now()
	metadata := thread.ConfigMetadata{
		Type:            "task",
		TaskID:          subThreadID,
		ParentThreadID:  currentThreadID,
		ParentTaskID:    toolCtx.CurrentTaskID,
		SubagentType:    input.SubagentType,
		Description:     input.Description,
		Prompt:          prompt,
		RunInBackground: input.RunInBackground,
		StartedAt:       created.UTC(),
	}
	rec := &taskRecord{
		taskID:         subThreadID,
		status:         "in_progress",
		created:        created,
		parentThreadID: currentThreadID,
		parentTaskID:   toolCtx.CurrentTaskID,
		depth:          childDepth,
		subThreadID:    subThreadID,
	}

	globalTasks.mu.Lock()
	if _, exists := globalTasks.tasks[subThreadID]; exists {
		globalTasks.mu.Unlock()
		return errResult(call, fmt.Sprintf("create sub-thread: duplicate sub-thread ID %s", subThreadID)), nil
	}
	globalTasks.tasks[subThreadID] = rec
	globalTasks.mu.Unlock()
	if _, err := bootstrapTaskThread(toolCtx, subThreadID, metadata); err != nil {
		globalTasks.mu.Lock()
		delete(globalTasks.tasks, subThreadID)
		globalTasks.mu.Unlock()
		return errResult(call, fmt.Sprintf("create sub-thread: %v", err)), nil
	}

	startSubAgentRun(rec, subAgent, subThreadID, subAgentPromptRequest(toolCtx, input, subThreadID, rec.depth))

	if input.RunInBackground {
		return backgroundTaskResult(call, rec.taskID, rec.status), nil
	}

	return thread.ToolExecuteResult{Async: taskHandle(call, rec, subThreadID)}, nil
}

func backgroundTaskResult(call message.ToolCallPart, taskID, status string) thread.ToolExecuteResult {
	return thread.ToolExecuteResult{
		Result: message.ToolResultPart{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Output: message.JSONOutput{Value: mustMarshalJSON(map[string]any{
				"task_id":   taskID,
				"thread_id": taskID,
				"status":    status,
			})},
		},
	}
}

func taskStatus(taskID string) string {
	globalTasks.mu.Lock()
	rec := globalTasks.tasks[taskID]
	globalTasks.mu.Unlock()
	if rec == nil {
		return "completed"
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if strings.TrimSpace(rec.status) == "" {
		return "completed"
	}
	return rec.status
}

// IsTaskRunning reports whether taskID is currently owned by an in-memory
// sub-agent task runner.
func IsTaskRunning(taskID string) bool {
	globalTasks.mu.Lock()
	rec := globalTasks.tasks[taskID]
	globalTasks.mu.Unlock()
	if rec == nil {
		return false
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.status == "in_progress"
}

func marshalTaskContinuation(subThreadID string) json.RawMessage {
	if subThreadID == "" {
		return nil
	}
	data, err := json.Marshal(taskContinuation{TaskID: subThreadID, SubThreadID: subThreadID})
	if err != nil {
		return nil
	}
	return data
}

func unmarshalTaskContinuation(data json.RawMessage) (string, error) {
	var cont taskContinuation
	if err := json.Unmarshal(data, &cont); err != nil {
		return "", fmt.Errorf("invalid task continuation: %w", err)
	}
	if cont.SubThreadID == "" {
		cont.SubThreadID = cont.TaskID
	}
	if cont.SubThreadID == "" {
		return "", fmt.Errorf("invalid task continuation: missing subThreadId")
	}
	return cont.SubThreadID, nil
}

// continueTask re-attaches to an in-flight or completed task after a crash or
// after a question answer arrives.
func (e *Executor) continueTask(_ context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart, continuation json.RawMessage, req *api.AnswerQuestionRequest) (thread.ToolExecuteResult, error) {
	subThreadID, err := unmarshalTaskContinuation(continuation)
	if err != nil {
		return thread.ToolExecuteResult{}, err
	}
	if toolCtx == nil || toolCtx.Agent == nil {
		return thread.ToolExecuteResult{
			Result: errorResult(call, fmt.Sprintf("sub-thread %s lost after crash (sub-agent not configured)", subThreadID)),
		}, nil
	}
	subAgent := toolCtx.Agent

	var input taskInput
	if req == nil {
		if err := unmarshalInput(call, &input); err != nil {
			return thread.ToolExecuteResult{
				Result: errorResult(call, fmt.Sprintf("sub-thread %s: cannot recover input after crash: %v", subThreadID, err)),
			}, nil
		}
		input.SubagentType = normalizeSubagentType(input.SubagentType)
	}

	// Fast path: sub-thread is still alive in memory.
	globalTasks.mu.Lock()
	rec, ok := globalTasks.tasks[subThreadID]
	globalTasks.mu.Unlock()
	if ok {
		if req != nil {
			rec.mu.Lock()
			status := rec.status
			subThreadID := rec.subThreadID
			approvalID := rec.pendingApprovalID
			rec.mu.Unlock()

			if status != "waiting_for_answer" {
				return thread.ToolExecuteResult{
					Result: errorResult(call, fmt.Sprintf("sub-thread %s is not waiting for an answer", subThreadID)),
				}, nil
			}
			if approvalID == "" {
				pending, err := subAgent.PendingQuestion(subThreadID)
				if err != nil {
					return thread.ToolExecuteResult{}, fmt.Errorf("load sub-agent pending question: %w", err)
				}
				if pending == nil {
					return thread.ToolExecuteResult{
						Result: errorResult(call, fmt.Sprintf("sub-thread %s has no pending question", subThreadID)),
					}, nil
				}
				approvalID = pending.ApprovalID
			}
			if err := subAgent.SubmitAnswer(subThreadID, approvalID, *req); err != nil {
				return thread.ToolExecuteResult{}, fmt.Errorf("submit sub-agent answer: %w", err)
			}
			startSubAgentRun(rec, subAgent, subThreadID, agent.PromptRequest{ParentTaskID: subThreadID, SubagentDepth: rec.depth})
			return thread.ToolExecuteResult{Async: taskHandle(call, rec, subThreadID)}, nil
		}
		rec.mu.Lock()
		status := rec.status
		depth := rec.depth
		rec.mu.Unlock()
		if taskIsTerminal(status) && taskPrompt(input) != "" {
			if depth == 0 {
				depth = subagentDepthFromThreadID(subThreadID)
			}
			rec.mu.Lock()
			rec.depth = depth
			rec.mu.Unlock()
			startSubAgentRun(rec, subAgent, subThreadID, subAgentPromptRequest(toolCtx, input, subThreadID, depth))
		}
		return thread.ToolExecuteResult{Async: taskHandle(call, rec, subThreadID)}, nil
	}

	// Check whether the sub-agent already completed before the crash.
	if output, err := subAgent.FinalResponse(subThreadID); err == nil && output != "" && taskPrompt(input) == "" {
		return thread.ToolExecuteResult{
			Result: message.ToolResultPart{
				ToolCallID: call.ToolCallID,
				ToolName:   call.ToolName,
				Output:     message.TextOutput{Value: output},
			},
		}, nil
	}

	if req != nil {
		pending, err := subAgent.PendingQuestion(subThreadID)
		if err != nil {
			return thread.ToolExecuteResult{}, fmt.Errorf("load sub-agent pending question: %w", err)
		}
		if pending == nil {
			return thread.ToolExecuteResult{
				Result: errorResult(call, fmt.Sprintf("sub-thread %s has no pending question", subThreadID)),
			}, nil
		}

		rec = &taskRecord{
			taskID:         subThreadID,
			status:         "in_progress",
			created:        time.Now(),
			parentThreadID: contextThreadID(toolCtx, e.defaultThreadID),
			parentTaskID:   toolCtx.CurrentTaskID,
			depth:          subagentDepthFromThreadID(subThreadID),
			subThreadID:    subThreadID,
		}
		globalTasks.mu.Lock()
		globalTasks.tasks[subThreadID] = rec
		globalTasks.mu.Unlock()

		if err := subAgent.SubmitAnswer(subThreadID, pending.ApprovalID, *req); err != nil {
			return thread.ToolExecuteResult{}, fmt.Errorf("submit sub-agent answer: %w", err)
		}
		startSubAgentRun(rec, subAgent, subThreadID, agent.PromptRequest{ParentTaskID: subThreadID, SubagentDepth: rec.depth})
		return thread.ToolExecuteResult{Async: taskHandle(call, rec, subThreadID)}, nil
	}

	// No live task record exists. Add the supplied prompt to the existing
	// sub-thread; DefaultAgent.Prompt resumes interrupted turns when needed and
	// otherwise appends a fresh user prompt to cancelled or completed threads.

	rec = &taskRecord{
		taskID:         subThreadID,
		status:         "in_progress",
		created:        time.Now(),
		parentThreadID: contextThreadID(toolCtx, e.defaultThreadID),
		parentTaskID:   toolCtx.CurrentTaskID,
		depth:          subagentDepthFromThreadID(subThreadID),
		subThreadID:    subThreadID,
	}
	globalTasks.mu.Lock()
	globalTasks.tasks[subThreadID] = rec
	globalTasks.mu.Unlock()

	startSubAgentRun(rec, subAgent, subThreadID, subAgentPromptRequest(toolCtx, input, subThreadID, rec.depth))

	return thread.ToolExecuteResult{Async: taskHandle(call, rec, subThreadID)}, nil
}

func bootstrapTaskThread(toolCtx *thread.ToolContext, threadID string, metadata thread.ConfigMetadata) (agent.ThreadInfo, error) {
	if toolCtx == nil || toolCtx.Agent == nil {
		return agent.ThreadInfo{}, fmt.Errorf("no sub-agent configured")
	}
	info, err := toolCtx.Agent.CreateThread(context.Background(), agent.CreateThreadRequest{
		ID:          threadID,
		Name:        taskThreadName(metadata),
		LastMessage: metadata.Prompt,
		Metadata:    metadata.RawMessage(),
	})
	if err != nil {
		return agent.ThreadInfo{}, err
	}
	if toolCtx.EmitChunk != nil {
		toolCtx.EmitChunk(threadUpdateChunkFromInfo(info), nil)
	}
	return info, nil
}

func persistTaskThreadError(subAgent PromptTaskAgent, threadID, message string) {
	trimmed := strings.TrimSpace(message)
	_, _ = subAgent.UpdateThread(context.Background(), threadID, agent.UpdateThreadRequest{ErrorMessage: &trimmed})
}

func threadUpdateChunkFromInfo(info agent.ThreadInfo) message.ThreadUpdateChunk {
	return message.ThreadUpdateChunk{
		Data: message.ThreadUpdateData{
			Thread: message.ThreadUpdateInfo{
				ID:           info.ID,
				Name:         info.Name,
				CWD:          info.CWD,
				LastMessage:  info.LastMessage,
				ErrorMessage: info.ErrorMessage,
				Model:        info.Model,
				Reasoning:    info.Reasoning,
				ServiceTier:  info.ServiceTier,
				State:        string(info.State),
				TokenUsage: message.TokenUsageInfo{
					Total:           info.TokenUsage.Total,
					LastStep:        info.TokenUsage.LastStep,
					LastTurn:        info.TokenUsage.LastTurn,
					ModelMaxTokens:  info.TokenUsage.ModelMaxTokens,
					MaxOutputTokens: info.TokenUsage.MaxOutputTokens,
					Prices:          info.TokenUsage.Prices,
				},
				ActiveCommand: info.ActiveCommand,
				Metadata:      info.Metadata,
			},
		},
	}
}

func taskThreadName(metadata thread.ConfigMetadata) string {
	if title := strings.TrimSpace(metadata.Description); title != "" {
		return title
	}
	if prompt := strings.TrimSpace(metadata.Prompt); prompt != "" {
		return prompt
	}
	return metadata.TaskID
}

func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func startSubAgentRun(rec *taskRecord, subAgent PromptTaskAgent, subThreadID string, req agent.PromptRequest) {
	// Create the context before starting the goroutine so rec.cancel is always
	// set by the time taskHandle.Wait can observe it — no race on cancellation.
	subCtx, cancel := context.WithCancel(context.Background())

	rec.mu.Lock()
	rec.status = "in_progress"
	rec.output = ""
	rec.subThreadID = subThreadID
	rec.pendingApprovalID = ""
	rec.pendingQuestions = nil
	rec.done = make(chan struct{})
	rec.cancel = cancel
	rec.mu.Unlock()

	go runSubAgentGoroutine(subCtx, rec, subAgent, subThreadID, req)
}

// runSubAgentGoroutine runs a full agent turn loop for a sub-agent task.
// ctx is created by the caller before the goroutine starts so rec.cancel is
// always populated by the time taskHandle.Wait can fire.
func runSubAgentGoroutine(ctx context.Context, rec *taskRecord, subAgent PromptTaskAgent, subThreadID string, req agent.PromptRequest) {
	defer close(rec.done)
	defer rec.cancel()

	runErr := drainSubAgentRun(ctx, subAgent, subThreadID, req)

	var finalOutput string
	if ctx.Err() != nil {
		if output, err := subAgent.FinalResponse(subThreadID); err == nil {
			finalOutput = output
		}
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if ctx.Err() != nil {
		if rec.status != "failed" {
			rec.status = "cancelled"
		}
		if rec.output == "" {
			rec.output = finalOutput
		}
		persistTaskThreadError(subAgent, subThreadID, "")
		return
	}

	if runErr != nil {
		rec.status = "failed"
		rec.output = fmt.Sprintf("sub-agent failed: %v", runErr)
		persistTaskThreadError(subAgent, subThreadID, rec.output)
		return
	}

	pending, err := subAgent.PendingQuestion(subThreadID)
	if err != nil {
		rec.status = "failed"
		rec.output = fmt.Sprintf("sub-agent question lookup failed: %v", err)
		persistTaskThreadError(subAgent, subThreadID, rec.output)
		return
	}
	if pending != nil {
		questions, err := json.Marshal(pending.Questions)
		if err != nil {
			rec.status = "failed"
			rec.output = fmt.Sprintf("sub-agent question marshal failed: %v", err)
			persistTaskThreadError(subAgent, subThreadID, rec.output)
			return
		}
		credentials, err := json.Marshal(pending.Credentials)
		if err != nil {
			rec.status = "failed"
			rec.output = fmt.Sprintf("sub-agent credential marshal failed: %v", err)
			persistTaskThreadError(subAgent, subThreadID, rec.output)
			return
		}
		rec.status = "waiting_for_answer"
		rec.pendingApprovalID = pending.ApprovalID
		rec.pendingQuestions = questions
		rec.pendingCredentials = credentials
		rec.output = ""
		return
	}

	// Read the final assistant text from the completed thread.
	output, err := subAgent.FinalResponse(subThreadID)
	if err != nil {
		rec.status = "failed"
		rec.output = fmt.Sprintf("sub-agent completed but result unavailable: %v", err)
		persistTaskThreadError(subAgent, subThreadID, rec.output)
		return
	}

	rec.status = "completed"
	rec.output = output
	persistTaskThreadError(subAgent, subThreadID, "")
}

func drainSubAgentRun(ctx context.Context, subAgent PromptTaskAgent, subThreadID string, req agent.PromptRequest) error {
	if req.UserParts == nil {
		return drainSubAgentResume(ctx, subAgent, subThreadID, req)
	}
	if interrupted, err := subAgent.HasInterruptedTurn(subThreadID); err == nil && interrupted {
		return drainSubAgentResume(ctx, subAgent, subThreadID, req)
	}
	for _, err := range subAgent.Prompt(ctx, subThreadID, req) {
		if err == nil {
			continue
		}
		if errors.Is(err, agent.ErrInterruptedTurnRequiresResume) || errors.Is(err, agent.ErrPendingQuestionRequiresAnswer) {
			return drainSubAgentResume(ctx, subAgent, subThreadID, req)
		}
		return err
	}
	return nil
}

func drainSubAgentResume(ctx context.Context, subAgent PromptTaskAgent, subThreadID string, req agent.PromptRequest) error {
	resumeReq := req
	resumeReq.UserParts = nil
	result, err := subAgent.Resume(ctx, subThreadID, resumeReq)
	if err != nil {
		return err
	}
	return drainMessageStream(result.Stream)
}

func drainMessageStream(seq iter.Seq2[message.MessageChunk, error]) error {
	for _, err := range seq {
		if err != nil {
			return err
		}
	}
	return nil
}

// taskHandle builds the AsyncContinuationHandle that the turn loop waits on.
func taskHandle(call message.ToolCallPart, rec *taskRecord, subThreadID string) *thread.AsyncContinuationHandle {
	continuation := marshalTaskContinuation(subThreadID)
	return &thread.AsyncContinuationHandle{
		Continuation: continuation,
		Wait: func(ctx context.Context) (thread.AsyncWaitResult, error) {
			select {
			case <-rec.done:
			case <-ctx.Done():
				// The parent turn stopped waiting, but the sub-agent task is an
				// independent child run. Leave it alive so TaskOutput or a later
				// continuation can collect the result.
				return thread.AsyncWaitResult{
					Result: errorResult(call, "task cancelled"),
				}, nil
			}
			rec.mu.Lock()
			defer rec.mu.Unlock()
			if rec.status == "failed" {
				return thread.AsyncWaitResult{
					Result: errorResult(call, rec.output),
				}, nil
			}
			if rec.status == "waiting_for_answer" {
				return thread.AsyncWaitResult{
					Approval: &thread.ApprovalRequest{
						Questions:    rec.pendingQuestions,
						Continuation: continuation,
						Credentials:  rec.pendingCredentials,
					},
				}, nil
			}
			return thread.AsyncWaitResult{
				Result: message.ToolResultPart{
					ToolCallID: call.ToolCallID,
					ToolName:   call.ToolName,
					Output:     message.TextOutput{Value: taskOutputText(subThreadID, rec.status, rec.output)},
				},
			}, nil
		},
	}
}

// --- TodoWrite ---

type todoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

type todoWriteInput struct {
	Todos []todoItem `json:"todos"`
}

func renderTodoWriteSummary(todos []todoItem) string {
	completed := 0
	inProgress := 0
	pending := 0

	for _, todo := range todos {
		switch todo.Status {
		case "completed":
			completed++
		case "in_progress":
			inProgress++
		default:
			pending++
		}
	}

	var b strings.Builder
	b.WriteString("Todo list updated.\n\n")
	if len(todos) == 0 {
		b.WriteString("Current status is empty.")
		return b.String()
	}

	fmt.Fprintf(&b, "Current status is %d completed, %d in progress, and %d pending.\n\n",
		completed,
		inProgress,
		pending)
	b.WriteString("### Current tasks\n")
	for _, todo := range todos {
		content := strings.TrimSpace(todo.Content)
		if content == "" {
			content = "Untitled task"
		}

		switch todo.Status {
		case "completed":
			b.WriteString("- [x] ")
			b.WriteString(content)
		case "in_progress":
			b.WriteString("- [ ] ")
			b.WriteString(content)
			activeForm := strings.TrimSpace(todo.ActiveForm)
			if activeForm != "" && activeForm != content {
				b.WriteString(" _(in progress: ")
				b.WriteString(activeForm)
				b.WriteString(")_")
			} else {
				b.WriteString(" _(in progress)_")
			}
		default:
			b.WriteString("- [ ] ")
			b.WriteString(content)
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func (e *Executor) executeTodoWrite(_ context.Context, _ *thread.ToolContext, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input todoWriteInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}

	return textResult(call, renderTodoWriteSummary(input.Todos)), nil
}

// --- TaskOutput ---

const taskOutputDefaultTimeout = 30 * time.Second

type taskOutputInput struct {
	TaskID  string `json:"task_id"`
	Block   bool   `json:"block"`
	Timeout int    `json:"timeout"`
}

func taskOutputWaitTimeout(input int) time.Duration {
	if input <= 0 {
		return taskOutputDefaultTimeout
	}
	return time.Duration(min(input, 600_000)) * time.Millisecond
}

func taskEmptyOutputMessage(taskID, status string) string {
	switch status {
	case "cancelled":
		return fmt.Sprintf("Task %s was cancelled before producing a final response.", taskID)
	case "completed":
		return fmt.Sprintf("Task %s completed without a final response.", taskID)
	case "failed":
		return fmt.Sprintf("Task %s failed without a final response.", taskID)
	default:
		return "(no output yet)"
	}
}

func taskOutputText(taskID, status, output string) string {
	if output != "" {
		return output
	}
	return taskEmptyOutputMessage(taskID, status)
}

func (e *Executor) executeTaskOutput(toolCtx *thread.ToolContext, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskOutputInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.TaskID == "" {
		return errResult(call, "task_id is required"), nil
	}

	globalTasks.mu.Lock()
	rec, ok := globalTasks.tasks[input.TaskID]
	globalTasks.mu.Unlock()

	if !ok {
		return taskOutputFromPersistedThread(toolCtx, call, input.TaskID)
	}

	if input.Block {
		timeout := taskOutputWaitTimeout(input.Timeout)
		select {
		case <-rec.done:
		case <-time.After(timeout):
		}
	}

	rec.mu.Lock()
	status := rec.status
	output := rec.output
	rec.mu.Unlock()

	return textResult(call, taskOutputText(input.TaskID, status, output)), nil
}

func taskOutputFromPersistedThread(toolCtx *thread.ToolContext, call message.ToolCallPart, taskID string) (thread.ToolExecuteResult, error) {
	if toolCtx == nil || toolCtx.Agent == nil {
		return errResult(call, fmt.Sprintf("task %s not found", taskID)), nil
	}

	subAgent := toolCtx.Agent
	if output, err := subAgent.FinalResponse(taskID); err == nil && strings.TrimSpace(output) != "" {
		return textResult(call, output), nil
	}

	info, err := subAgent.GetThreadInfo(taskID)
	if err != nil || !isTaskThreadInfo(info, taskID) {
		return errToolResult(call, fmt.Sprintf("task %s not found", taskID))
	}

	status := persistedTaskStatus(info)
	return textResult(call, fmt.Sprintf(
		"Task %s was found on disk as thread %s, but it is not active in memory after restart. Persisted status: %s.\n\nUse Task with resume %q to continue it, or inspect the persisted thread transcript for details.",
		taskID,
		info.ID,
		status,
		taskID,
	)), nil
}

func isTaskThreadInfo(info agent.ThreadInfo, taskID string) bool {
	if strings.TrimSpace(info.ID) != taskID {
		return false
	}
	if strings.Contains(taskID, ".sub.") {
		return true
	}
	var metadata thread.ConfigMetadata
	if len(info.Metadata) > 0 && json.Unmarshal(info.Metadata, &metadata) == nil {
		return metadata.Type == "task" || metadata.TaskID == taskID
	}
	return false
}

func persistedTaskStatus(info agent.ThreadInfo) string {
	switch {
	case info.PendingQuestion:
		return "waiting_for_answer"
	case info.State == agent.ThreadStateInterrupted:
		return "interrupted"
	case info.State == agent.ThreadStateCancelled:
		return "cancelled"
	case strings.TrimSpace(info.ErrorMessage) != "":
		return "failed"
	default:
		return "in_progress"
	}
}

// --- TaskStop ---

type taskStopInput struct {
	TaskID  string `json:"task_id"`
	ShellID string `json:"shell_id"`
}

func (e *Executor) executeTaskStop(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskStopInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.TaskID == "" {
		input.TaskID = input.ShellID
	}
	if input.TaskID == "" {
		return errResult(call, "task_id is required"), nil
	}

	globalTasks.mu.Lock()
	rec, ok := globalTasks.tasks[input.TaskID]
	globalTasks.mu.Unlock()

	if !ok {
		return errResult(call, fmt.Sprintf("task %s not found", input.TaskID)), nil
	}

	rec.mu.Lock()
	if rec.status != "completed" && rec.status != "failed" {
		rec.status = "failed"
		rec.output += "\n[Task stopped by agent]"
		cancel := rec.cancel
		rec.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	} else {
		rec.mu.Unlock()
	}

	return textResult(call, fmt.Sprintf("Task %s stopped", input.TaskID)), nil
}
