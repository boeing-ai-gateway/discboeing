package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
	discobotagent "github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type dialTransport struct {
	addr string
}

func (t dialTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return nil, err
	}
	return (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
}

func TestAgentPromptCreatesSessionAndPromptsACP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePrompt(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	chunks, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("hello ACP")))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(chunks))
	}
	if _, ok := chunks[0].(message.StartChunk); !ok {
		t.Fatalf("first chunk = %T, want StartChunk", chunks[0])
	}
	finish, ok := chunks[1].(message.ResponseFinishChunk)
	if !ok {
		t.Fatalf("second chunk = %T, want ResponseFinishChunk", chunks[1])
	}
	if finish.FinishReason != string(protocol.StopReasonEndTurn) {
		t.Fatalf("finish reason = %q, want %q", finish.FinishReason, protocol.StopReasonEndTurn)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}

	threads, err := agent.ListThreads()
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(threads) != "[thread-1]" {
		t.Fatalf("threads = %v, want [thread-1]", threads)
	}
}

func TestAgentPromptStreamsSessionUpdates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptWithUpdates(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	chunks, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("hello ACP")))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 9 {
		t.Fatalf("chunk count = %d, want 9", len(chunks))
	}
	if _, ok := chunks[0].(message.StartChunk); !ok {
		t.Fatalf("first chunk = %T, want StartChunk", chunks[0])
	}
	start, ok := chunks[1].(message.TextStartChunk)
	if !ok {
		t.Fatalf("second chunk = %T, want TextStartChunk", chunks[1])
	}
	if start.ID != "response-1" {
		t.Fatalf("text start id = %q, want response-1", start.ID)
	}
	delta, ok := chunks[2].(message.TextDeltaChunk)
	if !ok {
		t.Fatalf("third chunk = %T, want TextDeltaChunk", chunks[2])
	}
	if delta.Delta != "hello " {
		t.Fatalf("delta = %q, want hello", delta.Delta)
	}
	delta, ok = chunks[3].(message.TextDeltaChunk)
	if !ok {
		t.Fatalf("fourth chunk = %T, want TextDeltaChunk", chunks[3])
	}
	if delta.ID != "response-1" || delta.Delta != "from ACP update" {
		t.Fatalf("delta = %#v, want response-1/from ACP update", delta)
	}
	if end, ok := chunks[4].(message.TextEndChunk); !ok || end.ID != "response-1" {
		t.Fatalf("fifth chunk = %#v, want TextEndChunk response-1", chunks[4])
	}
	if reasoning, ok := chunks[6].(message.ReasoningDeltaChunk); !ok || reasoning.ID != "thought-1" || reasoning.Delta != "thinking" {
		t.Fatalf("reasoning delta = %#v, want thought-1/thinking", chunks[6])
	}
	if end, ok := chunks[7].(message.ReasoningEndChunk); !ok || end.ID != "thought-1" {
		t.Fatalf("eighth chunk = %#v, want ReasoningEndChunk thought-1", chunks[7])
	}
	if _, ok := chunks[8].(message.ResponseFinishChunk); !ok {
		t.Fatalf("last chunk = %T, want ResponseFinishChunk", chunks[8])
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}

	messages, err := agent.Messages("thread-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want user and assistant messages", len(messages))
	}
	if messages[0].Role != "user" || len(messages[0].Parts) != 1 {
		t.Fatalf("user message = %#v, want one user part", messages[0])
	}
	userText, ok := messages[0].Parts[0].(message.UITextPart)
	if !ok || userText.Text != "hello ACP" {
		t.Fatalf("user part = %#v, want hello ACP", messages[0].Parts[0])
	}
	if messages[1].Role != "assistant" || len(messages[1].Parts) != 3 {
		t.Fatalf("assistant message = %#v, want text chunks and reasoning", messages[1])
	}
	final, err := agent.FinalResponse("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if final != "hello from ACP update" {
		t.Fatalf("final response = %q, want streamed assistant text", final)
	}
}

