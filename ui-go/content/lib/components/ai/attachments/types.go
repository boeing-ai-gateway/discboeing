package attachments

import (
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/classnames"
)

type AttachmentData struct {
	ID        string
	Type      string
	Filename  string
	Title     string
	MediaType string
	URL       string
}

type AttachmentView struct {
	Data      AttachmentData
	Variant   string
	Removable bool
}

func attachmentVariant(variant string) string {
	switch variant {
	case "inline", "list":
		return variant
	default:
		return "grid"
	}
}

func mediaCategory(data AttachmentData) string {
	if data.Type == "source-document" {
		return "source"
	}
	mediaType := data.MediaType
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return "image"
	case strings.HasPrefix(mediaType, "video/"):
		return "video"
	case strings.HasPrefix(mediaType, "audio/"):
		return "audio"
	case strings.HasPrefix(mediaType, "application/"), strings.HasPrefix(mediaType, "text/"):
		return "document"
	default:
		return "unknown"
	}
}

func attachmentLabel(data AttachmentData) string {
	if data.Type == "source-document" {
		if strings.TrimSpace(data.Title) != "" {
			return data.Title
		}
		if strings.TrimSpace(data.Filename) != "" {
			return data.Filename
		}
		return "Source"
	}
	if strings.TrimSpace(data.Filename) != "" {
		return data.Filename
	}
	if mediaCategory(data) == "image" {
		return "Image"
	}
	return "Attachment"
}

func canOpenAttachmentFullscreen(data AttachmentData) bool {
	return data.Type == "file" && mediaCategory(data) == "image" && strings.TrimSpace(data.URL) != ""
}

func attachmentClass(variant string, className string) string {
	switch attachmentVariant(variant) {
	case "inline":
		return classnames.CN("group relative flex h-8 cursor-pointer select-none items-center gap-1.5 rounded-md border border-border px-1.5 font-medium text-sm transition-all hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50", className)
	case "list":
		return classnames.CN("group relative flex w-full items-center gap-3 rounded-lg border p-3 hover:bg-accent/50", className)
	default:
		return classnames.CN("group relative size-24 overflow-hidden rounded-lg", className)
	}
}

func attachmentsClass(variant string, className string) string {
	base := "flex items-start"
	if attachmentVariant(variant) == "list" {
		base += " flex-col gap-2"
	} else {
		base += " flex-wrap gap-2"
	}
	if attachmentVariant(variant) == "grid" {
		base += " ml-auto w-fit"
	}
	return classnames.CN(base, className)
}

func previewClass(variant string, className string) string {
	switch attachmentVariant(variant) {
	case "inline":
		return classnames.CN("flex size-5 shrink-0 items-center justify-center overflow-hidden rounded bg-background", className)
	case "list":
		return classnames.CN("flex size-12 shrink-0 items-center justify-center overflow-hidden rounded bg-muted", className)
	default:
		return classnames.CN("flex size-full shrink-0 items-center justify-center overflow-hidden bg-muted", className)
	}
}

func previewIconClass(variant string) string {
	if attachmentVariant(variant) == "inline" {
		return "size-3 text-muted-foreground"
	}
	return "size-4 text-muted-foreground"
}

func previewImageClass(variant string) string {
	if attachmentVariant(variant) == "grid" {
		return "size-full object-cover"
	}
	return "size-full rounded object-cover"
}

func removeButtonClass(variant string, className string) string {
	base := "inline-flex items-center justify-center text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-ring focus-visible:ring-2 focus-visible:outline-hidden disabled:pointer-events-none disabled:opacity-50"
	switch attachmentVariant(variant) {
	case "inline":
		base += " size-5 rounded p-0 opacity-0 transition-opacity group-hover:opacity-100 [&>svg]:size-2.5"
	case "list":
		base += " size-8 shrink-0 rounded p-0 [&>svg]:size-4"
	default:
		base += " absolute top-2 right-2 size-6 rounded-full bg-background/80 p-0 opacity-0 backdrop-blur-sm transition-opacity group-hover:opacity-100 hover:bg-background [&>svg]:size-3"
	}
	return classnames.CN(base, className)
}

func removeLabel(label string) string {
	if strings.TrimSpace(label) == "" {
		return "Remove"
	}
	return label
}
