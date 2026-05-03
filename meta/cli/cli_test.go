package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWhoamiCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/whoami" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
			t.Fatalf("authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"principal":{"type":"bootstrap"}}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := Execute([]string{"--url", server.URL, "--token", "token-123", "whoami"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"type": "bootstrap"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestRawCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := Execute([]string{"--url", server.URL, "raw", "GET", "/health"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"status": "ok"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestProjectsCommandWithOrg(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/org/example.com/projects" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := Execute([]string{"--url", server.URL, "projects", "--org", "example.com"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"items": []`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestOAuthListCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/org/example.com/oauth-applications" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"oauth_app_123","name":"github","provider":"github"}]}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := Execute([]string{"--url", server.URL, "oauth", "--org", "example.com", "list"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "NAME") || !strings.Contains(out.String(), "github") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestOAuthListCommandJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/org/example.com/oauth-applications" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"oauth_app_123","name":"github","provider":"github"}]}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	if err := Execute([]string{"--url", server.URL, "oauth", "--org", "example.com", "-o", "json", "list"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"name": "github"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestApplyOAuthApplicationCreatesWhenMissing(t *testing.T) {
	var sawList, sawCreate bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/org/example.com/oauth-applications":
			sawList = true
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/org/example.com/oauth-applications":
			sawCreate = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["name"] != "github" || body["provider"] != "github" || body["clientId"] != "client-id" || body["clientSecret"] != "client-secret" {
				t.Fatalf("unexpected body: %#v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"oauth_app_123","name":"github"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	file := writeTempApplyFile(t, `type: OAuthApplication
name: github
organization: example.com
provider: github
clientId: client-id
clientSecret: client-secret
redirectUris:
  - https://meta.example.com/callback
`)
	var out bytes.Buffer
	if err := Execute([]string{"--url", server.URL, "apply", "-f", file}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !sawList || !sawCreate {
		t.Fatalf("sawList=%v sawCreate=%v", sawList, sawCreate)
	}
	if !strings.Contains(out.String(), "oauthapplication/github created") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestApplyOAuthApplicationUpdatesExisting(t *testing.T) {
	var sawList, sawPatch bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/org/example.com/oauth-applications":
			sawList = true
			_, _ = w.Write([]byte(`{"items":[{"id":"oauth_app_123","name":"github","provider":"github"}]}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/org/example.com/oauth-applications/oauth_app_123":
			sawPatch = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if _, ok := body["clientSecret"]; ok {
				t.Fatalf("clientSecret should be omitted when not declared: %#v", body)
			}
			if body["name"] != "github" || body["clientId"] != "new-client-id" {
				t.Fatalf("unexpected body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"id":"oauth_app_123","name":"github"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	file := writeTempApplyFile(t, `type: OAuthApplication
name: github
organization: example.com
provider: github
clientId: new-client-id
`)
	var out bytes.Buffer
	if err := Execute([]string{"--url", server.URL, "apply", "-f", file}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !sawList || !sawPatch {
		t.Fatalf("sawList=%v sawPatch=%v", sawList, sawPatch)
	}
	if !strings.Contains(out.String(), "oauthapplication/github configured") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func writeTempApplyFile(t *testing.T, content string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "apply-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}
