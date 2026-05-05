package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/encryption"
	"github.com/obot-platform/discobot/server/internal/keyvalidator"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/oauth"
	"github.com/obot-platform/discobot/server/internal/providers"
	"github.com/obot-platform/discobot/server/internal/store"
)

// Supported providers
const (
	ProviderAnthropic     = "anthropic"
	ProviderGitHubCopilot = "github-copilot"
	ProviderGitHub        = "github-git"
	ProviderCodex         = "codex"
	ProviderOpenAI        = "openai"
	ProviderTavily        = "tavily"
	ProviderDiscobot      = "discobot"
)

// mcpProviderPrefix is the credential provider prefix for MCP OAuth tokens.
// The full provider key is "mcp:{resourceUrl}".
const mcpProviderPrefix = "mcp:"

// customProviderPrefix identifies ad-hoc user-defined env bundle credentials.
const customProviderPrefix = "custom:"

// Auth types
const (
	AuthTypeAPIKey = "api_key"
	AuthTypeOAuth  = "oauth"
	AuthTypeID     = "id"
)

// oauthEnvVars maps provider IDs to their OAuth-specific environment variable names.
// If a provider has an OAuth-specific env var, it will be used instead of the provider's
// default env var when the credential type is OAuth.
var oauthEnvVars = map[string]string{
	ProviderAnthropic: "CLAUDE_CODE_OAUTH_TOKEN",
	// Add more OAuth-specific env vars here as needed
	// e.g., ProviderOpenAI: "OPENAI_OAUTH_TOKEN",
}

var (
	ErrCredentialNotFound = errors.New("credential not found")
	ErrInvalidProvider    = errors.New("invalid provider")
	ErrEncryptionFailed   = errors.New("encryption failed")
	ErrDecryptionFailed   = errors.New("decryption failed")
)

func providerAllowsMultipleCredentials(provider string) bool {
	return provider == ProviderGitHub
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeOAuthScopes(scope string) []string {
	fields := strings.FieldsFunc(scope, func(r rune) bool {
		return r == ' ' || r == ','
	})
	if len(fields) == 0 {
		return nil
	}
	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		scope := strings.TrimSpace(field)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	return result
}

// APIKeyCredential is a compatibility view over the first secret env var.
type APIKeyCredential struct {
	APIKey string `json:"api_key"`
}

// SecretEnvVar is a single environment variable stored in a credential.
type SecretEnvVar struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	OriginalKey string `json:"originalKey,omitempty"`
}

// SecretCredentialData represents one or more encrypted env vars for secret-style credentials.
// It is used for api_key, id, and custom env credentials.
type SecretCredentialData struct {
	EnvVars []SecretEnvVar `json:"envVars"`
}

// OAuthCredential represents OAuth tokens
type OAuthCredential struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitzero"`
	Scope        string    `json:"scope,omitempty"`
}

type CredentialVisibility struct {
	Tools    bool `json:"tools"`
	Console  bool `json:"console"`
	Services bool `json:"services"`
	Hooks    bool `json:"hooks"`
}

// CredentialInfo represents safe credential info for API responses (no secrets)
type CredentialInfo struct {
	ID           string               `json:"id"`
	Provider     string               `json:"provider"`
	Name         string               `json:"name"`
	Description  string               `json:"description,omitempty"`
	AuthType     string               `json:"authType"`
	IsConfigured bool                 `json:"isConfigured"`
	Inactive     bool                 `json:"inactive"`
	AgentVisible bool                 `json:"agentVisible"`
	Visibility   CredentialVisibility `json:"visibility"`
	EnvKeys      []string             `json:"envKeys,omitempty"`
	EnvVars      []SecretEnvVar       `json:"envVars,omitempty"`
	Scopes       []string             `json:"scopes,omitempty"`
	ExpiresAt    *time.Time           `json:"expiresAt,omitempty"` // For OAuth credentials
	CreatedAt    time.Time            `json:"createdAt"`
	UpdatedAt    time.Time            `json:"updatedAt"`
}

const (
	CredentialValidationStatusValid       = "valid"
	CredentialValidationStatusInvalid     = "invalid"
	CredentialValidationStatusUnsupported = "unsupported"
	CredentialValidationStatusError       = "error"
)

type CredentialValidationInfo struct {
	CredentialID string    `json:"credentialId"`
	Provider     string    `json:"provider"`
	AuthType     string    `json:"authType"`
	Status       string    `json:"status"`
	Message      string    `json:"message,omitempty"`
	CheckedAt    time.Time `json:"checkedAt,omitzero"`
}

// SessionCredentialAssignmentInfo is the client-safe representation of a credential assigned to a session.
type SessionCredentialAssignmentInfo struct {
	CredentialID        string                 `json:"credentialId"`
	SessionCredentialID string                 `json:"sessionCredentialId,omitempty"`
	EnvVar              string                 `json:"envVar,omitempty"`
	SourceEnvVar        string                 `json:"sourceEnvVar,omitempty"`
	AgentVisible        bool                   `json:"agentVisible"`
	Visibility          CredentialVisibility   `json:"visibility"`
	Uses                []SessionCredentialUse `json:"uses,omitempty"`
	Credential          CredentialInfo         `json:"credential"`
}

type SessionCredentialUse struct {
	ID                 string    `json:"id"`
	Description        string    `json:"description"`
	CreatedAt          time.Time `json:"createdAt,omitzero"`
	ExpiresAt          time.Time `json:"expiresAt,omitzero"`
	LastUsedAt         time.Time `json:"lastUsedAt,omitzero"`
	LastUsedToolCallID string    `json:"lastUsedToolCallId,omitempty"`
}

const sessionCredentialUseDuration = time.Hour

// CredentialService handles credential operations with encryption
type CredentialService struct {
	store            *store.Store
	cfg              *config.Config
	encryptor        *encryption.Encryptor
	keyValidators    *keyvalidator.Registry
	lastRefreshFail  map[string]time.Time // Track last refresh failure per provider
	refreshFailMutex sync.RWMutex         // Protect the map
}

type startupCredentialImportSpec struct {
	provider string
	authType string
	envVar   string
}

var startupCredentialImportSpecs = []startupCredentialImportSpec{
	{provider: ProviderAnthropic, authType: AuthTypeAPIKey, envVar: "ANTHROPIC_API_KEY"},
	{provider: ProviderAnthropic, authType: AuthTypeOAuth, envVar: "CLAUDE_CODE_OAUTH_TOKEN"},
	{provider: ProviderOpenAI, authType: AuthTypeAPIKey, envVar: "OPENAI_API_KEY"},
	{provider: ProviderCodex, authType: AuthTypeOAuth, envVar: "CODEX_TOKEN"},
	{provider: ProviderTavily, authType: AuthTypeAPIKey, envVar: "TAVILY_API_KEY"},
	{provider: ProviderDiscobot, authType: AuthTypeID, envVar: "DISCOBOT_TOKEN"},
}

// NewCredentialService creates a new credential service
func NewCredentialService(s *store.Store, cfg *config.Config) (*CredentialService, error) {
	return NewCredentialServiceWithValidators(s, cfg, keyvalidator.DefaultRegistry(nil))
}

