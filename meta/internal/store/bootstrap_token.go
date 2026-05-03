package store

import (
	"context"
	"time"

	"github.com/obot-platform/discobot/meta/internal/model"
	"gorm.io/gorm"
)

// CreateOrganizationBootstrapToken inserts an organization bootstrap token.
func (s *Store) CreateOrganizationBootstrapToken(ctx context.Context, token *model.OrganizationBootstrapToken) error {
	return s.writeDB.WithContext(ctx).Create(token).Error
}

// GetActiveOrganizationBootstrapTokenByHash reads an active bootstrap token by
// token hash.
func (s *Store) GetActiveOrganizationBootstrapTokenByHash(ctx context.Context, tokenHash string, now time.Time) (*model.OrganizationBootstrapToken, error) {
	var token model.OrganizationBootstrapToken
	err := s.readDB.WithContext(ctx).
		Where("token_hash = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)", tokenHash, model.OrganizationBootstrapTokenStatusActive, now).
		First(&token).Error
	if err != nil {
		return nil, notFound(err)
	}
	return &token, nil
}

// ListActiveOrganizationBootstrapTokens reads active bootstrap tokens for an
// organization.
func (s *Store) ListActiveOrganizationBootstrapTokens(ctx context.Context, organizationID string, now time.Time) ([]model.OrganizationBootstrapToken, error) {
	var tokens []model.OrganizationBootstrapToken
	err := s.readDB.WithContext(ctx).
		Where("organization_id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)", organizationID, model.OrganizationBootstrapTokenStatusActive, now).
		Order("created_at ASC").
		Find(&tokens).Error
	return tokens, err
}

// MarkOrganizationBootstrapTokenUsed updates LastUsedAt for an active bootstrap
// token.
func (s *Store) MarkOrganizationBootstrapTokenUsed(ctx context.Context, id string, usedAt time.Time) error {
	result := s.writeDB.WithContext(ctx).
		Model(&model.OrganizationBootstrapToken{}).
		Where("id = ?", id).
		Update("last_used_at", usedAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeActiveOrganizationBootstrapTokens revokes all active bootstrap tokens
// for an organization.
func (s *Store) RevokeActiveOrganizationBootstrapTokens(ctx context.Context, organizationID string, revokedAt time.Time) error {
	updates := map[string]any{
		"status":     model.OrganizationBootstrapTokenStatusRevoked,
		"revoked_at": revokedAt,
	}
	err := s.writeDB.WithContext(ctx).
		Model(&model.OrganizationBootstrapToken{}).
		Where("organization_id = ? AND status = ?", organizationID, model.OrganizationBootstrapTokenStatusActive).
		Updates(updates).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}