func TestAgentPromptEmitsThreadUpdateForSessionRename(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptWithSessionRename(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	chunks, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("rename me")))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunk count = %d, want 3", len(chunks))
	}
	if _, ok := chunks[0].(message.StartChunk); !ok {
		t.Fatalf("first chunk = %T, want StartChunk", chunks[0])
	}
	update, ok := chunks[1].(message.ThreadUpdateChunk)
	if !ok {
		t.Fatalf("second chunk = %T, want ThreadUpdateChunk", chunks[1])
	}
	if update.Data.Thread.ID != "thread-1" || update.Data.Thread.Name != "Renamed by ACP" {
		t.Fatalf("thread update = %#v, want renamed thread-1", update)
	}
	if _, ok := chunks[2].(message.ResponseFinishChunk); !ok {
		t.Fatalf("last chunk = %T, want ResponseFinishChunk", chunks[2])
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentPromptParksACPRequestPermissionUntilResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptWithPermission(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	promptChunks, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("run command")))
	if err != nil {
		t.Fatal(err)
	}
	if len(promptChunks) != 3 {
		t.Fatalf("prompt chunk count = %d, want start, question tool, approval request", len(promptChunks))
	}
	question, ok := promptChunks[1].(message.ToolCallChunk)
	if !ok || question.ToolName != "AskUserQuestion" || question.ToolCallID != "tool-1" {
		t.Fatalf("question tool chunk = %#v, want AskUserQuestion/tool-1", promptChunks[1])
	}
	request, ok := promptChunks[2].(message.ToolApprovalRequestChunk)
	if !ok || request.ApprovalID != "acp-permission-tool-1" || request.ToolCallID != "tool-1" {
		t.Fatalf("approval request chunk = %#v, want tool-1", promptChunks[2])
	}

	pending, err := agent.PendingQuestion("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if pending == nil {
		t.Fatal("expected pending permission question")
	}
	if pending.ApprovalID != "acp-permission-tool-1" {
		t.Fatalf("approval id = %q, want acp-permission-tool-1", pending.ApprovalID)
	}
	if len(pending.Questions) != 1 || len(pending.Questions[0].Options) != 2 {
		t.Fatalf("pending question = %#v, want one question with two options", pending)
	}
	if err := agent.SubmitAnswer("thread-1", pending.ApprovalID, api.AnswerQuestionRequest{
		Answers: map[string]string{
			pending.Questions[0].Question: "Allow once",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SubmitAnswer("thread-1", pending.ApprovalID, api.AnswerQuestionRequest{
		Answers: map[string]string{
			pending.Questions[0].Question: "Allow once",
		},
	}); err != nil {
		t.Fatalf("idempotent submit answer: %v", err)
	}

	resumed, err := agent.Resume(ctx, "thread-1", discobotagent.PromptRequest{})
	if err != nil {
		t.Fatal(err)
	}
	resumeChunks, err := collect(resumed.Stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumeChunks) != 2 {
		t.Fatalf("resume chunk count = %d, want approval response, finish", len(resumeChunks))
	}
	response, ok := resumeChunks[0].(message.ToolApprovalResponseChunk)
	if !ok || !response.Approved || response.ApprovalID != pending.ApprovalID {
		t.Fatalf("approval response chunk = %#v, want approved", resumeChunks[0])
	}
	if _, ok := resumeChunks[1].(message.ResponseFinishChunk); !ok {
		t.Fatalf("last chunk = %T, want ResponseFinishChunk", resumeChunks[1])
	}
	if pending, err := agent.PendingQuestion("thread-1"); err != nil {
		t.Fatal(err)
	} else if pending != nil {
		t.Fatalf("pending question after answer = %#v, want nil", pending)
	}
	if err := agent.SubmitAnswer("thread-1", request.ApprovalID, api.AnswerQuestionRequest{
		Answers: map[string]string{
			"anything": "ignored",
		},
	}); err != nil {
		t.Fatalf("submit after resume should be idempotent: %v", err)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentCancelClearsParkedACPRequestPermission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptWithPermissionOnly(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	promptChunks, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("run command")))
	if err != nil {
		t.Fatal(err)
	}
	if len(promptChunks) != 3 {
		t.Fatalf("prompt chunk count = %d, want parked permission chunks", len(promptChunks))
	}

	pending, err := agent.PendingQuestion("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if pending == nil {
		t.Fatal("expected pending permission question")
	}
	if !agent.Cancel("thread-1") {
		t.Fatal("cancel returned false, want true")
	}
	if pending, err := agent.PendingQuestion("thread-1"); err != nil {
		t.Fatal(err)
	} else if pending != nil {
		t.Fatalf("pending question after cancel = %#v, want nil", pending)
	}
	if _, err := agent.Resume(ctx, "thread-1", discobotagent.PromptRequest{}); !errors.Is(err, discobotagent.ErrInterruptedTurnRequiresResume) {
		t.Fatalf("resume after cancel error = %v, want ErrInterruptedTurnRequiresResume", err)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentQuestionAndResumeErrorsMirrorDefaultAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := thread.NewStore(t.TempDir())
	agent := New(nil, "/workspace", store)

	pending, err := agent.PendingQuestion("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if pending != nil {
		t.Fatalf("pending question = %#v, want nil", pending)
	}
	if err := agent.SubmitAnswer("thread-1", "missing", api.AnswerQuestionRequest{}); err == nil {
		t.Fatal("expected submit without pending question to fail")
	}
	if _, err := agent.Resume(ctx, "thread-1", discobotagent.PromptRequest{}); !errors.Is(err, discobotagent.ErrInterruptedTurnRequiresResume) {
		t.Fatalf("resume without pending error = %v, want ErrInterruptedTurnRequiresResume", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptWithPermissionOnly(ctx, ln)
	}()

	agent, err = Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	if _, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("run command"))); err != nil {
		t.Fatal(err)
	}
	interrupted, err := agent.HasInterruptedTurn("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if interrupted {
		t.Fatal("HasInterruptedTurn = true while waiting for answer, want false")
	}
	if _, err := agent.Resume(ctx, "thread-1", discobotagent.PromptRequest{}); !errors.Is(err, discobotagent.ErrPendingQuestionRequiresAnswer) {
		t.Fatalf("resume before answer error = %v, want ErrPendingQuestionRequiresAnswer", err)
	}
	if _, err := agent.Resume(ctx, "thread-1", agentRequest("new prompt")); !errors.Is(err, errUnsupported) {
		t.Fatalf("resume with prompt error = %v, want errUnsupported", err)
	}
	if err := agent.SubmitAnswer("thread-1", "wrong-approval", api.AnswerQuestionRequest{}); err == nil {
		t.Fatal("expected submit with wrong approval to fail")
	}

	agent.Cancel("thread-1")
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentPromptStreamsToolUpdates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptWithToolUpdates(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	chunks, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("check code")))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 4 {
		t.Fatalf("chunk count = %d, want 4", len(chunks))
	}
	if _, ok := chunks[0].(message.StartChunk); !ok {
		t.Fatalf("first chunk = %T, want StartChunk", chunks[0])
	}
	call, ok := chunks[1].(message.ToolCallChunk)
	if !ok {
		t.Fatalf("second chunk = %T, want ToolCallChunk", chunks[1])
	}
	if call.ToolCallID != "call-1" || call.ToolName != "read" || call.Input != `{"path":"main.go"}` {
		t.Fatalf("tool call = %#v, want read call-1 main.go", call)
	}
	if call.ProviderExecuted == nil || !*call.ProviderExecuted {
		t.Fatalf("provider executed = %v, want true", call.ProviderExecuted)
	}
	result, ok := chunks[2].(message.ToolResultChunk)
	if !ok {
		t.Fatalf("third chunk = %T, want ToolResultChunk", chunks[2])
	}
	if result.ToolCallID != "call-1" || result.ToolName != "read" {
		t.Fatalf("tool result = %#v, want call-1/read", result)
	}
	if result.IsError == nil || *result.IsError {
		t.Fatalf("tool result isError = %v, want false", result.IsError)
	}
	if !strings.Contains(string(result.Result), "Analysis complete") {
		t.Fatalf("tool result = %s, want Analysis complete", result.Result)
	}
	if _, ok := chunks[3].(message.ResponseFinishChunk); !ok {
		t.Fatalf("last chunk = %T, want ResponseFinishChunk", chunks[3])
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentListThreadsSyncsKnownACPSessions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptAndListSessions(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	if _, err := collect(agent.Prompt(ctx, "client-thread-1", agentRequest("hello ACP"))); err != nil {
		t.Fatal(err)
	}

	threads, err := agent.ListThreads()
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(threads) != "[client-thread-1]" {
		t.Fatalf("threads = %v, want [client-thread-1]", threads)
	}

	state, ok := agent.sessionManager.state.Get("client-thread-1")
	if !ok {
		t.Fatal("missing session state")
	}
	if state.SessionID != "acp-session-1" {
		t.Fatalf("session id = %q, want acp-session-1", state.SessionID)
	}
	session, ok := agent.sessionManager.loadStoredSession("client-thread-1")
	if !ok || session.Title == nil || *session.Title != "Updated ACP Session" {
		t.Fatalf("stored session = %#v, want Updated ACP Session", session)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentListThreadsImportsUnknownACPSessions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serveListAndLoadUnknownSession(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	threads, err := agent.ListThreads()
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 {
		t.Fatalf("thread count = %d, want 1: %v", len(threads), threads)
	}
	threadID := threads[0]
	if !strings.HasPrefix(threadID, "thread-") {
		t.Fatalf("thread id = %q, want generated thread-* id", threadID)
	}

	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metadata.ACPSession.SessionID != "unknown-acp-session" {
		t.Fatalf("stored ACP session id = %q, want unknown-acp-session", cfg.Metadata.ACPSession.SessionID)
	}
	messages, err := agent.Messages(threadID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if messages[0].Role != "user" || len(messages[0].Parts) != 1 {
		t.Fatalf("first message = %#v, want user text", messages[0])
	}
	userText, ok := messages[0].Parts[0].(message.UITextPart)
	if !ok || userText.Text != "imported user prompt" {
		t.Fatalf("first part = %#v, want imported user prompt", messages[0].Parts[0])
	}
	if messages[1].Role != "assistant" || len(messages[1].Parts) != 1 {
		t.Fatalf("second message = %#v, want assistant text", messages[1])
	}
	assistantText, ok := messages[1].Parts[0].(message.UITextPart)
	if !ok || assistantText.Text != "imported assistant response" {
		t.Fatalf("second part = %#v, want imported assistant response", messages[1].Parts[0])
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentPersistsACPSessionInThreadMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePrompt(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	if _, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("hello ACP"))); err != nil {
		t.Fatal(err)
	}

	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metadata.ACPSession.SessionID != "session-1" {
		t.Fatalf("stored ACP session id = %q, want session-1", cfg.Metadata.ACPSession.SessionID)
	}
	if cfg.Metadata.ACPSession.CWD != "/workspace" {
		t.Fatalf("stored ACP cwd = %q, want /workspace", cfg.Metadata.ACPSession.CWD)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentUsesStoredACPSessionForThread(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePromptExistingSession(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	if err := store.SaveConfig("thread-1", thread.Config{
		Metadata: thread.ConfigMetadata{
			ACPSession: thread.ACPSessionMetadata{
				CWD:       "/workspace",
				SessionID: "stored-session-1",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	if _, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("hello again"))); err != nil {
		t.Fatal(err)
	}

	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metadata.ACPSession.ResponseMeta["source"] != "load" {
		t.Fatalf("response meta = %#v, want source load", cfg.Metadata.ACPSession.ResponseMeta)
	}
	if cfg.Metadata.ACPSession.Title == nil || *cfg.Metadata.ACPSession.Title != "Loaded ACP Session" {
		t.Fatalf("stored title = %v, want Loaded ACP Session", cfg.Metadata.ACPSession.Title)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentMessagesLoadsStoredACPSessionIntoMemory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serveLoadExistingSessionMessages(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	if err := store.SaveConfig("thread-1", thread.Config{
		Metadata: thread.ConfigMetadata{
			ACPSession: thread.ACPSessionMetadata{
				CWD:       "/workspace",
				SessionID: "stored-session-1",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	messages, err := agent.Messages("thread-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want loaded user and assistant", len(messages))
	}
	if messages[0].Role != "user" || messages[1].Role != "assistant" {
		t.Fatalf("messages = %#v, want user then assistant", messages)
	}
	text, ok := messages[1].Parts[0].(message.UITextPart)
	if !ok || text.Text != "loaded assistant response" {
		t.Fatalf("assistant part = %#v, want loaded assistant response", messages[1].Parts[0])
	}

	cached, err := agent.Messages("thread-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) != len(messages) {
		t.Fatalf("cached message count = %d, want %d", len(cached), len(messages))
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentThreadCRUDDelegatesToThreadStore(t *testing.T) {
	ctx := context.Background()
	store := thread.NewStore(t.TempDir())
	agent := New(nil, "/workspace", store)

	created, err := agent.CreateThread(ctx, discobotagent.CreateThreadRequest{
		ID:   "thread-1",
		Name: "First thread",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "thread-1" || created.Name != "First thread" || created.CWD != "/workspace" {
		t.Fatalf("created thread = %#v, want id/name/default cwd", created)
	}

	info, err := agent.GetThreadInfo("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != created.ID || info.Name != created.Name || info.CWD != created.CWD {
		t.Fatalf("get thread = %#v, want %#v", info, created)
	}

	newName := "Renamed"
	updated, err := agent.UpdateThread(ctx, "thread-1", discobotagent.UpdateThreadRequest{Name: &newName})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Renamed" {
		t.Fatalf("updated name = %q, want Renamed", updated.Name)
	}

	infos, err := agent.ListThreadInfos()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].ID != "thread-1" || infos[0].Name != "Renamed" {
		t.Fatalf("thread infos = %#v, want renamed thread-1", infos)
	}

	if err := agent.DeleteThread(ctx, "thread-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.GetThreadInfo("thread-1"); err == nil {
		t.Fatal("expected deleted thread get to fail")
	}
}

func TestAgentResumesStoredACPSessionWhenLoadUnsupported(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serveResumeExistingSession(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	if err := store.SaveConfig("thread-1", thread.Config{
		Metadata: thread.ConfigMetadata{
			ACPSession: thread.ACPSessionMetadata{
				CWD:       "/workspace",
				SessionID: "stored-session-1",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	if _, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("hello again"))); err != nil {
		t.Fatal(err)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestAgentCreatesNewSessionWhenStoredSessionCannotBeLoaded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- servePrompt(ctx, ln)
	}()

	store := thread.NewStore(t.TempDir())
	if err := store.SaveConfig("thread-1", thread.Config{
		Metadata: thread.ConfigMetadata{
			ACPSession: thread.ACPSessionMetadata{
				CWD:       "/workspace",
				SessionID: "stale-session-1",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	agent, err := Connect(ctx, dialTransport{addr: ln.Addr().String()}, "/workspace", store)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()

	if _, err := collect(agent.Prompt(ctx, "thread-1", agentRequest("hello ACP"))); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metadata.ACPSession.SessionID != "session-1" {
		t.Fatalf("stored ACP session id = %q, want session-1", cfg.Metadata.ACPSession.SessionID)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func TestContentBlocksMapsSchemaBackedUIParts(t *testing.T) {
	blocks, err := contentBlocks([]message.UIPart{
		message.UITextPart{Text: "hello"},
		message.UIReasoningPart{Text: "thinking"},
		message.UIFilePart{
			URL:       "data:image/png;base64,aGVsbG8=",
			MediaType: "image/png",
			Filename:  "image.png",
		},
		message.UIFilePart{
			URL:       "data:audio/wav;base64,aGVsbG8=",
			MediaType: "audio/wav",
			Filename:  "audio.wav",
		},
		message.UIFilePart{
			URL:       "https://example.com/files/report.pdf",
			MediaType: "application/pdf",
			Filename:  "report.pdf",
		},
		message.UISourceURLPart{
			URL:   "https://example.com/source",
			Title: "Source",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []map[string]any{
		{"type": protocol.ContentBlockTextType, "text": "hello"},
		{"type": protocol.ContentBlockTextType, "text": "thinking"},
		{"type": protocol.ContentBlockImageType, "data": "aGVsbG8=", "mimeType": "image/png", "uri": "data:image/png;base64,aGVsbG8="},
		{"type": protocol.ContentBlockAudioType, "data": "aGVsbG8=", "mimeType": "audio/wav"},
		{"type": protocol.ContentBlockResourceLinkType, "mimeType": "application/pdf", "name": "report.pdf", "title": "report.pdf", "uri": "https://example.com/files/report.pdf"},
		{"type": protocol.ContentBlockResourceLinkType, "name": "Source", "title": "Source", "uri": "https://example.com/source"},
	}
	if len(blocks) != len(want) {
		t.Fatalf("block count = %d, want %d", len(blocks), len(want))
	}
	for i, block := range blocks {
		var got map[string]any
		if err := json.Unmarshal(block.Raw(), &got); err != nil {
			t.Fatalf("unmarshal block %d: %v", i, err)
		}
		if fmt.Sprint(got) != fmt.Sprint(want[i]) {
			t.Fatalf("block %d = %v, want %v", i, got, want[i])
		}
	}
}

func servePrompt(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if initReq.Params.ProtocolVersion != protocolVersion {
		return fmt.Errorf("protocol version = %d, want %d", initReq.Params.ProtocolVersion, protocolVersion)
	}
	if initReq.Params.ClientInfo == nil || initReq.Params.ClientInfo.Name != "discobot-agent-go" {
		return fmt.Errorf("client info = %#v, want discobot-agent-go", initReq.Params.ClientInfo)
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{ProtocolVersion: protocolVersion}); err != nil {
		return err
	}

	newReq, err := readRequest[protocol.NewSessionRequest](ctx, rpcConn, protocol.NewSessionRequestMethod)
	if err != nil {
		return err
	}
	if newReq.Params.Cwd != "/workspace" {
		return fmt.Errorf("cwd = %q, want /workspace", newReq.Params.Cwd)
	}
	if err := writeResponse(ctx, rpcConn, newReq.ID, protocol.NewSessionResponse{SessionID: "session-1"}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if promptReq.Params.SessionID != "session-1" {
		return fmt.Errorf("session id = %q, want session-1", promptReq.Params.SessionID)
	}
	if len(promptReq.Params.Prompt) != 1 {
		return fmt.Errorf("prompt block count = %d, want 1", len(promptReq.Params.Prompt))
	}
	var block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(promptReq.Params.Prompt[0].Raw(), &block); err != nil {
		return err
	}
	if block.Type != "text" || block.Text != "hello ACP" {
		return fmt.Errorf("prompt block = %#v, want text hello ACP", block)
	}
	return writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn})
}

func servePromptWithPermission(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{ProtocolVersion: protocolVersion}); err != nil {
		return err
	}

	newReq, err := readRequest[protocol.NewSessionRequest](ctx, rpcConn, protocol.NewSessionRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, newReq.ID, protocol.NewSessionResponse{SessionID: "session-1"}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if promptReq.Params.SessionID != "session-1" {
		return fmt.Errorf("session id = %q, want session-1", promptReq.Params.SessionID)
	}

	title := "Run Bash"
	status := protocol.ToolCallStatusPending
	permissionID, err := jsonrpc.MakeID(float64(99))
	if err != nil {
		return err
	}
	if err := writeRequest(ctx, rpcConn, permissionID, protocol.RequestPermissionRequestMethod, protocol.RequestPermissionRequest{
		SessionID: "session-1",
		ToolCall: protocol.ToolCallUpdate{
			RawInput:   json.RawMessage(`{"command":"echo hi"}`),
			Status:     &status,
			Title:      &title,
			ToolCallID: "tool-1",
		},
		Options: []protocol.PermissionOption{
			{
				OptionID: "allow-once",
				Kind:     protocol.PermissionOptionKindAllowOnce,
				Name:     "Allow once",
			},
			{
				OptionID: "reject-once",
				Kind:     protocol.PermissionOptionKindRejectOnce,
				Name:     "Reject once",
			},
		},
	}); err != nil {
		return err
	}
	permissionResponse, err := readResponse(ctx, rpcConn, permissionID)
	if err != nil {
		return err
	}
	var response protocol.RequestPermissionResponse
	if err := json.Unmarshal(permissionResponse.Result, &response); err != nil {
		return err
	}
	selected, ok := response.Outcome.Variant().(protocol.RequestPermissionOutcomeSelected)
	if !ok || selected.OptionID != "allow-once" {
		return fmt.Errorf("permission outcome = %#v, want allow-once", response.Outcome.Variant())
	}

	return writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn})
}

func servePromptWithPermissionOnly(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{ProtocolVersion: protocolVersion}); err != nil {
		return err
	}

	newReq, err := readRequest[protocol.NewSessionRequest](ctx, rpcConn, protocol.NewSessionRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, newReq.ID, protocol.NewSessionResponse{SessionID: "session-1"}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if promptReq.Params.SessionID != "session-1" {
		return fmt.Errorf("session id = %q, want session-1", promptReq.Params.SessionID)
	}

	title := "Run Bash"
	status := protocol.ToolCallStatusPending
	permissionID, err := jsonrpc.MakeID(float64(99))
	if err != nil {
		return err
	}
	return writeRequest(ctx, rpcConn, permissionID, protocol.RequestPermissionRequestMethod, protocol.RequestPermissionRequest{
		SessionID: "session-1",
		ToolCall: protocol.ToolCallUpdate{
			RawInput:   json.RawMessage(`{"command":"echo hi"}`),
			Status:     &status,
			Title:      &title,
			ToolCallID: "tool-1",
		},
		Options: []protocol.PermissionOption{
			{
				OptionID: "allow-once",
				Kind:     protocol.PermissionOptionKindAllowOnce,
				Name:     "Allow once",
			},
			{
				OptionID: "reject-once",
				Kind:     protocol.PermissionOptionKindRejectOnce,
				Name:     "Reject once",
			},
		},
	})
}

func servePromptWithUpdates(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{ProtocolVersion: protocolVersion}); err != nil {
		return err
	}

	newReq, err := readRequest[protocol.NewSessionRequest](ctx, rpcConn, protocol.NewSessionRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, newReq.ID, protocol.NewSessionResponse{SessionID: "session-1"}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
			"content": map[string]any{
				"_meta": map[string]any{
					"id": "response-1",
				},
				"type": protocol.ContentBlockTextType,
				"text": "hello ",
			},
		})),
	}); err != nil {
		return err
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
			"content": map[string]any{
				"_meta": map[string]any{
					"id": "response-1",
				},
				"type": protocol.ContentBlockTextType,
				"text": "from ACP update",
			},
		})),
	}); err != nil {
		return err
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateAgentThoughtChunkSessionUpdate,
			"content": map[string]any{
				"_meta": map[string]any{
					"id": "thought-1",
				},
				"type": protocol.ContentBlockTextType,
				"text": "thinking",
			},
		})),
	}); err != nil {
		return err
	}
	return writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn})
}

func servePromptWithSessionRename(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{ProtocolVersion: protocolVersion}); err != nil {
		return err
	}

	newReq, err := readRequest[protocol.NewSessionRequest](ctx, rpcConn, protocol.NewSessionRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, newReq.ID, protocol.NewSessionResponse{SessionID: "session-1"}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateSessionInfoUpdateSessionUpdate,
			"title":         "Renamed by ACP",
		})),
	}); err != nil {
		return err
	}
	return writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn})
}

func servePromptWithToolUpdates(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{ProtocolVersion: protocolVersion}); err != nil {
		return err
	}

	newReq, err := readRequest[protocol.NewSessionRequest](ctx, rpcConn, protocol.NewSessionRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, newReq.ID, protocol.NewSessionResponse{SessionID: "session-1"}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateToolCallSessionUpdate,
			"toolCallId":    "call-1",
			"title":         "Read main.go",
			"kind":          "read",
			"rawInput": map[string]any{
				"path": "main.go",
			},
			"status": "in_progress",
		})),
	}); err != nil {
		return err
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateToolCallUpdateSessionUpdate,
			"toolCallId":    "call-1",
			"status":        "completed",
			"content": []any{
				map[string]any{
					"type": "content",
					"content": map[string]any{
						"type": protocol.ContentBlockTextType,
						"text": "Analysis complete:\n- No syntax errors found",
					},
				},
			},
		})),
	}); err != nil {
		return err
	}
	return writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn})
}