// NewCredentialServiceWithValidators creates a credential service with an explicit key validator registry.
func NewCredentialServiceWithValidators(s *store.Store, cfg *config.Config, validators *keyvalidator.Registry) (*CredentialService, error) {
	enc, err := encryption.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}

	return &CredentialService{
		store:           s,
		cfg:             cfg,
		encryptor:       enc,
		keyValidators:   validators,
		lastRefreshFail: make(map[string]time.Time),
	}, nil
}

func generateSessionScopedID(prefix string) string {
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + uuid.NewString()[:8]
	}
	token := strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes[:]), "=")
	return prefix + strings.ToLower(token)
}

func sessionCredentialAssignmentBindingKey(credentialID, envVar string) string {
	return credentialID + "\x00" + strings.TrimSpace(envVar)
}

func normalizeSessionCredentialUses(existing []SessionCredentialUse, requested []SessionCredentialUse) []SessionCredentialUse {
	now := time.Now().UTC()
	existingByID := make(map[string]SessionCredentialUse, len(existing))
	existingByDescription := make(map[string]SessionCredentialUse, len(existing))
	for _, use := range existing {
		key := strings.TrimSpace(strings.ToLower(use.Description))
		if key == "" {
			continue
		}
		if use.ID == "" {
			use.ID = generateSessionScopedID("use_s_")
		}
		if use.CreatedAt.IsZero() {
			use.CreatedAt = now
		}
		if use.ExpiresAt.IsZero() {
			use.ExpiresAt = use.CreatedAt.Add(sessionCredentialUseDuration)
		}
		existingByID[use.ID] = use
		existingByDescription[key] = use
	}

	resultByDescription := make(map[string]SessionCredentialUse, len(requested))
	for _, use := range requested {
		desc := strings.TrimSpace(use.Description)
		if desc == "" {
			continue
		}
		key := strings.ToLower(desc)
		base, ok := existingByID[strings.TrimSpace(use.ID)]
		if !ok {
			base, ok = existingByDescription[key]
		}

		use.Description = desc
		if strings.TrimSpace(use.ID) == "" {
			if ok && base.ID != "" {
				use.ID = base.ID
			} else {
				use.ID = generateSessionScopedID("use_s_")
			}
		}
		if use.CreatedAt.IsZero() {
			if ok && !base.CreatedAt.IsZero() {
				use.CreatedAt = base.CreatedAt
			} else {
				use.CreatedAt = now
			}
		}
		if use.ExpiresAt.IsZero() {
			if ok && !base.ExpiresAt.IsZero() {
				use.ExpiresAt = base.ExpiresAt
			} else {
				use.ExpiresAt = use.CreatedAt.Add(sessionCredentialUseDuration)
			}
		} else {
			use.ExpiresAt = use.ExpiresAt.UTC()
		}
		if ok && base.LastUsedAt.After(use.LastUsedAt) {
			use.LastUsedAt = base.LastUsedAt
		}
		if strings.TrimSpace(use.LastUsedToolCallID) == "" && ok {
			use.LastUsedToolCallID = strings.TrimSpace(base.LastUsedToolCallID)
		}
		resultByDescription[key] = use
	}

	result := make([]SessionCredentialUse, 0, len(resultByDescription))
	for _, use := range resultByDescription {
		result = append(result, use)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Description < result[j].Description
	})
	return result
}

func parseSessionCredentialUses(data json.RawMessage) []SessionCredentialUse {
	if len(data) == 0 {
		return nil
	}
	var uses []SessionCredentialUse
	if err := json.Unmarshal(data, &uses); err != nil {
		return nil
	}
	now := time.Now().UTC()
	for i := range uses {
		if uses[i].ID == "" {
			uses[i].ID = generateSessionScopedID("use_s_")
		}
		if uses[i].CreatedAt.IsZero() {
			uses[i].CreatedAt = now
		}
		if uses[i].ExpiresAt.IsZero() {
			uses[i].ExpiresAt = uses[i].CreatedAt.Add(sessionCredentialUseDuration)
		}
	}
	return uses
}

func activeSessionCredentialUses(uses []SessionCredentialUse, now time.Time) []SessionCredentialUse {
	if len(uses) == 0 {
		return nil
	}
	active := make([]SessionCredentialUse, 0, len(uses))
	for _, use := range uses {
		if !use.ExpiresAt.IsZero() && !use.ExpiresAt.After(now) {
			continue
		}
		active = append(active, use)
	}
	return active
}

func defaultCredentialVisibility(provider string) CredentialVisibility {
	return CredentialVisibility{
		Tools: defaultCredentialAgentVisible(provider),
	}
}

func credentialVisibilityForModel(cred *model.Credential) CredentialVisibility {
	return CredentialVisibility{
		Tools:    cred.AgentVisible,
		Console:  cred.ConsoleVisible,
		Services: cred.ServiceVisible,
		Hooks:    cred.HookVisible,
	}
}

func credentialVisibilityForAssignment(assignment *model.SessionCredentialAssignment) CredentialVisibility {
	return CredentialVisibility{
		Tools:    assignment.AgentVisible,
		Console:  assignment.ConsoleVisible,
		Services: assignment.ServiceVisible,
		Hooks:    assignment.HookVisible,
	}
}

func effectiveCredentialVisibility(globalVisibility, sessionVisibility CredentialVisibility) CredentialVisibility {
	return CredentialVisibility{
		Tools:    globalVisibility.Tools || sessionVisibility.Tools,
		Console:  globalVisibility.Console || sessionVisibility.Console,
		Services: globalVisibility.Services || sessionVisibility.Services,
		Hooks:    globalVisibility.Hooks || sessionVisibility.Hooks,
	}
}

// List returns all credentials for a project (safe info only, no secrets)
func (s *CredentialService) List(ctx context.Context, projectID string) ([]CredentialInfo, error) {
	creds, err := s.store.ListCredentialsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	result := make([]CredentialInfo, len(creds))
	for i, c := range creds {
		result[i] = s.toCredentialInfo(c)
	}
	return result, nil
}

// ImportEnvCredentials creates credentials in a project from known process env vars.
// It is additive only: existing credentials are never modified or overridden.
func (s *CredentialService) ImportEnvCredentials(ctx context.Context, projectID string) error {
	creds, err := s.store.ListCredentialsByProject(ctx, projectID)
	if err != nil {
		return err
	}

	existingProviders := make(map[string]struct{}, len(creds))
	existingEnvVars := make(map[string]struct{})
	for _, cred := range creds {
		existingProviders[cred.Provider] = struct{}{}
		for _, envVar := range s.credentialEnvVars(cred) {
			existingEnvVars[envVar] = struct{}{}
		}
	}

	for _, spec := range startupCredentialImportSpecs {
		value := strings.TrimSpace(os.Getenv(spec.envVar))
		if value == "" {
			continue
		}
		if _, exists := existingProviders[spec.provider]; exists {
			continue
		}
		if _, exists := existingEnvVars[spec.envVar]; exists {
			continue
		}

		name := importedCredentialName(spec.provider, spec.authType)
		description := defaultCredentialDescription(spec.provider)

		switch spec.authType {
		case AuthTypeAPIKey:
			if _, err := s.SetAPIKeyWithMetadata(ctx, projectID, spec.provider, name, description, value, defaultCredentialVisibility(spec.provider), false); err != nil {
				return err
			}
		case AuthTypeID:
			if _, err := s.SetIDWithMetadata(ctx, projectID, spec.provider, name, description, value, defaultCredentialVisibility(spec.provider), false); err != nil {
				return err
			}
		case AuthTypeOAuth:
			if _, err := s.SetOAuthTokens(ctx, projectID, spec.provider, name, &OAuthCredential{
				AccessToken: value,
				TokenType:   "Bearer",
			}); err != nil {
				return err
			}
		default:
			continue
		}

		log.Printf("credentials: imported %s credential from %s into project %s", spec.provider, spec.envVar, projectID)

		existingProviders[spec.provider] = struct{}{}
		existingEnvVars[spec.envVar] = struct{}{}
	}

	return nil
}

