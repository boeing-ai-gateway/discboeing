package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/model"
)

func TestListSessionsByProject_Empty(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/sessions")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Sessions []any `json:"sessions"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(result.Sessions))
	}
}

func TestCreateSession_ViaChat(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	// Sessions are created implicitly via the chat endpoint
	// Format matches the thread chat UIMessage payload
	sessionID := "test-session-id-1"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "Create a new session"},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	defer resp.Body.Close()

	// Chat endpoint returns a normal JSON response.
	AssertStatus(t, resp, http.StatusOK)

	var chatResult map[string]any
	ParseJSON(t, resp, &chatResult)
	if chatResult["sessionId"] != sessionID {
		t.Fatalf("Expected sessionId %q, got %v", sessionID, chatResult["sessionId"])
	}
	if chatResult["threadId"] != sessionID {
		t.Fatalf("Expected default threadId %q, got %v", sessionID, chatResult["threadId"])
	}
	if chatResult["workspaceId"] != workspace.ID {
		t.Fatalf("Expected workspaceId %q, got %v", workspace.ID, chatResult["workspaceId"])
	}

	// Verify session was created by listing sessions
	listResp := client.Get("/api/projects/" + project.ID + "/sessions")
	defer listResp.Body.Close()

	var result struct {
		Sessions []map[string]any `json:"sessions"`
	}
	ParseJSON(t, listResp, &result)

	if len(result.Sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(result.Sessions))
		return
	}

	// Session name stays empty until it is populated elsewhere.
	if result.Sessions[0]["name"] != "" {
		t.Errorf("Expected empty session name, got '%v'", result.Sessions[0]["name"])
	}
}

func TestCreateSession_ViaChatWithSessionIDPath(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	sessionID := "test-session-id-session-field"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "Create a session using sessionId"},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var chatResult map[string]any
	ParseJSON(t, resp, &chatResult)
	if chatResult["sessionId"] != sessionID {
		t.Fatalf("Expected sessionId %q, got %v", sessionID, chatResult["sessionId"])
	}
	if chatResult["threadId"] != sessionID {
		t.Fatalf("Expected default threadId %q, got %v", sessionID, chatResult["threadId"])
	}
	if chatResult["workspaceId"] != workspace.ID {
		t.Fatalf("Expected workspaceId %q, got %v", workspace.ID, chatResult["workspaceId"])
	}
}

func TestCreateSession_ViaLegacyChatEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/chat", map[string]any{
		"messages": []map[string]any{},
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusNotFound)
}

func TestCreateSession_ViaEmptyChat(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	sessionID := "test-session-id-empty-chat"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages":    []map[string]any{},
		"workspaceId": workspace.ID,
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var chatResult map[string]any
	ParseJSON(t, resp, &chatResult)
	if chatResult["sessionId"] != sessionID {
		t.Fatalf("Expected sessionId %q, got %v", sessionID, chatResult["sessionId"])
	}
	if chatResult["threadId"] != sessionID {
		t.Fatalf("Expected default threadId %q, got %v", sessionID, chatResult["threadId"])
	}
	if chatResult["workspaceId"] != workspace.ID {
		t.Fatalf("Expected workspaceId %q, got %v", workspace.ID, chatResult["workspaceId"])
	}
	if chatResult["messageId"] != nil {
		t.Fatalf("Expected empty messageId for empty chat submission, got %v", chatResult["messageId"])
	}

	sessionResp := client.Get("/api/projects/" + project.ID + "/sessions/" + sessionID)
	defer sessionResp.Body.Close()
	AssertStatus(t, sessionResp, http.StatusOK)

	var sessionResult map[string]any
	ParseJSON(t, sessionResp, &sessionResult)
	if sessionResult["name"] != "" {
		t.Fatalf("expected empty session name for empty chat submission, got %v", sessionResult["name"])
	}
}

