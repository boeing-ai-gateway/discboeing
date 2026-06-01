package sync

import (
	"encoding/json"
	"testing"

	agentmessage "github.com/obot-platform/discobot/agent-go/message"
	serverapi "github.com/obot-platform/discobot/server/api"
)

func TestApplyChunkBuildsReasoningAndMetadata(t *testing.T) {
	messages := applyChunks(nil,
		agentmessage.StartChunk{MessageID: "assistant-1", MessageMetadata: json.RawMessage(`{"model":"gpt"}`)},
		agentmessage.ReasoningStartChunk{ID: "reasoning-1"},
		agentmessage.ReasoningDeltaChunk{ID: "reasoning-1", Delta: "think"},
		agentmessage.ReasoningDeltaChunk{ID: "reasoning-1", Delta: "ing"},
		agentmessage.ReasoningEndChunk{ID: "reasoning-1"},
		agentmessage.MessageMetadataChunk{MessageMetadata: json.RawMessage(`{"usage":{"input":1}}`)},
		agentmessage.ResponseFinishChunk{},
	)

	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	part, ok := messages[0].Parts[0].(agentmessage.UIReasoningPart)
	if !ok {
		t.Fatalf("part = %T, want UIReasoningPart", messages[0].Parts[0])
	}
	if part.Text != "thinking" || part.State != "done" {
		t.Fatalf("reasoning part = %#v, want done thinking", part)
	}
	assertJSONEqual(t, messages[0].Metadata, `{"model":"gpt","usage":{"input":1}}`)
}

func TestApplyChunkStreamsToolInputAndOutput(t *testing.T) {
	messages := applyChunks(nil,
		agentmessage.StartChunk{MessageID: "assistant-1"},
		agentmessage.ToolInputStartChunk{ToolCallID: "tool-1", ToolName: "Read", Title: "Read file"},
		agentmessage.ToolInputDeltaChunk{ToolCallID: "tool-1", InputTextDelta: `{"path"`},
		agentmessage.ToolInputDeltaChunk{ToolCallID: "tool-1", InputTextDelta: `:"README.md"}`},
		agentmessage.ToolInputAvailableChunk{ToolCallID: "tool-1", ToolName: "Read", Input: json.RawMessage(`{"path":"README.md"}`)},
		agentmessage.ToolOutputAvailableChunk{ToolCallID: "tool-1", Output: json.RawMessage(`"contents"`)},
	)

	part := toolPart(t, messages[0], "tool-1")
	if part.ToolName != "Read" || part.Title != "Read file" || part.State != "output-available" {
		t.Fatalf("tool part = %#v, want Read output-available", part)
	}
	assertJSONEqual(t, part.Input, `{"path":"README.md"}`)
	assertJSONEqual(t, part.Output, `"contents"`)
}

func TestApplyChunkHandlesToolInputErrorAndOutputError(t *testing.T) {
	messages := applyChunks(nil,
		agentmessage.StartChunk{MessageID: "assistant-1"},
		agentmessage.ToolInputErrorChunk{ToolCallID: "tool-1", ToolName: "Read", Input: json.RawMessage(`{"path":1}`), ErrorText: "bad input"},
		agentmessage.ToolOutputErrorChunk{ToolCallID: "tool-1", ErrorText: "failed"},
	)

	part := toolPart(t, messages[0], "tool-1")
	if part.State != "output-error" || part.ErrorText != "failed" {
		t.Fatalf("tool part = %#v, want output-error failed", part)
	}
	assertJSONEqual(t, part.Input, `{"path":1}`)
}

func TestApplyChunkHandlesToolApprovalAndDenial(t *testing.T) {
	messages := applyChunks(nil,
		agentmessage.StartChunk{MessageID: "assistant-1"},
		agentmessage.ToolInputAvailableChunk{ToolCallID: "tool-1", ToolName: "Bash", Input: json.RawMessage(`{"command":"rm"}`)},
		agentmessage.ToolApprovalRequestChunk{ToolCallID: "tool-1", ApprovalID: "approval-1"},
		agentmessage.ToolApprovalResponseDataChunk{Data: agentmessage.ToolApprovalResponseData{ApprovalID: "approval-1", Approved: false, Reason: "dangerous"}},
		agentmessage.ToolOutputDeniedChunk{ToolCallID: "tool-1"},
	)

	part := toolPart(t, messages[0], "tool-1")
	if part.State != "output-denied" {
		t.Fatalf("tool state = %q, want output-denied", part.State)
	}
	if part.Approval == nil || part.Approval.ID != "approval-1" || part.Approval.Approved == nil || *part.Approval.Approved || part.Approval.Reason != "dangerous" {
		t.Fatalf("approval = %#v, want denied approval", part.Approval)
	}
}

