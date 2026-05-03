package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/id"
	"github.com/obot-platform/discobot/meta/internal/model"
	"github.com/obot-platform/discobot/meta/internal/store"
)

type ErrorKind string

const (
	ErrorKindInvalidRequest ErrorKind = "invalid_request"
	ErrorKindNotFound       ErrorKind = "not_found"
	ErrorKindUnavailable    ErrorKind = "unavailable"
)

type ServiceError struct {
	Kind    ErrorKind
	Message string
	Err     error
}

func (e *ServiceError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Kind)
}

func (e *ServiceError) Unwrap() error { return e.Err }

func errorKind(err error) (ErrorKind, bool) {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr.Kind, true
	}
	return "", false
}

func IsInvalidRequest(err error) bool {
	kind, ok := errorKind(err)
	return ok && kind == ErrorKindInvalidRequest
}

func IsNotFound(err error) bool {
	kind, ok := errorKind(err)
	return ok && kind == ErrorKindNotFound
}

func IsUnavailable(err error) bool {
	kind, ok := errorKind(err)
	return ok && kind == ErrorKindUnavailable
}

func invalidRequest(message string) error {
	return &ServiceError{Kind: ErrorKindInvalidRequest, Message: message}
}

func notFound(message string, err error) error {
	return &ServiceError{Kind: ErrorKindNotFound, Message: message, Err: err}
}

func unavailable(message string, err error) error {
	return &ServiceError{Kind: ErrorKindUnavailable, Message: message, Err: err}
}

// OAuthApplicationService owns organization-scoped OAuth application behavior.
type OAuthApplicationService struct {
	Store             *store.Store
	DatabaseEncryptor dbcrypt.Encryptor
}

type OAuthApplicationInput struct {
	Name                    *string        `json:"name"`
	Provider                *string        `json:"provider"`
	ClientID                *string        `json:"clientId"`
	ClientSecret            *string        `json:"clientSecret"`
	RedirectURIs            []string       `json:"redirectUris"`
	GrantTypes              []string       `json:"grantTypes"`
	ResponseTypes           []string       `json:"responseTypes"`
	Scopes                  []string       `json:"scopes"`
	TokenEndpointAuthMethod *string        `json:"tokenEndpointAuthMethod"`
	Status                  *string        `json:"status"`
	ProviderConfig          map[string]any `json:"providerConfig"`
	GitHub                  map[string]any `json:"github"`
	Google                  map[string]any `json:"google"`
}

type OAuthApplication struct {
	ID                      string         `json:"id"`
	OrganizationID          string         `json:"organizationId"`
	Name                    string         `json:"name"`
	Provider                string         `json:"provider"`
	ClientID                string         `json:"clientId"`
	HasClientSecret         bool           `json:"hasClientSecret"`
	RedirectURIs            []string       `json:"redirectUris"`
	GrantTypes              []string       `json:"grantTypes"`
	ResponseTypes           []string       `json:"responseTypes"`
	Scopes                  []string       `json:"scopes"`
	TokenEndpointAuthMethod string         `json:"tokenEndpointAuthMethod"`
	Status                  string         `json:"status"`
	ProviderConfig          map[string]any `json:"providerConfig,omitempty"`
	CreatedByPrincipal      string         `json:"createdByPrincipal"`
	CreatedAt               time.Time      `json:"createdAt"`
	UpdatedAt               time.Time      `json:"updatedAt"`
}

func (s *OAuthApplicationService) CreateOAuthApplication(ctx context.Context, organizationDomain, createdByPrincipal string, input OAuthApplicationInput) (*OAuthApplication, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	organization, err := s.organizationByDomain(ctx, organizationDomain)
	if err != nil {
		return nil, err
	}
	if err := input.validateCreate(); err != nil {
		return nil, err
	}
	app, err := input.toModel(organization.ID, createdByPrincipal, nil)
	if err != nil {
		return nil, err
	}
	if err := s.encryptClientSecret(ctx, app, input.ClientSecret); err != nil {
		return nil, err
	}
	if err := s.Store.CreateOAuthApplication(ctx, app); err != nil {
		return nil, unavailable("failed to create OAuth application", err)
	}
	result := oauthApplicationFromModel(app)
	return &result, nil
}

func (s *OAuthApplicationService) ListOAuthApplications(ctx context.Context, organizationDomain string) ([]OAuthApplication, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	organization, err := s.organizationByDomain(ctx, organizationDomain)
	if err != nil {
		return nil, err
	}
	apps, err := s.Store.ListOAuthApplications(ctx, organization.ID)
	if err != nil {
		return nil, unavailable("failed to list OAuth applications", err)
	}
	items := make([]OAuthApplication, 0, len(apps))
	for i := range apps {
		items = append(items, oauthApplicationFromModel(&apps[i]))
	}
	return items, nil
}

