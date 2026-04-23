package frontmatter

import (
	"testing"

	"github.com/obot-platform/discobot/agent-go/providers"
)

type testDiscobotCredentialApprovedUse struct {
	Description string `yaml:"description"`
}

type testDiscobotCredentialRequest struct {
	EnvVar       string                              `yaml:"env-var"`
	ApprovedUses []testDiscobotCredentialApprovedUse `yaml:"approved-uses"`
}

type testScriptFrontmatter struct {
	Name              string                          `yaml:"name"`
	ArgumentHint      string                          `yaml:"argument-hint"`
	Visible           *bool                           `yaml:"visible"`
	LegacyDiscobotUI  *bool                           `yaml:"discobot-ui"`
	LegacyActiveLabel string                          `yaml:"discobot-active-label"`
	LegacyIcon        string                          `yaml:"discobot-icon"`
	LegacyCredential  []testDiscobotCredentialRequest `yaml:"discobot-credential-request"`
}

type testSubAgentFrontmatter struct {
	Name             string                     `yaml:"name"`
	SupportingModels providers.SupportingModels `yaml:"supporting-models"`
}

type testSystemPromptFrontmatter struct {
	AllowedTools []string `yaml:"allowed-tools"`
}

type testTopLevelTrimFrontmatter struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Nested      testNestedTrimMetadata `yaml:"nested"`
}

type testNestedTrimMetadata struct {
	Label string `yaml:"label"`
}

func TestParseMarkdown_NormalizesKeyStyles(t *testing.T) {
	content := `---
name: release
argument_hint: optional range
visible: false
discobot_ui: true
discobot-active-label: Running
discobot_icon: rocket
discobot-credential-request:
  - env_var: GH_TOKEN
    approved_uses:
      - description: create GitHub releases
---
Body.`

	doc, err := ParseMarkdown[testScriptFrontmatter](content)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.HasMetadata {
		t.Fatal("expected metadata")
	}
	if doc.Metadata.Name != "release" {
		t.Fatalf("name = %q, want release", doc.Metadata.Name)
	}
	if doc.Metadata.ArgumentHint != "optional range" {
		t.Fatalf("argument hint = %q", doc.Metadata.ArgumentHint)
	}
	if doc.Metadata.Visible == nil || *doc.Metadata.Visible {
		t.Fatalf("visible = %#v, want false", doc.Metadata.Visible)
	}
	if doc.Metadata.LegacyDiscobotUI == nil || !*doc.Metadata.LegacyDiscobotUI {
		t.Fatal("expected discobot ui metadata")
	}
	if doc.Metadata.LegacyActiveLabel != "Running" {
		t.Fatalf("active label = %q, want Running", doc.Metadata.LegacyActiveLabel)
	}
	if doc.Metadata.LegacyIcon != "rocket" {
		t.Fatalf("icon = %q, want rocket", doc.Metadata.LegacyIcon)
	}
	if len(doc.Metadata.LegacyCredential) != 1 {
		t.Fatalf("credential requests = %d, want 1", len(doc.Metadata.LegacyCredential))
	}
	if doc.Metadata.LegacyCredential[0].EnvVar != "GH_TOKEN" {
		t.Fatalf("env var = %q, want GH_TOKEN", doc.Metadata.LegacyCredential[0].EnvVar)
	}
}

func TestParseMarkdown_TrimsOnlyTopLevelStringFields(t *testing.T) {
	content := `---
name: "  release  "
description: "  Ship it.  "
nested:
  label: "  keep surrounding spaces  "
---
Body.`

	doc, err := ParseMarkdown[testTopLevelTrimFrontmatter](content)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Metadata.Name != "release" {
		t.Fatalf("name = %q, want release", doc.Metadata.Name)
	}
	if doc.Metadata.Description != "Ship it." {
		t.Fatalf("description = %q, want Ship it.", doc.Metadata.Description)
	}
	if doc.Metadata.Nested.Label != "  keep surrounding spaces  " {
		t.Fatalf("nested label = %q, want spaces preserved", doc.Metadata.Nested.Label)
	}
}

func TestParseScript_ParsesCommentedFrontmatterAndBody(t *testing.T) {
	content := `#!/usr/bin/env bash
#---
# name: deploy
# description: Deploy the app
# visible: true
#---
echo deploy
`

	type scriptDoc struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Visible     *bool  `yaml:"visible"`
	}

	doc, err := ParseScript[scriptDoc](content)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.HasMetadata {
		t.Fatal("expected metadata")
	}
	if doc.Metadata.Name != "deploy" {
		t.Fatalf("name = %q, want deploy", doc.Metadata.Name)
	}
	if doc.Body != content {
		t.Fatalf("body = %q, want original script content", doc.Body)
	}
}

func TestParseMarkdown_DecodesSupportingModelsString(t *testing.T) {
	content := `---
name: reviewer
supporting_models: authorizer=gpt-5-mini, planner=gpt-5
---
Prompt.`

	doc, err := ParseMarkdown[testSubAgentFrontmatter](content)
	if err != nil {
		t.Fatal(err)
	}
	want := providers.SupportingModels{
		providers.SupportingModelType("authorizer"): "gpt-5-mini",
		providers.SupportingModelType("planner"):    "gpt-5",
	}
	if len(doc.Metadata.SupportingModels) != len(want) {
		t.Fatalf("supporting models = %#v, want %#v", doc.Metadata.SupportingModels, want)
	}
	for key, value := range want {
		if doc.Metadata.SupportingModels[key] != value {
			t.Fatalf("supportingModels[%s] = %q, want %q", key, doc.Metadata.SupportingModels[key], value)
		}
	}
}

func TestParseMarkdown_NoFrontmatter(t *testing.T) {
	content := "Just a regular markdown file.\n\nNo frontmatter here."

	doc, err := ParseMarkdown[testSystemPromptFrontmatter](content)
	if err != nil {
		t.Fatal(err)
	}
	if doc.HasMetadata {
		t.Errorf("expected no metadata")
	}
	if doc.Body != content {
		t.Errorf("body = %q, want %q", doc.Body, content)
	}
}

func TestParseMarkdown_MalformedYAML(t *testing.T) {
	content := `---
name: [invalid yaml
  missing bracket
---
Body content.`

	_, err := ParseMarkdown[testSystemPromptFrontmatter](content)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}
