package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/store"
)

const testEncryptionKey = "01234567890123456789012345678901"

func newTestDB(t *testing.T) *DB {
	t.Helper()

	cfg := &config.Config{
		DatabaseDSN:    "sqlite3://" + filepath.Join(t.TempDir(), "test.db"),
		DatabaseDriver: "sqlite",
		EncryptionKey:  []byte(testEncryptionKey),
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() failed: %v", err)
		}
	})
	return db
}

func createLegacySessionsTable(t *testing.T, db *DB) {
	t.Helper()

	if err := db.Exec(`
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'initializing',
			sandbox_status TEXT NOT NULL DEFAULT 'initializing',
			thread_status TEXT NOT NULL DEFAULT 'idle',
			error_message TEXT,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Error; err != nil {
		t.Fatalf("creating sessions table failed: %v", err)
	}
}

func insertLegacySession(t *testing.T, db *DB, status, sandboxStatus string) {
	t.Helper()

	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO sessions (id, project_id, workspace_id, name, status, sandbox_status, thread_status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"session-1",
		"project-1",
		"workspace-1",
		"Session",
		status,
		sandboxStatus,
		"idle",
		now,
		now,
	).Error; err != nil {
		t.Fatalf("inserting session failed: %v", err)
	}
}

func readSessionStatuses(t *testing.T, db *DB) (string, string) {
	t.Helper()

	var row struct {
		Status        string
		SandboxStatus string `gorm:"column:sandbox_status"`
	}
	if err := db.Raw(`SELECT status, sandbox_status FROM sessions WHERE id = ?`, "session-1").Scan(&row).Error; err != nil {
		t.Fatalf("reading session statuses failed: %v", err)
	}
	return row.Status, row.SandboxStatus
}

func TestSyncLegacySessionStatusFromSandboxStatus(t *testing.T) {
	db := newTestDB(t)
	createLegacySessionsTable(t, db)
	insertLegacySession(t, db, "initializing", "ready")

	if err := db.syncLegacySessionStatusFromSandboxStatus(); err != nil {
		t.Fatalf("syncLegacySessionStatusFromSandboxStatus() failed: %v", err)
	}

	status, sandboxStatus := readSessionStatuses(t, db)
	if status != "ready" {
		t.Fatalf("status = %q, want %q", status, "ready")
	}
	if sandboxStatus != "ready" {
		t.Fatalf("sandbox_status = %q, want %q", sandboxStatus, "ready")
	}
}

func TestUpdateSessionStatusMirrorsLegacyStatusColumn(t *testing.T) {
	db := newTestDB(t)
	createLegacySessionsTable(t, db)
	insertLegacySession(t, db, "initializing", "initializing")

	st := store.New(db.DB, db.ReadDB)
	if err := st.UpdateSessionStatus(context.Background(), "session-1", "stopped", nil); err != nil {
		t.Fatalf("UpdateSessionStatus() failed: %v", err)
	}

	status, sandboxStatus := readSessionStatuses(t, db)
	if status != "stopped" {
		t.Fatalf("status = %q, want %q", status, "stopped")
	}
	if sandboxStatus != "stopped" {
		t.Fatalf("sandbox_status = %q, want %q", sandboxStatus, "stopped")
	}
}

func TestMigrateAddsSessionSandboxStatusMessage(t *testing.T) {
	db := newTestDB(t)
	createLegacySessionsTable(t, db)

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() failed: %v", err)
	}

	if !db.Migrator().HasColumn("sessions", "sandbox_status_message") {
		t.Fatal("sessions.sandbox_status_message column was not added")
	}
}
