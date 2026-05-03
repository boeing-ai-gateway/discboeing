package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/meta/internal/handlers"
	metastatic "github.com/obot-platform/discobot/meta/static"
)

// Route defines an HTTP route with its handler and metadata.
type Route struct {
	Method      string
	Pattern     string
	OperationID string
	Handler     http.HandlerFunc
	Meta        Meta
}

// Meta contains route documentation and metadata.
type Meta struct {
	Group       string  `json:"group"`
	Description string  `json:"description"`
	Params      []Param `json:"params,omitempty"`
	Body        any     `json:"body,omitempty"`
}

// Param describes a route parameter.
type Param struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required,omitempty"`
	Example  string `json:"example,omitempty"`
}

// RouteInfo is the JSON output format for route documentation UIs.
type RouteInfo struct {
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	OperationID string  `json:"operationId,omitempty"`
	Group       string  `json:"group"`
	Description string  `json:"description"`
	Params      []Param `json:"params,omitempty"`
	Body        any     `json:"body,omitempty"`
}

type contextKey string

const requestRouteInfoKey contextKey = "requestRouteInfo"

// RequestRouteInfo is generated-route metadata for the current HTTP request.
//
// RegisterGenerated populates this context value after chi matches a route and
// before handler wrappers, such as authorization, run.
type RequestRouteInfo struct {
	Method      string
	Pattern     string
	OperationID string
	PathParams  map[string]string
	QueryParams url.Values
}

// WithRequestRouteInfo stores generated route metadata on a context.
func WithRequestRouteInfo(ctx context.Context, info RequestRouteInfo) context.Context {
	return context.WithValue(ctx, requestRouteInfoKey, info)
}

// RequestRouteInfoFromContext returns generated route metadata from a context.
func RequestRouteInfoFromContext(ctx context.Context) (RequestRouteInfo, bool) {
	info, ok := ctx.Value(requestRouteInfoKey).(RequestRouteInfo)
	return info, ok
}

// RequestRouteInfoFromRequest returns generated route metadata from a request.
func RequestRouteInfoFromRequest(r *http.Request) (RequestRouteInfo, bool) {
	return RequestRouteInfoFromContext(r.Context())
}

// Registry stores route metadata for documentation.
type Registry struct {
	mu     sync.RWMutex
	routes *[]RouteInfo
	prefix string
}

// NewRegistry creates a new route registry.
func NewRegistry() *Registry {
	routes := make([]RouteInfo, 0)
	return &Registry{routes: &routes}
}

var pathParamRegex = regexp.MustCompile(`\{([^}]+)\}`)

// Register adds a route to chi and stores its metadata.
func (reg *Registry) Register(r chi.Router, route Route) {
	handler := withRequestRouteInfo(route, route.Handler)
	switch route.Method {
	case http.MethodGet:
		r.Get(route.Pattern, handler)
	case http.MethodPost:
		r.Post(route.Pattern, handler)
	case http.MethodPut:
		r.Put(route.Pattern, handler)
	case http.MethodDelete:
		r.Delete(route.Pattern, handler)
	case http.MethodPatch:
		r.Patch(route.Pattern, handler)
	case http.MethodOptions:
		r.Options(route.Pattern, handler)
	case http.MethodHead:
		r.Head(route.Pattern, handler)
	}

	fullPath := reg.prefix + route.Pattern
	params := mergeParams(extractPathParams(fullPath), route.Meta.Params)

	reg.mu.Lock()
	*reg.routes = append(*reg.routes, RouteInfo{
		Method:      route.Method,
		Path:        fullPath,
		OperationID: route.OperationID,
		Group:       route.Meta.Group,
		Description: route.Meta.Description,
		Params:      params,
		Body:        route.Meta.Body,
	})
	reg.mu.Unlock()
}

func withRequestRouteInfo(route Route, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		info := RequestRouteInfo{
			Method:      route.Method,
			Pattern:     route.Pattern,
			OperationID: route.OperationID,
			PathParams:  pathParamValues(r, route.Pattern),
			QueryParams: queryParamValues(r),
		}
		next.ServeHTTP(w, r.WithContext(WithRequestRouteInfo(r.Context(), info)))
	}
}

func pathParamValues(r *http.Request, pattern string) map[string]string {
	params := map[string]string{}
	for _, param := range extractPathParams(pattern) {
		if value := chi.URLParam(r, param.Name); value != "" {
			params[param.Name] = value
		}
	}
	return params
}

