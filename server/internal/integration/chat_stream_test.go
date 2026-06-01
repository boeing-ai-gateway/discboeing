package integration

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type chatSSEFrame struct {
	Event string
	Data  string
}

func readChatSSEFrames(body io.Reader) ([]chatSSEFrame, error) {
	scanner := bufio.NewScanner(body)
	frames := []chatSSEFrame{}
	current := chatSSEFrame{}
	hasCurrent := false

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if hasCurrent {
				frames = append(frames, current)
				current = chatSSEFrame{}
				hasCurrent = false
			}
			continue
		}

		hasCurrent = true
		switch {
		case strings.HasPrefix(line, "event: "):
			current.Event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data := strings.TrimPrefix(line, "data: ")
			if current.Data == "" {
				current.Data = data
			} else {
				current.Data += "\n" + data
			}
		}
	}

	if hasCurrent {
		frames = append(frames, current)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}

	return frames, nil
}

func threadChatStreamPath(projectID, sessionID, threadID string) string {
	return "/api/projects/" + projectID + "/sessions/" + sessionID + "/threads/" + threadID + "/stream"
}

func TestChatStream_SessionNotFound(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	// Request stream for non-existent session - the nested session route rejects it before the stream handler runs.
	resp := client.Get(threadChatStreamPath(project.ID, "nonexistent-session", "nonexistent-thread"))
	defer resp.Body.Close()

	// Missing session = 404 Not Found from session route validation
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestChatStream_SessionBelongsToOtherProject(t *testing.T) {
	ts := NewTestServer(t)

	// Create two users with their own projects
	user1 := ts.CreateTestUser("user1@example.com")
	project1 := ts.CreateTestProject(user1, "Project 1")
	workspace1 := ts.CreateTestWorkspace(project1, "/home/user1/code")
	session1 := ts.CreateTestSession(workspace1, "Session 1")

	user2 := ts.CreateTestUser("user2@example.com")
	project2 := ts.CreateTestProject(user2, "Project 2")

	// User2 tries to access user1's session via their own project
	client2 := ts.AuthenticatedClient(user2)
	resp := client2.Get(threadChatStreamPath(project2.ID, session1.ID, session1.ID))
	defer resp.Body.Close()

	// Should return 403 Forbidden
	AssertStatus(t, resp, http.StatusForbidden)
}

func TestChatStream_ValidSession_NoActiveStream(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	// Request stream for a valid session with no active completion.
	// The stream endpoint should still be available and return an SSE response.
	resp := client.Get(threadChatStreamPath(project.ID, session.ID, session.ID))
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %s", ct)
	}

	frames, err := readChatSSEFrames(resp.Body)
	if err != nil {
		t.Fatalf("Error reading SSE stream: %v", err)
	}
	if len(frames) != 0 {
		t.Fatalf("expected idle stream to close without forwarding events, got %d frames", len(frames))
	}
}

