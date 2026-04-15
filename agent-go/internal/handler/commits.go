package handler

import (
	"net/http"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/gitops"
)

// GetCommits handles GET /commits — returns format-patch output for changes
// relative to a target commit.
// Query params:
//   - target: required, the target commit hash
func (h *Handler) GetCommits(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		h.JSON(w, http.StatusBadRequest, api.CommitsErrorResponse{
			Error:   "invalid_target",
			Message: "target query parameter is required",
		})
		return
	}

	result, commitsErr := gitops.GetCommitPatches(h.agentCwd, target)
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