// Get returns credential info for a specific provider (safe info only, no secrets)
func (s *CredentialService) Get(ctx context.Context, projectID, provider string) (*CredentialInfo, error) {
	cred, err := s.store.GetCredentialByProvider(ctx, projectID, provider)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}

	info := s.toCredentialInfo(cred)
	return &info, nil
}

// GetByID returns credential info for a specific credential ID without secrets.
func (s *CredentialService) GetByID(ctx context.Context, projectID, credentialID string) (*CredentialInfo, error) {
	cred, err := s.store.GetCredentialByIDForProject(ctx, projectID, credentialID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}

	info := s.toCredentialInfo(cred)
	return &info, nil
}

// ValidateByID validates a stored credential by credential ID.
func (s *CredentialService) ValidateByID(ctx context.Context, projectID, credentialID string) (*CredentialValidationInfo, error) {
	cred, err := s.store.GetCredentialByIDForProject(ctx, projectID, credentialID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}
	info := s.validateStoredCredential(ctx, cred)
	return &info, nil
}

// ValidateByProvider validates a stored credential by provider.
func (s *CredentialService) ValidateByProvider(ctx context.Context, projectID, provider string) (*CredentialValidationInfo, error) {
	cred, err := s.store.GetCredentialByProvider(ctx, projectID, provider)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}
	info := s.validateStoredCredential(ctx, cred)
	return &info, nil
}

// ValidateAll validates every stored credential in a project.
func (s *CredentialService) ValidateAll(ctx context.Context, projectID string) ([]CredentialValidationInfo, error) {
	creds, err := s.store.ListCredentialsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	result := make([]CredentialValidationInfo, len(creds))
	for i, cred := range creds {
		result[i] = s.validateStoredCredential(ctx, cred)
	}
	return result, nil
}

// UpdateMetadata updates non-secret credential fields without changing stored secret values.
func (s *CredentialService) UpdateMetadata(ctx context.Context, projectID, credentialID, name, description string, visibility CredentialVisibility, inactive bool) (*CredentialInfo, error) {
	cred, err := s.store.GetCredentialByIDForProject(ctx, projectID, credentialID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}

	if strings.TrimSpace(name) != "" {
		cred.Name = name
	}
	description = strings.TrimSpace(description)
	if description == "" {
		cred.Description = nil
	} else {
		cred.Description = &description
	}
	cred.AgentVisible = visibility.Tools
	cred.ConsoleVisible = visibility.Console
	cred.ServiceVisible = visibility.Services
	cred.HookVisible = visibility.Hooks
	cred.Inactive = inactive
	if err := s.store.UpdateCredential(ctx, cred); err != nil {
		return nil, err
	}
	info := s.toCredentialInfo(cred)
	if cred.AuthType != AuthTypeOAuth {
		if data, err := s.getSecretData(cred); err == nil {
			info.EnvKeys = secretEnvKeys(data.EnvVars)
		}
	}
	return &info, nil
}

