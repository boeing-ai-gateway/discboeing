package database

import (
	"path/filepath"
	"testing"

	"github.com/obot-platform/discobot/meta/internal/config"
	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestNewSQLiteCreatesSeparateReadPool(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "meta.db")
	db, err := New(&config.Config{DatabaseDSN: "sqlite3://" + dbPath, DatabaseDriver: "sqlite"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	if !db.IsSQLite() {
		t.Fatalf("expected sqlite database")
	}
	if db.ReadDB == nil {
		t.Fatalf("expected file-backed sqlite to have a separate read pool")
	}
	if db.ReadDB == db.DB {
		t.Fatalf("expected read and write pools to be different handles")
	}
}

func TestSQLiteMigrateAndReadPool(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "meta.db")
	db, err := New(&config.Config{DatabaseDSN: "sqlite3://" + dbPath, DatabaseDriver: "sqlite"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := db.Create(&model.Organization{Name: "Public", Domain: model.PublicOrganizationDomain}).Error; err != nil {
		t.Fatalf("Create organization error = %v", err)
	}

	var org model.Organization
	if err := db.ReadDB.First(&org, "domain = ?", model.PublicOrganizationDomain).Error; err != nil {
		t.Fatalf("read pool query error = %v", err)
	}
	if org.ID == "" {
		t.Fatalf("expected BeforeCreate to set organization ID")
	}
}
