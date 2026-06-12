package clisession

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
)

type Remote struct {
	baseURL    string
	workspace  string
	httpClient *http.Client
	token      string

	mu                   sync.Mutex
	answeredCompletionID map[string]string
}

func NewRemote(baseURL, token, workspace string) *Remote {
	return &Remote{
		baseURL:   strings.TrimRight(baseURL, "/"),
		workspace: workspace,
		token:     token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		answeredCompletionID: map[string]string{},
	}
}

func (s *Remote) WorkspaceRoot() string { return s.workspace }
func (s *Remote) Close()                {}

func (s *Remote) ListCommands(ctx context.Context) ([]api.Command, error) {
	var resp api.ListCommandsResponse
	if err := s.doJSON(ctx, http.MethodGet, "/commands", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Commands, nil
}

func (s *Remote) ListThreads(ctx context.Context) ([]api.Thread, error) {
	var resp api.ListThreadsResponse
	if err := s.doJSON(ctx, http.MethodGet, "/threads", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Threads, nil
}

func (s *Remote) GetThread(ctx context.Context, threadID string) (api.Thread, error) {
	var thread api.Thread
	if err := s.doJSON(ctx, http.MethodGet, "/threads/"+threadID, nil, &thread); err != nil {
		return api.Thread{}, err
	}
	return thread, nil
}

func (s *Remote) UpdateThread(ctx context.Context, threadID string, req api.UpdateThreadRequest) (api.Thread, error) {
	var thread api.Thread
	if err := s.doJSON(ctx, http.MethodPatch, "/threads/"+threadID, req, &thread); err != nil {
		return api.Thread{}, err
	}
	return thread, nil
}

func (s *Remote) Messages(ctx context.Context, threadID string) ([]message.UIMessage, error) {
	var resp api.ListMessagesResponse
	if err := s.doJSON(ctx, http.MethodGet, "/threads/"+threadID+"/messages", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Messages, nil
}

func (s *Remote) HasInterruptedTurn(context.Context, string) (bool, error) {
	return false, nil
}

func (s *Remote) PendingQuestion(ctx context.Context, threadID string) (*agent.PendingQuestion, error) {
	var resp api.PendingQuestionResponse
	if err := s.doJSON(ctx, http.MethodGet, "/threads/"+threadID+"/chat/question", nil, &resp); err != nil {
		apiErr := new(apiError)
		if errors.As(err, &apiErr) && apiErr.status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	if resp.Question == nil || resp.Status != "pending" {
		return nil, nil
	}
	question := &agent.PendingQuestion{
		ApprovalID:  resp.Question.ToolUseID,
		Questions:   resp.Question.Questions,
		Credentials: resp.Question.Credentials,
		Metadata:    resp.Question.Metadata,
		Context:     resp.Question.Context,
	}
	return question, nil
}

func (s *Remote) SubmitAnswer(ctx context.Context, threadID, approvalID string, req api.AnswerQuestionRequest) error {
	var resp api.AnswerQuestionResponse
	if err := s.doJSON(ctx, http.MethodPost, "/threads/"+threadID+"/chat/answer/"+approvalID, req, &resp); err != nil {
		return err
	}
	s.mu.Lock()
	if strings.TrimSpace(resp.CompletionID) != "" {
		s.answeredCompletionID[threadID] = resp.CompletionID
	}
	s.mu.Unlock()
	return nil
}

func (s *Remote) Prompt(ctx context.Context, threadID string, req agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	uiMessage := message.UIMessage{Role: "user", Parts: req.UserParts, Metadata: req.Metadata}
	body := api.ChatRequest{
		Messages:     []message.UIMessage{uiMessage},
		Model:        req.Model,
		Reasoning:    req.Reasoning,
		FreshContext: req.FreshContext,
		SubagentType: req.SubagentType,
		MaxTurns:     req.MaxTurns,
	}
	completionID, err := s.startChat(ctx, "/threads/"+threadID+"/chat", body)
	if err != nil {
		return nil, err
	}
	return s.streamCompletion(ctx, threadID, completionID), nil
}

func (s *Remote) Resume(ctx context.Context, threadID string, _ agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	s.mu.Lock()
	completionID := s.answeredCompletionID[threadID]
	delete(s.answeredCompletionID, threadID)
	s.mu.Unlock()
	if strings.TrimSpace(completionID) == "" {
		return nil, fmt.Errorf("no resumable remote completion for thread %s", threadID)
	}
	return s.streamCompletion(ctx, threadID, completionID), nil
}

func (s *Remote) startChat(ctx context.Context, path string, body any) (string, error) {
	var started api.ChatStartedResponse
	if err := s.doJSON(ctx, http.MethodPost, path, body, &started); err != nil {
		return "", err
	}
	if started.Status == "queued" {
		return "", fmt.Errorf("chat request was queued")
	}
	if strings.TrimSpace(started.CompletionID) == "" {
		return "", fmt.Errorf("chat response did not include a completion id")
	}
	return started.CompletionID, nil
}

func (s *Remote) streamCompletion(ctx context.Context, threadID, completionID string) iter.Seq2[message.MessageChunk, error] {
	return func(yield func(message.MessageChunk, error) bool) {
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			<-streamCtx.Done()
			if !errors.Is(streamCtx.Err(), context.Canceled) {
				return
			}
			cancelCtx, cancelCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelCancel()
			_ = s.doJSON(cancelCtx, http.MethodPost, "/threads/"+threadID+"/chat/cancel", map[string]any{}, nil)
		}()

		req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, s.baseURL+"/threads/"+threadID+"/chat/stream", nil)
		if err != nil {
			yield(nil, err)
			return
		}
		req.Header.Set("Last-Event-ID", completionID+":-1")
		resp, err := s.do(req)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		frames, errs := readSSE(resp.Body)
		for frame := range frames {
			if frame.Event != "chunk" {
				continue
			}
			chunk, err := message.UnmarshalChunk(frame.Data)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if status, ok := chunk.(message.CompletionStatusChunk); ok {
				if status.Data.CompletionID == completionID && !status.Data.IsRunning {
					return
				}
				continue
			}
			if !yield(chunk, nil) {
				return
			}
		}
		if err := <-errs; err != nil && !errors.Is(err, context.Canceled) {
			yield(nil, err)
		}
	}
}

type sseFrame struct {
	Event string
	Data  []byte
}

func readSSE(body io.Reader) (<-chan sseFrame, <-chan error) {
	frames := make(chan sseFrame)
	errs := make(chan error, 1)
	go func() {
		defer close(frames)
		defer close(errs)
		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var event string
		var data bytes.Buffer
		flush := func() bool {
			if data.Len() == 0 {
				event = ""
				return true
			}
			payload := bytes.TrimSuffix(data.Bytes(), []byte("\n"))
			frames <- sseFrame{Event: event, Data: append([]byte(nil), payload...)}
			event = ""
			data.Reset()
			return true
		}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				flush()
				continue
			}
			switch {
			case strings.HasPrefix(line, "event:"):
				event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				data.WriteString(strings.TrimPrefix(line, "data:"))
				data.WriteByte('\n')
			}
		}
		if err := scanner.Err(); err != nil {
			errs <- err
			return
		}
		flush()
		errs <- nil
	}()
	return frames, errs
}

func (s *Remote) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *Remote) do(req *http.Request) (*http.Response, error) {
	if strings.TrimSpace(s.token) != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var apiResp api.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return nil, &apiError{status: resp.StatusCode, message: resp.Status}
		}
		return nil, &apiError{status: resp.StatusCode, message: apiResp.Error}
	}
	return resp, nil
}
