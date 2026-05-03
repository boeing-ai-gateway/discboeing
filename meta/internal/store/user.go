package store

import (
	"context"

	"github.com/obot-platform/discobot/meta/internal/model"
)

// CreateUser inserts a Meta user through the write database handle.
func (s *Store) CreateUser(ctx context.Context, user *model.User) error {
	return s.writeDB.WithContext(ctx).Create(user).Error
}

// GetUserByID reads a Meta user by ID through the read database handle.
func (s *Store) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	if err := s.readDB.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, notFound(err)
	}
	return &user, nil
}

// GetUserByPrimaryEmail reads a Meta user by primary email through the read
// database handle.
func (s *Store) GetUserByPrimaryEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := s.readDB.WithContext(ctx).First(&user, "primary_email = ?", email).Error; err != nil {
		return nil, notFound(err)
	}
	return &user, nil
}

// DeleteUser deletes a Meta user by ID through the write database handle.
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	result := s.writeDB.WithContext(ctx).Delete(&model.User{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
