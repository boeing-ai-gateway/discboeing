package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	targetProviderID  = "codex"
	sourceProviderID  = "openai"
	defaultBaseURL    = "https://chatgpt.com/backend-api/codex"
	defaultCatalogURL = "https://raw.githubusercontent.com/openai/codex/main/codex-rs/models-manager/models.json"
	// The authenticated Codex models endpoint requires a client_version query
	// parameter. The sync tool is not tied to a bundled Codex CLI version, so
	// use a stable placeholder unless callers provide CODEX_CLIENT_VERSION.
	defaultClientVersion = "0.0.0"
)

var defaultReasoningLevelOverrides = map[string]string{
	// Codex currently advertises xhigh for GPT-5.4, but Discboeing uses medium
	// as the default to keep new sessions balanced unless users opt in.
	"gpt-5.4": "medium",
}

type overlayFile map[string]map[string]map[string]any

type rawData map[string]providerEntry

type providerEntry struct {
	ID     string                   `json:"id"`
	Name   string                   `json:"name"`
	API    string                   `json:"api"`
	Env    []string                 `json:"env"`
	Doc    string                   `json:"doc"`
	Models map[string]modelMetadata `json:"models"`
}

type modelMetadata struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Family     string          `json:"family"`
	Reasoning  bool            `json:"reasoning"`
	ToolCall   bool            `json:"tool_call"`
	Limit      modelLimit      `json:"limit"`
	Modalities modelModalities `json:"modalities"`
}

type modelModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type modelLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

type codexModelsResponse struct {
	Models []codexModel `json:"models"`
}

type codexModel struct {
	Slug                     string                `json:"slug"`
	DisplayName              string                `json:"display_name"`
	DefaultReasoningLevel    string                `json:"default_reasoning_level"`
	SupportedReasoningLevels []codexReasoningLevel `json:"supported_reasoning_levels"`
	ContextWindow            *int                  `json:"context_window"`
	MaxContextWindow         *int                  `json:"max_context_window"`
	InputModalities          []string              `json:"input_modalities"`
	ShellType                string                `json:"shell_type"`
	ApplyPatchToolType       string                `json:"apply_patch_tool_type"`
	SupportsParallelTools    bool                  `json:"supports_parallel_tool_calls"`
	ExperimentalTools        []string              `json:"experimental_supported_tools"`
}

type codexReasoningLevel struct {
	Effort string `json:"effort"`
}

type config struct {
	OverlayPath   string
	BasePath      string
	BaseURL       string
	CatalogURL    string
	ClientVersion string
	Token         string
	AccountID     string
	Mode          string
}