func servePromptExistingSession(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: protocol.AgentCapabilities{
			LoadSession: true,
		},
	}); err != nil {
		return err
	}

	loadReq, err := readRequest[protocol.LoadSessionRequest](ctx, rpcConn, protocol.LoadSessionRequestMethod)
	if err != nil {
		return err
	}
	if loadReq.Params.Cwd != "/workspace" {
		return fmt.Errorf("load cwd = %q, want /workspace", loadReq.Params.Cwd)
	}
	if loadReq.Params.SessionID != "stored-session-1" {
		return fmt.Errorf("load session id = %q, want stored-session-1", loadReq.Params.SessionID)
	}
	if len(loadReq.Params.MCPServers) != 0 {
		return fmt.Errorf("load mcp servers = %d, want 0", len(loadReq.Params.MCPServers))
	}
	updatedAt := "2026-04-30T00:00:00Z"
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "stored-session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateSessionInfoUpdateSessionUpdate,
			"title":         "Loaded ACP Session",
			"updatedAt":     updatedAt,
		})),
	}); err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, loadReq.ID, protocol.LoadSessionResponse{
		Meta: map[string]any{"source": "load"},
	}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if promptReq.Params.SessionID != "stored-session-1" {
		return fmt.Errorf("session id = %q, want stored-session-1", promptReq.Params.SessionID)
	}
	return writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn})
}