// ListSessionAssignments returns all credentials for a session, including
// the global credential visibility and any session-specific visibility overrides.
func (s *CredentialService) ListSessionAssignments(ctx context.Context, projectID, sessionID string) ([]SessionCredentialAssignmentInfo, error) {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.ProjectID != projectID {
		return nil, ErrCredentialNotFound
	}
	creds, err := s.store.ListCredentialsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	assignments, err := s.store.ListSessionCredentialAssignments(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	assignmentsByCredentialID := make(map[string][]*model.SessionCredentialAssignment, len(assignments))
	for _, assignment := range assignments {
		assignmentsByCredentialID[assignment.CredentialID] = append(assignmentsByCredentialID[assignment.CredentialID], assignment)
	}
	result := make([]SessionCredentialAssignmentInfo, 0, len(creds)+len(assignments))
	for _, cred := range creds {
		globalVisibility := credentialVisibilityForModel(cred)
		info := s.toCredentialInfo(cred)
		info.AgentVisible = globalVisibility.Tools
		info.Visibility = globalVisibility

		credentialAssignments := assignmentsByCredentialID[cred.ID]
		if len(credentialAssignments) == 0 {
			result = append(result, SessionCredentialAssignmentInfo{
				CredentialID: cred.ID,
				AgentVisible: globalVisibility.Tools,
				Visibility:   globalVisibility,
				Credential:   info,
			})
			continue
		}

		for _, assignment := range credentialAssignments {
			sessionVisibility := credentialVisibilityForAssignment(assignment)
			result = append(result, SessionCredentialAssignmentInfo{
				CredentialID:        cred.ID,
				SessionCredentialID: assignment.SessionCredentialID,
				EnvVar:              assignment.EnvVar,
				SourceEnvVar:        assignment.SourceEnvVar,
				AgentVisible:        sessionVisibility.Tools,
				Visibility:          sessionVisibility,
				Uses:                parseSessionCredentialUses(assignment.UsesJSON),
				Credential:          info,
			})
		}
	}
	return result, nil
}

// SetSessionAssignments replaces session-specific visibility overrides for credentials.
func (s *CredentialService) SetSessionAssignments(ctx context.Context, projectID, sessionID string, assignments []SessionCredentialAssignmentInfo) ([]SessionCredentialAssignmentInfo, error) {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.ProjectID != projectID {
		return nil, ErrCredentialNotFound
	}
	existingAssignments, err := s.store.ListSessionCredentialAssignments(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	existingByBindingKey := make(map[string]*model.SessionCredentialAssignment, len(existingAssignments))
	for _, assignment := range existingAssignments {
		existingByBindingKey[sessionCredentialAssignmentBindingKey(assignment.CredentialID, assignment.EnvVar)] = assignment
	}
	if err := s.store.DeleteSessionCredentialAssignments(ctx, sessionID); err != nil {
		return nil, err
	}
	baseTime := time.Now().UTC()
	for i, assignment := range assignments {
		cred, err := s.store.GetCredentialByIDForProject(ctx, projectID, assignment.CredentialID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, ErrCredentialNotFound
			}
			return nil, err
		}
		envVar := strings.TrimSpace(assignment.EnvVar)
		existing := existingByBindingKey[sessionCredentialAssignmentBindingKey(cred.ID, envVar)]
		sessionCredentialID := strings.TrimSpace(assignment.SessionCredentialID)
		if sessionCredentialID == "" && existing != nil {
			sessionCredentialID = existing.SessionCredentialID
		}
		if sessionCredentialID == "" {
			sessionCredentialID = generateSessionScopedID("cred_s_")
		}
		if envVar == "" && existing != nil {
			envVar = existing.EnvVar
		}
		sourceEnvVar := strings.TrimSpace(assignment.SourceEnvVar)
		if sourceEnvVar == "" && existing != nil {
			sourceEnvVar = existing.SourceEnvVar
		}
		var existingUses []SessionCredentialUse
		if existing != nil {
			existingUses = parseSessionCredentialUses(existing.UsesJSON)
		}
		timestamp := baseTime.Add(time.Duration(i) * time.Nanosecond)
		createdAt := timestamp
		if existing != nil && !existing.CreatedAt.IsZero() {
			createdAt = existing.CreatedAt
		}
		uses := normalizeSessionCredentialUses(existingUses, assignment.Uses)
		usesJSON, err := json.Marshal(uses)
		if err != nil {
			return nil, err
		}
		if err := s.store.UpsertSessionCredentialAssignment(ctx, &model.SessionCredentialAssignment{
			SessionID:           sessionID,
			CredentialID:        cred.ID,
			SessionCredentialID: sessionCredentialID,
			EnvVar:              envVar,
			SourceEnvVar:        sourceEnvVar,
			AgentVisible:        assignment.Visibility.Tools,
			ConsoleVisible:      assignment.Visibility.Console,
			ServiceVisible:      assignment.Visibility.Services,
			HookVisible:         assignment.Visibility.Hooks,
			UsesJSON:            usesJSON,
			CreatedAt:           createdAt,
			UpdatedAt:           timestamp,
		}); err != nil {
			return nil, err
		}
	}
	return s.ListSessionAssignments(ctx, projectID, sessionID)
}

// SetAPIKey creates or updates an API key credential.
func (s *CredentialService) SetAPIKey(ctx context.Context, projectID, provider, name, apiKey string) (*CredentialInfo, error) {
	return s.SetAPIKeyWithMetadata(ctx, projectID, provider, name, "", apiKey, defaultCredentialVisibility(provider), false)
}

// SetAPIKeyWithMetadata creates or updates an API key credential with explicit metadata.
func (s *CredentialService) SetAPIKeyWithMetadata(ctx context.Context, projectID, provider, name, description, apiKey string, visibility CredentialVisibility, inactive bool) (*CredentialInfo, error) {
	return s.SetAPIKeyCredentialWithMetadata(ctx, projectID, "", provider, name, description, apiKey, visibility, inactive)
}

// SetAPIKeyCredentialWithMetadata creates or updates a secret credential with explicit metadata.
func (s *CredentialService) SetAPIKeyCredentialWithMetadata(ctx context.Context, projectID, credentialID, provider, name, description, apiKey string, visibility CredentialVisibility, inactive bool) (*CredentialInfo, error) {
	return s.setSecretCredential(ctx, projectID, credentialID, provider, name, description, AuthTypeAPIKey, []SecretEnvVar{{
		Key:   firstProviderEnvVar(provider),
		Value: apiKey,
	}}, visibility, inactive)
}

// SetID creates or updates an ID credential.
func (s *CredentialService) SetID(ctx context.Context, projectID, provider, name, value string) (*CredentialInfo, error) {
	return s.SetIDWithMetadata(ctx, projectID, provider, name, "", value, defaultCredentialVisibility(provider), false)
}

// SetIDWithMetadata creates or updates an ID credential with explicit metadata.
func (s *CredentialService) SetIDWithMetadata(ctx context.Context, projectID, provider, name, description, value string, visibility CredentialVisibility, inactive bool) (*CredentialInfo, error) {
	return s.SetIDCredentialWithMetadata(ctx, projectID, "", provider, name, description, value, visibility, inactive)
}

// SetIDCredentialWithMetadata creates or updates an ID credential with explicit metadata.
func (s *CredentialService) SetIDCredentialWithMetadata(ctx context.Context, projectID, credentialID, provider, name, description, value string, visibility CredentialVisibility, inactive bool) (*CredentialInfo, error) {
	return s.setSecretCredential(ctx, projectID, credentialID, provider, name, description, AuthTypeID, []SecretEnvVar{{
		Key:   firstProviderEnvVar(provider),
		Value: value,
	}}, visibility, inactive)
}

// SetCustomCredential creates or updates a custom env bundle credential.
func (s *CredentialService) SetCustomCredential(ctx context.Context, projectID, credentialID, name, description string, envVars []SecretEnvVar, visibility CredentialVisibility, inactive bool) (*CredentialInfo, error) {
	provider := customProviderPrefix + uuid.NewString()
	if credentialID != "" {
		existing, err := s.store.GetCredentialByIDForProject(ctx, projectID, credentialID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		if existing != nil {
			provider = existing.Provider
		}
	}
	return s.setSecretCredential(ctx, projectID, credentialID, provider, name, description, AuthTypeAPIKey, envVars, visibility, inactive)
}

func (s *CredentialService) setSecretCredential(ctx context.Context, projectID, credentialID, provider, name, description, authType string, envVars []SecretEnvVar, visibility CredentialVisibility, inactive bool) (*CredentialInfo, error) {
	if !isValidProvider(provider) {
		return nil, ErrInvalidProvider
	}

	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)

	var (
		existing *model.Credential
		err      error
	)
	if credentialID != "" {
		existing, err = s.store.GetCredentialByIDForProject(ctx, projectID, credentialID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
	} else if !providerAllowsMultipleCredentials(provider) {
		existing, err = s.store.GetCredentialByProvider(ctx, projectID, provider)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
	}

	normalizedEnvVars := normalizeSecretEnvVars(envVars)
	var existingData *SecretCredentialData
	if existing != nil && existing.AuthType != AuthTypeOAuth {
		existingData, err = s.getSecretData(existing)
		if err != nil {
			return nil, err
		}
		normalizedEnvVars = mergeSecretEnvVars(existingData.EnvVars, normalizedEnvVars)
	}
	if authType == AuthTypeAPIKey && s.shouldValidateAPIKeyOnSave(existingData, normalizedEnvVars) {
		if err := s.keyValidators.ValidateAPIKey(ctx, provider, firstSecretEnvVarValue(normalizedEnvVars)); err != nil {
			return nil, err
		}
	}

	data := SecretCredentialData{EnvVars: normalizedEnvVars}
	encrypted, err := s.encryptor.EncryptJSON(data)
	if err != nil {
		return nil, ErrEncryptionFailed
	}

	var descriptionPtr *string
	if description != "" {
		descriptionPtr = &description
	}

	if existing != nil {
		existing.Name = name
		existing.Description = descriptionPtr
		existing.AuthType = authType
		existing.EncryptedData = encrypted
		existing.IsConfigured = true
		existing.Inactive = inactive
		existing.AgentVisible = visibility.Tools
		existing.ConsoleVisible = visibility.Console
		existing.ServiceVisible = visibility.Services
		existing.HookVisible = visibility.Hooks
		if err := s.store.UpdateCredential(ctx, existing); err != nil {
			return nil, err
		}
		info := s.toCredentialInfo(existing)
		info.EnvKeys = secretEnvKeys(data.EnvVars)
		return &info, nil
	}

	cred := &model.Credential{
		ProjectID:      projectID,
		Provider:       provider,
		Name:           name,
		Description:    descriptionPtr,
		AuthType:       authType,
		EncryptedData:  encrypted,
		IsConfigured:   true,
		Inactive:       inactive,
		AgentVisible:   visibility.Tools,
		ConsoleVisible: visibility.Console,
		ServiceVisible: visibility.Services,
		HookVisible:    visibility.Hooks,
	}
	if err := s.store.CreateCredential(ctx, cred); err != nil {
		return nil, err
	}

	info := s.toCredentialInfo(cred)
	info.EnvKeys = secretEnvKeys(data.EnvVars)
	return &info, nil
}

// SetOAuthTokens creates or updates OAuth tokens for a credential
func (s *CredentialService) SetOAuthTokens(ctx context.Context, projectID, provider, name string, tokens *OAuthCredential) (*CredentialInfo, error) {
	return s.SetOAuthTokensWithMetadata(ctx, projectID, "", provider, name, "", defaultCredentialVisibility(provider), false, tokens)
}

// SetOAuthTokensWithMetadata creates or updates OAuth tokens for a credential with explicit metadata.
func (s *CredentialService) SetOAuthTokensWithMetadata(ctx context.Context, projectID, credentialID, provider, name, description string, visibility CredentialVisibility, inactive bool, tokens *OAuthCredential) (*CredentialInfo, error) {
	if !isValidProvider(provider) {
		return nil, ErrInvalidProvider
	}
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		name = importedCredentialName(provider, AuthTypeOAuth)
	}

	encrypted, err := s.encryptor.EncryptJSON(tokens)
	if err != nil {
		return nil, ErrEncryptionFailed
	}

	var (
		existing *model.Credential
	)
	if credentialID != "" {
		existing, err = s.store.GetCredentialByIDForProject(ctx, projectID, credentialID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, ErrCredentialNotFound
			}
			return nil, err
		}
	} else if !providerAllowsMultipleCredentials(provider) {
		existing, err = s.store.GetCredentialByProvider(ctx, projectID, provider)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
	}

	resolvedDescription := description
	if resolvedDescription == "" {
		resolvedDescription = defaultCredentialDescription(provider)
	}
	var descriptionPtr *string
	if resolvedDescription != "" {
		descriptionPtr = &resolvedDescription
	}

	if existing != nil {
		existing.Name = name
		existing.Description = descriptionPtr
		existing.AuthType = AuthTypeOAuth
		existing.EncryptedData = encrypted
		existing.IsConfigured = true
		existing.Inactive = inactive
		existing.AgentVisible = visibility.Tools
		existing.ConsoleVisible = visibility.Console
		existing.ServiceVisible = visibility.Services
		existing.HookVisible = visibility.Hooks
		if err := s.store.UpdateCredential(ctx, existing); err != nil {
			return nil, err
		}
		info := s.toCredentialInfo(existing)
		return &info, nil
	}

	cred := &model.Credential{
		ProjectID:      projectID,
		Provider:       provider,
		Name:           name,
		Description:    descriptionPtr,
		AuthType:       AuthTypeOAuth,
		EncryptedData:  encrypted,
		IsConfigured:   true,
		Inactive:       inactive,
		AgentVisible:   visibility.Tools,
		ConsoleVisible: visibility.Console,
		ServiceVisible: visibility.Services,
		HookVisible:    visibility.Hooks,
	}
	if err := s.store.CreateCredential(ctx, cred); err != nil {
		return nil, err
	}

	info := s.toCredentialInfo(cred)
	return &info, nil
}

