package modelsdev

import "testing"

func TestLookup(t *testing.T) {
	t.Run("known model", func(t *testing.T) {
		m := Lookup("openai", "gpt-4o")
		if m == nil {
			t.Fatal("expected gpt-4o to be found")
		}
		if m.Name == "" {
			t.Error("expected non-empty name")
		}
		if m.ContextWindow == 0 {
			t.Error("expected non-zero context window")
		}
		if m.MaxOutputTokens == 0 {
			t.Error("expected non-zero max output tokens")
		}
	})

	t.Run("reasoning model", func(t *testing.T) {
		m := Lookup("openai", "o3")
		if m == nil {
			t.Fatal("expected o3 to be found")
		}
		if !m.Reasoning {
			t.Error("expected o3 to have Reasoning=true")
		}
	})

	t.Run("custom tools capability from overlay", func(t *testing.T) {
		m := Lookup("openai", "gpt-5")
		if m == nil {
			t.Fatal("expected gpt-5 to be found")
		}
		if !m.CustomTools {
			t.Error("expected gpt-5 to support custom tools")
		}
	})

	t.Run("codex overlay model includes context and modalities", func(t *testing.T) {
		m := Lookup("codex", "gpt-5.1-codex-mini")
		if m == nil {
			t.Fatal("expected gpt-5.1-codex-mini to be found")
		}
		if m.Name != "GPT-5.1 Codex Mini" {
			t.Fatalf("expected display name from overlay, got %q", m.Name)
		}
		if m.ContextWindow != 272000 {
			t.Fatalf("expected context window 272000, got %d", m.ContextWindow)
		}
		if len(m.ReasoningLevels) != 2 || m.ReasoningLevels[0] != "medium" || m.ReasoningLevels[1] != "high" {
			t.Fatalf("expected reasoning levels [medium high], got %v", m.ReasoningLevels)
		}
		if !m.SupportsInputModality("image") {
			t.Error("expected codex model to support image input modality")
		}
	})

	t.Run("codex spark is not exposed under openai", func(t *testing.T) {
		if m := Lookup("openai", "gpt-5.3-codex-spark"); m != nil {
			t.Fatalf("expected gpt-5.3-codex-spark to be absent from openai, got %+v", *m)
		}
	})

	t.Run("codex spark overlay copies important openai metadata", func(t *testing.T) {
		m := Lookup("codex", "gpt-5.3-codex-spark")
		if m == nil {
			t.Fatal("expected gpt-5.3-codex-spark to be found")
		}
		if m.Name != "GPT-5.3 Codex Spark" {
			t.Fatalf("expected display name from overlay, got %q", m.Name)
		}
		if m.Family != "gpt-codex-spark" {
			t.Fatalf("expected family gpt-codex-spark, got %q", m.Family)
		}
		if m.ContextWindow != 128000 {
			t.Fatalf("expected context window 128000, got %d", m.ContextWindow)
		}
		if m.MaxOutputTokens != 32000 {
			t.Fatalf("expected max output tokens 32000, got %d", m.MaxOutputTokens)
		}
		if !m.SupportsInputModality("pdf") {
			t.Error("expected codex spark to support pdf input modality")
		}
		if !m.CustomTools {
			t.Error("expected codex spark to support custom tools")
		}
		if m.Capabilities.ReasoningSummary == nil {
			t.Fatal("expected codex spark reasoning summary capability to be set")
		}
		if *m.Capabilities.ReasoningSummary {
			t.Error("expected codex spark reasoning summary capability to be false")
		}
		if m.SupportsReasoningSummary() {
			t.Error("expected codex spark to omit reasoning summaries")
		}
	})

	t.Run("reasoning summary defaults to reasoning support when unset", func(t *testing.T) {
		m := Lookup("openai", "o3")
		if m == nil {
			t.Fatal("expected o3 to be found")
		}
		if m.Capabilities.ReasoningSummary != nil {
			t.Error("expected explicit reasoning summary capability to remain unset")
		}
		if !m.SupportsReasoningSummary() {
			t.Error("expected reasoning models to allow reasoning summaries by default")
		}
	})

	t.Run("custom tools default false", func(t *testing.T) {
		m := Lookup("openai", "gpt-4o")
		if m == nil {
			t.Fatal("expected gpt-4o to be found")
		}
		if m.CustomTools {
			t.Error("expected gpt-4o to not support custom tools by default")
		}
	})

	t.Run("image modality support", func(t *testing.T) {
		m := Lookup("openai", "gpt-4o")
		if m == nil {
			t.Fatal("expected gpt-4o to be found")
		}
		if !m.SupportsInputModality("image") {
			t.Error("expected gpt-4o to support image input modality")
		}
		if m.SupportsInputModality("pdf") {
			t.Log("gpt-4o supports pdf input in current models metadata")
		}
	})

	t.Run("pdf modality support", func(t *testing.T) {
		m := Lookup("anthropic", "claude-3-7-sonnet-20250219")
		if m == nil {
			t.Fatal("expected claude-3-7-sonnet-20250219 to be found")
		}
		if !m.SupportsInputModality("pdf") {
			t.Error("expected claude-3-7-sonnet-20250219 to support pdf input modality")
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		m := Lookup("openai", "nonexistent-model")
		if m != nil {
			t.Error("expected nil for unknown model")
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		m := Lookup("nonexistent-provider", "gpt-4o")
		if m != nil {
			t.Error("expected nil for unknown provider")
		}
	})
}

func TestAllForProvider(t *testing.T) {
	t.Run("returns models for known provider", func(t *testing.T) {
		models := AllForProvider("openai")
		if len(models) == 0 {
			t.Fatal("expected models for openai")
		}
		// Verify at least one model has full metadata.
		found := false
		for _, m := range models {
			if m.ID == "gpt-4o" && m.ContextWindow > 0 {
				found = true
			}
		}
		if !found {
			t.Error("expected gpt-4o with context window in results")
		}
	})

	t.Run("returns nil for unknown provider", func(t *testing.T) {
		models := AllForProvider("nonexistent")
		if models != nil {
			t.Errorf("expected nil, got %d models", len(models))
		}
	})
}
