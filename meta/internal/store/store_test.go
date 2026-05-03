package store

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNewFallsBackToWriteDBForReads(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	st := New(db, nil)
	if st.DB() != db {
		t.Fatalf("expected write DB handle to be retained")
	}
	if st.ReadDB() != db {
		t.Fatalf("expected read DB to fall back to write DB")
	}
}

func TestNotFoundMapsGormRecordNotFound(t *testing.T) {
	if err := notFound(gorm.ErrRecordNotFound); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