func TestCreateSession_ViaCreateSessionEndpointWithoutWorkspace(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	sessionID := "test-session-id-no-workspace-1"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{},
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var createResult map[string]string
	ParseJSON(t, resp, &createResult)
	if createResult["sessionId"] != sessionID {
		t.Fatalf("Expected created session id %q, got %q", sessionID, createResult["sessionId"])
	}

	sessionResp := client.Get("/api/projects/" + project.ID + "/sessions/" + sessionID)
	defer sessionResp.Body.Close()
	AssertStatus(t, sessionResp, http.StatusOK)

	var sessionResult map[string]any
	ParseJSON(t, sessionResp, &sessionResult)

	workspaceID, ok := sessionResult["workspaceId"].(string)
	if !ok || workspaceID == "" {
		t.Fatalf("Expected non-empty workspaceId on created session, got %v", sessionResult["workspaceId"])
	}

	workspaceResp := client.Get("/api/projects/" + project.ID + "/workspaces/" + workspaceID)
	defer workspaceResp.Body.Close()
	AssertStatus(t, workspaceResp, http.StatusOK)

	var workspaceResult map[string]any
	ParseJSON(t, workspaceResp, &workspaceResult)

	path, ok := workspaceResult["path"].(string)
	if !ok || path == "" {
		t.Fatalf("Expected non-empty workspace path, got %v", workspaceResult["path"])
	}
	if !strings.HasPrefix(path, ts.Config.WorkspaceDir) {
		t.Fatalf("Expected workspace path %q to be under %q", path, ts.Config.WorkspaceDir)
	}
	if workspaceResult["sourceType"] != "managed" {
		t.Fatalf("Expected sourceType managed, got %v", workspaceResult["sourceType"])
	}
	if workspaceResult["autoGenerated"] != true {
		t.Fatalf("Expected autoGenerated true, got %v", workspaceResult["autoGenerated"])
	}
}

func TestCreateSession_ViaChatWithoutWorkspace(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	sessionID := "test-session-id-no-workspace-2"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "Create a session without selecting a workspace"},
				},
			},
		},
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	sessionResp := client.Get("/api/projects/" + project.ID + "/sessions/" + sessionID)
	defer sessionResp.Body.Close()
	AssertStatus(t, sessionResp, http.StatusOK)

	var sessionResult map[string]any
	ParseJSON(t, sessionResp, &sessionResult)

	workspaceID, ok := sessionResult["workspaceId"].(string)
	if !ok || workspaceID == "" {
		t.Fatalf("Expected non-empty workspaceId on chat-created session, got %v", sessionResult["workspaceId"])
	}

	workspaceResp := client.Get("/api/projects/" + project.ID + "/workspaces/" + workspaceID)
	defer workspaceResp.Body.Close()
	AssertStatus(t, workspaceResp, http.StatusOK)

	var workspaceResult map[string]any
	ParseJSON(t, workspaceResp, &workspaceResult)

	path, ok := workspaceResult["path"].(string)
	if !ok || path == "" {
		t.Fatalf("Expected non-empty workspace path, got %v", workspaceResult["path"])
	}
	if !strings.HasPrefix(path, ts.Config.WorkspaceDir) {
		t.Fatalf("Expected workspace path %q to be under %q", path, ts.Config.WorkspaceDir)
	}
	if workspaceResult["sourceType"] != "managed" {
		t.Fatalf("Expected sourceType managed, got %v", workspaceResult["sourceType"])
	}
	if workspaceResult["autoGenerated"] != true {
		t.Fatalf("Expected autoGenerated true, got %v", workspaceResult["autoGenerated"])
	}
}

func TestCreateSession_ViaChatWithWorkspace(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	// Sessions are created implicitly via the chat endpoint with workspace
	// Format matches the thread chat UIMessage payload
	sessionID := "test-session-id-2"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "Hello agent"},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	// Verify session was created by listing sessions
	listResp := client.Get("/api/projects/" + project.ID + "/sessions")
	defer listResp.Body.Close()

	var result struct {
		Sessions []map[string]any `json:"sessions"`
	}
	ParseJSON(t, listResp, &result)

	if len(result.Sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(result.Sessions))
		return
	}
}