// GetAPIKey retrieves and decrypts an API key credential
func (s *CredentialService) GetAPIKey(ctx context.Context, projectID, provider string) (*APIKeyCredential, error) {
	cred, err := s.store.GetCredentialByProvider(ctx, projectID, provider)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}

	if cred.AuthType != AuthTypeAPIKey && cred.AuthType != AuthTypeID {
		return nil, errors.New("credential is not a secret type")
	}

	data, err := s.getSecretData(cred)
	if err != nil {
		return nil, err
	}
	if len(data.EnvVars) == 0 {
		return &APIKeyCredential{}, nil
	}

	return &APIKeyCredential{APIKey: data.EnvVars[0].Value}, nil
}

// GetOAuthTokens retrieves and decrypts OAuth tokens.
// If the token is expired and a refresh token is available, it will automatically refresh.
func (s *CredentialService) GetOAuthTokens(ctx context.Context, projectID, provider string) (*OAuthCredential, error) {
	cred, err := s.store.GetCredentialByProvider(ctx, projectID, provider)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}

	if cred.AuthType != AuthTypeOAuth {
		return nil, errors.New("credential is not an OAuth type")
	}

	var tokens OAuthCredential
	if err := s.encryptor.DecryptJSON(cred.EncryptedData, &tokens); err != nil {
		return nil, ErrDecryptionFailed
	}

	// Check if token is expired (with 5 minute buffer for safety)
	if !tokens.ExpiresAt.IsZero() && time.Now().Add(5*time.Minute).After(tokens.ExpiresAt) {
		// Token is expired or about to expire
		if tokens.RefreshToken != "" {
			// Check if we recently failed to refresh this token (backoff mechanism)
			s.refreshFailMutex.RLock()
			lastFail, hasFailed := s.lastRefreshFail[provider]
			s.refreshFailMutex.RUnlock()

			// If we failed within the last 5 minutes, don't try again
			if hasFailed && time.Since(lastFail) < 5*time.Minute {
				log.Printf("Token for provider %s is expired, but skipping refresh (last attempt failed %v ago)",
					provider, time.Since(lastFail).Round(time.Second))
				return &tokens, nil
			}

			log.Printf("Token for provider %s is expired, attempting refresh", provider)
			refreshed, err := s.RefreshOAuthTokens(ctx, projectID, provider)
			if err != nil {
				log.Printf("Failed to refresh token for provider %s: %v", provider, err)
				// Record the failure time
				s.refreshFailMutex.Lock()
				s.lastRefreshFail[provider] = time.Now()
				s.refreshFailMutex.Unlock()
				// Return the expired token anyway, let the API call fail with proper auth error
				return &tokens, nil
			}
			// Clear the failure record on success
			s.refreshFailMutex.Lock()
			delete(s.lastRefreshFail, provider)
			s.refreshFailMutex.Unlock()
			log.Printf("Successfully refreshed token for provider %s", provider)
			return refreshed, nil
		}
		log.Printf("Token for provider %s is expired but no refresh token available", provider)
	}

	return &tokens, nil
}

