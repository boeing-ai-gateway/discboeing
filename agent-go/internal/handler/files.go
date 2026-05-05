package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/files"
)

var (
	errArtifactURIRequired = errors.New("artifact URI query parameter required")
	errInvalidArtifactURI  = errors.New("invalid artifact URI")
)

// ListFiles handles GET /files — lists directory contents.
func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}
	hidden := r.URL.Query().Get("hidden") == "true"

	result, fileErr := files.ListDirectory(path, h.agentCwd, hidden)
	if fileErr != nil {
		h.Error(w, fileErr.Status, fileErr.Message)
		return
	}

	h.JSON(w, http.StatusOK, api.ListFilesResponse{
		Path:    result.Path,
		Entries: toAPIFileEntries(result.Entries),
	})
}

// SearchFiles handles GET /files/search — fuzzy search files in workspace.
func (h *Handler) SearchFiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	result, fileErr := files.SearchFiles(query, h.agentCwd, limit)
	if fileErr != nil {
		h.Error(w, fileErr.Status, fileErr.Message)
		return
	}

	h.JSON(w, http.StatusOK, api.SearchFilesResponse{
		Query:   result.Query,
		Results: toAPISearchEntries(result.Results),
	})
}

// ReadFile handles GET /files/read — reads file content.
func (h *Handler) ReadFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		h.Error(w, http.StatusBadRequest, "path query parameter required")
		return
	}

	result, fileErr := files.ReadFile(path, h.agentCwd)
	if fileErr != nil {
		h.Error(w, fileErr.Status, fileErr.Message)
		return
	}

	h.JSON(w, http.StatusOK, api.ReadFileResponse{
		Path:     result.Path,
		Content:  result.Content,
		Encoding: result.Encoding,
		Size:     result.Size,
	})
}

// ReadThreadArtifact handles GET /threads/{id}/artifacts/read — reads a
// thread-local artifact via its artifacts:// URI.
func (h *Handler) ReadThreadArtifact(w http.ResponseWriter, r *http.Request) {
	threadID := strings.TrimSpace(chi.URLParam(r, "id"))
	if threadID == "" {
		h.Error(w, http.StatusBadRequest, "thread ID is required")
		return
	}
	if h.browserManager == nil {
		h.Error(w, http.StatusServiceUnavailable, "browser manager unavailable")
		return
	}

	artifactPath, err := parseThreadArtifactURI(r.URL.Query().Get("uri"))
	if err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	result, fileErr := h.browserManager.ReadThreadArtifact(threadID, artifactPath)
	if fileErr != nil {
		h.Error(w, fileErr.Status, fileErr.Message)
		return
	}

	h.JSON(w, http.StatusOK, api.ReadFileResponse{
		Path:     result.Path,
		Content:  result.Content,
		Encoding: result.Encoding,
		Size:     result.Size,
	})
}

func parseThreadArtifactURI(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errArtifactURIRequired
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errInvalidArtifactURI
	}
	if parsed.Scheme != "artifacts" {
		return "", errInvalidArtifactURI
	}

	path := strings.TrimSpace(strings.TrimPrefix(filepathFromArtifactURI(parsed), "/"))
	if path == "" {
		return "", errArtifactURIRequired
	}
	return path, nil
}

func filepathFromArtifactURI(parsed *url.URL) string {
	switch {
	case parsed.Host != "" && parsed.Path != "":
		return parsed.Host + parsed.Path
	case parsed.Host != "":
		return parsed.Host
	default:
		return parsed.Path
	}
}

// WriteFile handles POST /files/write — writes file content.
func (h *Handler) WriteFile(w http.ResponseWriter, r *http.Request) {
	var req api.WriteFileRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		h.Error(w, http.StatusBadRequest, "path is required")
		return
	}
	if req.Content == "" {
		h.Error(w, http.StatusBadRequest, "content is required")
		return
	}

	encoding := req.Encoding
	if encoding == "" {
		encoding = "utf8"
	}

	result, fileErr := files.WriteFile(req.Path, req.Content, encoding, h.agentCwd)
	if fileErr != nil {
		h.Error(w, fileErr.Status, fileErr.Message)
		return
	}

	h.JSON(w, http.StatusOK, api.WriteFileResponse{
		Path: result.Path,
		Size: result.Size,
	})
}

// DeleteFile handles POST /files/delete — deletes a file or directory.
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	var req api.DeleteFileRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		h.Error(w, http.StatusBadRequest, "path is required")
		return
	}

	result, fileErr := files.DeleteFile(req.Path, h.agentCwd)
	if fileErr != nil {
		h.Error(w, fileErr.Status, fileErr.Message)
		return
	}

	h.JSON(w, http.StatusOK, api.DeleteFileResponse{
		Path: result.Path,
		Type: result.Type,
	})
}

// RenameFile handles POST /files/rename — renames/moves a file or directory.
func (h *Handler) RenameFile(w http.ResponseWriter, r *http.Request) {
	var req api.RenameFileRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OldPath == "" {
		h.Error(w, http.StatusBadRequest, "oldPath is required")
		return
	}
	if req.NewPath == "" {
		h.Error(w, http.StatusBadRequest, "newPath is required")
		return
	}

	result, fileErr := files.RenameFile(req.OldPath, req.NewPath, h.agentCwd)
	if fileErr != nil {
		h.Error(w, fileErr.Status, fileErr.Message)
		return
	}

	h.JSON(w, http.StatusOK, api.RenameFileResponse{
		OldPath: result.OldPath,
		NewPath: result.NewPath,
	})
}

// toAPIFileEntries converts internal file entries to API file entries.
func toAPIFileEntries(entries []files.FileEntry) []api.FileEntry {
	result := make([]api.FileEntry, len(entries))
	for i, e := range entries {
		result[i] = api.FileEntry{
			Name: e.Name,
			Type: e.Type,
			Size: e.Size,
		}
	}
	return result
}

// toAPISearchEntries converts internal search entries to API search entries.
func toAPISearchEntries(entries []files.SearchResultEntry) []api.SearchResultEntry {
	result := make([]api.SearchResultEntry, len(entries))
	for i, e := range entries {
		result[i] = api.SearchResultEntry{
			Path:  e.Path,
			Type:  e.Type,
			Score: e.Score,
		}
	}
	return result
}
