package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/gitops"
)

// GetCommits handles GET /commits — returns format-patch output for changes
// relative to a target commit.
// Query params:
//   - target: optional, the base commit hash
//   - head: optional, the tip commit hash to export instead of the current HEAD
//   - cwd: optional, the git working directory to read from
func (h *Handler) GetCommits(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	head := r.URL.Query().Get("head")
	workDir := h.agentCwd
	if cwd := strings.TrimSpace(r.URL.Query().Get("cwd")); cwd != "" {
		if filepath.IsAbs(cwd) {
			workDir = filepath.Clean(cwd)
		} else {
			workDir = filepath.Clean(filepath.Join(h.agentCwd, cwd))
		}
	}

	result, commitsErr := gitops.GetCommitPatchesAtHead(workDir, target, head)
	if commitsErr != nil {
		status := http.StatusInternalServerError
		switch commitsErr.Code {
		case "invalid_target":
			status = http.StatusBadRequest
		case "not_git_repo":
			status = http.StatusBadRequest
		case "no_commits":
			status = http.StatusNotFound
		}

		h.JSON(w, status, api.CommitsErrorResponse{
			Error:      commitsErr.Code,
			Message:    commitsErr.Message,
			IsClean:    commitsErr.IsClean,
			HeadCommit: commitsErr.HeadCommit,
		})
		return
	}

	h.JSON(w, http.StatusOK, api.CommitsResponse{
		Patches:     result.Patches,
		CommitCount: result.CommitCount,
		HeadCommit:  result.HeadCommit,
	})
}