func TestCreateSession_NameRemainsEmptyAfterPrompt(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	// Session name stays empty until it is populated elsewhere.
	longPrompt := "This is a very long prompt that should be truncated to fit within the 50 character limit for session names"
	sessionID := "test-session-id-3"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": longPrompt},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	// Verify session name remains empty.
	listResp := client.Get("/api/projects/" + project.ID + "/sessions")
	defer listResp.Body.Close()

	var result struct {
		Sessions []map[string]any `json:"sessions"`
	}
	ParseJSON(t, listResp, &result)

	if len(result.Sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(result.Sessions))
		return
	}

	name := result.Sessions[0]["name"].(string)
	if name != "" {
		t.Errorf("Expected empty session name, got %q", name)
	}
}

func TestGetSession(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSession(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/sessions/" + session.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	ParseJSON(t, resp, &result)

	if result["id"] != session.ID {
		t.Errorf("Expected id '%s', got '%v'", session.ID, result["id"])
	}
	if result["name"] != "Test Session" {
		t.Errorf("Expected name 'Test Session', got '%v'", result["name"])
	}
}

func TestUpdateSession(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSession(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	resp := client.Put("/api/projects/"+project.ID+"/sessions/"+session.ID, map[string]string{
		"name":   "Updated Session",
		"status": "stopped",
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	ParseJSON(t, resp, &result)

	if result["name"] != "Updated Session" {
		t.Errorf("Expected name 'Updated Session', got '%v'", result["name"])
	}
	if result["status"] != "stopped" {
		t.Errorf("Expected status 'stopped', got '%v'", result["status"])
	}
}

func TestDeleteSession(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSession(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	resp := client.Delete("/api/projects/" + project.ID + "/sessions/" + session.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	// Verify session status is "removing" (async deletion)
	resp = client.Get("/api/projects/" + project.ID + "/sessions/" + session.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)
	var result map[string]any
	ParseJSON(t, resp, &result)
	if result["status"] != "removing" {
		t.Errorf("Expected status 'removing', got '%v'", result["status"])
	}
}

func TestListSessionsByProject_WithData(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	ts.CreateTestSession(workspace, "Session 1")
	ts.CreateTestSession(workspace, "Session 2")
	ts.CreateTestSession(workspace, "Session 3")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/sessions")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Sessions []struct {
			ID        string `json:"id"`
			CreatedAt string `json:"createdAt"`
			Timestamp string `json:"timestamp"`
		} `json:"sessions"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(result.Sessions))
	}

	for _, session := range result.Sessions {
		if session.CreatedAt == "" {
			t.Fatalf("Expected session %s to include createdAt", session.ID)
		}
		if _, err := time.Parse(time.RFC3339, session.CreatedAt); err != nil {
			t.Fatalf("Expected session %s createdAt to be RFC3339, got %q: %v", session.ID, session.CreatedAt, err)
		}
		if session.Timestamp == "" {
			t.Fatalf("Expected session %s to include timestamp", session.ID)
		}
	}
}

func TestListSessionFiles(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	client := ts.AuthenticatedClient(user)

	// Create a session with sandbox (uses mock provider's default handler which supports /files)
	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")

	resp := client.Get("/api/projects/" + project.ID + "/sessions/" + session.ID + "/files?path=.")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Path    string `json:"path"`
		Entries []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Size int64  `json:"size,omitempty"`
		} `json:"entries"`
	}
	ParseJSON(t, resp, &result)

	// Mock returns README.md and src directory
	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result.Entries))
	}
	if result.Path != "." {
		t.Errorf("Expected path '.', got %s", result.Path)
	}
}

func TestListMessagesRouteNotFound(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSession(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/sessions/" + session.ID + "/messages")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusNotFound)
}

// ============================================================================
// Session Commit Tests
// ============================================================================

func TestCommitSessionRouteNotFound(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/sessions/"+session.ID+"/commit", nil)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusNotFound)
}

func TestGetSession_MapsInProgressCommitIntoStatusAndIncludesTargetRef(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	// Set internal commit state and verify the public response is flattened.
	session.CommitStatus = "committing"
	operation := "commit"
	session.CommitOperation = &operation
	targetRef := "HEAD"
	session.TargetRef = &targetRef
	if err := ts.Store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	resp := client.Get("/api/projects/" + project.ID + "/sessions/" + session.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	ParseJSON(t, resp, &result)

	if result["status"] != "committing" {
		t.Errorf("Expected status 'committing', got %v", result["status"])
	}
	if result["sandboxStatus"] != "ready" {
		t.Errorf("Expected sandboxStatus 'ready', got %v", result["sandboxStatus"])
	}
	if result["targetRef"] != "HEAD" {
		t.Errorf("Expected targetRef 'HEAD', got %v", result["targetRef"])
	}
	if _, ok := result["commitStatus"]; ok {
		t.Errorf("Expected commitStatus to be omitted, got %v", result["commitStatus"])
	}
	if _, ok := result["commitOperation"]; ok {
		t.Errorf("Expected commitOperation to be omitted, got %v", result["commitOperation"])
	}
}

func TestGetSession_MapsFailedCommitIntoStatusAndError(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	// Set commit status to failed with error.
	session.CommitStatus = "failed"
	commitError := "Patch conflict on file.txt"
	session.CommitError = &commitError
	if err := ts.Store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	resp := client.Get("/api/projects/" + project.ID + "/sessions/" + session.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	ParseJSON(t, resp, &result)

	if result["status"] != "error" {
		t.Errorf("Expected status 'error', got %v", result["status"])
	}
	if result["errorMessage"] != "Patch conflict on file.txt" {
		t.Errorf("Expected errorMessage 'Patch conflict on file.txt', got %v", result["errorMessage"])
	}
	if _, ok := result["commitStatus"]; ok {
		t.Errorf("Expected commitStatus to be omitted, got %v", result["commitStatus"])
	}
	if _, ok := result["commitError"]; ok {
		t.Errorf("Expected commitError to be omitted, got %v", result["commitError"])
	}
}

func TestListSessions_SeparatesLifecycleAndCommitStatus(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/home/user/code")
	session := ts.CreateTestSessionWithSandbox(workspace, "Test Session")
	client := ts.AuthenticatedClient(user)

	session.CommitStatus = model.CommitStatusCompleted
	commitOperation := model.CommitOperationCommit
	session.CommitOperation = &commitOperation
	appliedCommit := "def456"
	session.AppliedCommit = &appliedCommit
	if err := ts.Store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	resp := client.Get("/api/projects/" + project.ID + "/sessions")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Sessions []map[string]any `json:"sessions"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(result.Sessions))
	}

	if result.Sessions[0]["status"] != model.SessionStatusReady {
		t.Errorf("Expected status %q, got %v", model.SessionStatusReady, result.Sessions[0]["status"])
	}
	if result.Sessions[0]["sandboxStatus"] != model.SessionStatusReady {
		t.Errorf("Expected sandboxStatus %q, got %v", model.SessionStatusReady, result.Sessions[0]["sandboxStatus"])
	}
	if result.Sessions[0]["commitStatus"] != model.CommitStatusCompleted {
		t.Errorf("Expected commitStatus %q, got %v", model.CommitStatusCompleted, result.Sessions[0]["commitStatus"])
	}
	if result.Sessions[0]["commitOperation"] != model.CommitOperationCommit {
		t.Errorf("Expected commitOperation %q, got %v", model.CommitOperationCommit, result.Sessions[0]["commitOperation"])
	}
	if result.Sessions[0]["appliedCommit"] != "def456" {
		t.Errorf("Expected appliedCommit 'def456', got %v", result.Sessions[0]["appliedCommit"])
	}
	if _, ok := result.Sessions[0]["commitError"]; ok {
		t.Errorf("Expected commitError to be omitted, got %v", result.Sessions[0]["commitError"])
	}
}