func queryParamValues(r *http.Request) url.Values {
	values := url.Values{}
	for name, got := range r.URL.Query() {
		values[name] = append([]string(nil), got...)
	}
	return values
}

// WithPrefix returns a new registry that shares storage but adds a prefix.
func (reg *Registry) WithPrefix(pattern string) *Registry {
	return &Registry{prefix: reg.prefix + pattern, routes: reg.routes}
}

// Routes returns all registered route metadata.
func (reg *Registry) Routes() []RouteInfo {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	result := make([]RouteInfo, len(*reg.routes))
	copy(result, *reg.routes)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Method < result[j].Method
		}
		return result[i].Path < result[j].Path
	})
	return result
}

func extractPathParams(pattern string) []Param {
	matches := pathParamRegex.FindAllStringSubmatch(pattern, -1)
	params := make([]Param, 0, len(matches))
	for _, match := range matches {
		params = append(params, Param{Name: match[1], In: "path", Required: true})
	}
	return params
}

func mergeParams(pathParams, metaParams []Param) []Param {
	if len(metaParams) == 0 {
		return pathParams
	}
	metaMap := make(map[string]Param, len(metaParams))
	for _, p := range metaParams {
		metaMap[p.Name] = p
	}
	result := make([]Param, 0, len(pathParams)+len(metaParams))
	seen := make(map[string]bool, len(pathParams))
	for _, p := range pathParams {
		if override, ok := metaMap[p.Name]; ok {
			override.In = "path"
			override.Required = true
			result = append(result, override)
		} else {
			result = append(result, p)
		}
		seen[p.Name] = true
	}
	for _, p := range metaParams {
		if !seen[p.Name] {
			result = append(result, p)
		}
	}
	return result
}

// HandlerWrapper wraps a generated route handler before it is registered.
type HandlerWrapper func(Route, http.HandlerFunc) http.HandlerFunc

// RegisterGenerated registers all generated OpenAPI routes with handlers.
func RegisterGenerated(r chi.Router, h *handlers.Handlers) *Registry {
	return RegisterGeneratedWithWrapper(r, h, nil)
}

// RegisterGeneratedWithWrapper registers generated routes with an optional wrapper.
func RegisterGeneratedWithWrapper(r chi.Router, h *handlers.Handlers, wrapper HandlerWrapper) *Registry {
	reg := NewRegistry()
	for _, route := range GeneratedRoutes(h) {
		handler := route.Handler
		if wrapper != nil {
			handler = wrapper(route, handler)
		}
		route.Handler = handler
		reg.Register(r, route)
	}
	RegisterDocumentationRoutes(r, reg)
	return reg
}

// RegisterDocumentationRoutes registers API docs, OpenAPI, and OpenAPI UI routes.
func RegisterDocumentationRoutes(r chi.Router, reg *Registry) {
	r.Get("/api/routes", RoutesHandler(reg))
	r.Get("/api-ui", APIUIHandler())
	r.Get("/api-ui/", APIUIHandler())
	r.Get("/openapi.yaml", OpenAPIYAMLHandler())
	r.Get("/\"/openapi.yaml\"", OpenAPIYAMLHandler())
	r.Get("/docs", OpenAPIUIHandler())
	r.Get("/docs/", OpenAPIUIHandler())
	r.Get("/swagger", redirectHandler("/docs"))
	r.Get("/swagger/", redirectHandler("/docs"))
}

// RoutesHandler returns route metadata in the same shape used by the existing API UI.
func RoutesHandler(reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reg.Routes())
	}
}

// OpenAPIYAMLHandler serves the generated OpenAPI document.
func OpenAPIYAMLHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write([]byte(OpenAPIYAML()))
	}
}

// APIUIHandler serves the shared Discobot route explorer backed by /api/routes.
func APIUIHandler() http.HandlerFunc {
	data, err := metastatic.Files.ReadFile("api-ui.html")
	if err != nil {
		panic(fmt.Errorf("read api UI asset: %w", err))
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}

// OpenAPIUIHandler serves a standard OpenAPI UI page backed by /openapi.yaml.
func OpenAPIUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(openAPIUIHTML))
	}
}

func redirectHandler(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}

const openAPIUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Discobot Meta API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: window.location.origin + '/openapi.yaml',
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis],
      layout: 'BaseLayout'
    });
  </script>
</body>
</html>`

func bodyExample(method string) any {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return map[string]any{}
	default:
		return nil
	}
}

func routeError(operationID string, err error) error {
	return fmt.Errorf("route %s: %w", operationID, err)
}
