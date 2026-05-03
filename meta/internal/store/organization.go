package store

import (
	"context"

	"github.com/obot-platform/discobot/meta/internal/model"
	"gorm.io/gorm"
)

// CreateOrganization inserts a Meta organization through the write database
// handle.
func (s *Store) CreateOrganization(ctx context.Context, organization *model.Organization) error {
	return s.writeDB.WithContext(ctx).Create(organization).Error
}

// GetOrganizationByID reads a Meta organization by ID through the read database
// handle.
func (s *Store) GetOrganizationByID(ctx context.Context, id string) (*model.Organization, error) {
	var organization model.Organization
	if err := s.readDB.WithContext(ctx).First(&organization, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &organization, nil
}

// GetOrganizationByDomain reads a Meta organization by domain through the read
// database handle.
func (s *Store) GetOrganizationByDomain(ctx context.Context, domain string) (*model.Organization, error) {
	var organization model.Organization
	if err := s.readDB.WithContext(ctx).First(&organization, "domain = ?", domain).Error; err != nil {
		return nil, notFound(err)
	}
	return &organization, nil
}

// DeleteOrganization deletes a Meta organization by ID through the write
// database handle.
func (s *Store) DeleteOrganization(ctx context.Context, id string) error {
	result := s.writeDB.WithContext(ctx).Delete(&model.Organization{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateOrganizationMember inserts an organization member through the write
// database handle.
func (s *Store) CreateOrganizationMember(ctx context.Context, member *model.OrganizationMember) error {
	return s.writeDB.WithContext(ctx).Create(member).Error
}

// HasOrganizationOwnerOrAdmin reports whether an organization has any real
// owner/admin user membership.
func (s *Store) HasOrganizationOwnerOrAdmin(ctx context.Context, organizationID string) (bool, error) {
	var count int64
	err := s.readDB.WithContext(ctx).
		Model(&model.OrganizationMember{}).
		Where("organization_id = ? AND role IN ?", organizationID, []string{model.OrganizationRoleOwner, model.OrganizationRoleAdmin}).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// OrganizationMemberRole returns a user's role in an organization.
func (s *Store) OrganizationMemberRole(ctx context.Context, organizationID, userID string) (string, error) {
	var member model.OrganizationMember
	err := s.readDB.WithContext(ctx).
		First(&member, "organization_id = ? AND user_id = ?", organizationID, userID).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", ErrNotFound
		}
		return "", err
	}
	return member.Role, nil
}
