package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/keyvalidator"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/providers"
)

func clearStartupCredentialEnv(t *testing.T) {
	t.Helper()
	for _, spec := range startupCredentialImportSpecs {
		t.Setenv(spec.envVar, "")
	}
}

type recordingKeyValidator struct {
	called bool
	count  int
	apiKey string
	err    error
}

func (v *recordingKeyValidator) Validate(_ context.Context, apiKey string) error {
	v.called = true
	v.count++
	v.apiKey = apiKey
	return v.err
}

func TestSetAPIKeyWithMetadata_ValidatesProviderKey(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789"), ValidateAPIKeys: true}
	validator := &recordingKeyValidator{}

	credSvc, err := NewCredentialServiceWithValidators(
		st,
		cfg,
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{ProviderAnthropic: validator}),
	)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	_, err = credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		"test-project",
		ProviderAnthropic,
		"Anthropic",
		"",
		"sk-ant-test-123",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("SetAPIKeyWithMetadata failed: %v", err)
	}
	if !validator.called {
		t.Fatal("expected validator to be called")
	}
	if validator.count != 1 {
		t.Fatalf("expected validator to be called once, got %d", validator.count)
	}
	if validator.apiKey != "sk-ant-test-123" {
		t.Fatalf("expected validator to receive API key, got %q", validator.apiKey)
	}
}

func TestSetAPIKeyWithMetadata_SkipsValidationWhenKeyUnchanged(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789"), ValidateAPIKeys: true}
	validator := &recordingKeyValidator{}

	credSvc, err := NewCredentialServiceWithValidators(
		st,
		cfg,
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{ProviderAnthropic: validator}),
	)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	info, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		"test-project",
		ProviderAnthropic,
		"Original Name",
		"",
		"sk-ant-test-123",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("initial SetAPIKeyWithMetadata failed: %v", err)
	}

	_, err = credSvc.SetAPIKeyCredentialWithMetadata(
		context.Background(),
		"test-project",
		info.ID,
		ProviderAnthropic,
		"Updated Name",
		"",
		"sk-ant-test-123",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("second SetAPIKeyCredentialWithMetadata failed: %v", err)
	}
	if validator.count != 1 {
		t.Fatalf("expected unchanged key update to skip revalidation, got %d calls", validator.count)
	}
}

func TestSetAPIKeyWithMetadata_RevalidatesWhenKeyChanges(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789"), ValidateAPIKeys: true}
	validator := &recordingKeyValidator{}

	credSvc, err := NewCredentialServiceWithValidators(
		st,
		cfg,
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{ProviderAnthropic: validator}),
	)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	info, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		"test-project",
		ProviderAnthropic,
		"Original Name",
		"",
		"sk-ant-test-123",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("initial SetAPIKeyWithMetadata failed: %v", err)
	}

	_, err = credSvc.SetAPIKeyCredentialWithMetadata(
		context.Background(),
		"test-project",
		info.ID,
		ProviderAnthropic,
		"Updated Name",
		"",
		"sk-ant-test-456",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("second SetAPIKeyCredentialWithMetadata failed: %v", err)
	}
	if validator.count != 2 {
		t.Fatalf("expected changed key update to revalidate, got %d calls", validator.count)
	}
	if validator.apiKey != "sk-ant-test-456" {
		t.Fatalf("expected validator to see updated key, got %q", validator.apiKey)
	}
}

func TestSetAPIKeyWithMetadata_StopsOnValidationError(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789"), ValidateAPIKeys: true}
	validator := &recordingKeyValidator{err: &keyvalidator.ValidationError{
		Provider: "Anthropic",
		Message:  "Anthropic rejected the API key: invalid x-api-key",
	}}

	credSvc, err := NewCredentialServiceWithValidators(
		st,
		cfg,
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{ProviderAnthropic: validator}),
	)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	_, err = credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		"test-project",
		ProviderAnthropic,
		"Anthropic",
		"",
		"sk-ant-test-123",
		CredentialVisibility{},
		false,
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, keyvalidator.ErrValidationFailed) {
		t.Fatalf("expected validation failure, got %v", err)
	}

	_, getErr := credSvc.Get(context.Background(), "test-project", ProviderAnthropic)
	if !errors.Is(getErr, ErrCredentialNotFound) {
		t.Fatalf("expected credential not to be saved, got %v", getErr)
	}
}

func TestSetIDWithMetadata_DoesNotValidateAPIKey(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789"), ValidateAPIKeys: true}
	validator := &recordingKeyValidator{}

	credSvc, err := NewCredentialServiceWithValidators(
		st,
		cfg,
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{ProviderDiscobot: validator}),
	)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	_, err = credSvc.SetIDWithMetadata(
		context.Background(),
		"test-project",
		ProviderDiscobot,
		"Discobot ID",
		"",
		"discobot_123",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("SetIDWithMetadata failed: %v", err)
	}
	if validator.called {
		t.Fatal("expected ID credential save to skip key validation")
	}
}

func TestSetOAuthTokensWithMetadata_DoesNotValidateAPIKey(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789"), ValidateAPIKeys: true}
	validator := &recordingKeyValidator{}

	credSvc, err := NewCredentialServiceWithValidators(
		st,
		cfg,
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{ProviderAnthropic: validator}),
	)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	_, err = credSvc.SetOAuthTokensWithMetadata(
		context.Background(),
		"test-project",
		"",
		ProviderAnthropic,
		"Anthropic OAuth",
		"",
		CredentialVisibility{},
		false,
		&OAuthCredential{AccessToken: "oauth-token", TokenType: "Bearer"},
	)
	if err != nil {
		t.Fatalf("SetOAuthTokensWithMetadata failed: %v", err)
	}
	if validator.called {
		t.Fatal("expected OAuth credential save to skip key validation")
	}
}