// RefreshOAuthTokens refreshes OAuth tokens for a provider
func (s *CredentialService) RefreshOAuthTokens(ctx context.Context, projectID, provider string) (*OAuthCredential, error) {
	// Get existing credential directly from store to avoid recursion
	cred, err := s.store.GetCredentialByProvider(ctx, projectID, provider)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}

	if cred.AuthType != AuthTypeOAuth {
		return nil, errors.New("credential is not an OAuth type")
	}

	// Decrypt existing tokens
	var tokens OAuthCredential
	if err := s.encryptor.DecryptJSON(cred.EncryptedData, &tokens); err != nil {
		return nil, ErrDecryptionFailed
	}

	if tokens.RefreshToken == "" {
		return nil, errors.New("no refresh token available")
	}

	// Refresh based on provider
	var newTokenResp *oauth.TokenResponse
	switch provider {
	case ProviderAnthropic:
		p := oauth.NewAnthropicProvider(s.cfg.AnthropicClientID)
		newTokenResp, err = p.Refresh(ctx, tokens.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh %s token: %w", provider, err)
		}
	case ProviderCodex:
		p := oauth.NewCodexProvider(s.cfg.CodexClientID)
		newTokenResp, err = p.Refresh(ctx, tokens.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh %s token: %w", provider, err)
		}
	default:
		return nil, fmt.Errorf("token refresh not implemented for provider: %s", provider)
	}

	// Update stored tokens
	updatedTokens := &OAuthCredential{
		AccessToken:  newTokenResp.AccessToken,
		RefreshToken: newTokenResp.RefreshToken,
		TokenType:    newTokenResp.TokenType,
		ExpiresAt:    newTokenResp.ExpiresAt,
		Scope:        newTokenResp.Scope,
	}

	_, err = s.SetOAuthTokensWithMetadata(
		ctx,
		projectID,
		cred.ID,
		provider,
		cred.Name,
		strings.TrimSpace(derefString(cred.Description)),
		credentialVisibilityForModel(cred),
		cred.Inactive,
		updatedTokens,
	)
	if err != nil {
		return nil, err
	}

	return updatedTokens, nil
}

// Delete removes a credential by ID.
func (s *CredentialService) Delete(ctx context.Context, projectID, credentialID string) error {
	cred, err := s.store.GetCredentialByIDForProject(ctx, projectID, credentialID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCredentialNotFound
		}
		return err
	}
	if err := s.store.DeleteSessionCredentialAssignmentsForCredential(ctx, cred.ID); err != nil {
		return err
	}
	return s.store.DeleteCredentialByID(ctx, credentialID)
}

// CredentialEnvVar represents a credential value with its target environment variable.
// Used for passing credentials to agent containers.
type CredentialEnvVar struct {
	CredentialID        string                 `json:"credentialId,omitempty"`
	SessionCredentialID string                 `json:"sessionCredentialId,omitempty"`
	Uses                []SessionCredentialUse `json:"uses,omitempty"`
	EnvVar              string                 `json:"envVar"`
	Value               string                 `json:"value"`
	Provider            string                 `json:"provider"`
	AuthType            string                 `json:"authType"` // "api_key", "id", or "oauth"
	AgentVisible        bool                   `json:"agentVisible"`
	ConsoleVisible      bool                   `json:"consoleVisible"`
	ServiceVisible      bool                   `json:"serviceVisible"`
	HookVisible         bool                   `json:"hookVisible"`
	ExpiresAt           int64                  `json:"expiresAt,omitempty"` // OAuth only (unix timestamp)
	CreatedAt           time.Time              `json:"-"`
	UpdatedAt           time.Time              `json:"-"`
}

// GetAllDecrypted returns all configured credentials for a project as environment variable mappings.
func (s *CredentialService) GetAllDecrypted(ctx context.Context, projectID string) ([]CredentialEnvVar, error) {
	creds, err := s.store.ListCredentialsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return s.mapCredentialsToEnvVars(ctx, projectID, creds)
}