func (s *OAuthApplicationService) GetOAuthApplication(ctx context.Context, organizationDomain, appID string) (*OAuthApplication, error) {
	app, err := s.getOAuthApplicationModel(ctx, organizationDomain, appID)
	if err != nil {
		return nil, err
	}
	result := oauthApplicationFromModel(app)
	return &result, nil
}

func (s *OAuthApplicationService) UpdateOAuthApplication(ctx context.Context, organizationDomain, appID string, input OAuthApplicationInput) (*OAuthApplication, error) {
	app, err := s.getOAuthApplicationModel(ctx, organizationDomain, appID)
	if err != nil {
		return nil, err
	}
	if err := input.apply(app); err != nil {
		return nil, err
	}
	if err := s.encryptClientSecret(ctx, app, input.ClientSecret); err != nil {
		return nil, err
	}
	if err := s.Store.UpdateOAuthApplication(ctx, app); err != nil {
		return nil, unavailable("failed to update OAuth application", err)
	}
	result := oauthApplicationFromModel(app)
	return &result, nil
}

func (s *OAuthApplicationService) DeleteOAuthApplication(ctx context.Context, organizationDomain, appID string) error {
	if err := s.ready(); err != nil {
		return err
	}
	organization, err := s.organizationByDomain(ctx, organizationDomain)
	if err != nil {
		return err
	}
	if err := s.Store.DeleteOAuthApplication(ctx, organization.ID, appID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return notFound("OAuth application not found", err)
		}
		return unavailable("failed to delete OAuth application", err)
	}
	return nil
}

func (s *OAuthApplicationService) DecryptClientSecret(ctx context.Context, app *model.OAuthApplication) (string, error) {
	if len(app.ClientSecretEncrypted) == 0 {
		return "", nil
	}
	if s.DatabaseEncryptor == nil {
		return "", unavailable("database encryptor is not configured", nil)
	}
	plaintext, err := store.DecryptField(ctx, s.DatabaseEncryptor, app, store.FieldOAuthApplicationClientSecret)
	if err != nil {
		return "", unavailable("decrypt OAuth client secret", err)
	}
	return string(plaintext), nil
}

func (s *OAuthApplicationService) getOAuthApplicationModel(ctx context.Context, organizationDomain, appID string) (*model.OAuthApplication, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	organization, err := s.organizationByDomain(ctx, organizationDomain)
	if err != nil {
		return nil, err
	}
	app, err := s.Store.GetOAuthApplication(ctx, organization.ID, appID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, notFound("OAuth application not found", err)
		}
		return nil, unavailable("failed to read OAuth application", err)
	}
	return app, nil
}

func (s *OAuthApplicationService) organizationByDomain(ctx context.Context, organizationDomain string) (*model.Organization, error) {
	if strings.TrimSpace(organizationDomain) == "" {
		return nil, invalidRequest("organization domain is required")
	}
	organization, err := s.Store.GetOrganizationByDomain(ctx, organizationDomain)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, notFound("organization not found", err)
		}
		return nil, unavailable("failed to read organization", err)
	}
	return organization, nil
}

func (s *OAuthApplicationService) ready() error {
	if s == nil || s.Store == nil {
		return unavailable("store is not configured", nil)
	}
	return nil
}

func (s *OAuthApplicationService) encryptClientSecret(ctx context.Context, app *model.OAuthApplication, secret *string) error {
	if secret == nil {
		return nil
	}
	if s.DatabaseEncryptor == nil {
		return unavailable("database encryptor is not configured", nil)
	}
	if err := store.SetEncryptedField(ctx, s.DatabaseEncryptor, app, store.FieldOAuthApplicationClientSecret, []byte(*secret)); err != nil {
		return unavailable("encrypt OAuth client secret", err)
	}
	return nil
}

func (i OAuthApplicationInput) validateCreate() error {
	if i.Name == nil || strings.TrimSpace(*i.Name) == "" {
		return invalidRequest("name is required")
	}
	if i.Provider == nil || strings.TrimSpace(*i.Provider) == "" {
		return invalidRequest("provider is required")
	}
	if i.ClientID == nil || strings.TrimSpace(*i.ClientID) == "" {
		return invalidRequest("clientId is required")
	}
	return validateOAuthProvider(strings.TrimSpace(*i.Provider))
}