func serveLoadExistingSessionMessages(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: protocol.AgentCapabilities{
			LoadSession: true,
		},
	}); err != nil {
		return err
	}

	loadReq, err := readRequest[protocol.LoadSessionRequest](ctx, rpcConn, protocol.LoadSessionRequestMethod)
	if err != nil {
		return err
	}
	if loadReq.Params.SessionID != "stored-session-1" {
		return fmt.Errorf("load session id = %q, want stored-session-1", loadReq.Params.SessionID)
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "stored-session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateUserMessageChunkSessionUpdate,
			"content": map[string]any{
				"type": protocol.ContentBlockTextType,
				"text": "loaded user prompt",
			},
		})),
	}); err != nil {
		return err
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "stored-session-1",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
			"content": map[string]any{
				"type": protocol.ContentBlockTextType,
				"text": "loaded assistant response",
			},
		})),
	}); err != nil {
		return err
	}
	return writeResponse(ctx, rpcConn, loadReq.ID, protocol.LoadSessionResponse{})
}

func serveResumeExistingSession(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: protocol.AgentCapabilities{
			SessionCapabilities: protocol.SessionCapabilities{
				Resume: &protocol.SessionResumeCapabilities{},
			},
		},
	}); err != nil {
		return err
	}

	resumeReq, err := readRequest[protocol.ResumeSessionRequest](ctx, rpcConn, protocol.ResumeSessionRequestMethod)
	if err != nil {
		return err
	}
	if resumeReq.Params.Cwd != "/workspace" {
		return fmt.Errorf("resume cwd = %q, want /workspace", resumeReq.Params.Cwd)
	}
	if resumeReq.Params.SessionID != "stored-session-1" {
		return fmt.Errorf("resume session id = %q, want stored-session-1", resumeReq.Params.SessionID)
	}
	if len(resumeReq.Params.MCPServers) != 0 {
		return fmt.Errorf("resume mcp servers = %d, want 0", len(resumeReq.Params.MCPServers))
	}
	if err := writeResponse(ctx, rpcConn, resumeReq.ID, protocol.ResumeSessionResponse{}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if promptReq.Params.SessionID != "stored-session-1" {
		return fmt.Errorf("session id = %q, want stored-session-1", promptReq.Params.SessionID)
	}
	return writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn})
}