// GetAllForSession returns all credentials for a session with effective visibility applied.
func (s *CredentialService) GetAllForSession(ctx context.Context, projectID, sessionID string) ([]CredentialEnvVar, error) {
	creds, err := s.store.ListCredentialsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	assignments, err := s.store.ListSessionCredentialAssignments(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	assignmentByCredentialID := make(map[string][]*model.SessionCredentialAssignment, len(assignments))
	for _, assignment := range assignments {
		assignmentByCredentialID[assignment.CredentialID] = append(assignmentByCredentialID[assignment.CredentialID], assignment)
	}
	return s.mapCredentialsToEnvVarsWithAssignments(ctx, projectID, creds, assignmentByCredentialID)
}

type CredentialVisibilityContext string

const (
	CredentialVisibilityContextTools    CredentialVisibilityContext = "tools"
	CredentialVisibilityContextConsole  CredentialVisibilityContext = "console"
	CredentialVisibilityContextServices CredentialVisibilityContext = "services"
	CredentialVisibilityContextHooks    CredentialVisibilityContext = "hooks"
)

// GetVisibleEnvVarsForSession returns only the credentials that should be exposed
// to the requested runtime context for the given session.
func (s *CredentialService) GetVisibleEnvVarsForSession(ctx context.Context, sessionID string, visibilityContext CredentialVisibilityContext) (map[string]string, error) {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	envVars, err := s.GetAllForSession(ctx, sess.ProjectID, sessionID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(envVars, func(i, j int) bool {
		leftUpdatedAt := envVars[i].UpdatedAt
		if leftUpdatedAt.IsZero() {
			leftUpdatedAt = envVars[i].CreatedAt
		}
		rightUpdatedAt := envVars[j].UpdatedAt
		if rightUpdatedAt.IsZero() {
			rightUpdatedAt = envVars[j].CreatedAt
		}
		if !leftUpdatedAt.Equal(rightUpdatedAt) {
			return leftUpdatedAt.Before(rightUpdatedAt)
		}
		leftCreatedAt := envVars[i].CreatedAt
		rightCreatedAt := envVars[j].CreatedAt
		if !leftCreatedAt.Equal(rightCreatedAt) {
			return leftCreatedAt.Before(rightCreatedAt)
		}
		return envVars[i].EnvVar < envVars[j].EnvVar
	})
	result := make(map[string]string, len(envVars))
	for _, envVar := range envVars {
		visible := false
		switch visibilityContext {
		case CredentialVisibilityContextConsole:
			visible = envVar.ConsoleVisible
		case CredentialVisibilityContextServices:
			visible = envVar.ServiceVisible
		case CredentialVisibilityContextHooks:
			visible = envVar.HookVisible
		default:
			visible = envVar.AgentVisible
		}
		if !visible {
			continue
		}
		result[envVar.EnvVar] = envVar.Value
	}
	return result, nil
}

func (s *CredentialService) mapCredentialsToEnvVars(ctx context.Context, projectID string, creds []*model.Credential) ([]CredentialEnvVar, error) {
	return s.mapCredentialsToEnvVarsWithAssignments(ctx, projectID, creds, nil)
}

func (s *CredentialService) mapCredentialsToEnvVarsWithAssignments(ctx context.Context, projectID string, creds []*model.Credential, assignmentsByCredentialID map[string][]*model.SessionCredentialAssignment) ([]CredentialEnvVar, error) {
	result := make([]CredentialEnvVar, 0, len(creds))
	var mcpTokens []MCPTokenData

	for _, c := range creds {
		if !c.IsConfigured {
			continue
		}
		if c.Inactive {
			continue
		}

		if strings.HasPrefix(c.Provider, mcpProviderPrefix) {
			var token MCPTokenData
			if err := s.encryptor.DecryptJSON(c.EncryptedData, &token); err != nil {
				log.Printf("Warning: Failed to decrypt MCP token for provider %s: %v", c.Provider, err)
				continue
			}
			mcpTokens = append(mcpTokens, token)
			continue
		}

		globalVisibility := credentialVisibilityForModel(c)
		credentialAssignments := assignmentsByCredentialID[c.ID]
		if len(credentialAssignments) == 0 {
			credentialAssignments = []*model.SessionCredentialAssignment{nil}
		}

		for _, assignment := range credentialAssignments {
			visibility := globalVisibility
			sessionCredentialID := ""
			envVar := ""
			sourceEnvVar := ""
			var uses []SessionCredentialUse
			createdAt := c.CreatedAt
			updatedAt := c.UpdatedAt
			if assignment != nil {
				sessionVisibility := credentialVisibilityForAssignment(assignment)
				visibility = effectiveCredentialVisibility(globalVisibility, sessionVisibility)
				sessionCredentialID = assignment.SessionCredentialID
				envVar = assignment.EnvVar
				sourceEnvVar = assignment.SourceEnvVar
				createdAt = assignment.CreatedAt
				updatedAt = assignment.UpdatedAt
				parsedUses := parseSessionCredentialUses(assignment.UsesJSON)
				uses = activeSessionCredentialUses(parsedUses, time.Now().UTC())
				if len(parsedUses) > 0 && len(uses) == 0 {
					continue
				}
			}

			switch c.AuthType {
			case AuthTypeAPIKey, AuthTypeID:
				data, err := s.getSecretData(c)
				if err != nil {
					continue
				}
				if envVar != "" {
					selected := sourceEnvVar
					if selected == "" {
						selected = envVar
					}
					for _, secretEnvVar := range data.EnvVars {
						if secretEnvVar.Key != selected {
							continue
						}
						result = append(result, CredentialEnvVar{
							CredentialID:        c.ID,
							SessionCredentialID: sessionCredentialID,
							Uses:                uses,
							EnvVar:              envVar,
							Value:               secretEnvVar.Value,
							Provider:            c.Provider,
							AuthType:            c.AuthType,
							AgentVisible:        visibility.Tools,
							ConsoleVisible:      visibility.Console,
							ServiceVisible:      visibility.Services,
							HookVisible:         visibility.Hooks,
							CreatedAt:           createdAt,
							UpdatedAt:           updatedAt,
						})
						break
					}
					continue
				}
				for _, secretEnvVar := range data.EnvVars {
					result = append(result, CredentialEnvVar{
						CredentialID:        c.ID,
						SessionCredentialID: sessionCredentialID,
						Uses:                uses,
						EnvVar:              secretEnvVar.Key,
						Value:               secretEnvVar.Value,
						Provider:            c.Provider,
						AuthType:            c.AuthType,
						AgentVisible:        visibility.Tools,
						ConsoleVisible:      visibility.Console,
						ServiceVisible:      visibility.Services,
						HookVisible:         visibility.Hooks,
						CreatedAt:           createdAt,
						UpdatedAt:           updatedAt,
					})
				}
			case AuthTypeOAuth:
				tokens, err := s.GetOAuthTokens(ctx, projectID, c.Provider)
				if err != nil {
					log.Printf("Warning: Failed to get OAuth tokens for provider %s: %v", c.Provider, err)
					continue
				}
				if tokens.AccessToken != "" {
					boundEnvVar := envVar
					if boundEnvVar == "" {
						boundEnvVar = firstProviderEnvVar(c.Provider)
						if oauthEnvVar, exists := oauthEnvVars[c.Provider]; exists {
							boundEnvVar = oauthEnvVar
						}
					}
					if boundEnvVar == "" {
						continue
					}
					result = append(result, CredentialEnvVar{
						CredentialID:        c.ID,
						SessionCredentialID: sessionCredentialID,
						Uses:                uses,
						EnvVar:              boundEnvVar,
						Value:               tokens.AccessToken,
						Provider:            c.Provider,
						AuthType:            AuthTypeOAuth,
						AgentVisible:        visibility.Tools,
						ConsoleVisible:      visibility.Console,
						ServiceVisible:      visibility.Services,
						HookVisible:         visibility.Hooks,
						ExpiresAt:           tokens.ExpiresAt.Unix(),
						CreatedAt:           createdAt,
						UpdatedAt:           updatedAt,
					})
				}
			}
		}
	}

	if len(mcpTokens) > 0 {
		tokensJSON, err := json.Marshal(mcpTokens)
		if err != nil {
			log.Printf("Warning: Failed to marshal MCP tokens: %v", err)
		} else {
			result = append(result, CredentialEnvVar{
				EnvVar:         "MCP_OAUTH_TOKENS",
				Value:          string(tokensJSON),
				Provider:       "mcp-oauth-tokens",
				AuthType:       AuthTypeOAuth,
				AgentVisible:   true,
				ConsoleVisible: false,
				ServiceVisible: false,
				HookVisible:    false,
			})
		}
	}

	return result, nil
}

// MCPTokenData represents an OAuth token for an MCP server.
// This is the data stored per-resource-URL and also the element type in
// the MCP_OAUTH_TOKENS JSON array delivered to the agent container.
type MCPTokenData struct {
	URL          string `json:"url"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"` // unix timestamp
}

// StoreMCPToken upserts an OAuth token for an MCP server, keyed by resource URL.
// The credential provider key is "mcp:{resourceUrl}".
func (s *CredentialService) StoreMCPToken(ctx context.Context, projectID string, data MCPTokenData) error {
	if data.URL == "" {
		return fmt.Errorf("url is required")
	}

	providerKey := mcpProviderPrefix + data.URL

	encrypted, err := s.encryptor.EncryptJSON(data)
	if err != nil {
		return ErrEncryptionFailed
	}

	existing, err := s.store.GetCredentialByProvider(ctx, projectID, providerKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}

	if existing != nil {
		existing.AuthType = AuthTypeOAuth
		existing.EncryptedData = encrypted
		existing.IsConfigured = true
		existing.AgentVisible = true
		return s.store.UpdateCredential(ctx, existing)
	}

	cred := &model.Credential{
		ProjectID:     projectID,
		Provider:      providerKey,
		Name:          "MCP OAuth: " + data.URL,
		AuthType:      AuthTypeOAuth,
		EncryptedData: encrypted,
		IsConfigured:  true,
		AgentVisible:  true,
	}
	return s.store.CreateCredential(ctx, cred)
}

func (s *CredentialService) getSecretData(cred *model.Credential) (*SecretCredentialData, error) {
	var data SecretCredentialData
	if err := s.encryptor.DecryptJSON(cred.EncryptedData, &data); err == nil {
		data.EnvVars = normalizeSecretEnvVars(data.EnvVars)
		return &data, nil
	}

	// Backward compatibility: older secret credentials stored a single api_key field.
	var legacy APIKeyCredential
	if err := s.encryptor.DecryptJSON(cred.EncryptedData, &legacy); err != nil {
		return nil, ErrDecryptionFailed
	}

	key := firstProviderEnvVar(cred.Provider)
	if key == "" {
		key = "VALUE"
	}
	data.EnvVars = []SecretEnvVar{{Key: key, Value: legacy.APIKey}}
	return &data, nil
}

func (s *CredentialService) validateStoredCredential(ctx context.Context, cred *model.Credential) CredentialValidationInfo {
	result := CredentialValidationInfo{
		CredentialID: cred.ID,
		Provider:     cred.Provider,
		AuthType:     cred.AuthType,
	}

	if cred.AuthType != AuthTypeAPIKey {
		result.Status = CredentialValidationStatusUnsupported
		result.Message = "Validation is only supported for API key credentials"
		return result
	}
	if s.keyValidators == nil || !s.keyValidators.HasValidator(cred.Provider) {
		result.Status = CredentialValidationStatusUnsupported
		result.Message = "Validation is not supported for this provider"
		return result
	}

	result.CheckedAt = time.Now().UTC()
	data, err := s.getSecretData(cred)
	if err != nil {
		result.Status = CredentialValidationStatusError
		result.Message = "Failed to read stored credential"
		return result
	}

	if err := s.keyValidators.ValidateAPIKey(ctx, cred.Provider, firstSecretEnvVarValue(data.EnvVars)); err != nil {
		if errors.Is(err, keyvalidator.ErrValidationFailed) {
			result.Status = CredentialValidationStatusInvalid
			result.Message = err.Error()
			return result
		}
		result.Status = CredentialValidationStatusError
		result.Message = err.Error()
		return result
	}

	result.Status = CredentialValidationStatusValid
	return result
}

func normalizeSecretEnvVars(envVars []SecretEnvVar) []SecretEnvVar {
	result := make([]SecretEnvVar, 0, len(envVars))
	seen := map[string]struct{}{}
	for _, envVar := range envVars {
		key := strings.TrimSpace(envVar.Key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, SecretEnvVar{Key: key, Value: envVar.Value, OriginalKey: strings.TrimSpace(envVar.OriginalKey)})
	}
	return result
}

func firstSecretEnvVarValue(envVars []SecretEnvVar) string {
	for _, envVar := range envVars {
		if strings.TrimSpace(envVar.Key) == "" {
			continue
		}
		return envVar.Value
	}
	return ""
}

func (s *CredentialService) shouldValidateAPIKeyOnSave(existingData *SecretCredentialData, envVars []SecretEnvVar) bool {
	if s == nil || s.cfg == nil || !s.cfg.ValidateAPIKeys {
		return false
	}
	if existingData == nil {
		return true
	}
	return firstSecretEnvVarValue(existingData.EnvVars) != firstSecretEnvVarValue(envVars)
}

func mergeSecretEnvVars(existingEnvVars, updatedEnvVars []SecretEnvVar) []SecretEnvVar {
	existingValues := make(map[string]string, len(existingEnvVars))
	for _, envVar := range normalizeSecretEnvVars(existingEnvVars) {
		existingValues[envVar.Key] = envVar.Value
	}

	result := make([]SecretEnvVar, 0, len(updatedEnvVars))
	for _, envVar := range normalizeSecretEnvVars(updatedEnvVars) {
		if strings.TrimSpace(envVar.Value) != "" {
			result = append(result, SecretEnvVar{Key: envVar.Key, Value: envVar.Value})
			continue
		}
		// When the key was renamed, look up the existing value under the original key
		// so the secret is preserved under the new name.
		lookupKey := envVar.Key
		if strings.TrimSpace(envVar.OriginalKey) != "" {
			lookupKey = strings.TrimSpace(envVar.OriginalKey)
		}
		if existingValue, ok := existingValues[lookupKey]; ok {
			result = append(result, SecretEnvVar{Key: envVar.Key, Value: existingValue})
		}
	}
	return result
}

func secretEnvKeys(envVars []SecretEnvVar) []string {
	keys := make([]string, 0, len(envVars))
	for _, envVar := range envVars {
		if strings.TrimSpace(envVar.Key) == "" {
			continue
		}
		keys = append(keys, strings.TrimSpace(envVar.Key))
	}
	return keys
}

func importedCredentialName(provider, authType string) string {
	if authType == AuthTypeID {
		if p := providers.Get(provider); p != nil && strings.TrimSpace(p.ConfiguredName) != "" {
			return strings.TrimSpace(p.ConfiguredName)
		}
	}
	if p := providers.Get(provider); p != nil && strings.TrimSpace(p.Name) != "" {
		return strings.TrimSpace(p.Name)
	}
	return provider
}

func (s *CredentialService) credentialEnvVars(c *model.Credential) []string {
	switch c.AuthType {
	case AuthTypeAPIKey, AuthTypeID:
		data, err := s.getSecretData(c)
		if err != nil {
			return nil
		}
		return secretEnvKeys(data.EnvVars)
	case AuthTypeOAuth:
		envVar := firstProviderEnvVar(c.Provider)
		if oauthEnvVar, exists := oauthEnvVars[c.Provider]; exists {
			envVar = oauthEnvVar
		}
		if strings.TrimSpace(envVar) == "" {
			return nil
		}
		return []string{envVar}
	default:
		return nil
	}
}

func firstProviderEnvVar(provider string) string {
	envVars := providers.GetEnvVars(provider)
	if len(envVars) == 0 {
		return ""
	}
	return envVars[0]
}

func defaultCredentialAgentVisible(_ string) bool {
	return false
}

func defaultCredentialDescription(provider string) string {
	switch provider {
	case ProviderOpenAI:
		return "Used internally for OpenAI model access."
	case ProviderTavily:
		return "Used internally for Tavily-backed tools."
	default:
		return ""
	}
}

// isValidProvider checks if a provider is supported
func isValidProvider(provider string) bool {
	if strings.HasPrefix(provider, customProviderPrefix) || strings.HasPrefix(provider, mcpProviderPrefix) {
		return true
	}
	switch provider {
	case ProviderAnthropic, ProviderGitHubCopilot, ProviderGitHub, ProviderCodex, ProviderOpenAI, ProviderTavily, ProviderDiscobot:
		return true
	default:
		return false
	}
}

// toCredentialInfo converts a model.Credential to CredentialInfo (safe for API)
// toCredentialInfo converts a model.Credential to CredentialInfo (safe for API)
// For OAuth credentials, it decrypts the data to extract the expiration time
func (s *CredentialService) toCredentialInfo(c *model.Credential) CredentialInfo {
	info := CredentialInfo{
		ID:           c.ID,
		Provider:     c.Provider,
		Name:         strings.TrimSpace(c.Name),
		AuthType:     c.AuthType,
		IsConfigured: c.IsConfigured,
		Inactive:     c.Inactive,
		AgentVisible: c.AgentVisible,
		Visibility:   credentialVisibilityForModel(c),
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
	if c.Description != nil {
		info.Description = *c.Description
	}

	if c.AuthType == AuthTypeOAuth && c.IsConfigured {
		info.EnvKeys = s.credentialEnvVars(c)
		var tokens OAuthCredential
		if err := s.encryptor.DecryptJSON(c.EncryptedData, &tokens); err == nil {
			info.Scopes = normalizeOAuthScopes(tokens.Scope)
			if !tokens.ExpiresAt.IsZero() {
				info.ExpiresAt = &tokens.ExpiresAt
			}
		}
		return info
	}

	if data, err := s.getSecretData(c); err == nil {
		info.EnvKeys = secretEnvKeys(data.EnvVars)
	}

	return info
}
