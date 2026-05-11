package integration

import (
	"net/http"
	"testing"
)

func TestSessionDisplayName_SetAndGet(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	// Create a session via chat
	sessionID := "test-displayname-session-1"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "Help me fix authentication bug"},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	resp.Body.Close()
	AssertStatus(t, resp, http.StatusOK)

	// Get the session - should have original name, no displayName
	getResp := client.Get("/api/projects/" + project.ID + "/sessions/" + sessionID)
	defer getResp.Body.Close()
	AssertStatus(t, getResp, http.StatusOK)

	var session map[string]any
	ParseJSON(t, getResp, &session)

	if session["name"] != "" {
		t.Errorf("Expected empty name initially, got '%v'", session["name"])
	}
	if _, exists := session["displayName"]; exists && session["displayName"] != nil && session["displayName"] != "" {
		t.Errorf("Expected no displayName initially, got '%v'", session["displayName"])
	}

	// Update the session with a displayName
	updateResp := client.Patch("/api/projects/"+project.ID+"/sessions/"+sessionID, map[string]any{
		"displayName": "Auth Bug Fix",
	})
	defer updateResp.Body.Close()
	AssertStatus(t, updateResp, http.StatusOK)

	var updatedSession map[string]any
	ParseJSON(t, updateResp, &updatedSession)

	// Verify displayName is set
	if updatedSession["displayName"] != "Auth Bug Fix" {
		t.Errorf("Expected displayName 'Auth Bug Fix', got '%v'", updatedSession["displayName"])
	}
	// Original name should stay empty until populated elsewhere.
	if updatedSession["name"] != "" {
		t.Errorf("Expected original name to remain empty, got '%v'", updatedSession["name"])
	}

	// Get session again to verify persistence
	getResp2 := client.Get("/api/projects/" + project.ID + "/sessions/" + sessionID)
	defer getResp2.Body.Close()
	AssertStatus(t, getResp2, http.StatusOK)

	var persistedSession map[string]any
	ParseJSON(t, getResp2, &persistedSession)

	if persistedSession["displayName"] != "Auth Bug Fix" {
		t.Errorf("Expected persisted displayName 'Auth Bug Fix', got '%v'", persistedSession["displayName"])
	}
	if persistedSession["name"] != "" {
		t.Errorf("Expected original name to remain empty after persistence, got '%v'", persistedSession["name"])
	}
}

func TestSessionDisplayName_ClearDisplayName(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	// Create a session
	sessionID := "test-displayname-session-2"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "Original prompt text"},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	resp.Body.Close()
	AssertStatus(t, resp, http.StatusOK)

	// Set a displayName
	updateResp := client.Patch("/api/projects/"+project.ID+"/sessions/"+sessionID, map[string]any{
		"displayName": "Custom Name",
	})
	updateResp.Body.Close()
	AssertStatus(t, updateResp, http.StatusOK)

	// Clear the displayName by setting it to null
	clearResp := client.Patch("/api/projects/"+project.ID+"/sessions/"+sessionID, map[string]any{
		"displayName": nil,
	})
	defer clearResp.Body.Close()
	AssertStatus(t, clearResp, http.StatusOK)

	var clearedSession map[string]any
	ParseJSON(t, clearResp, &clearedSession)

	// displayName should be cleared (null or empty)
	if clearedSession["displayName"] != nil && clearedSession["displayName"] != "" {
		t.Errorf("Expected displayName to be cleared, got '%v'", clearedSession["displayName"])
	}
	// Original name should still be empty until populated elsewhere.
	if clearedSession["name"] != "" {
		t.Errorf("Expected original name to remain empty, got '%v'", clearedSession["name"])
	}
}

func TestSessionDisplayName_InList(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	// Create multiple sessions with different displayName configurations
	session1ID := "test-displayname-list-1"
	resp1 := client.Post(threadChatPath(project.ID, session1ID, session1ID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "First session prompt"},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	resp1.Body.Close()
	AssertStatus(t, resp1, http.StatusOK)

	session2ID := "test-displayname-list-2"
	resp2 := client.Post(threadChatPath(project.ID, session2ID, session2ID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "Second session prompt"},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	resp2.Body.Close()
	AssertStatus(t, resp2, http.StatusOK)

	// Set displayName on second session only
	updateResp := client.Patch("/api/projects/"+project.ID+"/sessions/"+session2ID, map[string]any{
		"displayName": "My Custom Session",
	})
	updateResp.Body.Close()
	AssertStatus(t, updateResp, http.StatusOK)

	// List sessions and verify displayName is included
	listResp := client.Get("/api/projects/" + project.ID + "/sessions")
	defer listResp.Body.Close()
	AssertStatus(t, listResp, http.StatusOK)

	var result struct {
		Sessions []map[string]any `json:"sessions"`
	}
	ParseJSON(t, listResp, &result)

	if len(result.Sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(result.Sessions))
		return
	}

	// Find our sessions in the list
	var session1, session2 map[string]any
	for _, s := range result.Sessions {
		switch s["id"] {
		case session1ID:
			session1 = s
		case session2ID:
			session2 = s
		}
	}

	if session1 == nil || session2 == nil {
		t.Fatal("Could not find both sessions in list")
	}

	// Session 1 should have no displayName
	if _, exists := session1["displayName"]; exists && session1["displayName"] != nil && session1["displayName"] != "" {
		t.Errorf("Session 1 should have no displayName, got '%v'", session1["displayName"])
	}

	// Session 2 should have displayName
	if session2["displayName"] != "My Custom Session" {
		t.Errorf("Session 2 expected displayName 'My Custom Session', got '%v'", session2["displayName"])
	}

	// Both names should stay empty until populated elsewhere.
	if session1["name"] != "" {
		t.Errorf("Session 1 expected empty name, got '%v'", session1["name"])
	}
	if session2["name"] != "" {
		t.Errorf("Session 2 expected empty name, got '%v'", session2["name"])
	}
}

func TestSessionDisplayName_EmptyString(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	client := ts.AuthenticatedClient(user)

	// Create a session
	sessionID := "test-displayname-empty"
	resp := client.Post(threadChatPath(project.ID, sessionID, sessionID), map[string]any{
		"messages": []map[string]any{
			{
				"id":   "msg-1",
				"role": "user",
				"parts": []map[string]any{
					{"type": "text", "text": "Test prompt"},
				},
			},
		},
		"workspaceId": workspace.ID,
	})
	resp.Body.Close()
	AssertStatus(t, resp, http.StatusOK)

	// Try to set displayName to empty string (should be treated as clearing it)
	updateResp := client.Patch("/api/projects/"+project.ID+"/sessions/"+sessionID, map[string]any{
		"displayName": "",
	})
	defer updateResp.Body.Close()
	AssertStatus(t, updateResp, http.StatusOK)

	var updatedSession map[string]any
	ParseJSON(t, updateResp, &updatedSession)

	// Empty string should result in no displayName (or empty)
	if updatedSession["displayName"] != nil && updatedSession["displayName"] != "" {
		t.Errorf("Expected displayName to be empty/null, got '%v'", updatedSession["displayName"])
	}
	// Original name remains empty.
	if updatedSession["name"] != "" {
		t.Errorf("Expected original name to remain empty, got '%v'", updatedSession["name"])
	}
}
