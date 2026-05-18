package parts

import (
	"fmt"
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func diffReviewFiles(snapshot viewmodel.DockPanelSnapshot) []viewmodel.DiffReviewFileSnapshot {
	if len(snapshot.DiffReview.Files) > 0 {
		return snapshot.DiffReview.Files
	}

	return nil
}

func diffReviewFileCount(snapshot viewmodel.DockPanelSnapshot) int {
	if len(snapshot.DiffReview.Files) > 0 {
		return len(snapshot.DiffReview.Files)
	}

	return snapshot.DiffFileCount
}

func diffReviewFileCountLabel(count int) string {
	if count == 1 {
		return "1 changed file"
	}

	return fmt.Sprintf("%d changed files", count)
}

func diffReviewTargetLabel(target string) string {
	switch target {
	case "":
		return "Merge base"
	case "HEAD":
		return "HEAD"
	default:
		return target
	}
}

func diffReviewStyle(snapshot viewmodel.DiffReviewPanelSnapshot) string {
	if snapshot.DiffStyle == "split" {
		return "split"
	}

	return "unified"
}

func diffReviewStyleButtonClass(active bool, side string) string {
	className := "h-8 px-3 text-sm "
	switch side {
	case "left":
		className += "rounded-r-none "
	case "right":
		className += "rounded-l-none border-l border-border "
	}
	if active {
		return className + "bg-secondary text-secondary-foreground"
	}

	return className + "hover:bg-accent hover:text-accent-foreground"
}

func diffReviewStatusLabel(status string) string {
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

func diffReviewStatusBadgeClass(status string) string {
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

func diffReviewFileLineLabel(file viewmodel.DiffReviewFileSnapshot) string {
	if file.LineCount > 0 {
		return strconv.Itoa(file.LineCount) + " diff lines"
	}

	return ""
}

func diffReviewCommentStyle(comment viewmodel.DiffReviewSelectionCommentSnapshot) string {
	return "top: " + strconv.Itoa(comment.Top) + "px; left: " + strconv.Itoa(comment.Left) + "px;"
}

func diffReviewCommentSubmitLabel(submitting bool) string {
	if submitting {
		return "Submitting…"
	}

	return "Submit"
}

func diffReviewMaximizeTitle(snapshot viewmodel.DockPanelSnapshot) string {
	if snapshot.DockMaximized {
		return "Restore split view"
	}

	return "Maximize diff review panel"
}

func diffReviewApproveLabel(file viewmodel.DiffReviewFileSnapshot) string {
	if file.Approved {
		return "Mark " + file.Path + " not approved"
	}

	return "Mark " + file.Path + " approved"
}
