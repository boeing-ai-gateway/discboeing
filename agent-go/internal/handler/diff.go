package handler

import (
	"net/http"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/files"
	"github.com/obot-platform/discobot/agent-go/internal/gitops"
)

// GetDiff handles GET /diff — returns workspace diff.
// Query params:
//   - path: optional, single file diff
//   - format: optional, "full" (default) or "files"
//   - target: optional commit/ref to diff against (defaults to HEAD)
func (h *Handler) GetDiff(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	format := r.URL.Query().Get("format")
	target := r.URL.Query().Get("target")

	// Validate single file path if provided
	if path != "" {
		if _, err := files.ValidatePath(path, h.agentCwd); err != nil {
			h.Error(w, http.StatusBadRequest, "Invalid path")
			return
		}
	}

	diff, err := gitops.GetDiff(h.agentCwd, path, target)
	if err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Handle single file request
	if path != "" {
		var file *gitops.FileDiffEntry
		for i := range diff.Files {
			if diff.Files[i].Path == path {
				file = &diff.Files[i]
				break
			}
		}

		if file == nil {
			h.JSON(w, http.StatusOK, api.SingleFileDiffResponse{
				Path:   path,
				Status: "unchanged",
				Patch:  "",
			})
			return
		}

		h.JSON(w, http.StatusOK, api.SingleFileDiffResponse{
			Path:      file.Path,
			Status:    file.Status,
			OldPath:   file.OldPath,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Binary:    file.Binary,
			Patch:     file.Patch,
		})
		return
	}

	// Handle format=files — return file entries with status only
	if format == "files" {
		fileEntries := make([]api.DiffFileEntry, len(diff.Files))
		for i, f := range diff.Files {
			fileEntries[i] = api.DiffFileEntry{
				Path:    f.Path,
				Status:  f.Status,
				OldPath: f.OldPath,
			}
		}

		h.JSON(w, http.StatusOK, api.DiffFilesResponse{
			Files: fileEntries,
			Stats: diff.Stats,
		})
		return
	}

	h.JSON(w, http.StatusOK, api.DiffResponse(diff))
}
