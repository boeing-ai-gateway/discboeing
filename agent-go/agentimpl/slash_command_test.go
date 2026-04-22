package agentimpl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/message"
)

func TestResolveSlashCommand_Script(t *testing.T) {
	root := t.TempDir()
	scriptDir := filepath.Join(root, ".discobot", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "commit"), []byte("#!/bin/sh\n#---\n# description: script\n#---\nprintf 'ok\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	parts := []message.UIPart{message.UITextPart{Text: "/commit fix the bug", State: "done"}}
	resolvedParts, originalText, activeCommand, slashCommand := resolveSlashCommand(root, parts)
	if originalText != "/commit fix the bug" {
		t.Fatalf("originalText = %q", originalText)
	}
	if activeCommand != "commit" {
		t.Fatalf("activeCommand = %q", activeCommand)
	}
	if slashCommand == nil {
		t.Fatal("expected slash command metadata")
	}
	if slashCommand.Kind != agent.CommandKindScript {
		t.Fatalf("slashCommand.kind = %q", slashCommand.Kind)
	}
	if slashCommand.Text != "" {
		t.Fatalf("slashCommand.text = %q", slashCommand.Text)
	}

	textPart, ok := resolvedParts[0].(message.UITextPart)
	if !ok {
		t.Fatalf("expected UITextPart, got %T", resolvedParts[0])
	}
	if textPart.Text != "/commit fix the bug" {
		t.Fatalf("textPart.Text = %q", textPart.Text)
	}
	meta, ok := message.UnmarshalProviderMetadata(textPart.ProviderMetadata)
	if !ok {
		t.Fatal("expected provider metadata")
	}
	if meta.OriginalCommand != "/commit fix the bug" {
		t.Fatalf("originalCommand = %q", meta.OriginalCommand)
	}
	if meta.CommandKind != string(agent.CommandKindScript) {
		t.Fatalf("commandKind = %q", meta.CommandKind)
	}
}
