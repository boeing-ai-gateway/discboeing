package integration

import (
	"net/http"
	"testing"
)

type credentialResponse struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
}

func TestListCredentials_Empty(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	resp := client.Get("/api/projects/" + project.ID + "/credentials")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Credentials []map[string]any `json:"credentials"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Credentials) != 0 {
		t.Errorf("Expected empty credentials list, got %d", len(result.Credentials))
	}
}

func TestCreateCredential_APIKey(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "anthropic",
		"name":     "My Anthropic Key",
		"apiKey":   "sk-ant-test-123456",
	})
	AssertStatus(t, resp, http.StatusOK)

	var cred map[string]any
	ParseJSON(t, resp, &cred)

	if cred["provider"] != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got %v", cred["provider"])
	}
	if cred["name"] != "My Anthropic Key" {
		t.Errorf("Expected name 'My Anthropic Key', got %v", cred["name"])
	}
	if cred["authType"] != "api_key" {
		t.Errorf("Expected authType 'api_key', got %v", cred["authType"])
	}
	if cred["isConfigured"] != true {
		t.Errorf("Expected isConfigured true, got %v", cred["isConfigured"])
	}
	// Verify the API key is NOT returned
	if _, ok := cred["apiKey"]; ok {
		t.Error("API key should not be returned in response")
	}
}

func TestCreateCredential_ID(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "discobot",
		"name":     "My Discobot ID",
		"authType": "id",
		"apiKey":   "discobot-test-id-123",
	})
	AssertStatus(t, resp, http.StatusOK)

	var cred map[string]any
	ParseJSON(t, resp, &cred)

	if cred["provider"] != "discobot" {
		t.Errorf("Expected provider 'discobot', got %v", cred["provider"])
	}
	if cred["name"] != "My Discobot ID" {
		t.Errorf("Expected name 'My Discobot ID', got %v", cred["name"])
	}
	if cred["authType"] != "id" {
		t.Errorf("Expected authType 'id', got %v", cred["authType"])
	}
	if cred["isConfigured"] != true {
		t.Errorf("Expected isConfigured true, got %v", cred["isConfigured"])
	}
	if _, ok := cred["apiKey"]; ok {
		t.Error("credential secret should not be returned in response")
	}
}

func TestCreateCredential_BlankNameRemainsEmptyForBuiltInProvider(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "anthropic",
		"apiKey":   "sk-ant-test-123456",
	})
	AssertStatus(t, resp, http.StatusOK)

	var cred credentialResponse
	ParseJSON(t, resp, &cred)
	if cred.Name != "" {
		t.Fatalf("expected blank credential name, got %q", cred.Name)
	}
}

func TestCreateCredential_BlankNameRemainsEmptyForCustomEnvVars(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]any{
		"envVars": []map[string]string{{
			"key":   "FOO_TOKEN",
			"value": "foo-secret",
		}, {
			"key":   "BAR_TOKEN",
			"value": "bar-secret",
		}},
	})
	AssertStatus(t, resp, http.StatusOK)

	var cred credentialResponse
	ParseJSON(t, resp, &cred)
	if cred.Name != "" {
		t.Fatalf("expected blank credential name, got %q", cred.Name)
	}
	if cred.Provider == "" {
		t.Fatal("expected generated custom credential provider")
	}
}

func TestCreateCredential_MissingAPIKey(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "anthropic",
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestCreateCredential_InvalidProvider(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "invalid-provider",
		"apiKey":   "sk-test-123",
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestGetCredential(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)

	// Create credential first
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "openai",
		"name":     "My OpenAI Key",
		"apiKey":   "sk-test-openai-123",
	})
	AssertStatus(t, resp, http.StatusOK)

	// Get the credential
	resp = client.Get("/api/projects/" + project.ID + "/credentials/openai")
	AssertStatus(t, resp, http.StatusOK)

	var cred map[string]any
	ParseJSON(t, resp, &cred)

	if cred["provider"] != "openai" {
		t.Errorf("Expected provider 'openai', got %v", cred["provider"])
	}
	if cred["name"] != "My OpenAI Key" {
		t.Errorf("Expected name 'My OpenAI Key', got %v", cred["name"])
	}
}

func TestGetCredential_NotFound(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	resp := client.Get("/api/projects/" + project.ID + "/credentials/anthropic")
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestGetCredentialByID_DoesNotReturnSecretValues(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)
	createResp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]any{
		"provider": "custom",
		"name":     "Secrets",
		"envVars": []map[string]string{{
			"key":   "FOO_TOKEN",
			"value": "foo-secret",
		}, {
			"key":   "BAR_TOKEN",
			"value": "bar-secret",
		}},
	})
	AssertStatus(t, createResp, http.StatusOK)

	var created credentialResponse
	ParseJSON(t, createResp, &created)

	resp := client.Get("/api/projects/" + project.ID + "/credentials/" + created.ID)
	AssertStatus(t, resp, http.StatusOK)

	var credential map[string]any
	ParseJSON(t, resp, &credential)
	if _, ok := credential["envVars"]; ok {
		t.Fatal("expected envVars to be omitted from credential response")
	}
	if credential["envKeys"] == nil {
		t.Fatal("expected envKeys to remain available in credential response")
	}
}

func TestDeleteCredential(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)

	// Create credential first
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "anthropic",
		"apiKey":   "sk-test-123",
	})
	AssertStatus(t, resp, http.StatusOK)

	// Delete it
	resp = client.Delete("/api/projects/" + project.ID + "/credentials/anthropic")
	AssertStatus(t, resp, http.StatusOK)

	// Verify it's gone
	resp = client.Get("/api/projects/" + project.ID + "/credentials/anthropic")
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestUpdateCredential(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)

	// Create credential
	resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "anthropic",
		"name":     "Original Name",
		"apiKey":   "sk-old-key",
	})
	AssertStatus(t, resp, http.StatusOK)

	// Update it (same provider creates/updates)
	resp = client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
		"provider": "anthropic",
		"name":     "Updated Name",
		"apiKey":   "sk-new-key",
	})
	AssertStatus(t, resp, http.StatusOK)

	var cred map[string]any
	ParseJSON(t, resp, &cred)

	if cred["name"] != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %v", cred["name"])
	}

	// Verify only one credential exists
	resp = client.Get("/api/projects/" + project.ID + "/credentials")
	AssertStatus(t, resp, http.StatusOK)

	var credList struct {
		Credentials []map[string]any `json:"credentials"`
	}
	ParseJSON(t, resp, &credList)

	if len(credList.Credentials) != 1 {
		t.Errorf("Expected 1 credential, got %d", len(credList.Credentials))
	}
}

func TestListCredentials_WithData(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)

	// Create multiple credentials
	providers := []string{"anthropic", "openai", "github-copilot"}
	for _, provider := range providers {
		resp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]string{
			"provider": provider,
			"apiKey":   "sk-test-" + provider,
		})
		AssertStatus(t, resp, http.StatusOK)
	}

	// List all
	resp := client.Get("/api/projects/" + project.ID + "/credentials")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Credentials []map[string]any `json:"credentials"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Credentials) != 3 {
		t.Errorf("Expected 3 credentials, got %d", len(result.Credentials))
	}
}

