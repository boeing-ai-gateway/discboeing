package store

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestUserStoreLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}

	ctx := context.Background()
	st := New(db, nil)
	user := &model.User{
		PrimaryEmail:  "user@example.com",
		EmailVerified: true,
	}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if user.ID == "" {
		t.Fatalf("expected CreateUser to run BeforeCreate and assign an ID")
	}

	byID, err := st.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if byID.PrimaryEmail != user.PrimaryEmail {
		t.Fatalf("expected email %q, got %q", user.PrimaryEmail, byID.PrimaryEmail)
	}

	byEmail, err := st.GetUserByPrimaryEmail(ctx, user.PrimaryEmail)
	if err != nil {
		t.Fatalf("GetUserByPrimaryEmail() error = %v", err)
	}
	if byEmail.ID != user.ID {
		t.Fatalf("expected user ID %q, got %q", user.ID, byEmail.ID)
	}

	if err := st.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if _, err := st.GetUserByID(ctx, user.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	var deleted model.User
	if err := db.Unscoped().First(&deleted, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("expected soft-deleted user to remain in unscoped query: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatalf("expected user DeletedAt to be set")
	}
	if err := st.DeleteUser(ctx, user.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting missing user, got %v", err)
	}
}