func TestChatStream_RequiresAuthentication(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSession(workspace, "Test Session")

	// Make unauthenticated request
	resp, err := http.Get(ts.Server.URL + threadChatStreamPath(project.ID, session.ID, session.ID))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 401 Unauthorized
	AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestChatStream_MissingThreadId(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSession(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	// Request stream without thread ID.
	// chi router treats /threads//stream as /threads/{threadId}/stream with an empty threadId.
	resp := client.Get("/api/projects/" + project.ID + "/sessions/" + session.ID + "/threads//stream")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusBadRequest)
}

// TestChatStream_ActiveStream_FirstMessageConsumed verifies the stream forwards
// the first buffered message instead of dropping it.
func TestChatStream_ActiveStream_FirstMessageConsumed(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	// Create session with sandbox
	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")

	// Configure mock sandbox with a custom HTTP handler that simulates
	// an active SSE stream with multiple messages
	messages := []string{
		`{"type":"text","text":"First message"}`,
		`{"type":"text","text":"Second message"}`,
		`{"type":"text","text":"Third message"}`,
	}

	ts.MockSandbox.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle GET /chat/stream for SSE streams
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/chat/stream") {
			http.NotFound(w, r)
			return
		}

		// Check if this is an SSE stream request
		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			// Write all messages immediately so the first event is already buffered.
			for _, msg := range messages {
				_, _ = fmt.Fprintf(w, "event: chunk\n")
				_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}

			_, _ = fmt.Fprintf(w, "event: done\n")
			_, _ = fmt.Fprintf(w, "data: {}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		// Non-SSE GET returns empty messages
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[]}`))
	})

	// Request the stream
	resp := client.Get(threadChatStreamPath(project.ID, session.ID, session.ID))
	defer resp.Body.Close()

	// Verify we got 200 OK with SSE headers
	AssertStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
	}

	frames, err := readChatSSEFrames(resp.Body)
	if err != nil {
		t.Fatalf("Error reading SSE stream: %v", err)
	}
	receivedMessages := []string{}
	for _, frame := range frames {
		if frame.Event == "chunk" {
			receivedMessages = append(receivedMessages, frame.Data)
		}
	}

	// Verify we received all messages including the first one
	if len(receivedMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(receivedMessages))
	}

	// Verify each message was received correctly
	for i, expected := range messages {
		if i >= len(receivedMessages) {
			t.Errorf("Missing message %d: %s", i, expected)
			continue
		}
		if receivedMessages[i] != expected {
			t.Errorf("Message %d mismatch:\nExpected: %s\nGot: %s", i, expected, receivedMessages[i])
		}
	}

	// Most importantly: verify the first message was NOT lost
	if len(receivedMessages) > 0 && !strings.Contains(receivedMessages[0], "First message") {
		t.Error("First buffered message was lost")
	}
}

// TestChatStream_ActiveStream_SlowMessages tests that the stream properly
// handles messages that arrive slowly (not all buffered at once).
func TestChatStream_ActiveStream_SlowMessages(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")

	// Configure mock sandbox to send messages with delays
	ts.MockSandbox.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/chat/stream") {
			http.NotFound(w, r)
			return
		}

		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			// Send first message immediately
			_, _ = fmt.Fprintf(w, "event: chunk\n")
			_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"type":"text","text":"Message 1"}`)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			// Wait a bit before sending second message
			time.Sleep(10 * time.Millisecond)
			_, _ = fmt.Fprintf(w, "event: chunk\n")
			_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"type":"text","text":"Message 2"}`)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			_, _ = fmt.Fprintf(w, "event: done\n")
			_, _ = fmt.Fprintf(w, "data: {}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[]}`))
	})

	resp := client.Get(threadChatStreamPath(project.ID, session.ID, session.ID))
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	// Read messages with a reasonable timeout
	receivedMessages := []string{}
	done := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		defer close(done)
		frames, err := readChatSSEFrames(resp.Body)
		if err != nil {
			errCh <- err
			return
		}
		for _, frame := range frames {
			if frame.Event == "chunk" {
				receivedMessages = append(receivedMessages, frame.Data)
			}
		}
	}()

	// Wait for messages or timeout
	select {
	case <-done:
		// Success
		select {
		case err := <-errCh:
			t.Fatalf("Error reading SSE stream: %v", err)
		default:
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for stream messages")
	}

	if len(receivedMessages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(receivedMessages))
	}

	if len(receivedMessages) > 0 && !strings.Contains(receivedMessages[0], "Message 1") {
		t.Error("First message was not received correctly")
	}
}

// TestChatStream_ActiveStream_OnlyDone tests the edge case where the sandbox
// immediately closes the stream with only a terminal done event.
func TestChatStream_ActiveStream_OnlyDone(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")

	// Configure mock sandbox to send only DONE signal
	ts.MockSandbox.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/chat/stream") {
			http.NotFound(w, r)
			return
		}

		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			_, _ = fmt.Fprintf(w, "event: done\n")
			_, _ = fmt.Fprintf(w, "data: {}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[]}`))
	})

	resp := client.Get(threadChatStreamPath(project.ID, session.ID, session.ID))
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	// Read and verify the server does not forward the terminal done event.
	frames, err := readChatSSEFrames(resp.Body)
	if err != nil {
		t.Fatalf("Error reading SSE stream: %v", err)
	}
	if len(frames) != 0 {
		t.Fatalf("expected no forwarded events, got %d", len(frames))
	}
}

func chatWebSocketPath(projectID string) string {
	return "/api/projects/" + projectID + "/ws"
}

type chatWebSocketFrame struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId,omitempty"`
	ThreadID  string `json:"threadId,omitempty"`
	Event     string `json:"event,omitempty"`
	Data      string `json:"data,omitempty"`
	ID        string `json:"id,omitempty"`
	Error     string `json:"error,omitempty"`
	Replay    bool   `json:"replay,omitempty"`
}

