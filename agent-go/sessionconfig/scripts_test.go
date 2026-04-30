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

func TestDiscoverScripts_SystemDir(t *testing.T) {
	root := t.TempDir()
	systemRoot := t.TempDir()
	originalRoots := discobotSystemRoots
	discobotSystemRoots = []string{systemRoot}
	t.Cleanup(func() { discobotSystemRoots = originalRoots })

	scriptsDir := filepath.Join(systemRoot, "scripts")
	mkdirAll(t, scriptsDir)
	writeFile(t, filepath.Join(scriptsDir, "release"), `#!/bin/sh
#---
# name: release
# description: Release from system dir
#---
echo ok
`)
	if err := os.Chmod(filepath.Join(scriptsDir, "release"), 0o755); err != nil {
		t.Fatal(err)
	}

	scripts, _, err := discoverScriptsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(scripts))
	}
	if scripts[0].Name != "release" {
		t.Fatalf("script name = %q, want release", scripts[0].Name)
	}
}

func TestLookupScript_SystemDir(t *testing.T) {
	root := t.TempDir()
	systemRoot := t.TempDir()
	originalRoots := discobotSystemRoots
	discobotSystemRoots = []string{systemRoot}
	t.Cleanup(func() { discobotSystemRoots = originalRoots })

	scriptsDir := filepath.Join(systemRoot, "scripts")
	mkdirAll(t, scriptsDir)
	path := filepath.Join(scriptsDir, "release")
	writeFile(t, path, `#!/bin/sh
#---
# name: release
# description: Release from system dir
#---
echo ok
`)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}

	script, found, err := lookupScriptWithHome(root, "release", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected script to be found")
	}
	if script.Name != "release" {
		t.Fatalf("script name = %q, want release", script.Name)
	}
}