func servePromptAndListSessions(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: protocol.AgentCapabilities{
			SessionCapabilities: protocol.SessionCapabilities{
				List: &protocol.SessionListCapabilities{},
			},
		},
	}); err != nil {
		return err
	}

	newReq, err := readRequest[protocol.NewSessionRequest](ctx, rpcConn, protocol.NewSessionRequestMethod)
	if err != nil {
		return err
	}
	if newReq.Params.Cwd != "/workspace" {
		return fmt.Errorf("cwd = %q, want /workspace", newReq.Params.Cwd)
	}
	if err := writeResponse(ctx, rpcConn, newReq.ID, protocol.NewSessionResponse{SessionID: "acp-session-1"}); err != nil {
		return err
	}

	promptReq, err := readRequest[protocol.PromptRequest](ctx, rpcConn, protocol.PromptRequestMethod)
	if err != nil {
		return err
	}
	if promptReq.Params.SessionID != "acp-session-1" {
		return fmt.Errorf("session id = %q, want acp-session-1", promptReq.Params.SessionID)
	}
	if err := writeResponse(ctx, rpcConn, promptReq.ID, protocol.PromptResponse{StopReason: protocol.StopReasonEndTurn}); err != nil {
		return err
	}

	listReq, err := readRequest[protocol.ListSessionsRequest](ctx, rpcConn, protocol.ListSessionsRequestMethod)
	if err != nil {
		return err
	}
	if listReq.Params.Cwd == nil || *listReq.Params.Cwd != "/workspace" {
		return fmt.Errorf("list cwd = %v, want /workspace", listReq.Params.Cwd)
	}
	title := "Updated ACP Session"
	return writeResponse(ctx, rpcConn, listReq.ID, protocol.ListSessionsResponse{
		Sessions: []protocol.SessionInfo{
			{SessionID: "acp-session-1", Cwd: "/workspace", Title: &title},
			{SessionID: "unmapped-acp-session", Cwd: "/workspace"},
		},
	})
}

