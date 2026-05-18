package toolrenderers

import (
	"encoding/json"
	"fmt"
	"strings"
)

type RequestCommitPullDiffEntry struct {
	Path          string
	OldPath       string
	Status        string
	Additions     int
	Deletions     int
	LineCount     int
	Binary        bool
	CommitHash    string
	CommitSubject string
	Patch         string
	HasParams     bool
}

type requestCommitPullDiffPayload struct {
	Path        string `json:"path"`
	OldPath     string `json:"oldPath,omitempty"`
	Patch       string `json:"patch"`
	CommitHash  string `json:"commitHash,omitempty"`
	DiffStyle   string `json:"diffStyle,omitempty"`
	Virtualized bool   `json:"virtualized"`
}

type RequestCommitPullView struct {
	Input            string
	Output           string
	ErrorText        string
	State            string
	Open             bool
	Raw              bool
	Queued           bool
	ApprovalStatus   string
	ApprovalError    string
	PreviewStatus    string
	PreviewError     string
	Question         string
	Summary          string
	CommitTitle      string
	CommitHash       string
	CommitBody       string
	CommitCount      int
	FilesChanged     int
	Additions        int
	Deletions        int
	LineCount        int
	RawPatch         string
	DiffFiles        []RequestCommitPullDiffEntry
	RejectionSummary string
}

const (
	requestCommitPullApprovedText         = "The user approved pulling the prepared sandbox commit into the host workspace."
	requestCommitPullRejectedPrefix       = "The user rejected pulling the prepared sandbox commit into the host workspace."
	requestCommitPullRejectedReasonPrefix = requestCommitPullRejectedPrefix + " Reason: "
)

func requestCommitPullStatusLabel(status string) string {
	switch status {
	case "added":
		return "Added"
	case "deleted":
		return "Deleted"
	case "renamed":
		return "Renamed"
	default:
		return "Modified"
	}
}

func requestCommitPullStatusClasses(status string) string {
	switch status {
	case "added":
		return "border-green-500/20 bg-green-500/10 text-green-700 dark:text-green-300"
	case "deleted":
		return "border-red-500/20 bg-red-500/10 text-red-700 dark:text-red-300"
	case "renamed":
		return "border-blue-500/20 bg-blue-500/10 text-blue-700 dark:text-blue-300"
	default:
		return "border-border bg-muted text-muted-foreground"
	}
}

func requestCommitPullPatchLines(patch string) []string {
	patch = normalizeNewlines(strings.TrimSpace(patch))
	if patch == "" {
		return nil
	}
	return strings.Split(patch, "\n")
}

func requestCommitPullPatchLineClass(line string) string {
	switch {
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return "bg-green-500/10 text-green-800 dark:text-green-200"
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return "bg-red-500/10 text-red-800 dark:text-red-200"
	case strings.HasPrefix(line, "@@"):
		return "bg-blue-500/10 text-blue-800 dark:text-blue-200"
	default:
		return "text-foreground"
	}
}

func requestCommitPullStatusBadgeClass(status string) string {
	switch status {
	case "added":
		return "text-green-500 border-green-500/40"
	case "modified":
		return "text-yellow-500 border-yellow-500/40"
	case "deleted":
		return "text-red-500 border-red-500/40"
	case "renamed":
		return "text-purple-500 border-purple-500/40"
	default:
		return "text-muted-foreground border-border"
	}
}

func requestCommitPullViewerStatusLabel(status string) string {
	switch status {
	case "added", "modified", "deleted", "renamed":
		return requestCommitPullStatusLabel(status)
	default:
		return "Changed"
	}
}

func requestCommitPullChangedFileLabel(count int) string {
	if count == 1 {
		return "1 changed file"
	}
	return fmt.Sprintf("%d changed files", count)
}

func requestCommitPullFileTotals(files []RequestCommitPullDiffEntry) (int, int) {
	additions := 0
	deletions := 0
	for _, file := range files {
		additions += file.Additions
		deletions += file.Deletions
	}
	return additions, deletions
}

func requestCommitPullDiffStyleButtonClass(active bool, side string) string {
	base := "h-8 px-3 text-sm "
	switch side {
	case "left":
		base += "rounded-r-none "
	case "right":
		base += "rounded-l-none border-l border-border "
	}
	if active {
		return base + "bg-secondary text-secondary-foreground"
	}
	return base + "hover:bg-accent hover:text-accent-foreground"
}

func requestCommitPullDiffPayloadJSON(file RequestCommitPullDiffEntry, diffStyle string) string {
	if diffStyle != "split" {
		diffStyle = "unified"
	}
	data, err := json.Marshal(requestCommitPullDiffPayload{
		Path:        file.Path,
		OldPath:     file.OldPath,
		Patch:       file.Patch,
		CommitHash:  file.CommitHash,
		DiffStyle:   diffStyle,
		Virtualized: false,
	})
	if err != nil {
		return "{}"
	}
	return string(data)
}

func requestCommitPullQuestion(view RequestCommitPullView) string {
	if strings.TrimSpace(view.Question) != "" {
		return strings.TrimSpace(view.Question)
	}
	return "Allow the sandbox commit to be pulled into the workspace?"
}

func requestCommitPullCommitTitle(view RequestCommitPullView) string {
	if strings.TrimSpace(view.CommitTitle) != "" {
		return strings.TrimSpace(view.CommitTitle)
	}
	return "Untitled commit"
}

func requestCommitPullShortHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

func requestCommitPullWasApproved(view RequestCommitPullView) bool {
	return strings.TrimSpace(view.Output) == requestCommitPullApprovedText || view.ApprovalStatus == "answered"
}

func requestCommitPullRejectionSummary(view RequestCommitPullView) (string, bool) {
	if strings.TrimSpace(view.RejectionSummary) != "" {
		return strings.TrimSpace(view.RejectionSummary), true
	}
	output := strings.TrimSpace(view.Output)
	if !strings.HasPrefix(output, requestCommitPullRejectedPrefix) {
		return "", false
	}
	if reason, ok := strings.CutPrefix(output, requestCommitPullRejectedReasonPrefix); ok {
		return strings.TrimSpace(reason), true
	}
	return "", true
}

func requestCommitPullCommitCountLabel(count int) string {
	if count == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", count)
}