func TestApplyChunkStartReplacesExistingAssistantMessage(t *testing.T) {
	messages := []serverapi.Message{
		{
			ID:       "assistant-existing",
			Role:     "assistant",
			Metadata: json.RawMessage(`{"step":1}`),
			Parts: []agentmessage.UIPart{
				agentmessage.UITextPart{Type: "text", Text: "stale reply", State: "done"},
			},
		},
	}

	messages = applyChunks(messages,
		agentmessage.StartChunk{MessageID: "assistant-existing"},
		agentmessage.TextStartChunk{ID: "text-1"},
		agentmessage.TextDeltaChunk{ID: "text-1", Delta: "fresh reply"},
		agentmessage.TextEndChunk{ID: "text-1"},
		agentmessage.ResponseFinishChunk{},
	)

	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	if messages[0].ID != "assistant-existing" || string(messages[0].Metadata) != "" {
		t.Fatalf("message = %#v, want replacement without metadata", messages[0])
	}
	if messageText(messages[0]) != "fresh reply" {
		t.Fatalf("message text = %q, want fresh reply", messageText(messages[0]))
	}
}

func TestApplyChunkResumeAppendsToExistingAssistantMessage(t *testing.T) {
	messages := []serverapi.Message{testMessage("assistant-existing", "assistant", "partial reply")}
	messages = applyChunks(messages,
		agentmessage.ThreadResumeChunk{Data: agentmessage.ThreadResumeData{ThreadID: "thread-1", MessageID: "assistant-existing"}},
		agentmessage.TextStartChunk{ID: "text-2"},
		agentmessage.TextDeltaChunk{ID: "text-2", Delta: "continued reply"},
		agentmessage.TextEndChunk{ID: "text-2"},
		agentmessage.ResponseFinishChunk{},
	)

	if len(messages) != 1 || messages[0].ID != "assistant-existing" {
		t.Fatalf("messages = %#v, want single resumed assistant message", messages)
	}
	if len(messages[0].Parts) != 2 {
		t.Fatalf("parts len = %d, want 2", len(messages[0].Parts))
	}
	first, ok := messages[0].Parts[0].(agentmessage.UITextPart)
	if !ok || first.Text != "partial reply" || first.State != "done" {
		t.Fatalf("first part = %#v, want done partial reply", messages[0].Parts[0])
	}
	second, ok := messages[0].Parts[1].(agentmessage.UITextPart)
	if !ok || second.Text != "continued reply" || second.State != "done" {
		t.Fatalf("second part = %#v, want done continued reply", messages[0].Parts[1])
	}
}

func TestApplyChunkResumedToolInputDeltasCreateToolPart(t *testing.T) {
	messages := []serverapi.Message{testMessage("assistant-resume-tool", "assistant", "working")}
	messages = applyChunks(messages,
		agentmessage.ThreadResumeChunk{Data: agentmessage.ThreadResumeData{ThreadID: "thread-1", MessageID: "assistant-resume-tool"}},
		agentmessage.ToolInputDeltaChunk{ToolCallID: "tool-resume-1", InputTextDelta: `{"questions":`},
		agentmessage.ToolInputDeltaChunk{ToolCallID: "tool-resume-1", InputTextDelta: `[{"header":"Scope","question":"Proceed?","multiSelect":false,"options":[{"label":"Yes","description":"Continue"}]}]}`},
		agentmessage.ToolInputAvailableChunk{ToolCallID: "tool-resume-1", ToolName: "AskUserQuestion", Input: json.RawMessage(`{"questions":[{"header":"Scope","question":"Proceed?","multiSelect":false,"options":[{"label":"Yes","description":"Continue"}]}]}`)},
		agentmessage.ToolApprovalRequestChunk{ToolCallID: "tool-resume-1", ApprovalID: "approval-resume-1"},
	)

	if len(messages) != 1 || len(messages[0].Parts) != 2 {
		t.Fatalf("message = %#v, want text plus dynamic tool", messages)
	}
	part := toolPart(t, messages[0], "tool-resume-1")
	if part.ToolName != "AskUserQuestion" || part.State != "approval-requested" {
		t.Fatalf("tool part = %#v, want AskUserQuestion approval-requested", part)
	}
	if part.Approval == nil || part.Approval.ID != "approval-resume-1" {
		t.Fatalf("approval = %#v, want approval-resume-1", part.Approval)
	}
	assertJSONEqual(t, part.Input, `{"questions":[{"header":"Scope","question":"Proceed?","multiSelect":false,"options":[{"label":"Yes","description":"Continue"}]}]}`)
}

