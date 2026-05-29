package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/service"
)

var githubShorthandPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})/[A-Za-z0-9._-]+$`)

const githubRepoListCacheTTL = 2 * time.Minute

type githubRepoListCacheEntry struct {
	Repos     []githubRepositoryListItem
	ExpiresAt time.Time
}

var githubRepoListCache = struct {
	sync.RWMutex
	Entries map[string]githubRepoListCacheEntry
}{Entries: make(map[string]githubRepoListCacheEntry)}

var (
	errGitAuthenticationRequired = errors.New("git authentication required")
	errGitAuthorizationFailed    = errors.New("git authorization failed")
)

type validateWorkspaceResponse struct {
	Path           string           `json:"path"`
	SourceType     string           `json:"sourceType"`
	Valid          bool             `json:"valid"`
	Classification string           `json:"classification"`
	Error          string           `json:"error,omitempty"`
	Suggestions    []api.Suggestion `json:"suggestions"`
	AuthProvider   string           `json:"authProvider,omitempty"`
	AuthRequired   bool             `json:"authRequired,omitempty"`
	AuthMessage    string           `json:"authMessage,omitempty"`
}

// ValidateWorkspace validates a workspace path/repo and returns suggestions.
// POST /api/projects/{projectId}/workspaces/validate
func (h *Handler) ValidateWorkspace(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req api.ValidateWorkspaceRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Path = strings.TrimSpace(req.Path)
	if req.SourceType == "" {
		req.SourceType = "local"
	}

	if req.SourceType != "local" && req.SourceType != "git" {
		h.Error(w, http.StatusBadRequest, "sourceType must be local or git")
		return
	}

	if req.Path == "" {
		h.JSON(w, http.StatusOK, validateWorkspaceResponse{
			Path:           "",
			SourceType:     req.SourceType,
			Valid:          false,
			Classification: service.LocalWorkspaceClassificationInvalid,
			Suggestions:    []api.Suggestion{},
		})
		return
	}

	if req.SourceType == "local" {
		normalizedPath, classification, err := h.workspaceService.ValidateLocalWorkspacePath(req.Path)
		response := validateWorkspaceResponse{
			Path:           normalizedPath,
			SourceType:     "local",
			Classification: classification,
			Suggestions:    getDirectorySuggestions(req.Path),
		}
		if err != nil {
			response.Error = err.Error()
		} else {
			response.Valid = classification == service.LocalWorkspaceClassificationNew ||
				classification == service.LocalWorkspaceClassificationEmpty ||
				classification == service.LocalWorkspaceClassificationExistingGit
		}

		h.JSON(w, http.StatusOK, response)
		return
	}

	normalizedPath := normalizeGitPath(req.Path)
	response := validateWorkspaceResponse{
		Path:           normalizedPath,
		SourceType:     "git",
		Classification: "invalid",
		Suggestions:    getRepoSuggestions(r.Context(), req.Path, ""),
	}

	if !looksLikeGitRepositoryInput(req.Path) {
		response.Error = "Enter a repository URL or org/repo."
		h.JSON(w, http.StatusOK, response)
		return
	}

	githubToken, hasGitHubToken, tokenErr := h.getGitHubToken(r.Context(), projectID)
	if tokenErr != nil {
		response.Error = fmt.Sprintf("failed to check GitHub credential: %v", tokenErr)
		h.JSON(w, http.StatusOK, response)
		return
	}

	response.Suggestions = getRepoSuggestions(r.Context(), req.Path, githubToken)

	if isGitHubRepositoryURL(normalizedPath) && !hasGitHubToken {
		response.AuthProvider = service.ProviderGitHub
		response.AuthRequired = true
		response.AuthMessage = "Sign in to GitHub to validate and clone private repositories."
	}

	if err := validateGitRemote(r.Context(), normalizedPath, githubToken); err != nil {
		response.Error = err.Error()

		if isGitHubRepositoryURL(normalizedPath) {
			if !hasGitHubToken || errors.Is(err, errGitAuthenticationRequired) || errors.Is(err, errGitAuthorizationFailed) || isGitHubRepositoryNotFound(err) {
				response.AuthProvider = service.ProviderGitHub
				response.AuthRequired = true
				if !hasGitHubToken {
					response.AuthMessage = "Sign in to GitHub to validate and clone private repositories."
				} else if isGitHubRepositoryNotFound(err) {
					response.AuthMessage = "Repository not found. If this repo is private, ensure your GitHub credential has repo access and org SSO authorization."
				} else {
					response.AuthMessage = "GitHub authentication failed. Reconnect your GitHub credential and try again."
				}
			}
		}

		h.JSON(w, http.StatusOK, response)
		return
	}

	response.Valid = true
	response.Classification = "cloneable"
	h.JSON(w, http.StatusOK, response)
}

func normalizeGitPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if githubShorthandPattern.MatchString(trimmed) {
		return fmt.Sprintf("https://github.com/%s", trimmed)
	}

	if strings.HasPrefix(trimmed, "github.com/") {
		return "https://" + trimmed
	}

	if strings.HasPrefix(trimmed, "www.github.com/") {
		return "https://" + strings.TrimPrefix(trimmed, "www.")
	}

	return trimmed
}

func looksLikeGitRepositoryInput(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}

	if githubShorthandPattern.MatchString(trimmed) {
		return true
	}

	if strings.HasPrefix(trimmed, "github.com/") || strings.HasPrefix(trimmed, "www.github.com/") {
		return true
	}

	if strings.HasPrefix(trimmed, "git@") {
		return strings.Contains(trimmed, ":")
	}

	if strings.Contains(trimmed, "://") {
		parsedURL, err := url.Parse(trimmed)
		if err != nil {
			return false
		}
		return parsedURL.Scheme != "" && parsedURL.Host != ""
	}

	return false
}

func getRepoSuggestions(ctx context.Context, query string, githubToken string) []api.Suggestion {
	normalizedGitHubQuery := normalizeGitHubRepoQuery(query)

	if githubToken != "" && normalizedGitHubQuery != "" {
		repoSuggestions, repoErr := getGitHubRepoSuggestions(ctx, query, githubToken)
		orgSuggestions := []api.Suggestion{}
		var orgErr error

		if !strings.Contains(normalizedGitHubQuery, "/") {
			orgSuggestions, orgErr = getGitHubOrgSuggestions(ctx, normalizedGitHubQuery, githubToken)
		}

		if repoErr == nil && orgErr == nil && (len(repoSuggestions) > 0 || len(orgSuggestions) > 0) {
			merged := make([]api.Suggestion, 0, len(repoSuggestions)+len(orgSuggestions))
			seen := make(map[string]struct{}, len(repoSuggestions)+len(orgSuggestions))

			appendSuggestion := func(suggestion api.Suggestion) {
				if suggestion.Value == "" {
					return
				}
				if _, exists := seen[suggestion.Value]; exists {
					return
				}
				seen[suggestion.Value] = struct{}{}
				merged = append(merged, suggestion)
			}

			for _, suggestion := range repoSuggestions {
				appendSuggestion(suggestion)
			}
			for _, suggestion := range orgSuggestions {
				appendSuggestion(suggestion)
			}

			if len(merged) > 10 {
				return merged[:10]
			}

			return merged
		}

		if strings.Contains(normalizedGitHubQuery, "/") {
			return []api.Suggestion{}
		}
	}

	staticSuggestions := getStaticRepoSuggestions(query)
	return staticSuggestions
}

func getStaticRepoSuggestions(query string) []api.Suggestion {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []api.Suggestion{}
	}

	seen := map[string]struct{}{}
	suggestions := make([]api.Suggestion, 0, 3)
	appendSuggestion := func(value string) {
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		suggestions = append(suggestions, api.Suggestion{Value: value, Type: "repo", Valid: true})
	}

	if looksLikeGitRepositoryInput(trimmed) {
		if !githubShorthandPattern.MatchString(trimmed) {
			appendSuggestion(normalizeGitPath(trimmed))
		}
	}

	if short, ok := strings.CutPrefix(trimmed, "https://github.com/"); ok {
		short = strings.TrimSuffix(short, ".git")
		short = strings.Trim(short, "/")
		if githubShorthandPattern.MatchString(short) {
			appendSuggestion(short)
		}
	}

	if len(suggestions) > 10 {
		return suggestions[:10]
	}

	return suggestions
}

type githubRepositorySearchResponse struct {
	Items []struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
	} `json:"items"`
}

type githubUserSearchResponse struct {
	Items []struct {
		Login string `json:"login"`
	} `json:"items"`
}

func getGitHubRepoSuggestions(ctx context.Context, query string, githubToken string) ([]api.Suggestion, error) {
	if githubToken == "" {
		return nil, nil
	}

	if owner, repoPrefix, ok := splitGitHubOwnerRepoPrefix(normalizeGitHubRepoQuery(query)); ok {
		ownerSuggestions, ownerErr := getGitHubOwnerRepoSuggestions(ctx, owner, repoPrefix, githubToken)
		if ownerErr == nil && len(ownerSuggestions) > 0 {
			return ownerSuggestions, nil
		}
	}

	searchQuery := buildGitHubSearchQuery(query)
	if searchQuery == "" {
		return nil, nil
	}

	requestContext, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	requestURL := "https://api.github.com/search/repositories?q=" + url.QueryEscape(searchQuery) + "&per_page=10"
	req, err := http.NewRequestWithContext(requestContext, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	setGitHubRequestHeaders(req, githubToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("github repository search failed with status %d", resp.StatusCode)
	}

	var payload githubRepositorySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	suggestions := make([]api.Suggestion, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.FullName == "" {
			continue
		}
		suggestions = append(suggestions, api.Suggestion{
			Value: item.FullName,
			Type:  "repo",
			Valid: true,
		})
	}

	return suggestions, nil
}

func getGitHubOrgSuggestions(ctx context.Context, query string, githubToken string) ([]api.Suggestion, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return nil, nil
	}

	requestContext, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	search := trimmedQuery + " in:login type:org"
	requestURL := "https://api.github.com/search/users?q=" + url.QueryEscape(search) + "&per_page=5"
	req, err := http.NewRequestWithContext(requestContext, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	setGitHubRequestHeaders(req, githubToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("github org search failed with status %d", resp.StatusCode)
	}

	var payload githubUserSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	suggestions := make([]api.Suggestion, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.Login == "" {
			continue
		}

		classification := "org"
		suggestions = append(suggestions, api.Suggestion{
			Value:          item.Login + "/",
			Type:           "repo",
			Valid:          false,
			Classification: &classification,
		})
	}

	return suggestions, nil
}

type githubRepositoryListItem struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
}

func getGitHubOwnerRepoSuggestions(ctx context.Context, owner, repoPrefix, githubToken string) ([]api.Suggestion, error) {
	if owner == "" {
		return nil, nil
	}

	if cachedRepos, ok := getCachedGitHubOwnerRepos(githubToken, owner); ok {
		return filterGitHubOwnerRepoSuggestions(cachedRepos, strings.ToLower(repoPrefix)), nil
	}

	requestContext, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	repos, err := fetchGitHubOwnerRepos(requestContext, owner, githubToken)
	if err != nil {
		return nil, err
	}

	setCachedGitHubOwnerRepos(githubToken, owner, repos)
	return filterGitHubOwnerRepoSuggestions(repos, strings.ToLower(repoPrefix)), nil
}

func fetchGitHubOwnerRepos(ctx context.Context, owner, githubToken string) ([]githubRepositoryListItem, error) {
	endpoints := []string{
		"https://api.github.com/orgs/" + url.PathEscape(owner) + "/repos?per_page=100&type=all&sort=updated",
		"https://api.github.com/users/" + url.PathEscape(owner) + "/repos?per_page=100&type=owner&sort=updated",
	}

	merged := make([]githubRepositoryListItem, 0, 100)
	seen := make(map[string]struct{}, 100)
	var lastErr error

	for _, endpoint := range endpoints {
		items, err := fetchGitHubRepositoryList(ctx, endpoint, githubToken)
		if err != nil {
			lastErr = err
			continue
		}

		for _, item := range items {
			if item.FullName == "" {
				continue
			}
			if _, exists := seen[item.FullName]; exists {
				continue
			}

			seen[item.FullName] = struct{}{}
			merged = append(merged, item)
		}
	}

	if len(merged) == 0 && lastErr != nil {
		return nil, lastErr
	}

	return merged, nil
}

func filterGitHubOwnerRepoSuggestions(repos []githubRepositoryListItem, lowerPrefix string) []api.Suggestion {
	suggestions := make([]api.Suggestion, 0, 10)
	for _, item := range repos {
		if item.FullName == "" {
			continue
		}
		if lowerPrefix != "" && !strings.HasPrefix(strings.ToLower(item.Name), lowerPrefix) {
			continue
		}

		suggestions = append(suggestions, api.Suggestion{
			Value: item.FullName,
			Type:  "repo",
			Valid: true,
		})

		if len(suggestions) >= 10 {
			break
		}
	}

	return suggestions
}

func getCachedGitHubOwnerRepos(githubToken string, owner string) ([]githubRepositoryListItem, bool) {
	cacheKey := buildGitHubRepoListCacheKey(githubToken, owner)
	now := time.Now()

	githubRepoListCache.RLock()
	entry, ok := githubRepoListCache.Entries[cacheKey]
	githubRepoListCache.RUnlock()

	if !ok || now.After(entry.ExpiresAt) {
		if ok {
			githubRepoListCache.Lock()
			delete(githubRepoListCache.Entries, cacheKey)
			githubRepoListCache.Unlock()
		}
		return nil, false
	}

	return append([]githubRepositoryListItem(nil), entry.Repos...), true
}

func setCachedGitHubOwnerRepos(githubToken string, owner string, repos []githubRepositoryListItem) {
	cacheKey := buildGitHubRepoListCacheKey(githubToken, owner)
	now := time.Now()

	githubRepoListCache.Lock()
	for key, entry := range githubRepoListCache.Entries {
		if now.After(entry.ExpiresAt) {
			delete(githubRepoListCache.Entries, key)
		}
	}

	githubRepoListCache.Entries[cacheKey] = githubRepoListCacheEntry{
		Repos:     append([]githubRepositoryListItem(nil), repos...),
		ExpiresAt: now.Add(githubRepoListCacheTTL),
	}
	githubRepoListCache.Unlock()
}

func buildGitHubRepoListCacheKey(githubToken string, owner string) string {
	sum := sha256.Sum256([]byte(githubToken))
	return strings.ToLower(strings.TrimSpace(owner)) + ":" + hex.EncodeToString(sum[:])
}

func fetchGitHubRepositoryList(ctx context.Context, requestURL string, githubToken string) ([]githubRepositoryListItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	setGitHubRequestHeaders(req, githubToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("github repo list failed with status %d", resp.StatusCode)
	}

	var payload []githubRepositoryListItem
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func splitGitHubOwnerRepoPrefix(query string) (owner string, repoPrefix string, ok bool) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" || !strings.Contains(trimmed, "/") {
		return "", "", false
	}

	owner, repoPrefix, _ = strings.Cut(trimmed, "/")
	owner = strings.TrimSpace(owner)
	repoPrefix = strings.TrimSpace(repoPrefix)
	if owner == "" {
		return "", "", false
	}

	return owner, repoPrefix, true
}

func setGitHubRequestHeaders(req *http.Request, githubToken string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "discobot-server")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}
}

func buildGitHubSearchQuery(input string) string {
	normalized := normalizeGitHubRepoQuery(input)
	if normalized == "" {
		return ""
	}

	if strings.Contains(normalized, "/") {
		owner, repo, _ := strings.Cut(normalized, "/")
		owner = strings.TrimSpace(owner)
		repo = strings.TrimSpace(repo)
		if owner == "" {
			return ""
		}
		if repo == "" {
			return "user:" + owner
		}
		return repo + " in:name user:" + owner
	}

	return normalized + " in:name"
}

func normalizeGitHubRepoQuery(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	if path, ok := strings.CutPrefix(trimmed, "git@github.com:"); ok {
		return strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	}

	if path, ok := strings.CutPrefix(trimmed, "github.com/"); ok {
		return strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	}

	if path, ok := strings.CutPrefix(trimmed, "www.github.com/"); ok {
		return strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		parsedURL, err := url.Parse(trimmed)
		if err != nil {
			return trimmed
		}
		if parsedURL.Host == "github.com" || parsedURL.Host == "www.github.com" {
			return strings.Trim(strings.TrimSuffix(parsedURL.Path, ".git"), "/")
		}
		return trimmed
	}

	return strings.Trim(strings.TrimSuffix(trimmed, ".git"), "/")
}

func isGitHubRepositoryURL(raw string) bool {
	lowerValue := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(lowerValue, "git@github.com:") {
		return true
	}

	parsedURL, err := url.Parse(lowerValue)
	if err != nil {
		return false
	}

	return parsedURL.Host == "github.com" || parsedURL.Host == "www.github.com"
}

func isGitHubRepositoryNotFound(err error) bool {
	if err == nil {
		return false
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "repository not found")
}

func validateGitRemote(ctx context.Context, remoteURL string, githubToken string) error {
	validationURL := remoteURL
	if convertedURL, ok := githubHTTPSValidationURL(remoteURL); ok {
		validationURL = convertedURL
	}

	args := []string{
		"-c", "credential.helper=",
		"-c", "core.askPass=",
		"-c", "credential.interactive=never",
	}
	if isGitHubRepositoryURL(validationURL) && githubToken != "" {
		authValue := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + githubToken))
		args = append(args, "-c", "http.extraHeader=AUTHORIZATION: basic "+authValue)
	}
	args = append(args, "ls-remote", "--exit-code", validationURL, "HEAD")

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = cleanGitValidationEnv()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		lower := strings.ToLower(message)
		switch {
		case strings.Contains(lower, "could not read username"),
			strings.Contains(lower, "terminal prompts disabled"),
			strings.Contains(lower, "authentication failed"),
			strings.Contains(lower, "could not authenticate"):
			return fmt.Errorf("repository is not cloneable: %w", errGitAuthenticationRequired)
		case strings.Contains(lower, "authorization failed"),
			strings.Contains(lower, "access denied"),
			strings.Contains(lower, "403"):
			return fmt.Errorf("repository is not cloneable: %w", errGitAuthorizationFailed)
		case message != "":
			return fmt.Errorf("repository is not cloneable: %s", message)
		default:
			return fmt.Errorf("repository is not cloneable: %w", err)
		}
	}

	return nil
}

func githubHTTPSValidationURL(raw string) (string, bool) {
	normalized := normalizeGitHubRepoQuery(raw)
	if normalized == "" || !strings.Contains(normalized, "/") {
		return "", false
	}
	if !isGitHubRepositoryURL(raw) {
		return "", false
	}
	return "https://github.com/" + normalized, true
}

func cleanGitValidationEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	return env
}

func (h *Handler) getGitHubToken(ctx context.Context, projectID string) (string, bool, error) {
	tokens, err := h.credentialService.GetOAuthTokens(ctx, projectID, service.ProviderGitHub)
	if err == nil && tokens != nil && tokens.AccessToken != "" {
		return tokens.AccessToken, true, nil
	}

	if err != nil && !errors.Is(err, service.ErrCredentialNotFound) {
		return "", false, err
	}

	apiKeyCredential, err := h.credentialService.GetAPIKey(ctx, projectID, service.ProviderGitHub)
	if err == nil && apiKeyCredential != nil && apiKeyCredential.APIKey != "" {
		return apiKeyCredential.APIKey, true, nil
	}

	if err != nil && !errors.Is(err, service.ErrCredentialNotFound) {
		return "", false, err
	}

	return "", false, nil
}

// ListWorkspaces returns all workspaces for a project
func (h *Handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	workspaces, err := h.workspaceService.ListWorkspaces(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list workspaces")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"workspaces": mapWorkspaceResponses(workspaces)})
}

// CreateWorkspace creates a new workspace
func (h *Handler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req api.CreateWorkspaceRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Path == "" {
		h.Error(w, http.StatusBadRequest, "Path is required")
		return
	}
	sourceType := "local"
	if req.SourceType != nil && *req.SourceType != "" {
		sourceType = *req.SourceType
	}

	workspace, err := h.workspaceService.CreateWorkspace(r.Context(), projectID, req.Path, sourceType)
	if err != nil {
		// Pass through the detailed error message from the service
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Update display name if provided
	if req.DisplayName != nil {
		// Get the model workspace and update it
		modelWorkspace, err := h.store.GetWorkspaceByID(r.Context(), workspace.ID)
		if err != nil {
			h.Error(w, http.StatusInternalServerError, "Failed to get workspace for update")
			return
		}
		modelWorkspace.DisplayName = req.DisplayName
		if err := h.store.UpdateWorkspace(r.Context(), modelWorkspace); err != nil {
			h.Error(w, http.StatusInternalServerError, "Failed to update workspace")
			return
		}
		// Update the response object
		workspace.DisplayName = req.DisplayName
	}

	// Enqueue workspace initialization job
	if err := h.jobQueue.Enqueue(r.Context(), jobs.WorkspaceInitPayload{ProjectID: projectID, WorkspaceID: workspace.ID}); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to enqueue workspace initialization")
		return
	}

	h.JSON(w, http.StatusCreated, mapWorkspaceResponse(workspace))
}

// GetWorkspace returns a single workspace
func (h *Handler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceId")

	workspace, err := h.workspaceService.GetWorkspaceWithSessions(r.Context(), workspaceID)
	if err != nil {
		h.Error(w, http.StatusNotFound, "Workspace not found")
		return
	}

	h.JSON(w, http.StatusOK, mapWorkspaceResponse(workspace))
}

// UpdateWorkspace updates a workspace
func (h *Handler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceId")

	// Parse raw JSON to detect which fields were sent
	var rawReq map[string]any
	if err := h.DecodeJSON(r, &rawReq); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the existing workspace
	workspace, err := h.store.GetWorkspaceByID(r.Context(), workspaceID)
	if err != nil {
		h.Error(w, http.StatusNotFound, "Workspace not found")
		return
	}

	modified := false

	// Update path if provided
	if path, ok := rawReq["path"].(string); ok {
		// UpdateWorkspace in service returns the full workspace, but we need to re-fetch
		// to ensure we have the latest model
		_, err = h.workspaceService.UpdateWorkspace(r.Context(), workspaceID, path)
		if err != nil {
			h.Error(w, http.StatusInternalServerError, "Failed to update workspace")
			return
		}
		// Re-fetch to get updated workspace
		workspace, err = h.store.GetWorkspaceByID(r.Context(), workspaceID)
		if err != nil {
			h.Error(w, http.StatusInternalServerError, "Failed to get updated workspace")
			return
		}
		modified = true
	}

	// Update display name if the field was sent (even if null to clear it)
	if displayName, ok := rawReq["displayName"]; ok {
		if displayName == nil {
			workspace.DisplayName = nil
		} else if str, ok := displayName.(string); ok {
			workspace.DisplayName = &str
		}
		modified = true
	}

	// Save if we modified the workspace
	if modified {
		if err := h.store.UpdateWorkspace(r.Context(), workspace); err != nil {
			h.Error(w, http.StatusInternalServerError, "Failed to update workspace")
			return
		}
	}

	// Map to service workspace for response
	serviceWorkspace, err := h.workspaceService.GetWorkspace(r.Context(), workspaceID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to get updated workspace")
		return
	}
	h.JSON(w, http.StatusOK, mapWorkspaceResponse(serviceWorkspace))
}

// DeleteWorkspace deletes a workspace
// Query params:
//   - deleteFiles: if "true", also delete the workspace files from disk
func (h *Handler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceId")
	deleteFiles := r.URL.Query().Get("deleteFiles") == "true"

	if err := h.workspaceService.DeleteWorkspace(r.Context(), workspaceID, deleteFiles); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to delete workspace")
		return
	}

	h.JSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GetProviders returns all sandbox providers with their status.
// GET /api/projects/{projectId}/workspaces/providers
func (h *Handler) GetProviders(w http.ResponseWriter, _ *http.Request) {
	h.JSON(w, http.StatusOK, map[string]any{
		"providers": h.sandboxService.ListProviderStatuses(),
		"default":   h.sandboxService.DefaultProviderName(),
	})
}

// GetProvider returns the status of a specific sandbox provider.
// GET /api/projects/{projectId}/workspaces/providers/{provider}
func (h *Handler) GetProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")

	status, ok := h.sandboxService.ProviderStatus(name)
	if !ok {
		h.Error(w, http.StatusNotFound, "Provider not found")
		return
	}

	h.JSON(w, http.StatusOK, status)
}
