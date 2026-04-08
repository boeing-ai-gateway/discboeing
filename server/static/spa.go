package static

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	builtUIDir               = "ui/dist"
	fallbackUIPath           = "ui/fallback/index.html"
	immutableAssetCacheValue = "public, max-age=31536000, immutable"
	htmlCacheValue           = "no-cache"
)

var reservedPrefixes = []string{"/api", "/auth", "/debug", "/health"}

type spaHandler struct {
	builtFS      fs.FS
	builtHandler http.Handler
	fallbackHTML []byte
	injectedHTML []byte
}

func NewSPAHandler() (http.Handler, error) {
	var builtFS fs.FS
	if sub, err := fs.Sub(Files, builtUIDir); err == nil {
		builtFS = sub
	}

	fallbackHTML, err := readFallbackHTML(builtFS)
	if err != nil {
		return nil, err
	}

	return newSPAHandlerWithFS(builtFS, fallbackHTML)
}

func newSPAHandlerWithFS(builtFS fs.FS, fallbackHTML []byte) (http.Handler, error) {
	var builtHandler http.Handler
	if builtFS != nil {
		builtHandler = http.FileServer(http.FS(builtFS))
	}

	injectedHTML, err := injectRuntimeConfig(fallbackHTML, map[string]string{"apiRoot": "/api"})
	if err != nil {
		return nil, err
	}

	return &spaHandler{
		builtFS:      builtFS,
		builtHandler: builtHandler,
		fallbackHTML: fallbackHTML,
		injectedHTML: injectedHTML,
	}, nil
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	requestPath := cleanRequestPath(r.URL.Path)
	if hasReservedPrefix(requestPath) {
		http.NotFound(w, r)
		return
	}

	if requestPath == "/" || requestPath == "/index.html" {
		h.serveHTML(w, r)
		return
	}

	if h.builtHandler != nil && hasBuiltAsset(h.builtFS, requestPath) {
		applyCacheHeaders(w.Header(), requestPath)
		h.builtHandler.ServeHTTP(w, r)
		return
	}
	if path.Ext(requestPath) != "" {
		http.NotFound(w, r)
		return
	}

	h.serveHTML(w, r)
}

func (h *spaHandler) serveHTML(w http.ResponseWriter, r *http.Request) {
	applyCacheHeaders(w.Header(), "/index.html")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(h.injectedHTML))
}

func readFallbackHTML(builtFS fs.FS) ([]byte, error) {
	if builtFS != nil {
		for _, name := range []string{"200.html", "index.html"} {
			if content, err := fs.ReadFile(builtFS, name); err == nil {
				return content, nil
			}
		}
	}

	return fs.ReadFile(Files, fallbackUIPath)
}

func injectRuntimeConfig(html []byte, config map[string]string) ([]byte, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	script := []byte("<script>window.__DISCOBOT_CONFIG__=" + string(configJSON) + ";</script>")
	if bytes.Contains(html, []byte("</head>")) {
		return bytes.Replace(html, []byte("</head>"), append(script, []byte("</head>")...), 1), nil
	}
	if bytes.Contains(html, []byte("</body>")) {
		return bytes.Replace(html, []byte("</body>"), append(script, []byte("</body>")...), 1), nil
	}
	return append(html, script...), nil
}

func hasBuiltAsset(builtFS fs.FS, requestPath string) bool {
	if builtFS == nil {
		return false
	}

	name := strings.TrimPrefix(requestPath, "/")
	info, err := fs.Stat(builtFS, name)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	_, err = fs.Stat(builtFS, path.Join(name, "index.html"))
	return err == nil
}

func cleanRequestPath(requestPath string) string {
	if requestPath == "" {
		return "/"
	}
	cleaned := path.Clean("/" + requestPath)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func hasReservedPrefix(requestPath string) bool {
	for _, prefix := range reservedPrefixes {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}

func applyCacheHeaders(headers http.Header, requestPath string) {
	if requestPath == "/" || strings.HasSuffix(requestPath, ".html") {
		headers.Set("Cache-Control", htmlCacheValue)
		return
	}
	if strings.HasPrefix(requestPath, "/_app/immutable/") || strings.HasPrefix(requestPath, "/assets/") {
		headers.Set("Cache-Control", immutableAssetCacheValue)
		return
	}
	headers.Del("Cache-Control")
}
