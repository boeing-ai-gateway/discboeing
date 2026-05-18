package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/obot-platform/discobot/ui-go/internal/config"
	uisession "github.com/obot-platform/discobot/ui-go/internal/session"
)

func TestSetStaticCacheHeaders(t *testing.T) {
	tests := map[string]string{
		"/app.css":                              "no-cache",
		"/assets/app.js":                        "no-cache",
		"/vendor/datastar.js":                   "no-cache",
		"/files/geist-sans-latin-400.woff2":     "no-cache",
		"/assets/chunks/chunk-43AOXMG4.js":      "public, max-age=31536000, immutable",
		"/assets/chunks/emacs-lisp-6XHGK2T3.js": "public, max-age=31536000, immutable",
	}

	for path, expected := range tests {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			setStaticCacheHeaders(w, path)
			if got := w.Header().Get("Cache-Control"); got != expected {
				t.Fatalf("Cache-Control = %q, want %q", got, expected)
			}
		})
	}
}

func TestSetStaticCacheHeadersPreservesExistingPolicy(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("Cache-Control", "no-store")

	setStaticCacheHeaders(w, "/assets/chunks/chunk-ABC123.js")

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func TestStaticAssetsDoNotCreateSessions(t *testing.T) {
	staticDir := t.TempDir()
	chunkDir := filepath.Join(staticDir, "assets", "chunks")
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "app.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "chunk-ABC123.js"), []byte("export {};"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{Port: "0", StaticDir: staticDir}, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()

	tests := map[string]string{
		"/app.css":                       "no-cache",
		"/assets/chunks/chunk-ABC123.js": "public, max-age=31536000, immutable",
	}
	for path, cacheControl := range tests {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := recorder.Header().Get("Set-Cookie"); got != "" {
				t.Fatalf("Set-Cookie = %q, want empty", got)
			}
			if got := recorder.Header().Get("Cache-Control"); got != cacheControl {
				t.Fatalf("Cache-Control = %q, want %q", got, cacheControl)
			}
		})
	}
}

func TestEnsureSessionAcceptsEmbeddedFallbacks(t *testing.T) {
	const sessionID = "0123456789abcdef0123456789abcdef"
	server := New(config.Config{Port: "0"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := server.ensureSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := uisession.ID(r.Context())
		if !ok {
			t.Fatal("missing session ID on request context")
		}
		if id != sessionID {
			t.Fatalf("session ID = %q, want %q", id, sessionID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := map[string]*http.Request{
		"query": httptest.NewRequest(http.MethodPost, "/ui/commands/sidebar/toggle-collapsed?"+uisession.QueryParam+"="+sessionID, nil),
		"header": func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/ui/commands/sidebar/toggle-collapsed", nil)
			req.Header.Set(uisession.HeaderName, sessionID)
			return req
		}(),
	}

	for name, req := range tests {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
			}
		})
	}
}
