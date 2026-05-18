package parts

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func filesPanelMaximizeTitle(maximized bool) string {
	if maximized {
		return "Restore split view"
	}
	return "Maximize files panel"
}

func filesPanelDirtyDotClass(dirty bool) string {
	base := "size-2 shrink-0 rounded-full"
	if dirty {
		return base + " bg-sidebar-primary"
	}
	return base + " bg-sidebar-foreground/30"
}

func filesPanelDirtyTitle(dirty bool) string {
	if dirty {
		return "Unsaved changes"
	}
	return "No unsaved changes"
}

func filesPanelSwitchClass(checked bool) string {
	base := "relative inline-flex h-5 w-9 shrink-0 rounded-full border border-transparent transition-colors disabled:pointer-events-none disabled:opacity-50"
	if checked {
		return base + " bg-sidebar-primary"
	}
	return base + " bg-input"
}

func filesPanelSwitchThumbClass(checked bool) string {
	base := "pointer-events-none block size-4 rounded-full bg-background shadow transition-transform"
	if checked {
		return base + " translate-x-4"
	}
	return base + " translate-x-0"
}

func filesPanelActiveTitle(snapshot viewmodel.FilesPanelSnapshot) string {
	if snapshot.ActivePath != "" {
		return snapshot.ActivePath
	}
	return "Files panel"
}

func filesPanelDirty(snapshot viewmodel.FilesPanelSnapshot) bool {
	return snapshot.ActivePath != "" && snapshot.ActiveBuffer.Dirty
}

func filesPanelFileLabel(path string) string {
	if path == "" {
		return ""
	}
	label := filepath.Base(path)
	if label == "." || label == string(filepath.Separator) {
		return path
	}
	return label
}

func filesPanelStatusLetter(status string) string {
	switch status {
	case "added":
		return "A"
	case "modified":
		return "M"
	case "deleted":
		return "D"
	case "renamed":
		return "R"
	default:
		return ""
	}
}

func filesPanelStatusBadgeClass(status string) string {
	switch status {
	case "added":
		return "border-green-500/40 text-green-500"
	case "modified":
		return "border-yellow-500/40 text-yellow-500"
	case "deleted":
		return "border-red-500/40 text-red-500"
	case "renamed":
		return "border-purple-500/40 text-purple-500"
	default:
		return "border-border text-muted-foreground"
	}
}

func filesPanelTabClass(active bool) string {
	base := "flex shrink-0 items-center gap-2 rounded-md border px-3 py-1.5 text-sm transition"
	if active {
		return base + " border-sidebar-border bg-background text-foreground shadow-sm"
	}
	return base + " border-transparent bg-sidebar-accent/60 text-sidebar-foreground/75 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
}

func filesPanelTreeButtonClass(node viewmodel.FilesPanelNode, selected bool) string {
	base := "flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm text-sidebar-foreground/80 transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
	if selected {
		base += " bg-sidebar-accent text-sidebar-accent-foreground shadow-inner"
	}
	if node.Status == "deleted" {
		base += " text-sidebar-foreground/40 line-through"
	}
	return base
}

func filesPanelTreeIconClass(node viewmodel.FilesPanelNode) string {
	base := "size-4 shrink-0"
	if node.Type == "directory" {
		if node.Changed {
			return base + " text-yellow-500"
		}
		return base + " text-sidebar-foreground/55"
	}
	switch node.Status {
	case "added":
		return base + " text-green-500"
	case "modified":
		return base + " text-yellow-500"
	case "deleted":
		return base + " text-red-500"
	case "renamed":
		return base + " text-purple-500"
	default:
		return base + " text-sidebar-foreground/55"
	}
}

func filesPanelTreePadding(depth int) string {
	return "padding-left: " + strconv.Itoa(8+depth*14) + "px"
}

func filesPanelActiveStatus(snapshot viewmodel.FilesPanelSnapshot) string {
	for _, tab := range snapshot.OpenTabs {
		if tab.Path == snapshot.ActivePath {
			return tab.Status
		}
	}
	return ""
}

func filesPanelCanRenderText(snapshot viewmodel.FilesPanelSnapshot) bool {
	return snapshot.ActivePath != "" && snapshot.ActiveBuffer.Encoding == "utf8"
}

func filesPanelCanEdit(snapshot viewmodel.FilesPanelSnapshot) bool {
	return filesPanelCanRenderText(snapshot) && !snapshot.ActiveBuffer.FromBase
}

func filesPanelIsMarkdown(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".mdx"
}

func filesPanelIsImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".bmp", ".avif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func filesPanelIsPDF(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".pdf"
}

func filesPanelImageMimeType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".bmp":
		return "image/bmp"
	case ".avif":
		return "image/avif"
	case ".tif", ".tiff":
		return "image/tiff"
	default:
		return "image/png"
	}
}

func filesPanelDataURL(mimeType string, content string) string {
	if content == "" {
		return ""
	}
	return "data:" + mimeType + ";base64," + content
}

func filesPanelActiveBufferLoaded(snapshot viewmodel.FilesPanelSnapshot) bool {
	return snapshot.ActivePath != "" && snapshot.ActiveBuffer.Encoding != ""
}