func main() {
	cfg := config{
		OverlayPath:   defaultOverlayPath(),
		BasePath:      defaultBasePath(),
		BaseURL:       strings.TrimRight(envOr("CODEX_API_BASE", defaultBaseURL), "/"),
		CatalogURL:    envOr("CODEX_MODELS_CATALOG_URL", defaultCatalogURL),
		ClientVersion: envOr("CODEX_CLIENT_VERSION", defaultClientVersion),
		Token:         strings.TrimSpace(os.Getenv("CODEX_TOKEN")),
		AccountID:     strings.TrimSpace(os.Getenv("CHATGPT_ACCOUNT_ID")),
		Mode:          "refresh",
	}

	flag.StringVar(&cfg.OverlayPath, "overlay", cfg.OverlayPath, "Path to model-overlay.json")
	flag.StringVar(&cfg.BasePath, "base", cfg.BasePath, "Path to models-dev-api.json")
	flag.StringVar(&cfg.BaseURL, "base-url", cfg.BaseURL, "Codex API base URL")
	flag.StringVar(&cfg.CatalogURL, "catalog-url", cfg.CatalogURL, "Fallback URL for Codex bundled models.json")
	flag.StringVar(&cfg.ClientVersion, "client-version", cfg.ClientVersion, "Codex client version sent to the authenticated models endpoint (defaults to CODEX_CLIENT_VERSION)")
	flag.StringVar(&cfg.Token, "token", cfg.Token, "Codex token (defaults to CODEX_TOKEN)")
	flag.StringVar(&cfg.AccountID, "account-id", cfg.AccountID, "ChatGPT account ID (defaults to CHATGPT_ACCOUNT_ID)")
	flag.StringVar(&cfg.Mode, "mode", cfg.Mode, "Sync mode: missing or refresh")
	flag.Parse()

	if cfg.Mode != "missing" && cfg.Mode != "refresh" {
		fatalf("invalid -mode %q, expected missing or refresh", cfg.Mode)
	}

	overlay, err := readOverlay(cfg.OverlayPath)
	if err != nil {
		fatalf("read overlay: %v", err)
	}
	base, err := readBase(cfg.BasePath)
	if err != nil {
		fatalf("read base data: %v", err)
	}

	client, err := newHTTPClient()
	if err != nil {
		fatalf("create HTTP client: %v", err)
	}
	models, source, err := fetchCodexModels(client, cfg)
	if err != nil {
		fatalf("fetch codex models: %v", err)
	}

	var reasoningNoneSupport map[string]bool
	if source == "api" {
		reasoningNoneSupport = probeReasoningNoneSupport(client, cfg, models)
	}
	syncCodexOverlay(overlay, base, models, cfg.Mode, reasoningNoneSupport)
	if err := writeOverlay(cfg.OverlayPath, overlay); err != nil {
		fatalf("write overlay: %v", err)
	}

	fmt.Printf("Updated %s\n", cfg.OverlayPath)
	fmt.Printf("Source: %s\n", source)
}

func defaultOverlayPath() string {
	candidates := []string{filepath.Join("modelsdev", "model-overlay.json"), "model-overlay.json"}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("modelsdev", "model-overlay.json")
}

func defaultBasePath() string {
	candidates := []string{filepath.Join("modelsdev", "models-dev-api.json"), "models-dev-api.json"}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("modelsdev", "models-dev-api.json")
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func newHTTPClient() (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if certPath := strings.TrimSpace(os.Getenv("NODE_EXTRA_CA_CERTS")); certPath != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system cert pool: %w", err)
		}
		if pool == nil {
			pool = x509.NewCertPool()
		}
		pemData, err := os.ReadFile(certPath)
		if err != nil {
			return nil, fmt.Errorf("read NODE_EXTRA_CA_CERTS %q: %w", certPath, err)
		}
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("append NODE_EXTRA_CA_CERTS %q", certPath)
		}
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	return &http.Client{Transport: transport, Timeout: 30 * time.Second}, nil
}

func readOverlay(path string) (overlayFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var overlay overlayFile
	if err := json.Unmarshal(data, &overlay); err != nil {
		return nil, err
	}
	if overlay == nil {
		overlay = make(overlayFile)
	}
	return overlay, nil
}

func readBase(path string) (rawData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var base rawData
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}
	return base, nil
}

func fetchCodexModels(client *http.Client, cfg config) ([]codexModel, string, error) {
	if cfg.Token != "" {
		models, err := fetchCodexModelsFromAPI(client, cfg)
		if err == nil {
			return models, "api", nil
		}
		fmt.Fprintf(os.Stderr, "warning: codex API fetch failed, falling back to catalog: %v\n", err)
	}
	models, err := fetchCodexModelsFromCatalog(client, cfg.CatalogURL)
	if err != nil {
		return nil, "", err
	}
	return models, "catalog", nil
}

func fetchCodexModelsFromAPI(client *http.Client, cfg config) ([]codexModel, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if cfg.ClientVersion != "" {
		query := req.URL.Query()
		query.Set("client_version", cfg.ClientVersion)
		req.URL.RawQuery = query.Encode()
	}
	setCodexHeaders(req, cfg)
	return doModelsRequest(client, req)
}

func fetchCodexModelsFromCatalog(client *http.Client, url string) ([]codexModel, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return doModelsRequest(client, req)
}

func doModelsRequest(client *http.Client, req *http.Request) ([]codexModel, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: status %d", req.URL, resp.StatusCode)
	}
	var parsed codexModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return parsed.Models, nil
}

