package integration

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestDeleteProjectCacheVolume_CallsProviderClearCache(t *testing.T) {
	t.Parallel()

	ts := NewTestServer(t)
	user := ts.CreateTestUser("cache-owner@example.com")
	project := ts.CreateTestProject(user, "cache-project")
	client := ts.AuthenticatedClient(user)

	called := 0
	ts.MockSandbox.ClearCacheFunc = func(_ context.Context, projectID string) error {
		called++
		if projectID != project.ID {
			t.Fatalf("projectID = %q, want %q", projectID, project.ID)
		}
		return nil
	}

	resp := client.Delete("/api/projects/" + project.ID + "/cache")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Success bool `json:"success"`
	}
	ParseJSON(t, resp, &result)

	if !result.Success {
		t.Fatal("expected success response")
	}
	if called != 1 {
		t.Fatalf("ClearCache called %d times, want 1", called)
	}
}

func TestDeleteProjectCacheVolume_PropagatesProviderError(t *testing.T) {
	t.Parallel()

	ts := NewTestServer(t)
	user := ts.CreateTestUser("cache-owner-error@example.com")
	project := ts.CreateTestProject(user, "cache-project-error")
	client := ts.AuthenticatedClient(user)

	ts.MockSandbox.ClearCacheFunc = func(_ context.Context, projectID string) error {
		if projectID != project.ID {
			t.Fatalf("projectID = %q, want %q", projectID, project.ID)
		}
		return errors.New("boom")
	}

	resp := client.Delete("/api/projects/" + project.ID + "/cache")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusInternalServerError)
}