func TestUpdateCredentialByForeignIDReturnsNotFound(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	projectA := ts.CreateTestProject(user, "cred-project-a")
	projectB := ts.CreateTestProject(user, "cred-project-b")

	client := ts.AuthenticatedClient(user)
	createResp := client.Post("/api/projects/"+projectB.ID+"/credentials", map[string]string{
		"provider": "anthropic",
		"name":     "Project B credential",
		"apiKey":   "sk-project-b",
	})
	AssertStatus(t, createResp, http.StatusOK)

	var created credentialResponse
	ParseJSON(t, createResp, &created)

	resp := client.Post("/api/projects/"+projectA.ID+"/credentials", map[string]string{
		"credentialId": created.ID,
		"provider":     "anthropic",
		"name":         "Cross-project overwrite",
		"apiKey":       "sk-project-a",
	})
	AssertStatus(t, resp, http.StatusNotFound)

	resp = client.Get("/api/projects/" + projectA.ID + "/credentials/anthropic")
	AssertStatus(t, resp, http.StatusNotFound)

	resp = client.Get("/api/projects/" + projectB.ID + "/credentials/" + created.ID)
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &created)
	if created.Name != "Project B credential" {
		t.Fatalf("expected project B credential to remain unchanged, got %q", created.Name)
	}
}