func probeReasoningNoneSupport(client *http.Client, cfg config, models []codexModel) map[string]bool {
	if cfg.Token == "" {
		return nil
	}
	support := make(map[string]bool)
	for _, model := range models {
		if strings.TrimSpace(model.Slug) == "" || hasReasoningLevel(model, "none") {
			continue
		}
		ok, err := probeReasoningNone(client, cfg, model.Slug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: probe reasoning none for %s failed: %v\n", model.Slug, err)
			continue
		}
		if ok {
			support[model.Slug] = true
		}
	}
	return support
}

func probeReasoningNone(client *http.Client, cfg config, modelID string) (bool, error) {
	payload := map[string]any{
		"model":        modelID,
		"instructions": "You are a helpful assistant.",
		"input": []map[string]any{{
			"role": "user",
			"content": []map[string]string{{
				"type": "input_text",
				"text": "Reply with ok.",
			}},
		}},
		"store":  false,
		"stream": true,
		"reasoning": map[string]any{
			"effort": "none",
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequest(http.MethodPost, cfg.BaseURL+"/responses", bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	setCodexHeaders(req, cfg)
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

func setCodexHeaders(req *http.Request, cfg config) {
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	if cfg.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", cfg.AccountID)
	}
}

func syncCodexOverlay(overlay overlayFile, base rawData, models []codexModel, mode string, reasoningNoneSupport map[string]bool) {
	if overlay[targetProviderID] == nil {
		overlay[targetProviderID] = map[string]map[string]any{}
	}
	codexOverlay := overlay[targetProviderID]
	baseModels := map[string]modelMetadata{}
	if provider, ok := base[sourceProviderID]; ok && provider.Models != nil {
		baseModels = provider.Models
	}

	remoteModels := make(map[string]codexModel, len(models))
	for _, model := range models {
		if strings.TrimSpace(model.Slug) == "" {
			continue
		}
		remoteModels[model.Slug] = model
	}

	for _, modelID := range targetModelIDs(mode, codexOverlay, remoteModels) {
		existing := codexOverlay[modelID]
		if isRemoved(existing) {
			continue
		}
		remote, ok := remoteModels[modelID]
		if !ok {
			continue
		}
		codexOverlay[modelID] = syncedEntry(existing, remote, baseModels[modelID], reasoningNoneSupport[modelID])
	}

	if mode == "refresh" {
		pruneStaleModels(codexOverlay, remoteModels, baseModels)
	}
}

func targetModelIDs(mode string, codexOverlay map[string]map[string]any, remoteModels map[string]codexModel) []string {
	set := make(map[string]struct{})
	if mode == "refresh" {
		for modelID := range remoteModels {
			set[modelID] = struct{}{}
		}
		for modelID := range codexOverlay {
			if modelID == "$provider" {
				continue
			}
			if isRemoved(codexOverlay[modelID]) {
				set[modelID] = struct{}{}
			}
		}
	} else {
		for modelID := range remoteModels {
			if _, exists := codexOverlay[modelID]; exists {
				continue
			}
			set[modelID] = struct{}{}
		}
	}
	modelIDs := make([]string, 0, len(set))
	for modelID := range set {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	return modelIDs
}

func pruneStaleModels(codexOverlay map[string]map[string]any, remoteModels map[string]codexModel, baseModels map[string]modelMetadata) {
	for modelID, entry := range codexOverlay {
		if modelID == "$provider" || isRemoved(entry) {
			continue
		}
		if _, ok := remoteModels[modelID]; ok {
			continue
		}
		if _, ok := baseModels[modelID]; ok {
			continue
		}
		delete(codexOverlay, modelID)
	}
}

func isRemoved(entry map[string]any) bool {
	removed, _ := entry["remove"].(bool)
	return removed
}

func syncedEntry(existing map[string]any, remote codexModel, baseModel modelMetadata, supportsReasoningNone bool) map[string]any {
	levels := supportedReasoningLevels(remote)
	if supportsReasoningNone {
		levels = appendReasoningLevel(levels, "none")
	}
	contextWindow := resolvedContextWindow(remote)
	entry := map[string]any{
		"customTools":     supportsTools(remote),
		"reasoning":       supportsReasoning(remote, levels),
		"reasoningLevels": levels,
		"tool_call":       supportsTools(remote),
	}
	if defaultLevel := defaultReasoningLevel(remote); defaultLevel != "" {
		entry["defaultReasonLevel"] = defaultLevel
	}
	if contextWindow > 0 {
		entry["contextWindow"] = contextWindow
	}
	if len(remote.InputModalities) > 0 {
		entry["inputModalities"] = append([]string(nil), remote.InputModalities...)
	}
	if strings.TrimSpace(remote.DisplayName) != "" {
		entry["name"] = normalizeDisplayName(remote.DisplayName)
	}

	if baseModel.Family != "" {
		entry["family"] = baseModel.Family
	}
	if baseModel.Limit.Output > 0 {
		entry["maxOutputTokens"] = baseModel.Limit.Output
	}
	if len(baseModel.Modalities.Output) > 0 {
		entry["outputModalities"] = append([]string(nil), baseModel.Modalities.Output...)
	}
	if len(baseModel.Modalities.Input) > 0 && len(remote.InputModalities) == 0 {
		entry["inputModalities"] = append([]string(nil), baseModel.Modalities.Input...)
	}
	if baseModel.Limit.Context > 0 && contextWindow == 0 {
		entry["contextWindow"] = baseModel.Limit.Context
	}
	if strings.TrimSpace(baseModel.Name) != "" {
		entry["name"] = baseModel.Name
	}

	copyFieldIfPresent(entry, existing, "capabilities")
	copyFieldIfPresent(entry, existing, "remove")
	return entry
}

func defaultReasoningLevel(remote codexModel) string {
	if override := defaultReasoningLevelOverrides[remote.Slug]; override != "" {
		return override
	}
	return remote.DefaultReasoningLevel
}

func resolvedContextWindow(remote codexModel) int {
	if remote.ContextWindow != nil && *remote.ContextWindow > 0 {
		return *remote.ContextWindow
	}
	if remote.MaxContextWindow != nil && *remote.MaxContextWindow > 0 {
		return *remote.MaxContextWindow
	}
	return 0
}

func supportedReasoningLevels(remote codexModel) []string {
	levels := make([]string, 0, len(remote.SupportedReasoningLevels))
	seen := make(map[string]struct{}, len(remote.SupportedReasoningLevels))
	for _, level := range remote.SupportedReasoningLevels {
		effort := strings.TrimSpace(level.Effort)
		if effort == "" {
			continue
		}
		if _, ok := seen[effort]; ok {
			continue
		}
		seen[effort] = struct{}{}
		levels = append(levels, effort)
	}
	return levels
}

func appendReasoningLevel(levels []string, level string) []string {
	for _, existing := range levels {
		if existing == level {
			return levels
		}
	}
	if level == "none" {
		return append([]string{level}, levels...)
	}
	return append(levels, level)
}

func hasReasoningLevel(remote codexModel, level string) bool {
	for _, candidate := range remote.SupportedReasoningLevels {
		if strings.TrimSpace(candidate.Effort) == level {
			return true
		}
	}
	return false
}

func supportsReasoning(remote codexModel, levels []string) bool {
	if len(levels) == 0 {
		return false
	}
	for _, level := range levels {
		if level != "none" {
			return true
		}
	}
	return remote.DefaultReasoningLevel != "" && remote.DefaultReasoningLevel != "none"
}

func supportsTools(remote codexModel) bool {
	if remote.ShellType != "" && remote.ShellType != "disabled" {
		return true
	}
	return remote.ApplyPatchToolType != ""
}

func normalizeDisplayName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, " ") {
		return trimmed
	}
	return strings.ReplaceAll(trimmed, "-", " ")
}

func copyFieldIfPresent(dst, src map[string]any, key string) {
	if value, ok := src[key]; ok {
		dst[key] = value
	}
}

func writeOverlay(path string, overlay overlayFile) error {
	data, err := json.MarshalIndent(overlay, "", "\t")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
