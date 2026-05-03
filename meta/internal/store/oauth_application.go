package store

import (
	"context"

	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/model"
)

// CreateOAuthApplication inserts an organization-scoped OAuth application.
func (s *Store) CreateOAuthApplication(ctx context.Context, app *model.OAuthApplication) error {
	return s.writeDB.WithContext(ctx).Create(app).Error
}

// ListOAuthApplications returns non-deleted OAuth applications for an organization.
func (s *Store) ListOAuthApplications(ctx context.Context, organizationID string) ([]model.OAuthApplication, error) {
	var apps []model.OAuthApplication
	err := s.readDB.WithContext(ctx).
		Where("organization_id = ? AND status <> ?", organizationID, model.OAuthApplicationStatusDeleted).
		Order("created_at ASC").
		Find(&apps).Error
	return apps, err
}

// GetOAuthApplication reads one non-deleted OAuth application by organization and ID.
func (s *Store) GetOAuthApplication(ctx context.Context, organizationID, appID string) (*model.OAuthApplication, error) {
	var app model.OAuthApplication
	err := s.readDB.WithContext(ctx).
		First(&app, "organization_id = ? AND id = ? AND status <> ?", organizationID, appID, model.OAuthApplicationStatusDeleted).
		Error
	if err != nil {
		return nil, notFound(err)
	}
	return &app, nil
}

// UpdateOAuthApplication updates one OAuth application.
func (s *Store) UpdateOAuthApplication(ctx context.Context, app *model.OAuthApplication) error {
	return s.writeDB.WithContext(ctx).Save(app).Error
}

// DeleteOAuthApplication soft-deletes one OAuth application.
func (s *Store) DeleteOAuthApplication(ctx context.Context, organizationID, appID string) error {
	return s.writeDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var app model.OAuthApplication
		err := tx.First(&app, "organization_id = ? AND id = ? AND status <> ?", organizationID, appID, model.OAuthApplicationStatusDeleted).Error
		if err != nil {
			return notFound(err)
		}
		app.Status = model.OAuthApplicationStatusDeleted
		if err := tx.Save(&app).Error; err != nil {
			return err
		}
		return tx.Delete(&app).Error
	})
}
