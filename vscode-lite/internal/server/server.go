package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/boeing-ai-gateway/discboeing/vscode-lite/internal/lsp"
	"github.com/boeing-ai-gateway/discboeing/vscode-lite/internal/vfs"
)

type Server struct {
	addr       string
	vfs        vfs.VFS
	lspManager *lsp.Manager
	staticDir  string
}

func New(addr string, workspace vfs.VFS, lspManager *lsp.Manager, staticDir string) *Server {
	return &Server{addr: addr, vfs: workspace, lspManager: lspManager, staticDir: staticDir}
}

func (s *Server) ListenAndServe() error {
	log.Printf("vscode-lite listening on %s for workspace %s", s.addr, s.vfs.Root())
	return http.ListenAndServe(s.addr, s.routes())
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/workspace", s.handleWorkspace)
	r.Get("/api/files/tree", s.handleTree)
	r.Get("/api/files/content", s.handleRead)
	r.Put("/api/files/content", s.handleWrite)
	r.Post("/api/files/rename", s.handleRename)
	r.Post("/api/files/delete", s.handleDelete)
	r.Get("/api/files/search", s.handleSearch)
	r.Get("/api/lsp/{language}", s.handleLSP)
	s.serveStatic(r)
	return r
}

func (s *Server) handleWorkspace(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"root": s.vfs.Root(), "languages": s.lspManager.Servers})
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	path := queryDefault(r, "path", ".")
	result, err := s.vfs.List(r.Context(), path, vfs.ListOptions{Hidden: r.URL.Query().Get("hidden") == "true"})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	result, err := s.vfs.Read(r.Context(), queryDefault(r, "path", "."))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}
	if err := s.vfs.Write(r.Context(), req.Path, []byte(req.Content)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := s.vfs.Rename(r.Context(), req.OldPath, req.NewPath); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := s.vfs.Delete(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	var results []vfs.FileInfo
	walk(ctxWithRequest(r), s.vfs, ".", query, limit, &results)
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleLSP(w http.ResponseWriter, r *http.Request) {
	conn, err := lsp.Accept(w, r)
	if err != nil {
		return
	}
	language := chi.URLParam(r, "language")
	if err := s.lspManager.Attach(r.Context(), language, conn); err != nil {
		log.Printf("lsp attach failed: %v", err)
		_ = conn.Close(websocket.StatusInternalError, err.Error())
	}
}

func (s *Server) serveStatic(r chi.Router) {
	if s.staticDir == "" {
		return
	}
	if _, err := fs.Stat(osDirFS(s.staticDir), "index.html"); err != nil {
		return
	}
	fileServer := http.FileServer(http.Dir(s.staticDir))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(s.staticDir, filepath.Clean(r.URL.Path))
		if info, err := fs.Stat(osDirFS(s.staticDir), strings.TrimPrefix(filepath.ToSlash(filepath.Clean(r.URL.Path)), "/")); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func queryDefault(r *http.Request, key, fallback string) string {
	if value := r.URL.Query().Get(key); value != "" {
		return value
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, vfs.ErrInvalidPath) {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func walk(ctx context.Context, filesystem vfs.VFS, path, query string, limit int, results *[]vfs.FileInfo) {
	if len(*results) >= limit {
		return
	}
	list, err := filesystem.List(ctx, path, vfs.ListOptions{})
	if err != nil {
		return
	}
	for _, entry := range list.Entries {
		if len(*results) >= limit {
			return
		}
		if strings.Contains(strings.ToLower(entry.Path), query) {
			*results = append(*results, entry)
		}
		if entry.IsDir {
			walk(ctx, filesystem, entry.Path, query, limit, results)
		}
	}
}

func ctxWithRequest(r *http.Request) context.Context { return r.Context() }

type osDirFS string

func (dir osDirFS) Open(name string) (fs.File, error) { return http.Dir(dir).Open(name) }