func TestValidateAll_ReturnsStatusesForStoredCredentials(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789")}
	validator := &recordingKeyValidator{err: &keyvalidator.ValidationError{
		Provider: "Anthropic",
		Message:  "Anthropic rejected the API key: invalid x-api-key",
	}}

	credSvc, err := NewCredentialServiceWithValidators(
		st,
		cfg,
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{ProviderAnthropic: validator}),
	)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	apiKey, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		"test-project",
		ProviderAnthropic,
		"Anthropic",
		"",
		"sk-ant-test-123",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("SetAPIKeyWithMetadata failed: %v", err)
	}
	idCred, err := credSvc.SetIDWithMetadata(
		context.Background(),
		"test-project",
		ProviderDiscobot,
		"Discobot ID",
		"",
		"discobot_123",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("SetIDWithMetadata failed: %v", err)
	}

	validations, err := credSvc.ValidateAll(context.Background(), "test-project")
	if err != nil {
		t.Fatalf("ValidateAll failed: %v", err)
	}
	if len(validations) != 2 {
		t.Fatalf("expected 2 validation results, got %d", len(validations))
	}

	byID := map[string]CredentialValidationInfo{}
	for _, validation := range validations {
		byID[validation.CredentialID] = validation
	}
	if byID[apiKey.ID].Status != CredentialValidationStatusInvalid {
		t.Fatalf("expected anthropic key to be invalid, got %s", byID[apiKey.ID].Status)
	}
	if byID[idCred.ID].Status != CredentialValidationStatusUnsupported {
		t.Fatalf("expected discobot id validation to be unsupported, got %s", byID[idCred.ID].Status)
	}
}

func TestValidateByID_InvalidAPIKeyReturnsInvalidStatus(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: []byte("test-key-32-bytes-long-123456789")}
	validator := &recordingKeyValidator{err: &keyvalidator.ValidationError{
		Provider: "Anthropic",
		Message:  "Anthropic rejected the API key: invalid x-api-key",
	}}

	credSvc, err := NewCredentialServiceWithValidators(
		st,
		cfg,
		keyvalidator.NewRegistry(map[string]keyvalidator.Validator{ProviderAnthropic: validator}),
	)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	info, err := credSvc.SetAPIKeyWithMetadata(
		context.Background(),
		"test-project",
		ProviderAnthropic,
		"Anthropic",
		"",
		"sk-ant-test-123",
		CredentialVisibility{},
		false,
	)
	if err != nil {
		t.Fatalf("SetAPIKeyWithMetadata failed: %v", err)
	}

	validation, err := credSvc.ValidateByID(context.Background(), "test-project", info.ID)
	if err != nil {
		t.Fatalf("ValidateByID failed: %v", err)
	}
	if validation.Status != CredentialValidationStatusInvalid {
		t.Fatalf("expected invalid status, got %s", validation.Status)
	}
	if validation.Message != "Anthropic rejected the API key: invalid x-api-key" {
		t.Fatalf("unexpected validation message %q", validation.Message)
	}
	if validation.CheckedAt.IsZero() {
		t.Fatal("expected CheckedAt to be set")
	}
}

func TestGetAllDecrypted_DiscobotID_UsesCorrectEnvVar(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	envVars := providers.GetEnvVars(ProviderDiscobot)
	if len(envVars) < 1 {
		t.Fatalf("Expected Discobot provider to have at least 1 env var, got %d", len(envVars))
	}
	if envVars[0] != "DISCOBOT_TOKEN" {
		t.Errorf("Expected first env var to be DISCOBOT_TOKEN, got %s", envVars[0])
	}

	_, err = credSvc.SetID(ctx, projectID, ProviderDiscobot, "Discobot Token", "discobot-token-123")
	if err != nil {
		t.Fatalf("Failed to set API key: %v", err)
	}

	envVarMappings, err := credSvc.GetAllDecrypted(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to get all decrypted: %v", err)
	}

	if len(envVarMappings) != 1 {
		t.Fatalf("Expected 1 env var mapping, got %d", len(envVarMappings))
	}
	if envVarMappings[0].AuthType != AuthTypeID {
		t.Errorf("Expected auth type %s, got %s", AuthTypeID, envVarMappings[0].AuthType)
	}
	if envVarMappings[0].EnvVar != "DISCOBOT_TOKEN" {
		t.Errorf("Expected env var DISCOBOT_TOKEN, got %s", envVarMappings[0].EnvVar)
	}
	if envVarMappings[0].Value != "discobot-token-123" {
		t.Errorf("Expected value 'discobot-token-123', got %s", envVarMappings[0].Value)
	}
}

func TestGetAllDecrypted_SkipsInactiveCredentials(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	info, err := credSvc.SetAPIKeyWithMetadata(
		ctx,
		projectID,
		ProviderAnthropic,
		"Anthropic",
		"",
		"sk-ant-test-123",
		CredentialVisibility{Tools: true},
		true,
	)
	if err != nil {
		t.Fatalf("Failed to set API key: %v", err)
	}
	if !info.Inactive {
		t.Fatal("expected credential to be inactive")
	}

	envVarMappings, err := credSvc.GetAllDecrypted(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to get all decrypted: %v", err)
	}
	if len(envVarMappings) != 0 {
		t.Fatalf("expected inactive credential to be skipped, got %d env vars", len(envVarMappings))
	}
}

func TestGetAllDecrypted_AnthropicOAuth_UsesCorrectEnvVar(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key (must be 32 bytes for AES-256)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Verify Anthropic provider has API key env var
	envVars := providers.GetEnvVars(ProviderAnthropic)
	if len(envVars) < 1 {
		t.Fatalf("Expected Anthropic provider to have at least 1 env var, got %d", len(envVars))
	}
	if envVars[0] != "ANTHROPIC_API_KEY" {
		t.Errorf("Expected first env var to be ANTHROPIC_API_KEY, got %s", envVars[0])
	}

	// Create an OAuth credential for Anthropic
	oauthTokens := &OAuthCredential{
		AccessToken: "oauth-token-test-123",
		TokenType:   "Bearer",
	}
	oauthInfo, err := credSvc.SetOAuthTokens(ctx, projectID, ProviderAnthropic, "OAuth Token", oauthTokens)
	if err != nil {
		t.Fatalf("Failed to set OAuth tokens: %v", err)
	}
	if oauthInfo.AuthType != AuthTypeOAuth {
		t.Errorf("Expected auth type %s, got %s", AuthTypeOAuth, oauthInfo.AuthType)
	}

	// Get all decrypted credentials
	envVarMappings, err := credSvc.GetAllDecrypted(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to get all decrypted: %v", err)
	}

	// Should have 1 mapping (OAuth)
	if len(envVarMappings) != 1 {
		t.Fatalf("Expected 1 env var mapping, got %d", len(envVarMappings))
	}

	// Verify it uses CLAUDE_CODE_OAUTH_TOKEN (second env var for Anthropic OAuth)
	if envVarMappings[0].EnvVar != "CLAUDE_CODE_OAUTH_TOKEN" {
		t.Errorf("Expected env var CLAUDE_CODE_OAUTH_TOKEN, got %s", envVarMappings[0].EnvVar)
	}
	if envVarMappings[0].Value != "oauth-token-test-123" {
		t.Errorf("Expected value 'oauth-token-test-123', got %s", envVarMappings[0].Value)
	}
}