func (i OAuthApplicationInput) toModel(organizationID, createdBy string, existing *model.OAuthApplication) (*model.OAuthApplication, error) {
	app := existing
	if app == nil {
		app = &model.OAuthApplication{
			ID:                      id.MustNew(id.TypeOAuthApplication),
			OrganizationID:          organizationID,
			RedirectURIsJSON:        mustMarshalStrings(defaultStrings(i.RedirectURIs, nil)),
			GrantTypesJSON:          mustMarshalStrings(defaultStrings(i.GrantTypes, []string{"authorization_code"})),
			ResponseTypesJSON:       mustMarshalStrings(defaultStrings(i.ResponseTypes, []string{"code"})),
			TokenEndpointAuthMethod: "client_secret_basic",
			Status:                  model.OAuthApplicationStatusActive,
			CreatedByPrincipal:      createdBy,
		}
	}
	if err := i.apply(app); err != nil {
		return nil, err
	}
	return app, nil
}

func (i OAuthApplicationInput) apply(app *model.OAuthApplication) error {
	if i.Name != nil {
		app.Name = strings.TrimSpace(*i.Name)
		if app.Name == "" {
			return invalidRequest("name cannot be empty")
		}
	}
	if i.Provider != nil {
		app.Provider = strings.TrimSpace(*i.Provider)
		if err := validateOAuthProvider(app.Provider); err != nil {
			return err
		}
	}
	if i.ClientID != nil {
		app.ClientID = strings.TrimSpace(*i.ClientID)
		if app.ClientID == "" {
			return invalidRequest("clientId cannot be empty")
		}
	}
	if i.RedirectURIs != nil {
		app.RedirectURIsJSON = mustMarshalStrings(cleanStrings(i.RedirectURIs))
	}
	if i.GrantTypes != nil {
		app.GrantTypesJSON = mustMarshalStrings(cleanStrings(i.GrantTypes))
	}
	if i.ResponseTypes != nil {
		app.ResponseTypesJSON = mustMarshalStrings(cleanStrings(i.ResponseTypes))
	}
	if i.Scopes != nil {
		app.Scopes = strings.Join(cleanStrings(i.Scopes), " ")
	}
	if i.TokenEndpointAuthMethod != nil {
		app.TokenEndpointAuthMethod = strings.TrimSpace(*i.TokenEndpointAuthMethod)
		if app.TokenEndpointAuthMethod == "" {
			return invalidRequest("tokenEndpointAuthMethod cannot be empty")
		}
	}
	if i.Status != nil {
		app.Status = strings.TrimSpace(*i.Status)
		if app.Status != model.OAuthApplicationStatusActive && app.Status != model.OAuthApplicationStatusDisabled {
			return invalidRequest("status must be active or disabled")
		}
	}
	providerConfig := i.ProviderConfig
	if app.Provider == model.OAuthApplicationProviderGitHub && i.GitHub != nil {
		providerConfig = i.GitHub
	}
	if app.Provider == model.OAuthApplicationProviderGoogle && i.Google != nil {
		providerConfig = i.Google
	}
	if providerConfig != nil {
		data, err := json.Marshal(providerConfig)
		if err != nil {
			return invalidRequest("providerConfig must be JSON serializable")
		}
		app.ProviderConfigJSON = data
	}
	return nil
}

func oauthApplicationFromModel(app *model.OAuthApplication) OAuthApplication {
	return OAuthApplication{
		ID:                      app.ID,
		OrganizationID:          app.OrganizationID,
		Name:                    app.Name,
		Provider:                app.Provider,
		ClientID:                app.ClientID,
		HasClientSecret:         len(app.ClientSecretEncrypted) > 0,
		RedirectURIs:            unmarshalStrings(app.RedirectURIsJSON),
		GrantTypes:              unmarshalStrings(app.GrantTypesJSON),
		ResponseTypes:           unmarshalStrings(app.ResponseTypesJSON),
		Scopes:                  strings.Fields(app.Scopes),
		TokenEndpointAuthMethod: app.TokenEndpointAuthMethod,
		Status:                  app.Status,
		ProviderConfig:          unmarshalMap(app.ProviderConfigJSON),
		CreatedByPrincipal:      app.CreatedByPrincipal,
		CreatedAt:               app.CreatedAt,
		UpdatedAt:               app.UpdatedAt,
	}
}

func validateOAuthProvider(provider string) error {
	if provider != model.OAuthApplicationProviderGitHub && provider != model.OAuthApplicationProviderGoogle {
		return invalidRequest("provider must be github or google")
	}
	return nil
}

func cleanStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func defaultStrings(values, fallback []string) []string {
	if values = cleanStrings(values); len(values) > 0 {
		return values
	}
	return fallback
}

func mustMarshalStrings(values []string) []byte {
	data, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return data
}

func unmarshalStrings(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return nil
	}
	return values
}

func unmarshalMap(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	return value
}

func serviceErrorMessage(err error) string {
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) && serviceErr.Message != "" {
		return serviceErr.Message
	}
	return fmt.Sprint(err)
}
