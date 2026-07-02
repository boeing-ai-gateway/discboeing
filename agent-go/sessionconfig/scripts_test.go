package sessionconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadScriptFile_ParsesDiscboeingMetadataAndCredentialRequests(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discboeing-commit-remote")
	content := `#!/usr/bin/env bash
#---
# name: discboeing-commit
# description: Commit session changes by opening a PR upstream
# discboeing-ui: true
# discboeing-label: Commit
# discboeing-icon: git-commit
# discboeing-group: Git
# discboeing-order: 10
# discboeing-credential-request:
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

	script, ok, err := loadScriptFile(path, "discboeing-commit-remote")
	if err != nil {
		t.Fatalf("loadScriptFile returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected script to load")
	}
	if script.Name != "discboeing-commit" {
		t.Fatalf("script name = %q, want discboeing-commit", script.Name)
	}
	if !script.Discboeing.UI {
		t.Fatal("expected discboeing ui metadata")
	}
	if script.Discboeing.Icon != "git-commit" {
		t.Fatalf("discboeing icon = %q, want git-commit", script.Discboeing.Icon)
	}
	if len(script.Discboeing.CredentialRequest) != 1 {
		t.Fatalf("credential requests = %d, want 1", len(script.Discboeing.CredentialRequest))
	}
	request := script.Discboeing.CredentialRequest[0]
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
	originalRoots := discboeingSystemRoots
	discboeingSystemRoots = []string{systemRoot}
	t.Cleanup(func() { discboeingSystemRoots = originalRoots })

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

func TestDiscoverScripts_DiscboeingSpecificOnly(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()

	discboeingScriptsDir := filepath.Join(home, ".discboeing", "scripts")
	mkdirAll(t, discboeingScriptsDir)
	discboeingPath := filepath.Join(discboeingScriptsDir, "release")
	writeFile(t, discboeingPath, `#!/bin/sh
#---
# name: release
# description: Discboeing script
#---
echo ok
`)
	if err := os.Chmod(discboeingPath, 0o755); err != nil {
		t.Fatal(err)
	}

	agentsScriptsDir := filepath.Join(home, ".agents", "scripts")
	mkdirAll(t, agentsScriptsDir)
	agentsPath := filepath.Join(agentsScriptsDir, "portable")
	writeFile(t, agentsPath, `#!/bin/sh
#---
# name: portable
# description: Portable script
#---
echo ignored
`)
	if err := os.Chmod(agentsPath, 0o755); err != nil {
		t.Fatal(err)
	}

	scripts, _, err := discoverScriptsWithHome(root, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected only Discboeing script, got %d", len(scripts))
	}
	if scripts[0].Name != "release" {
		t.Fatalf("script name = %q, want release", scripts[0].Name)
	}
}

func TestLookupScript_SystemDir(t *testing.T) {
	root := t.TempDir()
	systemRoot := t.TempDir()
	originalRoots := discboeingSystemRoots
	discboeingSystemRoots = []string{systemRoot}
	t.Cleanup(func() { discboeingSystemRoots = originalRoots })

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

func TestLookupScript_DoesNotUseAgentsScripts(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()

	agentsScriptsDir := filepath.Join(home, ".agents", "scripts")
	mkdirAll(t, agentsScriptsDir)
	agentsPath := filepath.Join(agentsScriptsDir, "portable")
	writeFile(t, agentsPath, `#!/bin/sh
#---
# name: portable
# description: Portable script
#---
echo ignored
`)
	if err := os.Chmod(agentsPath, 0o755); err != nil {
		t.Fatal(err)
	}

	_, found, err := lookupScriptWithHome(root, "portable", home, false)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected .agents script to be ignored")
	}
}

func TestLookupScript_FindsFrontmatterNameOverride(t *testing.T) {
	root := t.TempDir()
	scriptsDir := filepath.Join(root, ".discboeing", "scripts")
	mkdirAll(t, scriptsDir)
	path := filepath.Join(scriptsDir, "commit-remote")
	writeFile(t, path, `#!/bin/sh
#---
# name: commit
# description: Commit via remote
#---
echo ok
`)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}

	script, found, err := lookupScriptWithHome(root, "commit", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected script to be found by frontmatter name")
	}
	if script.Name != "commit" {
		t.Fatalf("script name = %q, want commit", script.Name)
	}
}

func TestDiscoverScripts_UsesDirectExecutablesOnly(t *testing.T) {
	root := t.TempDir()
	scriptsDir := filepath.Join(root, ".discboeing", "scripts")

	mkdirAll(t, scriptsDir)
	path := filepath.Join(scriptsDir, "status.sh")
	writeFile(t, path, `#!/bin/sh
#---
# description: Show git status
#---
echo ok
`)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}

	nestedDir := filepath.Join(scriptsDir, "nested")
	mkdirAll(t, nestedDir)
	nestedPath := filepath.Join(nestedDir, "deep.sh")
	writeFile(t, nestedPath, `#!/bin/sh
#---
# description: Nested scripts are ignored
#---
echo ignored
`)
	if err := os.Chmod(nestedPath, 0o755); err != nil {
		t.Fatal(err)
	}

	scripts, _, err := discoverScriptsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 direct script, got %d", len(scripts))
	}
	if scripts[0].Name != "status" {
		t.Fatalf("script name = %q, want status", scripts[0].Name)
	}
}
