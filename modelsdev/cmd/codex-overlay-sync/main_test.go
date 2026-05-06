package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestTargetModelIDsMissingOnlyAddsNewRemoteModels(t *testing.T) {
	codexOverlay := map[string]map[string]any{
		"$provider":     {"name": "ChatGPT Codex"},
		"gpt-5.3-codex": {"name": "GPT-5.3 Codex"},
	}
	remoteModels := map[string]codexModel{
		"gpt-5.3-codex": {Slug: "gpt-5.3-codex"},
		"gpt-5.4":       {Slug: "gpt-5.4"},
	}

	got := targetModelIDs("missing", codexOverlay, remoteModels)
	want := []string{"gpt-5.4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targetModelIDs() = %v, want %v", got, want)
	}
}

func TestSyncCodexOverlayRefreshesFromRemoteCatalog(t *testing.T) {
	overlay := overlayFile{
		targetProviderID: {
			"$provider": {"name": "ChatGPT Codex"},
			"gpt-5.3-codex": {
				"capabilities": map[string]any{"reasoningSummary": false},
			},
		},
	}
	base := rawData{
		sourceProviderID: {
			Models: map[string]modelMetadata{
				"gpt-5.3-codex": {
					ID:       "gpt-5.3-codex",
					Name:     "GPT-5.3 Codex",
					Family:   "gpt-codex",
					ToolCall: true,
					Limit: modelLimit{
						Output: 128000,
					},
					Modalities: modelModalities{
						Output: []string{"text"},
					},
				},
			},
		},
	}
	contextWindow := 272000
	models := []codexModel{{
		Slug:                     "gpt-5.3-codex",
		DisplayName:              "gpt-5.3-codex",
		DefaultReasoningLevel:    "medium",
		SupportedReasoningLevels: []codexReasoningLevel{{Effort: "low"}, {Effort: "medium"}, {Effort: "high"}, {Effort: "xhigh"}},
		ContextWindow:            &contextWindow,
		InputModalities:          []string{"text", "image"},
		ShellType:                "shell_command",
		ApplyPatchToolType:       "freeform",
	}}

	syncCodexOverlay(overlay, base, models, "refresh", nil)
	got := overlay[targetProviderID]["gpt-5.3-codex"]
	if got["name"] != "GPT-5.3 Codex" {
		t.Fatalf("name = %#v, want GPT-5.3 Codex", got["name"])
	}
	if got["family"] != "gpt-codex" {
		t.Fatalf("family = %#v, want gpt-codex", got["family"])
	}
	if got["contextWindow"] != 272000 {
		t.Fatalf("contextWindow = %#v, want 272000", got["contextWindow"])
	}
	if got["maxOutputTokens"] != 128000 {
		t.Fatalf("maxOutputTokens = %#v, want 128000", got["maxOutputTokens"])
	}
	if got["defaultReasonLevel"] != "medium" {
		t.Fatalf("defaultReasonLevel = %#v, want medium", got["defaultReasonLevel"])
	}
	if got["reasoning"] != true {
		t.Fatalf("reasoning = %#v, want true", got["reasoning"])
	}
	levels, ok := got["reasoningLevels"].([]string)
	if !ok || !reflect.DeepEqual(levels, []string{"low", "medium", "high", "xhigh"}) {
		t.Fatalf("reasoningLevels = %#v, want [low medium high xhigh]", got["reasoningLevels"])
	}
	if got["tool_call"] != true {
		t.Fatalf("tool_call = %#v, want true", got["tool_call"])
	}
	if got["customTools"] != true {
		t.Fatalf("customTools = %#v, want true", got["customTools"])
	}
	if _, ok := got["capabilities"].(map[string]any); !ok {
		t.Fatalf("expected capabilities to be preserved, got %#v", got["capabilities"])
	}
}