func TestUpdateCustomCredentialByForeignIDReturnsNotFound(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	projectA := ts.CreateTestProject(user, "cred-project-a")
	projectB := ts.CreateTestProject(user, "cred-project-b")

	client := ts.AuthenticatedClient(user)
	createResp := client.Post("/api/projects/"+projectB.ID+"/credentials", map[string]any{
		"provider": "custom",
		"name":     "Project B custom credential",
		"envVars": []map[string]string{{
			"key":   "PROJECT_B_TOKEN",
			"value": "secret-b",
		}},
	})
	AssertStatus(t, createResp, http.StatusOK)

	var created credentialResponse
	ParseJSON(t, createResp, &created)

	resp := client.Post("/api/projects/"+projectA.ID+"/credentials", map[string]any{
		"credentialId": created.ID,
		"provider":     "custom",
		"name":         "Cross-project custom overwrite",
		"envVars": []map[string]string{{
			"key":   "PROJECT_A_TOKEN",
			"value": "secret-a",
		}},
	})
	AssertStatus(t, resp, http.StatusNotFound)

	resp = client.Get("/api/projects/" + projectB.ID + "/credentials/" + created.ID)
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &created)
	if created.Name != "Project B custom credential" {
		t.Fatalf("expected project B custom credential to remain unchanged, got %q", created.Name)
	}
}

func TestDeleteCredentialByForeignIDReturnsNotFound(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	projectA := ts.CreateTestProject(user, "cred-project-a")
	projectB := ts.CreateTestProject(user, "cred-project-b")

	client := ts.AuthenticatedClient(user)
	createResp := client.Post("/api/projects/"+projectB.ID+"/credentials", map[string]string{
		"provider": "openai",
		"name":     "Project B OpenAI",
		"apiKey":   "sk-project-b-openai",
	})
	AssertStatus(t, createResp, http.StatusOK)

	var created credentialResponse
	ParseJSON(t, createResp, &created)

	resp := client.Delete("/api/projects/" + projectA.ID + "/credentials/" + created.ID)
	AssertStatus(t, resp, http.StatusNotFound)

	resp = client.Get("/api/projects/" + projectB.ID + "/credentials/" + created.ID)
	AssertStatus(t, resp, http.StatusOK)
}

func TestSetSessionCredentialsRejectsForeignCredentialID(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	projectA := ts.CreateTestProject(user, "cred-project-a")
	projectB := ts.CreateTestProject(user, "cred-project-b")

	workspace := ts.CreateTestWorkspace(projectA, "/tmp/project-a")
	session := ts.CreateTestSession(workspace, "project-a-session")
	client := ts.AuthenticatedClient(user)

	createResp := client.Post("/api/projects/"+projectB.ID+"/credentials", map[string]string{
		"provider": "anthropic",
		"name":     "Project B Anthropic",
		"apiKey":   "sk-project-b-anthropic",
	})
	AssertStatus(t, createResp, http.StatusOK)

	var created credentialResponse
	ParseJSON(t, createResp, &created)

	resp := client.Put("/api/projects/"+projectA.ID+"/sessions/"+session.ID+"/credentials", map[string]any{
		"credentials": []map[string]any{{
			"credentialId": created.ID,
			"agentVisible": true,
		}},
	})
	AssertStatus(t, resp, http.StatusNotFound)
}