func TestGetAllDecrypted_AnthropicAPIKey_UsesCorrectEnvVar(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key (must be 32 bytes for AES-256)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Verify Anthropic provider has API key env var
	envVars := providers.GetEnvVars(ProviderAnthropic)
	if len(envVars) < 1 {
		t.Fatalf("Expected Anthropic provider to have at least 1 env var, got %d", len(envVars))
	}
	if envVars[0] != "ANTHROPIC_API_KEY" {
		t.Errorf("Expected first env var to be ANTHROPIC_API_KEY, got %s", envVars[0])
	}

	// Create an API key credential for Anthropic
	apiKeyInfo, err := credSvc.SetAPIKey(ctx, projectID, ProviderAnthropic, "API Key", "sk-ant-test-123")
	if err != nil {
		t.Fatalf("Failed to set API key: %v", err)
	}
	if apiKeyInfo.AuthType != AuthTypeAPIKey {
		t.Errorf("Expected auth type %s, got %s", AuthTypeAPIKey, apiKeyInfo.AuthType)
	}

	// Get all decrypted credentials
	envVarMappings, err := credSvc.GetAllDecrypted(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to get all decrypted: %v", err)
	}

	// Should have 1 mapping (API key)
	if len(envVarMappings) != 1 {
		t.Fatalf("Expected 1 env var mapping, got %d", len(envVarMappings))
	}

	// Verify it uses ANTHROPIC_API_KEY (first env var for Anthropic API key)
	if envVarMappings[0].EnvVar != "ANTHROPIC_API_KEY" {
		t.Errorf("Expected env var ANTHROPIC_API_KEY, got %s", envVarMappings[0].EnvVar)
	}
	if envVarMappings[0].Value != "sk-ant-test-123" {
		t.Errorf("Expected value 'sk-ant-test-123', got %s", envVarMappings[0].Value)
	}
}

func TestGetAllDecrypted_OtherProviderOAuth_UsesFirstEnvVar(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key (must be 32 bytes for AES-256)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Create an OAuth credential for GitHub Copilot
	oauthTokens := &OAuthCredential{
		AccessToken: "github-copilot-token",
		TokenType:   "Bearer",
	}
	_, err = credSvc.SetOAuthTokens(ctx, projectID, ProviderGitHubCopilot, "GitHub Copilot OAuth", oauthTokens)
	if err != nil {
		t.Fatalf("Failed to set OAuth tokens: %v", err)
	}

	// Get all decrypted credentials
	envVarMappings, err := credSvc.GetAllDecrypted(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to get all decrypted: %v", err)
	}

	// Should have 1 mapping
	if len(envVarMappings) != 1 {
		t.Fatalf("Expected 1 env var mapping, got %d", len(envVarMappings))
	}

	// Verify it uses GITHUB_TOKEN (first env var for GitHub Copilot)
	if envVarMappings[0].EnvVar != "GITHUB_TOKEN" {
		t.Errorf("Expected env var GITHUB_TOKEN, got %s", envVarMappings[0].EnvVar)
	}
	if envVarMappings[0].Value != "github-copilot-token" {
		t.Errorf("Expected value 'github-copilot-token', got %s", envVarMappings[0].Value)
	}
}