func TestSyncCodexOverlayAddsProbedReasoningNone(t *testing.T) {
	overlay := overlayFile{targetProviderID: {}}
	contextWindow := 272000
	models := []codexModel{{
		Slug:                     "gpt-5.5",
		DisplayName:              "GPT-5.5",
		DefaultReasoningLevel:    "medium",
		SupportedReasoningLevels: []codexReasoningLevel{{Effort: "low"}, {Effort: "medium"}, {Effort: "high"}, {Effort: "xhigh"}},
		ContextWindow:            &contextWindow,
		ShellType:                "shell_command",
	}}

	syncCodexOverlay(overlay, nil, models, "refresh", map[string]bool{"gpt-5.5": true})
	levels, ok := overlay[targetProviderID]["gpt-5.5"]["reasoningLevels"].([]string)
	if !ok || !reflect.DeepEqual(levels, []string{"none", "low", "medium", "high", "xhigh"}) {
		t.Fatalf("reasoningLevels = %#v, want [none low medium high xhigh]", overlay[targetProviderID]["gpt-5.5"]["reasoningLevels"])
	}
}

func TestSyncedEntryOverridesGPT54DefaultReasoning(t *testing.T) {
	got := syncedEntry(nil, codexModel{
		Slug:                  "gpt-5.4",
		DefaultReasoningLevel: "xhigh",
		SupportedReasoningLevels: []codexReasoningLevel{
			{Effort: "low"},
			{Effort: "medium"},
			{Effort: "high"},
			{Effort: "xhigh"},
		},
	}, modelMetadata{}, false)

	if got["defaultReasonLevel"] != "medium" {
		t.Fatalf("defaultReasonLevel = %#v, want medium", got["defaultReasonLevel"])
	}
}

func TestSyncCodexOverlayPreservesRemovedEntriesAndPrunesStaleOnRefresh(t *testing.T) {
	overlay := overlayFile{
		targetProviderID: {
			"$provider":           {"name": "ChatGPT Codex"},
			"gpt-5.3-codex-spark": {"remove": true},
			"stale-model":         {"reasoning": true},
			"gpt-5.3-codex":       {"reasoning": true},
		},
	}
	base := rawData{
		sourceProviderID: {
			Models: map[string]modelMetadata{
				"gpt-5.3-codex-spark": {ID: "gpt-5.3-codex-spark"},
			},
		},
	}
	models := []codexModel{{Slug: "gpt-5.3-codex"}}

	syncCodexOverlay(overlay, base, models, "refresh", nil)
	if _, ok := overlay[targetProviderID]["stale-model"]; ok {
		t.Fatal("expected stale model to be deleted")
	}
	if removed, _ := overlay[targetProviderID]["gpt-5.3-codex-spark"]["remove"].(bool); !removed {
		t.Fatalf("remove = %#v, want true", overlay[targetProviderID]["gpt-5.3-codex-spark"]["remove"])
	}
}

