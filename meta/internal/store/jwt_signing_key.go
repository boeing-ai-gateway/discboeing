package store

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/model"
)

// CreateJWTSigningKey inserts a JWT signing key through the write database
// handle.
func (s *Store) CreateJWTSigningKey(ctx context.Context, key *model.JWTSigningKey) error {
	return s.writeDB.WithContext(ctx).Create(key).Error
}

// GetJWTSigningKeyByID reads a JWT signing key by row ID through the read
// database handle.
func (s *Store) GetJWTSigningKeyByID(ctx context.Context, id string) (*model.JWTSigningKey, error) {
	var key model.JWTSigningKey
	if err := s.readDB.WithContext(ctx).First(&key, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &key, nil
}

// GetJWTSigningKeyByKID reads a JWT signing key by public kid through the read
// database handle.
func (s *Store) GetJWTSigningKeyByKID(ctx context.Context, kid string) (*model.JWTSigningKey, error) {
	var key model.JWTSigningKey
	if err := s.readDB.WithContext(ctx).First(&key, "kid = ?", kid).Error; err != nil {
		return nil, notFound(err)
	}
	return &key, nil
}

// ListJWTSigningKeys reads all JWT signing keys for one issuer scope.
func (s *Store) ListJWTSigningKeys(ctx context.Context, organizationID *string) ([]*model.JWTSigningKey, error) {
	var keys []*model.JWTSigningKey
	db := s.readDB.WithContext(ctx).Order("created_at asc, id asc")
	if organizationID == nil {
		db = db.Where("organization_id IS NULL")
	} else {
		db = db.Where("organization_id = ?", *organizationID)
	}
	if err := db.Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// UpdateJWTSigningKeyStatus updates status and validity timestamps for one key.
func (s *Store) UpdateJWTSigningKeyStatus(ctx context.Context, id, status string, notBefore, notAfter *time.Time) error {
	updates := map[string]any{
		"status":     status,
		"not_before": notBefore,
		"not_after":  notAfter,
	}
	result := s.writeDB.WithContext(ctx).Model(&model.JWTSigningKey{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// WithTx runs fn inside a write transaction.
func (s *Store) WithTx(ctx context.Context, fn func(*Store) error) error {
	return s.writeDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(New(tx, tx))
	})
}
