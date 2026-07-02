package static

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAHandlerServesInjectedIndexAtRoot(t *testing.T) {
	handler, err := newSPAHandlerWithFS(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><head></head><body>built index</body></html>")},
	}, []byte("<html><head></head><body>fallback</body></html>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "window.__DISCBOEING_CONFIG__") {
		t.Fatalf("expected injected runtime config, got %q", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"apiRoot":"/api"`) {
		t.Fatalf("expected injected apiRoot, got %q", resp.Body.String())
	}
	if got := resp.Header().Get("Cache-Control"); got != htmlCacheValue {
		t.Fatalf("expected html cache header %q, got %q", htmlCacheValue, got)
	}
}

func TestSPAHandlerServesBuiltAsset(t *testing.T) {
	handler, err := newSPAHandlerWithFS(fstest.MapFS{
		"index.html":                       &fstest.MapFile{Data: []byte("<html><head></head><body>built index</body></html>")},
		"_app/immutable/assets/app-123.js": &fstest.MapFile{Data: []byte("console.log('asset');")},
	}, []byte("<html><head></head><body>fallback</body></html>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/assets/app-123.js", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "asset") {
		t.Fatalf("expected asset body, got %q", resp.Body.String())
	}
	if got := resp.Header().Get("Cache-Control"); got != immutableAssetCacheValue {
		t.Fatalf("expected asset cache header %q, got %q", immutableAssetCacheValue, got)
	}
}

func TestSPAHandlerFallsBackForClientRoute(t *testing.T) {
	handler, err := newSPAHandlerWithFS(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><head></head><body>built index</body></html>")},
		"200.html":   &fstest.MapFile{Data: []byte("<html><head></head><body>spa fallback</body></html>")},
	}, []byte("<html><head></head><body>spa fallback</body></html>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sessions/123", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "spa fallback") {
		t.Fatalf("expected fallback body, got %q", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "window.__DISCBOEING_CONFIG__") {
		t.Fatalf("expected injected runtime config, got %q", resp.Body.String())
	}
}

func TestSPAHandlerDoesNotInterceptReservedPrefixes(t *testing.T) {
	handler, err := newSPAHandlerWithFS(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><head></head><body>built index</body></html>")},
	}, []byte("<html><head></head><body>fallback</body></html>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, requestPath := range []string{"/api/missing", "/auth/missing", "/debug/missing", "/health/missing"} {
		t.Run(requestPath, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, requestPath, nil)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)

			if resp.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d", resp.Code)
			}
		})
	}
}

func TestSPAHandlerDoesNotFallbackForMissingAsset(t *testing.T) {
	handler, err := newSPAHandlerWithFS(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><head></head><body>built index</body></html>")},
	}, []byte("<html><head></head><body>fallback</body></html>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestSPAHandlerDoesNotFallbackForNonGET(t *testing.T) {
	handler, err := newSPAHandlerWithFS(nil, []byte("<html><head></head><body>fallback</body></html>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/sessions/123", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}