func TestProbeReasoningNoneSupport(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/responses" {
			http.NotFound(w, r)
			return
		}
		hits++
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want Bearer test-token", got)
		}
		if got := r.Header.Get("ChatGPT-Account-Id"); got != "acct_123" {
			t.Fatalf("ChatGPT-Account-Id = %q, want acct_123", got)
		}
		var payload struct {
			Model        string `json:"model"`
			Instructions string `json:"instructions"`
			Input        []struct {
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"input"`
			Stream    bool `json:"stream"`
			Reasoning struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Model != "gpt-5.5" || payload.Reasoning.Effort != "none" {
			t.Fatalf("payload = %+v, want model gpt-5.5 with effort none", payload)
		}
		if payload.Instructions == "" || !payload.Stream {
			t.Fatalf("payload = %+v, want instructions and stream", payload)
		}
		if len(payload.Input) != 1 || payload.Input[0].Role != "user" ||
			len(payload.Input[0].Content) != 1 || payload.Input[0].Content[0].Type != "input_text" {
			t.Fatalf("input = %+v, want one user input_text item", payload.Input)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	support := probeReasoningNoneSupport(server.Client(), config{
		BaseURL:   server.URL + "/api",
		Token:     "test-token",
		AccountID: "acct_123",
	}, []codexModel{
		{Slug: "gpt-5.5", SupportedReasoningLevels: []codexReasoningLevel{{Effort: "low"}}},
		{Slug: "already-none", SupportedReasoningLevels: []codexReasoningLevel{{Effort: "none"}}},
	})

	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if !support["gpt-5.5"] {
		t.Fatalf("expected gpt-5.5 to support reasoning none, got %v", support)
	}
	if support["already-none"] {
		t.Fatalf("expected already-none to be skipped, got %v", support)
	}
}

func TestSyncCodexOverlayPreservesBaseBackedModelsMissingFromRemote(t *testing.T) {
	overlay := overlayFile{
		targetProviderID: {
			"$provider": {"name": "ChatGPT Codex"},
			"gpt-5.3-codex-spark": {
				"name": "GPT-5.3 Codex Spark",
			},
		},
	}
	base := rawData{
		sourceProviderID: {
			Models: map[string]modelMetadata{
				"gpt-5.3-codex-spark": {ID: "gpt-5.3-codex-spark"},
			},
		},
	}

	syncCodexOverlay(overlay, base, nil, "refresh", nil)
	if _, ok := overlay[targetProviderID]["gpt-5.3-codex-spark"]; !ok {
		t.Fatal("expected base-backed model to be preserved")
	}
}

func TestFetchCodexModelsPrefersAPIAndFallsBackToCatalog(t *testing.T) {
	apiHits := 0
	catalogHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models":
			apiHits++
			if got := r.URL.Query().Get("client_version"); got != "test-version" {
				t.Fatalf("client_version = %q, want test-version", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Fatalf("Authorization = %q, want Bearer test-token", got)
			}
			if got := r.Header.Get("ChatGPT-Account-Id"); got != "acct_123" {
				t.Fatalf("ChatGPT-Account-Id = %q, want acct_123", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-5.4","default_reasoning_level":"medium","supported_reasoning_levels":[{"effort":"low"}]}]}`))
		case "/catalog/models.json":
			catalogHits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"slug":"fallback-model"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := server.Client()
	models, source, err := fetchCodexModels(client, config{
		BaseURL:       server.URL + "/api",
		CatalogURL:    server.URL + "/catalog/models.json",
		ClientVersion: "test-version",
		Token:         "test-token",
		AccountID:     "acct_123",
	})
	if err != nil {
		t.Fatalf("fetchCodexModels() error = %v", err)
	}
	if source != "api" {
		t.Fatalf("source = %q, want api", source)
	}
	if apiHits != 1 || catalogHits != 0 {
		t.Fatalf("apiHits=%d catalogHits=%d, want 1 and 0", apiHits, catalogHits)
	}
	if len(models) != 1 || models[0].Slug != "gpt-5.4" {
		t.Fatalf("models = %#v, want api response", models)
	}
}

func TestFetchCodexModelsFallsBackToCatalogWhenAPIUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models":
			http.Error(w, "boom", http.StatusUnauthorized)
		case "/catalog/models.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-5.3-codex","default_reasoning_level":"medium","supported_reasoning_levels":[{"effort":"medium"}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	models, source, err := fetchCodexModels(server.Client(), config{
		BaseURL:    server.URL + "/api",
		CatalogURL: server.URL + "/catalog/models.json",
		Token:      "test-token",
	})
	if err != nil {
		t.Fatalf("fetchCodexModels() error = %v", err)
	}
	if source != "catalog" {
		t.Fatalf("source = %q, want catalog", source)
	}
	if len(models) != 1 || models[0].Slug != "gpt-5.3-codex" {
		t.Fatalf("models = %#v, want catalog response", models)
	}
}
