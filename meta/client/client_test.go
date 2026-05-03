package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeneratedClientBuildsPathAndQuery(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(server.URL)
	state := "abc"
	_, err := client.Authorize(context.Background(), AuthorizeParams{
		ClientID:     "client-1",
		RedirectURI:  "https://app.example/callback",
		ResponseType: "code",
		Scope:        "openid profile",
		State:        &state,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/authorize" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotQuery == "" || gotQuery == "state=abc" {
		t.Fatalf("query was not populated correctly: %q", gotQuery)
	}
}

func TestGeneratedClientBuildsOrganizationPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(server.URL)
	_, err := client.ListOrganizationProjects(context.Background(), ListOrganizationProjectsParams{OrganizationDomain: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/org/example.com/projects" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestGeneratedClientBuildsJSONBodyFromParams(t *testing.T) {
	var gotPath string
	var gotContentType string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &gotBody); err != nil {
			t.Fatalf("decode body %q: %v", data, err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(server.URL)
	secret := "secret"
	_, err := client.CreateOrganizationOAuthApplication(context.Background(), CreateOrganizationOAuthApplicationParams{
		OrganizationDomain: "example.com",
		Name:               "GitHub Login",
		Provider:           "github",
		ClientID:           "github-client",
		ClientSecret:       &secret,
		RedirectURIs:       []string{"https://meta.example.com/oauth/github/callback"},
		GitHub:             map[string]any{"enterpriseBaseURL": "https://github.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/org/example.com/oauth-applications" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content type = %q", gotContentType)
	}
	if gotBody["name"] != "GitHub Login" || gotBody["provider"] != "github" || gotBody["clientId"] != "github-client" || gotBody["clientSecret"] != "secret" {
		t.Fatalf("body missing expected fields: %#v", gotBody)
	}
	if _, ok := gotBody["organizationDomain"]; ok {
		t.Fatalf("body should not include path params: %#v", gotBody)
	}
}

func TestGeneratedClientApplyCreatesResource(t *testing.T) {
	var calls []string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/org/example.com/oauth-applications":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/org/example.com/oauth-applications":
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &gotBody); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"oauth_app_1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL)
	result, err := client.Apply(context.Background(), "OAuthApplication", map[string]any{
		"type":         "OAuthApplication",
		"organization": "example.com",
		"name":         "GitHub Login",
		"provider":     "github",
		"clientId":     "github-client",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation != "created" || result.Type != "OAuthApplication" || result.Name != "GitHub Login" {
		t.Fatalf("result = %#v", result)
	}
	if len(calls) != 2 || calls[0] != "GET /v1/org/example.com/oauth-applications" || calls[1] != "POST /v1/org/example.com/oauth-applications" {
		t.Fatalf("calls = %#v", calls)
	}
	if gotBody["name"] != "GitHub Login" || gotBody["provider"] != "github" || gotBody["clientId"] != "github-client" {
		t.Fatalf("body = %#v", gotBody)
	}
}

func TestGeneratedClientApplyUpdatesResource(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/org/example.com/oauth-applications":
			_, _ = w.Write([]byte(`{"items":[{"id":"oauth_app_1","name":"GitHub Login","provider":"github"}]}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/org/example.com/oauth-applications/oauth_app_1":
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &gotBody); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"id":"oauth_app_1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL)
	result, err := client.ApplyOAuthApplication(context.Background(), ApplyOAuthApplicationParams{
		Organization: "example.com",
		Name:         "GitHub Login",
		Provider:     "github",
		RedirectURIs: []string{"https://meta.example.com/oauth/github/callback"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation != "configured" || result.ID != "oauth_app_1" {
		t.Fatalf("result = %#v", result)
	}
	if gotBody["clientSecret"] != nil {
		t.Fatalf("clientSecret should be omitted when absent: %#v", gotBody)
	}
	if _, ok := gotBody["redirectUris"]; !ok {
		t.Fatalf("redirectUris missing from body: %#v", gotBody)
	}
}