func TestApplyChunkFinishFinalizesStreamingTextAndReasoningParts(t *testing.T) {
	messages := applyChunks(nil,
		agentmessage.StartChunk{MessageID: "assistant-1"},
		agentmessage.TextStartChunk{ID: "text-1"},
		agentmessage.TextDeltaChunk{ID: "text-1", Delta: "unfinished text"},
		agentmessage.ReasoningStartChunk{ID: "reason-1"},
		agentmessage.ReasoningDeltaChunk{ID: "reason-1", Delta: "unfinished reasoning"},
		agentmessage.ResponseFinishChunk{},
	)

	if len(messages[0].Parts) != 2 {
		t.Fatalf("parts len = %d, want 2", len(messages[0].Parts))
	}
	text, ok := messages[0].Parts[0].(agentmessage.UITextPart)
	if !ok || text.State != "done" {
		t.Fatalf("text part = %#v, want done", messages[0].Parts[0])
	}
	reasoning, ok := messages[0].Parts[1].(agentmessage.UIReasoningPart)
	if !ok || reasoning.State != "done" {
		t.Fatalf("reasoning part = %#v, want done", messages[0].Parts[1])
	}
}

func TestApplyChunkDataAndErrorChunksDoNotMutateMessages(t *testing.T) {
	messages := []serverapi.Message{testMessage("assistant-1", "assistant", "hello")}
	snapshot := cloneMessagesForTest(messages)
	messages = applyChunks(messages,
		agentmessage.ThreadUpdateChunk{Data: agentmessage.ThreadUpdateData{Thread: agentmessage.ThreadUpdateInfo{ID: "thread-1", Name: "Updated"}}},
		agentmessage.CompletionStatusChunk{Data: agentmessage.CompletionStatusData{ThreadID: "thread-1", CompletionID: "completion-1", IsRunning: true}},
		agentmessage.DataChunk{DataType: "hooks-status", Data: json.RawMessage(`{"pendingHooks":["go-check"]}`)},
		agentmessage.DataChunk{DataType: "retry-status", Data: json.RawMessage(`{"message":"provider request failed: dial tcp timeout; retrying in 200ms (attempt 1/3)"}`)},
		agentmessage.ErrorChunk{ErrorText: "invalid model: no model providers are available; configure a provider, set MODEL, or pass --model"},
	)

	if len(messages) != 1 || messages[0].ID != snapshot[0].ID || messageText(messages[0]) != messageText(snapshot[0]) {
		t.Fatalf("messages = %#v, want unchanged snapshot %#v", messages, snapshot)
	}
}

func TestApplyChunkToolApprovalResponseAcceptedDuringResume(t *testing.T) {
	messages := []serverapi.Message{
		{
			ID:   "assistant-custom",
			Role: "assistant",
			Parts: []agentmessage.UIPart{
				agentmessage.DynamicToolPart{
					Type:       "dynamic-tool",
					ToolCallID: "tool-123",
					ToolName:   "AskUserQuestion",
					State:      "approval-requested",
					Approval:   &agentmessage.ToolApproval{ID: "approval-123"},
				},
			},
		},
	}
	messages = applyChunks(messages,
		agentmessage.ThreadResumeChunk{Data: agentmessage.ThreadResumeData{ThreadID: "thread-1", MessageID: "assistant-custom"}},
		agentmessage.ToolApprovalResponseDataChunk{Data: agentmessage.ToolApprovalResponseData{ApprovalID: "approval-123", Approved: true}},
		agentmessage.ResponseFinishChunk{},
	)

	part := toolPart(t, messages[0], "tool-123")
	if part.Approval == nil || part.Approval.Approved == nil || !*part.Approval.Approved {
		t.Fatalf("approval = %#v, want approved=true", part.Approval)
	}
}

