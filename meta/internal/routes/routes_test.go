package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/meta/internal/handlers"
)

func TestOpenAPIYAMLValidates(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(OpenAPIYAML()))
	if err != nil {
		t.Fatalf("load OpenAPI: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI: %v", err)
	}
}

func TestGeneratedRoutesFromOpenAPI(t *testing.T) {
	routes := GeneratedRoutes(handlers.New(handlers.Options{}))
	if len(routes) == 0 {
		t.Fatal("expected generated routes")
	}
	var foundToken bool
	for _, route := range routes {
		if route.OperationID == "token" && route.Method == http.MethodPost && route.Pattern == "/token" {
			foundToken = true
		}
	}
	if !foundToken {
		t.Fatal("expected POST /token route")
	}
}

func TestRegisterGeneratedRegistersDocs(t *testing.T) {
	r := chi.NewRouter()
	reg := RegisterGenerated(r, handlers.New(handlers.Options{}))
	if len(reg.Routes()) == 0 {
		t.Fatal("expected route metadata")
	}

	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("openapi status = %d", w.Code)
	}
	if body := w.Body.String(); body == "" || body[:7] != "openapi" {
		t.Fatalf("unexpected openapi body prefix: %q", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/%22/openapi.yaml%22", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("quoted openapi compatibility status = %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/docs", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("docs status = %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/swagger", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("swagger redirect status = %d", w.Code)
	}
}

func TestRegisterGeneratedStoresRequestRouteInfo(t *testing.T) {
	r := chi.NewRouter()
	NewRegistry().Register(r, Route{
		Method:      http.MethodGet,
		Pattern:     "/v1/projects/{projectId}",
		OperationID: "getProject",
		Handler: func(w http.ResponseWriter, r *http.Request) {
			info, ok := RequestRouteInfoFromRequest(r)
			if !ok {
				t.Fatal("missing request route info")
			}
			if info.OperationID != "getProject" || info.Pattern != "/v1/projects/{projectId}" {
				t.Fatalf("unexpected route info: %#v", info)
			}
			if info.PathParams["projectId"] != "prj_123" {
				t.Fatalf("projectId = %q", info.PathParams["projectId"])
			}
			if got := info.QueryParams.Get("scope"); got != "agent.chat" {
				t.Fatalf("scope = %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/projects/prj_123?scope=agent.chat", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d", w.Code)
	}
}
