package thread

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

// TurnPhase represents the phase of an active turn's lifecycle.
type TurnPhase string

const (
	PhaseStreaming        TurnPhase = "streaming"
	PhaseTools            TurnPhase = "tools"
	PhaseWaitingForAsync  TurnPhase = "waiting_for_async"
	PhaseWaitingForAnswer TurnPhase = "waiting_for_answer"
)

// TurnState is the on-disk record of an active turn.
// It exists in {threadID}/turn.json only while a turn is running.
// On clean completion, it is deleted.
type TurnState struct {
	ID                string         `json:"id"`
	ThreadID          string         `json:"threadId"`
	LeafID            string         `json:"leafId"`
	Config            TurnConfig     `json:"config"`
	CurrentStep       int            `json:"currentStep"`
	Phase             TurnPhase      `json:"phase"`
	TokenUsage        TokenUsageInfo `json:"tokenUsage,omitzero"`
	LeafMsgID         string         `json:"leafMsgId"`                   // updated as messages are saved
	AssistantMsgID    string         `json:"assistantMsgId"`              // pre-generated ID for the first assistant message
	PendingApprovalID string         `json:"pendingApprovalId,omitempty"` // tool call ID of the pending approval request
	StartedAt         *time.Time     `json:"startedAt,omitempty"`
	UpdatedAt         *time.Time     `json:"updatedAt,omitempty"`
	FinishedAt        *time.Time     `json:"finishedAt,omitempty"`
}

// TokenUsageInfo is the persisted usage summary for a step, turn, or thread.
type TokenUsageInfo struct {
	Total           message.Usage       `json:"total,omitzero"`
	LastStep        message.Usage       `json:"lastStep,omitzero"`
	LastTurn        message.Usage       `json:"lastTurn,omitzero"`
	ModelMaxTokens  int                 `json:"modelMaxTokens,omitempty"`
	MaxOutputTokens int                 `json:"maxOutputTokens,omitempty"`
	Prices          message.TokenPrices `json:"prices,omitzero"`
}

func (u TokenUsageInfo) IsZero() bool {
	return u.Total.IsZero() &&
		u.LastStep.IsZero() &&
		u.LastTurn.IsZero() &&
		u.ModelMaxTokens == 0 &&
		u.MaxOutputTokens == 0 &&
		u.Prices.IsZero()
}

// PendingQuestionState persists a pending approval to disk.
// Stored at {turnDir}/approve-{approvalId}.json while the turn is paused waiting for user input.
// ApprovalID identifies one specific approval prompt; ToolCallID identifies the
// underlying paused tool invocation. A single tool call may emit multiple
// approval prompts over time, each with its own ApprovalID.
type PendingQuestionState struct {
	ApprovalID   string          `json:"approvalId"`
	ToolCallID   string          `json:"toolCallId"`
	StepIndex    int             `json:"stepIndex"`
	ResumePhase  TurnPhase       `json:"resumePhase,omitempty"`  // phase to resume after this approval is answered
	Continuation json.RawMessage `json:"continuation,omitempty"` // executor-owned continuation payload for resuming this approval
	Questions    json.RawMessage `json:"questions,omitempty"`    // raw JSON array from tool input
	Credentials  json.RawMessage `json:"credentials,omitempty"`  // raw JSON array from tool input
	Metadata     json.RawMessage `json:"metadata,omitempty"`     // tool-specific approval metadata
	Context      string          `json:"context,omitempty"`
}

// QuestionAnswer persists the user's response to a pending approval.
type QuestionAnswer struct {
	ApprovalID  string            `json:"approvalId"`
	Answers     map[string]string `json:"answers,omitempty"`
	Credentials map[string]string `json:"credentials,omitempty"`
}

// StepAsyncContinuations holds executor-owned continuation metadata for async
// tool work that is still in progress for a step.
// Persisted to step-NNN-async.json so those continuations can be resumed after
// a crash.
type StepAsyncContinuations struct {
	Continuations []AsyncContinuationInfo `json:"continuations"`
}

// StepEventMessages tracks immutable message IDs written for a step after the
// assistant message is persisted. These are typically approval request,
// approval response, and tool result messages in execution order.
type StepEventMessages struct {
	MessageIDs []string `json:"messageIds"`
}

