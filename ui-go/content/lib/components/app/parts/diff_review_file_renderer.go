package parts

import (
	"encoding/json"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

type diffReviewPayload struct {
	Path        string `json:"path"`
	OldPath     string `json:"oldPath,omitempty"`
	Patch       string `json:"patch"`
	CommitHash  string `json:"commitHash,omitempty"`
	DiffStyle   string `json:"diffStyle,omitempty"`
	Virtualized bool   `json:"virtualized"`
}

func diffReviewPayloadJSON(file viewmodel.DiffReviewFileSnapshot) string {
	data, err := json.Marshal(diffReviewPayload{
		Path:        file.Path,
		OldPath:     file.OldPath,
		Patch:       file.Patch,
		CommitHash:  file.CommitHash,
		DiffStyle:   diffReviewResolvedStyle(file),
		Virtualized: file.Virtualized,
	})
	if err != nil {
		return "{}"
	}
	return string(data)
}

func diffReviewPatchLines(patch string) []string {
	patch = strings.TrimSpace(strings.ReplaceAll(patch, "\r\n", "\n"))
	if patch == "" {
		return nil
	}
	return strings.Split(patch, "\n")
}

func diffReviewPatchLineClass(line string) string {
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

func diffReviewRendererHostClass(file viewmodel.DiffReviewFileSnapshot) string {
	if file.Virtualized {
		return "hidden max-h-[70vh] overflow-auto p-3"
	}

	return "hidden p-3"
}

func diffReviewResolvedStyle(file viewmodel.DiffReviewFileSnapshot) string {
	if file.DiffStyle == "split" {
		return "split"
	}

	return "unified"
}

func diffReviewRendererKind(file viewmodel.DiffReviewFileSnapshot) string {
	if file.Virtualized {
		return "virtualized"
	}

	return "standard"
}