func TestGetOAuthTokens_AutoRefresh(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key (must be 32 bytes for AES-256)
	cfg := &config.Config{
		EncryptionKey:     []byte("test-key-32-bytes-long-123456789"),
		AnthropicClientID: "test-client-id",
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Create an expired OAuth credential for Anthropic
	// Set expiration to 1 hour ago to trigger refresh
	expiredTime := time.Now().Add(-1 * time.Hour)
	oauthTokens := &OAuthCredential{
		AccessToken:  "old-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    expiredTime,
	}
	_, err = credSvc.SetOAuthTokens(ctx, projectID, ProviderAnthropic, "Anthropic OAuth", oauthTokens)
	if err != nil {
		t.Fatalf("Failed to set OAuth tokens: %v", err)
	}

	// Note: In a real test, you would need to mock the HTTP client
	// to simulate the refresh token exchange with Anthropic.
	// For now, this test verifies the logic structure.

	// Get tokens - should attempt auto-refresh but fail due to missing HTTP mock
	tokens, err := credSvc.GetOAuthTokens(ctx, projectID, ProviderAnthropic)
	if err != nil {
		t.Fatalf("Failed to get OAuth tokens: %v", err)
	}

	// Since we can't mock the HTTP client here, we expect to get the old token back
	// In a production scenario with a real refresh token, this would return new tokens
	if tokens.AccessToken != "old-access-token" {
		t.Errorf("Expected old access token, got %s", tokens.AccessToken)
	}
}

func TestCredentialInfo_IncludesExpiresAt(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key (must be 32 bytes for AES-256)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Create an OAuth credential with expiration time
	expiresAt := time.Now().Add(24 * time.Hour)
	oauthTokens := &OAuthCredential{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    expiresAt,
	}
	_, err = credSvc.SetOAuthTokens(ctx, projectID, ProviderAnthropic, "Anthropic OAuth", oauthTokens)
	if err != nil {
		t.Fatalf("Failed to set OAuth tokens: %v", err)
	}

	// Get credential info
	info, err := credSvc.Get(ctx, projectID, ProviderAnthropic)
	if err != nil {
		t.Fatalf("Failed to get credential: %v", err)
	}

	// Verify expiresAt is included
	if info.ExpiresAt == nil {
		t.Errorf("Expected expiresAt to be set, but it was nil")
	} else {
		// Allow for small time differences due to processing time
		timeDiff := info.ExpiresAt.Sub(expiresAt).Abs()
		if timeDiff > time.Second {
			t.Errorf("Expected expiresAt to be %v, got %v (diff: %v)", expiresAt, *info.ExpiresAt, timeDiff)
		}
	}

	// Create an API key credential (should not have expiresAt)
	_, err = credSvc.SetAPIKey(ctx, projectID, ProviderOpenAI, "OpenAI API Key", "sk-test-123")
	if err != nil {
		t.Fatalf("Failed to set API key: %v", err)
	}

	info, err = credSvc.Get(ctx, projectID, ProviderOpenAI)
	if err != nil {
		t.Fatalf("Failed to get credential: %v", err)
	}

	// Verify expiresAt is NOT included for API key credentials
	if info.ExpiresAt != nil {
		t.Errorf("Expected expiresAt to be nil for API key credential, but it was %v", *info.ExpiresAt)
	}
}

func TestSetSessionAssignments_AssignsUseExpiration(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"
	sessionID := "test-session"
	createTestSession(t, st, sessionID, t.TempDir())

	info, err := credSvc.SetAPIKey(ctx, projectID, ProviderOpenAI, "OpenAI", "sk-test-123")
	if err != nil {
		t.Fatalf("Failed to set API key: %v", err)
	}

	assignments, err := credSvc.SetSessionAssignments(ctx, projectID, sessionID, []SessionCredentialAssignmentInfo{
		{
			CredentialID: info.ID,
			Visibility:   CredentialVisibility{Tools: true},
			Uses: []SessionCredentialUse{
				{Description: "run tests"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to set session assignments: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("Expected 1 assignment, got %d", len(assignments))
	}
	if len(assignments[0].Uses) != 1 {
		t.Fatalf("Expected 1 use, got %d", len(assignments[0].Uses))
	}

	use := assignments[0].Uses[0]
	if use.ExpiresAt.IsZero() {
		t.Fatal("Expected use expiration to be set")
	}
	if use.CreatedAt.IsZero() {
		t.Fatal("Expected use creation time to be set")
	}
	duration := use.ExpiresAt.Sub(use.CreatedAt)
	if duration < sessionCredentialUseDuration-time.Second || duration > sessionCredentialUseDuration+time.Second {
		t.Fatalf("Expected use duration around %v, got %v", sessionCredentialUseDuration, duration)
	}
}

func TestGetAllForSession_SkipsAssignmentsWithOnlyExpiredUses(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"
	sessionID := "test-session"
	createTestSession(t, st, sessionID, t.TempDir())

	info, err := credSvc.SetAPIKey(ctx, projectID, ProviderOpenAI, "OpenAI", "sk-test-123")
	if err != nil {
		t.Fatalf("Failed to set API key: %v", err)
	}

	expiredUse := SessionCredentialUse{
		ID:          "use_s_expired",
		Description: "create pull requests",
		CreatedAt:   time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:   time.Now().UTC().Add(-1 * time.Hour),
	}
	assignments, err := credSvc.SetSessionAssignments(ctx, projectID, sessionID, []SessionCredentialAssignmentInfo{
		{
			CredentialID: info.ID,
			Visibility:   CredentialVisibility{Tools: true},
			Uses:         []SessionCredentialUse{expiredUse},
		},
	})
	if err != nil {
		t.Fatalf("Failed to set session assignments: %v", err)
	}
	if len(assignments) != 1 || len(assignments[0].Uses) != 1 {
		t.Fatalf("Expected assignment with expired use to remain visible in session list")
	}

	envVars, err := credSvc.GetAllForSession(ctx, projectID, sessionID)
	if err != nil {
		t.Fatalf("Failed to get session credentials: %v", err)
	}
	if len(envVars) != 0 {
		t.Fatalf("Expected expired-use assignment to be omitted from session env vars, got %d entries", len(envVars))
	}
}

func TestSetOAuthTokensWithMetadata_AllowsMultipleGitHubCredentials(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	first, err := credSvc.SetOAuthTokensWithMetadata(
		ctx,
		projectID,
		"",
		ProviderGitHub,
		"GitHub Work",
		"",
		CredentialVisibility{Tools: true},
		false,
		&OAuthCredential{AccessToken: "token-1", TokenType: "Bearer", Scope: "repo read:user"},
	)
	if err != nil {
		t.Fatalf("Failed to create first GitHub credential: %v", err)
	}

	second, err := credSvc.SetOAuthTokensWithMetadata(
		ctx,
		projectID,
		"",
		ProviderGitHub,
		"GitHub Personal",
		"",
		CredentialVisibility{Tools: false},
		false,
		&OAuthCredential{AccessToken: "token-2", TokenType: "Bearer", Scope: "public_repo user:email"},
	)
	if err != nil {
		t.Fatalf("Failed to create second GitHub credential: %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("expected distinct GitHub credential IDs")
	}

	credentials, err := credSvc.List(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to list credentials: %v", err)
	}
	if len(credentials) != 2 {
		t.Fatalf("expected 2 GitHub credentials, got %d", len(credentials))
	}
}

func TestCredentialInfo_IncludesOAuthScopes(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	created, err := credSvc.SetOAuthTokensWithMetadata(
		ctx,
		projectID,
		"",
		ProviderGitHub,
		"GitHub",
		"",
		CredentialVisibility{},
		false,
		&OAuthCredential{AccessToken: "token", TokenType: "Bearer", Scope: "repo read:user repo"},
	)
	if err != nil {
		t.Fatalf("Failed to create GitHub credential: %v", err)
	}

	info, err := credSvc.GetByID(ctx, projectID, created.ID)
	if err != nil {
		t.Fatalf("Failed to fetch GitHub credential: %v", err)
	}

	if len(info.Scopes) != 2 {
		t.Fatalf("expected 2 unique scopes, got %v", info.Scopes)
	}
	if info.Scopes[0] != "repo" || info.Scopes[1] != "read:user" {
		t.Fatalf("unexpected scopes: %v", info.Scopes)
	}
}

func TestCredentialInfo_OAuthIncludesEnvKeys(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	created, err := credSvc.SetOAuthTokensWithMetadata(
		ctx,
		projectID,
		"",
		ProviderGitHub,
		"GitHub",
		"",
		CredentialVisibility{},
		false,
		&OAuthCredential{AccessToken: "token", TokenType: "Bearer", Scope: "repo"},
	)
	if err != nil {
		t.Fatalf("Failed to create GitHub credential: %v", err)
	}

	info, err := credSvc.GetByID(ctx, projectID, created.ID)
	if err != nil {
		t.Fatalf("Failed to fetch GitHub credential: %v", err)
	}

	if len(info.EnvKeys) != 1 || info.EnvKeys[0] != "GITHUB_TOKEN" {
		t.Fatalf("expected OAuth env key GITHUB_TOKEN, got %v", info.EnvKeys)
	}
}

func TestDirectToken_StoredWithOneYearExpiration(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Simulate storing a direct token (like from 'claude setup-token')
	directToken := "sk-ant-oat0-test-token-12345"
	expiresAt := time.Now().Add(365 * 24 * time.Hour) // 1 year

	oauthTokens := &OAuthCredential{
		AccessToken: directToken,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		// No refresh token for direct tokens
	}

	info, err := credSvc.SetOAuthTokens(ctx, projectID, ProviderAnthropic, "Anthropic Direct Token", oauthTokens)
	if err != nil {
		t.Fatalf("Failed to set OAuth tokens: %v", err)
	}

	// Verify credential info includes expiration
	if info.ExpiresAt == nil {
		t.Errorf("Expected expiresAt to be set for direct token")
	}

	// Retrieve the tokens and verify
	tokens, err := credSvc.GetOAuthTokens(ctx, projectID, ProviderAnthropic)
	if err != nil {
		t.Fatalf("Failed to get OAuth tokens: %v", err)
	}

	// Verify the access token is the direct token
	if tokens.AccessToken != directToken {
		t.Errorf("Expected access token to be %s, got %s", directToken, tokens.AccessToken)
	}

	// Verify no refresh token
	if tokens.RefreshToken != "" {
		t.Errorf("Expected no refresh token for direct token, got %s", tokens.RefreshToken)
	}

	// Verify expiration is approximately 1 year (allow 1 minute variance)
	expectedExpiry := time.Now().Add(365 * 24 * time.Hour)
	timeDiff := tokens.ExpiresAt.Sub(expectedExpiry).Abs()
	if timeDiff > time.Minute {
		t.Errorf("Expected expiration ~1 year from now, got %v (diff: %v)", tokens.ExpiresAt, timeDiff)
	}
}

func TestRefreshBackoff_PreventsRepeatedAttempts(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key
	cfg := &config.Config{
		EncryptionKey:     []byte("test-key-32-bytes-long-123456789"),
		AnthropicClientID: "test-client-id",
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Create an expired OAuth credential
	expiredTime := time.Now().Add(-1 * time.Hour)
	oauthTokens := &OAuthCredential{
		AccessToken:  "expired-token",
		RefreshToken: "invalid-refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    expiredTime,
	}
	_, err = credSvc.SetOAuthTokens(ctx, projectID, ProviderAnthropic, "Anthropic OAuth", oauthTokens)
	if err != nil {
		t.Fatalf("Failed to set OAuth tokens: %v", err)
	}

	// First call: should attempt refresh and fail (will try to call Anthropic API)
	tokens1, err := credSvc.GetOAuthTokens(ctx, projectID, ProviderAnthropic)
	if err != nil {
		t.Fatalf("Failed to get OAuth tokens: %v", err)
	}

	// Should return the expired token since refresh failed
	if tokens1.AccessToken != "expired-token" {
		t.Errorf("Expected expired token, got %s", tokens1.AccessToken)
	}

	// Verify backoff was recorded
	credSvc.refreshFailMutex.RLock()
	lastFail, hasFailed := credSvc.lastRefreshFail[ProviderAnthropic]
	credSvc.refreshFailMutex.RUnlock()

	if !hasFailed {
		t.Error("Expected refresh failure to be recorded")
	}

	// Second call immediately after: should skip refresh due to backoff
	tokens2, err := credSvc.GetOAuthTokens(ctx, projectID, ProviderAnthropic)
	if err != nil {
		t.Fatalf("Failed to get OAuth tokens: %v", err)
	}

	// Should still return the expired token
	if tokens2.AccessToken != "expired-token" {
		t.Errorf("Expected expired token, got %s", tokens2.AccessToken)
	}

	// Verify the last fail time hasn't changed (no new attempt)
	credSvc.refreshFailMutex.RLock()
	lastFail2 := credSvc.lastRefreshFail[ProviderAnthropic]
	credSvc.refreshFailMutex.RUnlock()

	if !lastFail2.Equal(lastFail) {
		t.Error("Expected backoff to prevent new refresh attempt")
	}
}

func TestGetAllDecrypted_WithExpiredToken_AttemptsRefresh(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key
	cfg := &config.Config{
		EncryptionKey:     []byte("test-key-32-bytes-long-123456789"),
		AnthropicClientID: "test-client-id",
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Create an expired OAuth credential
	expiredTime := time.Now().Add(-1 * time.Hour)
	oauthTokens := &OAuthCredential{
		AccessToken:  "expired-access-token",
		RefreshToken: "invalid-refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    expiredTime,
	}
	_, err = credSvc.SetOAuthTokens(ctx, projectID, ProviderAnthropic, "Anthropic OAuth", oauthTokens)
	if err != nil {
		t.Fatalf("Failed to set OAuth tokens: %v", err)
	}

	// GetAllDecrypted should trigger auto-refresh via GetOAuthTokens
	envVars, err := credSvc.GetAllDecrypted(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to get all decrypted: %v", err)
	}

	// Should still return the credential even though refresh failed
	if len(envVars) != 1 {
		t.Fatalf("Expected 1 credential, got %d", len(envVars))
	}

	// Should use the OAuth-specific env var
	if envVars[0].EnvVar != "CLAUDE_CODE_OAUTH_TOKEN" {
		t.Errorf("Expected CLAUDE_CODE_OAUTH_TOKEN, got %s", envVars[0].EnvVar)
	}

	// Should return the expired token since refresh failed
	if envVars[0].Value != "expired-access-token" {
		t.Errorf("Expected expired-access-token, got %s", envVars[0].Value)
	}
}

func TestSetCustomCredential_BlankValuesPreserveExistingSecrets(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	created, err := credSvc.SetCustomCredential(ctx, projectID, "", "", "", []SecretEnvVar{
		{Key: "FOO_TOKEN", Value: "foo-secret"},
		{Key: "BAR_TOKEN", Value: "bar-secret"},
	}, CredentialVisibility{}, false)
	if err != nil {
		t.Fatalf("Failed to create custom credential: %v", err)
	}

	updated, err := credSvc.SetCustomCredential(ctx, projectID, created.ID, "", "", []SecretEnvVar{
		{Key: "FOO_TOKEN", Value: ""},
		{Key: "BAR_TOKEN", Value: "updated-bar-secret"},
		{Key: "BAZ_TOKEN", Value: ""},
	}, CredentialVisibility{}, false)
	if err != nil {
		t.Fatalf("Failed to update custom credential: %v", err)
	}

	if len(updated.EnvKeys) != 2 || updated.EnvKeys[0] != "FOO_TOKEN" || updated.EnvKeys[1] != "BAR_TOKEN" {
		t.Fatalf("expected updated env keys to preserve populated keys, got %#v", updated.EnvKeys)
	}

	stored, err := st.GetCredentialByIDForProject(ctx, projectID, created.ID)
	if err != nil {
		t.Fatalf("Failed to load stored credential: %v", err)
	}
	data, err := credSvc.getSecretData(stored)
	if err != nil {
		t.Fatalf("Failed to decrypt stored credential: %v", err)
	}
	if len(data.EnvVars) != 2 {
		t.Fatalf("expected 2 stored env vars, got %d", len(data.EnvVars))
	}
	if data.EnvVars[0].Key != "FOO_TOKEN" || data.EnvVars[0].Value != "foo-secret" {
		t.Fatalf("expected FOO_TOKEN secret to be preserved, got %#v", data.EnvVars[0])
	}
	if data.EnvVars[1].Key != "BAR_TOKEN" || data.EnvVars[1].Value != "updated-bar-secret" {
		t.Fatalf("expected BAR_TOKEN secret to be updated, got %#v", data.EnvVars[1])
	}
}

func TestDirectToken_NoRefreshAttemptWhenExpired(t *testing.T) {
	// Create in-memory store
	st := setupTestStore(t)

	// Create config with encryption key
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	// Create credential service
	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"

	// Create an EXPIRED direct token (1 year ago)
	expiredTime := time.Now().Add(-365 * 24 * time.Hour)
	directToken := "sk-ant-oat0-expired-token"

	oauthTokens := &OAuthCredential{
		AccessToken: directToken,
		TokenType:   "Bearer",
		ExpiresAt:   expiredTime,
		// No refresh token for direct tokens
	}
	_, err = credSvc.SetOAuthTokens(ctx, projectID, ProviderAnthropic, "Anthropic Direct Token", oauthTokens)
	if err != nil {
		t.Fatalf("Failed to set OAuth tokens: %v", err)
	}

	// Get tokens - should NOT attempt refresh because there's no refresh token
	tokens, err := credSvc.GetOAuthTokens(ctx, projectID, ProviderAnthropic)
	if err != nil {
		t.Fatalf("Failed to get OAuth tokens: %v", err)
	}

	// Should return the expired direct token
	if tokens.AccessToken != directToken {
		t.Errorf("Expected %s, got %s", directToken, tokens.AccessToken)
	}

	// Verify no backoff was recorded (since no refresh was attempted)
	credSvc.refreshFailMutex.RLock()
	_, hasFailed := credSvc.lastRefreshFail[ProviderAnthropic]
	credSvc.refreshFailMutex.RUnlock()

	if hasFailed {
		t.Error("Expected no refresh failure to be recorded for direct token without refresh token")
	}
}

func TestImportEnvCredentials_CreatesKnownCredentials(t *testing.T) {
	clearStartupCredentialEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("TAVILY_API_KEY", "tvly")

	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	if err := credSvc.ImportEnvCredentials(context.Background(), "test-project"); err != nil {
		t.Fatalf("ImportEnvCredentials failed: %v", err)
	}

	creds, err := credSvc.List(context.Background(), "test-project")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(creds))
	}

	envVars, err := credSvc.GetAllDecrypted(context.Background(), "test-project")
	if err != nil {
		t.Fatalf("GetAllDecrypted failed: %v", err)
	}
	if len(envVars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(envVars))
	}

	got := make(map[string]string, len(envVars))
	for _, envVar := range envVars {
		got[envVar.EnvVar] = envVar.Value
	}
	if got["OPENAI_API_KEY"] != "sk-openai" {
		t.Fatalf("expected OPENAI_API_KEY to be imported, got %q", got["OPENAI_API_KEY"])
	}
	if got["TAVILY_API_KEY"] != "tvly" {
		t.Fatalf("expected TAVILY_API_KEY to be imported, got %q", got["TAVILY_API_KEY"])
	}
}

func TestImportEnvCredentials_IsIdempotent(t *testing.T) {
	clearStartupCredentialEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-openai")

	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"
	if err := credSvc.ImportEnvCredentials(ctx, projectID); err != nil {
		t.Fatalf("first import failed: %v", err)
	}
	if err := credSvc.ImportEnvCredentials(ctx, projectID); err != nil {
		t.Fatalf("second import failed: %v", err)
	}

	creds, err := credSvc.List(ctx, projectID)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential after repeated imports, got %d", len(creds))
	}
}

func TestImportEnvCredentials_DoesNotOverrideExistingEnvVar(t *testing.T) {
	clearStartupCredentialEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"
	if _, err := credSvc.SetCustomCredential(ctx, projectID, "", "", "", []SecretEnvVar{{
		Key:   "OPENAI_API_KEY",
		Value: "existing-secret",
	}}, CredentialVisibility{}, false); err != nil {
		t.Fatalf("Failed to create existing credential: %v", err)
	}

	if err := credSvc.ImportEnvCredentials(ctx, projectID); err != nil {
		t.Fatalf("ImportEnvCredentials failed: %v", err)
	}

	envVars, err := credSvc.GetAllDecrypted(ctx, projectID)
	if err != nil {
		t.Fatalf("GetAllDecrypted failed: %v", err)
	}
	if len(envVars) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(envVars))
	}
	if envVars[0].Value != "existing-secret" {
		t.Fatalf("expected existing secret to remain unchanged, got %q", envVars[0].Value)
	}
}

func TestImportEnvCredentials_BlankEnvVarsDoNothing(t *testing.T) {
	clearStartupCredentialEnv(t)
	t.Setenv("OPENAI_API_KEY", "   ")

	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	if err := credSvc.ImportEnvCredentials(context.Background(), "test-project"); err != nil {
		t.Fatalf("ImportEnvCredentials failed: %v", err)
	}

	creds, err := credSvc.List(context.Background(), "test-project")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(creds) != 0 {
		t.Fatalf("expected no credentials, got %d", len(creds))
	}
}

func TestImportEnvCredentials_PrefersExistingProvider(t *testing.T) {
	clearStartupCredentialEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-key")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-token")

	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	projectID := "test-project"
	if err := credSvc.ImportEnvCredentials(ctx, projectID); err != nil {
		t.Fatalf("ImportEnvCredentials failed: %v", err)
	}

	envVars, err := credSvc.GetAllDecrypted(ctx, projectID)
	if err != nil {
		t.Fatalf("GetAllDecrypted failed: %v", err)
	}
	if len(envVars) != 1 {
		t.Fatalf("expected 1 imported anthropic credential, got %d", len(envVars))
	}
	if envVars[0].EnvVar != "ANTHROPIC_API_KEY" {
		t.Fatalf("expected API key import to win for anthropic, got %s", envVars[0].EnvVar)
	}
}

func TestSessionCredentialVisibility_UsesGlobalAndSessionCombination(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	project := &model.Project{ID: "test-project", Name: "Test Project", Slug: "test-project"}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  project.ID,
		Path:       "/tmp/test-workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:          "test-session",
		ProjectID:   project.ID,
		WorkspaceID: workspace.ID,
		Name:        "Test Session",
		Status:      model.SessionStatusReady,
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	cred, err := credSvc.SetAPIKeyWithMetadata(
		ctx,
		project.ID,
		ProviderAnthropic,
		"Anthropic",
		"",
		"sk-ant-test-123",
		CredentialVisibility{Tools: true, Console: false, Services: true, Hooks: false},
		false,
	)
	if err != nil {
		t.Fatalf("Failed to create credential: %v", err)
	}

	assignments, err := credSvc.SetSessionAssignments(ctx, project.ID, session.ID, []SessionCredentialAssignmentInfo{{
		CredentialID: cred.ID,
		Visibility:   CredentialVisibility{Tools: true, Console: true, Services: false, Hooks: true},
	}})
	if err != nil {
		t.Fatalf("Failed to set session assignments: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}
	if !assignments[0].Visibility.Console {
		t.Fatal("expected raw session assignment to keep console visibility enabled")
	}
	if assignments[0].Credential.Visibility.Console {
		t.Fatal("expected credential visibility to keep the global console setting")
	}

	toolEnvVars, err := credSvc.GetVisibleEnvVarsForSession(ctx, session.ID, CredentialVisibilityContextTools)
	if err != nil {
		t.Fatalf("GetVisibleEnvVarsForSession(tools) failed: %v", err)
	}
	if toolEnvVars["ANTHROPIC_API_KEY"] != "sk-ant-test-123" {
		t.Fatalf("expected tools env var to remain visible, got %#v", toolEnvVars)
	}

	consoleEnvVars, err := credSvc.GetVisibleEnvVarsForSession(ctx, session.ID, CredentialVisibilityContextConsole)
	if err != nil {
		t.Fatalf("GetVisibleEnvVarsForSession(console) failed: %v", err)
	}
	if consoleEnvVars["ANTHROPIC_API_KEY"] != "sk-ant-test-123" {
		t.Fatalf("expected session assignment to be able to enable console visibility, got %#v", consoleEnvVars)
	}

	servicesEnvVars, err := credSvc.GetVisibleEnvVarsForSession(ctx, session.ID, CredentialVisibilityContextServices)
	if err != nil {
		t.Fatalf("GetVisibleEnvVarsForSession(services) failed: %v", err)
	}
	if servicesEnvVars["ANTHROPIC_API_KEY"] != "sk-ant-test-123" {
		t.Fatalf("expected global service visibility to remain enabled despite session assignment, got %#v", servicesEnvVars)
	}
}

func TestCredentialService_SetSessionAssignments_AllowsMultipleEnvVarBindingsForSameCredential(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	project := &model.Project{ID: "test-project", Name: "Test Project", Slug: "test-project"}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  project.ID,
		Path:       "/tmp/test-workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:          "test-session",
		ProjectID:   project.ID,
		WorkspaceID: workspace.ID,
		Name:        "Test Session",
		Status:      model.SessionStatusReady,
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	cred, err := credSvc.SetOAuthTokensWithMetadata(
		ctx,
		project.ID,
		"",
		ProviderGitHub,
		"GitHub",
		"",
		CredentialVisibility{Tools: true},
		false,
		&OAuthCredential{AccessToken: "gho_test_token"},
	)
	if err != nil {
		t.Fatalf("Failed to create credential: %v", err)
	}

	assignments, err := credSvc.SetSessionAssignments(ctx, project.ID, session.ID, []SessionCredentialAssignmentInfo{
		{
			CredentialID:        cred.ID,
			SessionCredentialID: "cred_s_shared",
			EnvVar:              "GH_TOKEN",
			SourceEnvVar:        "GITHUB_TOKEN",
			Visibility:          CredentialVisibility{Tools: true},
			Uses:                []SessionCredentialUse{{Description: "authenticate gh"}},
		},
		{
			CredentialID:        cred.ID,
			SessionCredentialID: "cred_s_shared",
			EnvVar:              "GITHUB_TOKEN",
			SourceEnvVar:        "GITHUB_TOKEN",
			Visibility:          CredentialVisibility{Tools: true},
			Uses:                []SessionCredentialUse{{Description: "authenticate git"}},
		},
	})
	if err != nil {
		t.Fatalf("Failed to set session assignments: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(assignments))
	}

	envVars, err := credSvc.GetAllForSession(ctx, project.ID, session.ID)
	if err != nil {
		t.Fatalf("GetAllForSession failed: %v", err)
	}
	found := map[string]bool{}
	for _, envVar := range envVars {
		if envVar.CredentialID != cred.ID {
			continue
		}
		if envVar.Value != "gho_test_token" {
			t.Fatalf("unexpected token value for %s", envVar.EnvVar)
		}
		if envVar.EnvVar == "GH_TOKEN" || envVar.EnvVar == "GITHUB_TOKEN" {
			found[envVar.EnvVar] = true
		}
	}
	if !found["GH_TOKEN"] || !found["GITHUB_TOKEN"] {
		t.Fatalf("expected both GH_TOKEN and GITHUB_TOKEN bindings, got %#v", envVars)
	}
}

func TestCredentialService_SetSessionAssignments_RemovesDeletedUse(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	project := &model.Project{ID: "test-project", Name: "Test Project", Slug: "test-project"}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  project.ID,
		Path:       "/tmp/test-workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:          "test-session",
		ProjectID:   project.ID,
		WorkspaceID: workspace.ID,
		Name:        "Test Session",
		Status:      model.SessionStatusReady,
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	cred, err := credSvc.SetOAuthTokensWithMetadata(
		ctx,
		project.ID,
		"",
		ProviderGitHub,
		"GitHub",
		"",
		CredentialVisibility{Tools: true},
		false,
		&OAuthCredential{AccessToken: "gho_test_token"},
	)
	if err != nil {
		t.Fatalf("Failed to create credential: %v", err)
	}

	assignments, err := credSvc.SetSessionAssignments(ctx, project.ID, session.ID, []SessionCredentialAssignmentInfo{{
		CredentialID:        cred.ID,
		SessionCredentialID: "cred_s_shared",
		EnvVar:              "GITHUB_TOKEN",
		Visibility:          CredentialVisibility{Tools: true},
		Uses: []SessionCredentialUse{
			{ID: "use_s_keep", Description: "authenticate git"},
			{ID: "use_s_drop", Description: "create pull requests"},
		},
	}})
	if err != nil {
		t.Fatalf("Failed to set session assignments: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}

	assignments, err = credSvc.SetSessionAssignments(ctx, project.ID, session.ID, []SessionCredentialAssignmentInfo{{
		CredentialID:        cred.ID,
		SessionCredentialID: assignments[0].SessionCredentialID,
		EnvVar:              "GITHUB_TOKEN",
		Visibility:          CredentialVisibility{Tools: true},
		Uses: []SessionCredentialUse{
			assignments[0].Uses[0],
		},
	}})
	if err != nil {
		t.Fatalf("Failed to update session assignments: %v", err)
	}
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment after update, got %d", len(assignments))
	}
	if len(assignments[0].Uses) != 1 {
		t.Fatalf("expected 1 remaining use, got %#v", assignments[0].Uses)
	}
	if assignments[0].Uses[0].ID != "use_s_keep" {
		t.Fatalf("expected kept use to remain, got %#v", assignments[0].Uses)
	}
}

func TestCredentialService_GetVisibleEnvVarsForSession_LatestBindingWins(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}

	credSvc, err := NewCredentialService(st, cfg)
	if err != nil {
		t.Fatalf("Failed to create credential service: %v", err)
	}

	ctx := context.Background()
	project := &model.Project{ID: "test-project", Name: "Test Project", Slug: "test-project"}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  project.ID,
		Path:       "/tmp/test-workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:          "test-session",
		ProjectID:   project.ID,
		WorkspaceID: workspace.ID,
		Name:        "Test Session",
		Status:      model.SessionStatusReady,
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	oldCred, err := credSvc.SetCustomCredential(
		ctx,
		project.ID,
		"",
		"Old token",
		"",
		[]SecretEnvVar{{Key: "TOKEN_A", Value: "old-value"}},
		CredentialVisibility{Console: true},
		false,
	)
	if err != nil {
		t.Fatalf("Failed to create old credential: %v", err)
	}
	newCred, err := credSvc.SetCustomCredential(
		ctx,
		project.ID,
		"",
		"New token",
		"",
		[]SecretEnvVar{{Key: "TOKEN_B", Value: "new-value"}},
		CredentialVisibility{Console: true},
		false,
	)
	if err != nil {
		t.Fatalf("Failed to create new credential: %v", err)
	}

	if _, err := credSvc.SetSessionAssignments(ctx, project.ID, session.ID, []SessionCredentialAssignmentInfo{
		{
			CredentialID: oldCred.ID,
			EnvVar:       "SHARED_TOKEN",
			SourceEnvVar: "TOKEN_A",
			Visibility:   CredentialVisibility{Console: true},
		},
		{
			CredentialID: newCred.ID,
			EnvVar:       "SHARED_TOKEN",
			SourceEnvVar: "TOKEN_B",
			Visibility:   CredentialVisibility{Console: true},
		},
	}); err != nil {
		t.Fatalf("Failed to set session assignments: %v", err)
	}

	envVars, err := credSvc.GetVisibleEnvVarsForSession(ctx, session.ID, CredentialVisibilityContextConsole)
	if err != nil {
		t.Fatalf("GetVisibleEnvVarsForSession(console) failed: %v", err)
	}
	if envVars["SHARED_TOKEN"] != "new-value" {
		t.Fatalf("expected latest binding to win, got %#v", envVars)
	}
}