func TestEditCustomCredential_KeyRename_PreservesSecret(t *testing.T) {
	// Renaming a key via the frontend sends originalKey so the backend can carry the
	// existing secret to the new key name.
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)

	createResp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]any{
		"envVars": []map[string]string{
			{"key": "FOO_TOKEN", "value": "foo-secret"},
			{"key": "BAR_TOKEN", "value": "bar-secret"},
		},
	})
	AssertStatus(t, createResp, http.StatusOK)

	var created struct {
		ID      string   `json:"id"`
		EnvKeys []string `json:"envKeys"`
	}
	ParseJSON(t, createResp, &created)

	// Rename FOO_TOKEN → FOO_RENAMED, leave BAR_TOKEN unchanged.
	editResp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]any{
		"credentialId": created.ID,
		"authType":     "api_key",
		"envVars": []map[string]string{
			{"key": "FOO_RENAMED", "value": "", "originalKey": "FOO_TOKEN"},
			{"key": "BAR_TOKEN", "value": ""},
		},
	})
	AssertStatus(t, editResp, http.StatusOK)

	var edited struct {
		EnvKeys []string `json:"envKeys"`
	}
	ParseJSON(t, editResp, &edited)

	if len(edited.EnvKeys) != 2 {
		t.Fatalf("expected 2 envKeys after rename, got %v", edited.EnvKeys)
	}
	found := map[string]bool{}
	for _, k := range edited.EnvKeys {
		found[k] = true
	}
	if !found["FOO_RENAMED"] {
		t.Errorf("expected FOO_RENAMED in envKeys, got %v", edited.EnvKeys)
	}
	if !found["BAR_TOKEN"] {
		t.Errorf("expected BAR_TOKEN in envKeys, got %v", edited.EnvKeys)
	}
	if found["FOO_TOKEN"] {
		t.Error("FOO_TOKEN should no longer appear after rename")
	}
}

func TestEditCustomCredential_FrontendEditPattern_PreservesOtherEnvVars(t *testing.T) {
	// This test simulates the exact request the frontend sends when editing a custom
	// credential. The frontend sends credentialId + authType but NO provider field,
	// along with all env var rows (some with new values, others with blank values
	// representing "keep the stored value").
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("cred@test.com")
	project := ts.CreateTestProject(user, "cred-project")

	client := ts.AuthenticatedClient(user)

	// Step 1: create a custom credential with 3 env vars (simulates initial create)
	createResp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]any{
		"envVars": []map[string]string{
			{"key": "FOO_TOKEN", "value": "foo-secret"},
			{"key": "BAR_TOKEN", "value": "bar-secret"},
			{"key": "BAZ_TOKEN", "value": "baz-secret"},
		},
	})
	AssertStatus(t, createResp, http.StatusOK)

	var created struct {
		ID       string   `json:"id"`
		EnvKeys  []string `json:"envKeys"`
		Provider string   `json:"provider"`
	}
	ParseJSON(t, createResp, &created)

	if len(created.EnvKeys) != 3 {
		t.Fatalf("expected 3 envKeys after create, got %v", created.EnvKeys)
	}

	// Step 2: simulate the frontend's edit request – only provider is absent,
	// credentialId is set, one env var gets a new value, others are blank (stored value kept).
	// This is the EXACT format the frontend's save() function sends for custom credentials.
	editResp := client.Post("/api/projects/"+project.ID+"/credentials", map[string]any{
		"credentialId": created.ID,
		"name":         "My Creds",
		"authType":     "api_key",
		"envVars": []map[string]string{
			{"key": "FOO_TOKEN", "value": "new-foo-secret"}, // updated
			{"key": "BAR_TOKEN", "value": ""},               // blank = keep stored
			{"key": "BAZ_TOKEN", "value": ""},               // blank = keep stored
		},
		"visibility": map[string]bool{
			"tools":    false,
			"console":  false,
			"services": false,
			"hooks":    false,
		},
		"inactive": false,
	})
	AssertStatus(t, editResp, http.StatusOK)

	var edited struct {
		ID      string   `json:"id"`
		EnvKeys []string `json:"envKeys"`
	}
	ParseJSON(t, editResp, &edited)

	// All 3 env vars must still be present after the edit.
	if len(edited.EnvKeys) != 3 {
		t.Fatalf("expected 3 envKeys after edit, got %v (all env vars were deleted!)", edited.EnvKeys)
	}

	// Verify from the list endpoint too.
	listResp := client.Get("/api/projects/" + project.ID + "/credentials")
	AssertStatus(t, listResp, http.StatusOK)

	var listResult struct {
		Credentials []struct {
			ID      string   `json:"id"`
			EnvKeys []string `json:"envKeys"`
		} `json:"credentials"`
	}
	ParseJSON(t, listResp, &listResult)

	if len(listResult.Credentials) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(listResult.Credentials))
	}
	if len(listResult.Credentials[0].EnvKeys) != 3 {
		t.Fatalf("expected 3 envKeys in list, got %v", listResult.Credentials[0].EnvKeys)
	}
}