func chatWebSocketURL(ts *TestServer, user *TestUser, projectID string) string {
	wsURL, err := url.Parse(ts.Server.URL)
	if err != nil {
		panic(err)
	}
	wsURL.Scheme = strings.Replace(wsURL.Scheme, "http", "ws", 1)
	wsURL.Path = chatWebSocketPath(projectID)
	if user != nil {
		query := wsURL.Query()
		query.Set("token", user.Token)
		wsURL.RawQuery = query.Encode()
	}
	return wsURL.String()
}

func dialChatWebSocket(t *testing.T, ts *TestServer, user *TestUser, projectID string) *websocket.Conn {
	t.Helper()

	headers := http.Header{}
	if user != nil {
		headers.Add("Cookie", fmt.Sprintf("discobot_session=%s", user.Token))
	}
	conn, resp, err := websocket.Dial(context.Background(), chatWebSocketURL(ts, nil, projectID), &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		statusCode := 0
		body := ""
		if resp != nil {
			statusCode = resp.StatusCode
			responseBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(responseBody)
		}
		t.Fatalf(
			"Failed to dial chat websocket: %v (status=%d body=%q)",
			err,
			statusCode,
			body,
		)
	}
	return conn
}

func readChatWebSocketFrame(t *testing.T, conn *websocket.Conn) chatWebSocketFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var frame chatWebSocketFrame
	if err := wsjson.Read(ctx, conn, &frame); err != nil {
		t.Fatalf("Failed to read websocket frame: %v", err)
	}
	return frame
}

func TestChatWebSocket_RequiresAuthentication(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")

	_, resp, err := websocket.Dial(context.Background(), chatWebSocketURL(ts, nil, project.ID), nil)
	if err == nil {
		t.Fatal("expected websocket dial to fail without authentication")
	}
	if resp == nil {
		t.Fatalf("expected websocket handshake response, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}
}

func TestChatWebSocket_SubscribeForwardsEvents(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")

	messages := []string{
		`{"type":"text","text":"First message"}`,
		`{"type":"text","text":"Second message"}`,
	}

	ts.MockSandbox.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/chat/stream") {
			http.NotFound(w, r)
			return
		}

		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			for index, msg := range messages {
				_, _ = fmt.Fprintf(w, "id: completion-1:%d\n", index)
				_, _ = fmt.Fprintf(w, "event: chunk\n")
				_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
			_, _ = fmt.Fprintf(w, "event: done\n")
			_, _ = fmt.Fprintf(w, "data: {}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[]}`))
	})

	conn := dialChatWebSocket(t, ts, user, project.ID)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	if err := wsjson.Write(context.Background(), conn, map[string]any{
		"type":      "subscribe",
		"stream":    "chat",
		"sessionId": session.ID,
		"threadId":  session.ID,
		"replay":    true,
	}); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	first := readChatWebSocketFrame(t, conn)
	if first.Type != "subscribed" {
		t.Fatalf("expected subscribed frame, got %#v", first)
	}

	receivedMessages := []string{}
	gotComplete := false
	for range 3 {
		frame := readChatWebSocketFrame(t, conn)
		switch frame.Type {
		case "event":
			if frame.Event == "chunk" {
				receivedMessages = append(receivedMessages, frame.Data)
			}
		case "complete":
			gotComplete = true
		}
	}

	if len(receivedMessages) != len(messages) {
		t.Fatalf("expected %d event frames, got %d", len(messages), len(receivedMessages))
	}
	for index, msg := range messages {
		if receivedMessages[index] != msg {
			t.Fatalf("message %d mismatch: expected %s got %s", index, msg, receivedMessages[index])
		}
	}
	if !gotComplete {
		t.Fatal("expected complete frame")
	}
}

func TestChatWebSocket_InvalidSessionReturnsError(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")

	conn := dialChatWebSocket(t, ts, user, project.ID)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	if err := wsjson.Write(context.Background(), conn, map[string]any{
		"type":      "subscribe",
		"stream":    "chat",
		"sessionId": "missing-session",
		"threadId":  "missing-thread",
		"replay":    true,
	}); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	frame := readChatWebSocketFrame(t, conn)
	if frame.Type != "error" {
		t.Fatalf("expected error frame, got %#v", frame)
	}
	if !strings.Contains(frame.Error, "session not found") {
		t.Fatalf("expected session not found error, got %q", frame.Error)
	}
}