func serveListAndLoadUnknownSession(ctx context.Context, ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcConn, err := (&mcp.IOTransport{Reader: conn, Writer: conn}).Connect(ctx)
	if err != nil {
		return err
	}
	defer rpcConn.Close()

	initReq, err := readRequest[protocol.InitializeRequest](ctx, rpcConn, protocol.InitializeRequestMethod)
	if err != nil {
		return err
	}
	if err := writeResponse(ctx, rpcConn, initReq.ID, protocol.InitializeResponse{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: protocol.AgentCapabilities{
			LoadSession: true,
			SessionCapabilities: protocol.SessionCapabilities{
				List: &protocol.SessionListCapabilities{},
			},
		},
	}); err != nil {
		return err
	}

	listReq, err := readRequest[protocol.ListSessionsRequest](ctx, rpcConn, protocol.ListSessionsRequestMethod)
	if err != nil {
		return err
	}
	if listReq.Params.Cwd == nil || *listReq.Params.Cwd != "/workspace" {
		return fmt.Errorf("list cwd = %v, want /workspace", listReq.Params.Cwd)
	}
	title := "Imported ACP Session"
	if err := writeResponse(ctx, rpcConn, listReq.ID, protocol.ListSessionsResponse{
		Sessions: []protocol.SessionInfo{{
			SessionID: "unknown-acp-session",
			Cwd:       "/workspace",
			Title:     &title,
		}},
	}); err != nil {
		return err
	}

	loadReq, err := readRequest[protocol.LoadSessionRequest](ctx, rpcConn, protocol.LoadSessionRequestMethod)
	if err != nil {
		return err
	}
	if loadReq.Params.SessionID != "unknown-acp-session" {
		return fmt.Errorf("load session id = %q, want unknown-acp-session", loadReq.Params.SessionID)
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "unknown-acp-session",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateUserMessageChunkSessionUpdate,
			"content": map[string]any{
				"type": protocol.ContentBlockTextType,
				"text": "imported user prompt",
			},
		})),
	}); err != nil {
		return err
	}
	if err := writeNotification(ctx, rpcConn, protocol.SessionNotificationMethod, protocol.SessionNotification{
		SessionID: "unknown-acp-session",
		Update: protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
			"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
			"content": map[string]any{
				"type": protocol.ContentBlockTextType,
				"text": "imported assistant response",
			},
		})),
	}); err != nil {
		return err
	}
	return writeResponse(ctx, rpcConn, loadReq.ID, protocol.LoadSessionResponse{Meta: map[string]any{"source": "import"}})
}

