package thread

import (
	"context"
	"errors"
	"testing"
)

func TestThreadErrorHelpers_ClearPersistAndIgnoreCancel(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.CreateThread("thread-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConfig("thread-1", Config{ErrorMessage: "old error"}); err != nil {
		t.Fatal(err)
	}

	if _, cleared := ClearError(store, "thread-1"); !cleared {
		t.Fatal("expected ClearError to report a cleared error")
	}
	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ErrorMessage != "" {
		t.Fatalf("error message = %q, want empty", cfg.ErrorMessage)
	}

	if persisted := PersistError(store, "thread-1", errors.New("provider failed")); !persisted {
		t.Fatal("expected PersistError to report a persisted error")
	}
	cfg, err = store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ErrorMessage != "provider failed" {
		t.Fatalf("error message = %q, want provider failed", cfg.ErrorMessage)
	}

	if persisted := PersistError(store, "thread-1", context.Canceled); persisted {
		t.Fatal("expected canceled error to be ignored")
	}
	cfg, err = store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ErrorMessage != "provider failed" {
		t.Fatalf("canceled error changed message to %q", cfg.ErrorMessage)
	}
}