// BrowserEvent records one browser CDP request/response pair attributed to a
// specific turn step without affecting approval or turn phase state.
type BrowserEvent struct {
	EventID    string             `json:"eventId"`
	StepIndex  int                `json:"stepIndex"`
	RequestID  string             `json:"requestId,omitempty"`
	Method     string             `json:"method,omitempty"`
	Direction  string             `json:"direction"`
	Payload    json.RawMessage    `json:"payload,omitempty"`
	Files      []BrowserEventFile `json:"files,omitempty"`
	RecordedAt *time.Time         `json:"recordedAt,omitempty"`
}

// BrowserEventFile references one browser artifact saved alongside turn state.
type BrowserEventFile struct {
	Path      string `json:"path"`
	URI       string `json:"uri,omitempty"`
	MediaType string `json:"mediaType"`
	Filename  string `json:"filename,omitempty"`
}

// BrowserEventEntry binds a browser event to the assistant message it belongs
// to so the UI can replay and render browser activity alongside the turn.
type BrowserEventEntry struct {
	TurnID             string       `json:"turnId"`
	AssistantMessageID string       `json:"assistantMessageId,omitempty"`
	StepIndex          int          `json:"stepIndex"`
	Event              BrowserEvent `json:"event"`
}

// AsyncContinuationInfo identifies one persisted async continuation owned by a
// tool executor.
type AsyncContinuationInfo struct {
	ToolCallID   string          `json:"toolCallId"`
	ToolName     string          `json:"toolName"`
	Continuation json.RawMessage `json:"continuation,omitempty"` // executor-owned continuation payload for resuming async work
	Input        string          `json:"input"`                  // original tool input JSON, passed back to Continue
}

// StepResult is written after a step's streaming completes.
// It contains the accumulated assistant Message and the
// list of tool calls that need execution.
type StepResult struct {
	AssistantMessageID string          `json:"assistantMessageId,omitempty"`
	ReplacesMessageID  string          `json:"replacesMessageId,omitempty"`
	AssistantMessage   message.Message `json:"assistantMessage"`
	ToolCalls          []ToolCallInfo  `json:"toolCalls,omitempty"`
	Usage              message.Usage   `json:"usage,omitzero"`
}

// ToolCallInfo identifies a tool call extracted from an assistant message.
type ToolCallInfo struct {
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Input      string `json:"input"`
}

// StepToolResults holds the tool results accumulated so far for a step.
// It is overwritten after each tool completes, growing the Results slice.
// Custom JSON handling because ToolResultPart.Output uses json:"-".
type StepToolResults struct {
	Results []message.ToolResultPart
}

func (s StepToolResults) MarshalJSON() ([]byte, error) {
	items := make([]json.RawMessage, len(s.Results))
	for i, r := range s.Results {
		var outputData json.RawMessage
		if r.Output != nil {
			var err error
			outputData, err = message.MarshalToolResultOutput(r.Output)
			if err != nil {
				return nil, fmt.Errorf("marshal tool result %d output: %w", i, err)
			}
		}
		data, err := json.Marshal(struct {
			ToolCallID string          `json:"toolCallId"`
			ToolName   string          `json:"toolName"`
			Output     json.RawMessage `json:"output,omitempty"`
		}{r.ToolCallID, r.ToolName, outputData})
		if err != nil {
			return nil, err
		}
		items[i] = data
	}
	return json.Marshal(struct {
		Results []json.RawMessage `json:"results"`
	}{items})
}

func (s *StepToolResults) UnmarshalJSON(data []byte) error {
	var raw struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Results = make([]message.ToolResultPart, len(raw.Results))
	for i, itemData := range raw.Results {
		var item struct {
			ToolCallID string          `json:"toolCallId"`
			ToolName   string          `json:"toolName"`
			Output     json.RawMessage `json:"output"`
		}
		if err := json.Unmarshal(itemData, &item); err != nil {
			return fmt.Errorf("unmarshal tool result %d: %w", i, err)
		}
		var output message.ToolResultOutput
		if len(item.Output) > 0 {
			var err error
			output, err = message.UnmarshalToolResultOutput(item.Output)
			if err != nil {
				return fmt.Errorf("unmarshal tool result %d output: %w", i, err)
			}
		}
		s.Results[i] = message.ToolResultPart{
			ToolCallID: item.ToolCallID,
			ToolName:   item.ToolName,
			Output:     output,
		}
	}
	return nil
}