type receivedRequest[T any] struct {
	ID     jsonrpc.ID
	Params T
}

func readRequest[T any](ctx context.Context, conn mcp.Connection, method string) (receivedRequest[T], error) {
	msg, err := conn.Read(ctx)
	if err != nil {
		return receivedRequest[T]{}, err
	}
	req, ok := msg.(*jsonrpc.Request)
	if !ok {
		return receivedRequest[T]{}, fmt.Errorf("unexpected JSON-RPC message %T", msg)
	}
	if req.Method != method {
		return receivedRequest[T]{}, fmt.Errorf("method = %q, want %q", req.Method, method)
	}
	var params T
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return receivedRequest[T]{}, err
	}
	return receivedRequest[T]{ID: req.ID, Params: params}, nil
}

func writeResponse(ctx context.Context, conn mcp.Connection, id jsonrpc.ID, result any) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return conn.Write(ctx, &jsonrpc.Response{ID: id, Result: data})
}

func writeNotification(ctx context.Context, conn mcp.Connection, method string, params any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return conn.Write(ctx, &jsonrpc.Request{Method: method, Params: data})
}

func writeRequest(ctx context.Context, conn mcp.Connection, id jsonrpc.ID, method string, params any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return conn.Write(ctx, &jsonrpc.Request{ID: id, Method: method, Params: data})
}

func readResponse(ctx context.Context, conn mcp.Connection, id jsonrpc.ID) (*jsonrpc.Response, error) {
	msg, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	resp, ok := msg.(*jsonrpc.Response)
	if !ok {
		return nil, fmt.Errorf("unexpected JSON-RPC message %T", msg)
	}
	if resp.ID.Raw() != id.Raw() {
		return nil, fmt.Errorf("response id = %v, want %v", resp.ID.Raw(), id.Raw())
	}
	return resp, nil
}

func agentRequest(text string) discobotagent.PromptRequest {
	return discobotagent.PromptRequest{
		UserParts: []message.UIPart{message.UITextPart{Text: text}},
	}
}

func mustRawJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func collect(seq iter.Seq2[message.MessageChunk, error]) ([]message.MessageChunk, error) {
	var chunks []message.MessageChunk
	for chunk, err := range seq {
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}
