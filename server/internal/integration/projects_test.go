package integration

import (
	"net/http"
	"testing"
)

func TestListProjects_Unauthenticated(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)

	resp, err := http.Get(ts.Server.URL + "/api/projects")
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusUnauthorized)
}

func TestListProjects_Empty(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var projects []any
	ParseJSON(t, resp, &projects)

	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}
}

func TestCreateProject(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects", map[string]string{
		"name": "Test Project",
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusCreated)

	var project map[string]any
	ParseJSON(t, resp, &project)

	if project["name"] != "Test Project" {
		t.Errorf("Expected name 'Test Project', got '%v'", project["name"])
	}
	if project["id"] == nil || project["id"] == "" {
		t.Error("Expected project to have an ID")
	}
}

func TestCreateProject_MissingName(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects", map[string]string{})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestGetProject(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	ParseJSON(t, resp, &result)

	if result["id"] != project.ID {
		t.Errorf("Expected id '%s', got '%v'", project.ID, result["id"])
	}
}

func TestGetProject_NotMember(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	owner := ts.CreateTestUser("owner@example.com")
	other := ts.CreateTestUser("other@example.com")
	project := ts.CreateTestProject(owner, "Test Project")
	client := ts.AuthenticatedClient(other)

	resp := client.Get("/api/projects/" + project.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusForbidden)
}

func TestUpdateProject(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Put("/api/projects/"+project.ID, map[string]string{
		"name": "Updated Project",
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	ParseJSON(t, resp, &result)

	if result["name"] != "Updated Project" {
		t.Errorf("Expected name 'Updated Project', got '%v'", result["name"])
	}
}

func TestDeleteProject(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Delete("/api/projects/" + project.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	// Verify project is deleted
	resp = client.Get("/api/projects/" + project.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusForbidden) // No longer a member since project is deleted
}

func TestGetProviderResources(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/resources")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	ParseJSON(t, resp, &result)

	if result["provider"] != "mock" {
		t.Errorf("Expected provider 'mock', got '%v'", result["provider"])
	}

	vm, ok := result["vm"].(map[string]any)
	if !ok {
		t.Fatalf("Expected vm object, got %#v", result["vm"])
	}
	if vm["memoryMB"] != float64(4096) {
		t.Errorf("Expected memoryMB 4096, got %v", vm["memoryMB"])
	}
	if vm["dataDiskGB"] != float64(100) {
		t.Errorf("Expected dataDiskGB 100, got %v", vm["dataDiskGB"])
	}
}

func TestUpdateProviderResources(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/resources", map[string]any{
		"memoryMB":   8192,
		"dataDiskGB": 200,
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]any
	ParseJSON(t, resp, &result)

	current, ok := result["current"].(map[string]any)
	if !ok {
		t.Fatalf("Expected current object, got %#v", result["current"])
	}
	if current["memoryMB"] != float64(8192) {
		t.Errorf("Expected memoryMB 8192, got %v", current["memoryMB"])
	}
	if current["dataDiskGB"] != float64(200) {
		t.Errorf("Expected dataDiskGB 200, got %v", current["dataDiskGB"])
	}
	if result["restartRequired"] != true {
		t.Errorf("Expected restartRequired true, got %v", result["restartRequired"])
	}

	storedProject, err := ts.Store.GetProjectByID(t.Context(), project.ID)
	if err != nil {
		t.Fatalf("Failed to load project: %v", err)
	}
	if storedProject.VZMemoryMB == nil || *storedProject.VZMemoryMB != 8192 {
		t.Fatalf("Expected stored VZ memory override 8192, got %#v", storedProject.VZMemoryMB)
	}
	if storedProject.VZDataDiskGB == nil || *storedProject.VZDataDiskGB != 200 {
		t.Fatalf("Expected stored VZ disk override 200, got %#v", storedProject.VZDataDiskGB)
	}
}

func TestUpdateProviderResources_DiskCannotDecrease(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/resources", map[string]any{
		"dataDiskGB": 50,
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestUpdateProviderResources_MemoryMustBeWholeGiB(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/resources", map[string]any{
		"memoryMB": 2500,
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestListProjectMembers(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/members")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var members []map[string]any
	ParseJSON(t, resp, &members)

	if len(members) != 1 {
		t.Errorf("Expected 1 member, got %d", len(members))
	}
}

func TestCreateInvitation(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/invitations", map[string]string{
		"email": "newuser@example.com",
		"role":  "member",
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusCreated)

	var invitation map[string]any
	ParseJSON(t, resp, &invitation)

	if invitation["email"] != "newuser@example.com" {
		t.Errorf("Expected email 'newuser@example.com', got '%v'", invitation["email"])
	}
}
