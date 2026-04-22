package sessionconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadScriptFile_ParsesDiscobotMetadataAndCredentialRequests(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discobot-commit-remote")
	content := `#!/usr/bin/env bash
#---
# name: discobot-commit
# description: Commit session changes by opening a PR upstream
# discobot-ui: true
# discobot-label: Commit
# discobot-icon: git-commit
# discobot-group: Git
# discobot-order: 10
# discobot-credential-request:
#   - env-var: GH_TOKEN
#     name: GitHub credential
#     justification: Authenticate PR creation.
#     approved-uses:
#       - description: authenticate GitHub CLI
#---
printf 'ok\n'
`
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	script, ok, err := loadScriptFile(path, "discobot-commit-remote")
	if err != nil {
		t.Fatalf("loadScriptFile returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected script to load")
	}
	if script.Name != "discobot-commit" {
		t.Fatalf("script name = %q, want discobot-commit", script.Name)
	}
	if !script.Discobot.UI {
		t.Fatal("expected discobot ui metadata")
	}
	if script.Discobot.Icon != "git-commit" {
		t.Fatalf("discobot icon = %q, want git-commit", script.Discobot.Icon)
	}
	if len(script.Discobot.CredentialRequest) != 1 {
		t.Fatalf("credential requests = %d, want 1", len(script.Discobot.CredentialRequest))
	}
	request := script.Discobot.CredentialRequest[0]
	if request.EnvVar != "GH_TOKEN" {
		t.Fatalf("credential env var = %q, want GH_TOKEN", request.EnvVar)
	}
	if len(request.ApprovedUses) != 1 || request.ApprovedUses[0].Description != "authenticate GitHub CLI" {
		t.Fatalf("approved uses = %#v", request.ApprovedUses)
	}
}