func TestApplyChunkAppendsSourceFileAndStepParts(t *testing.T) {
	messages := applyChunks(nil,
		agentmessage.StartChunk{MessageID: "assistant-1"},
		agentmessage.StartStepChunk{},
		agentmessage.SourceChunk{SourceType: "url", SourceID: "source-1", URL: "https://example.com", Title: "Example"},
		agentmessage.SourceChunk{SourceType: "document", SourceID: "source-2", MediaType: "text/plain", Title: "Doc", Filename: "doc.txt"},
		agentmessage.FileChunk{MediaType: "image/png", Data: "data:image/png;base64,abc"},
	)

	if len(messages[0].Parts) != 4 {
		t.Fatalf("parts len = %d, want 4", len(messages[0].Parts))
	}
	if _, ok := messages[0].Parts[0].(agentmessage.UIStepStartPart); !ok {
		t.Fatalf("part 0 = %T, want UIStepStartPart", messages[0].Parts[0])
	}
	if _, ok := messages[0].Parts[1].(agentmessage.UISourceURLPart); !ok {
		t.Fatalf("part 1 = %T, want UISourceURLPart", messages[0].Parts[1])
	}
	if _, ok := messages[0].Parts[2].(agentmessage.UISourceDocumentPart); !ok {
		t.Fatalf("part 2 = %T, want UISourceDocumentPart", messages[0].Parts[2])
	}
	if _, ok := messages[0].Parts[3].(agentmessage.UIFilePart); !ok {
		t.Fatalf("part 3 = %T, want UIFilePart", messages[0].Parts[3])
	}
}

func TestApplyChunkInsertsUserMessageBeforeExistingMessage(t *testing.T) {
	messages := []serverapi.Message{testMessage("assistant-1", "assistant", "hello")}
	messages = applyChunk(messages, agentmessage.UserMessageChunk{Data: agentmessage.UserMessageData{
		Message:               testMessage("user-1", "user", "question"),
		InsertBeforeMessageID: "assistant-1",
	}})

	if len(messages) != 2 || messages[0].ID != "user-1" || messages[1].ID != "assistant-1" {
		t.Fatalf("messages = %#v, want user before assistant", messages)
	}
}

func TestApplyChunkDoesNotMutatePreviousSnapshots(t *testing.T) {
	messages := applyChunk(nil, agentmessage.StartChunk{MessageID: "assistant-1"})
	snapshot := cloneMessagesForTest(messages)
	messages = applyChunk(messages, agentmessage.TextDeltaChunk{ID: "text-1", Delta: "new"})

	if messageText(snapshot[0]) != "" {
		t.Fatalf("snapshot text = %q, want unchanged empty text", messageText(snapshot[0]))
	}
	if messageText(messages[0]) != "new" {
		t.Fatalf("messages text = %q, want new", messageText(messages[0]))
	}
}

func TestApplyChunkDoesNotMutateInputMessages(t *testing.T) {
	messages := applyChunk(nil, agentmessage.StartChunk{MessageID: "assistant-1"})
	previous := messages

	updated := applyChunk(messages, agentmessage.TextDeltaChunk{ID: "text-1", Delta: "new"})

	if messageText(previous[0]) != "" {
		t.Fatalf("previous text = %q, want unchanged empty text", messageText(previous[0]))
	}
	if messageText(updated[0]) != "new" {
		t.Fatalf("updated text = %q, want new", messageText(updated[0]))
	}
}

func applyChunks(messages []serverapi.Message, chunks ...serverapi.MessageChunk) []serverapi.Message {
	for _, chunk := range chunks {
		messages = applyChunk(messages, chunk)
	}
	return messages
}

func toolPart(t *testing.T, message serverapi.Message, toolCallID string) agentmessage.DynamicToolPart {
	t.Helper()
	for _, part := range message.Parts {
		part, ok := part.(agentmessage.DynamicToolPart)
		if ok && part.ToolCallID == toolCallID {
			return part
		}
	}
	t.Fatalf("missing tool part %q in %#v", toolCallID, message.Parts)
	return agentmessage.DynamicToolPart{}
}

func assertJSONEqual(t *testing.T, got json.RawMessage, want string) {
	t.Helper()
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got JSON %q: %v", string(got), err)
	}
	var wantValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("unmarshal want JSON %q: %v", want, err)
	}
	if !jsonValuesEqual(gotValue, wantValue) {
		t.Fatalf("JSON = %s, want %s", string(got), want)
	}
}

func jsonValuesEqual(left, right any) bool {
	leftData, _ := json.Marshal(left)
	rightData, _ := json.Marshal(right)
	return string(leftData) == string(rightData)
}

func cloneMessagesForTest(messages []serverapi.Message) []serverapi.Message {
	data, err := json.Marshal(messages)
	if err != nil {
		panic(err)
	}
	var clone []serverapi.Message
	if err := json.Unmarshal(data, &clone); err != nil {
		panic(err)
	}
	return clone
}
