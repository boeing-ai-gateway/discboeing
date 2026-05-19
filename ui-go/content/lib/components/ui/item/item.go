package item

import "github.com/obot-platform/discobot/ui-go/content/lib/classnames"

func itemVariant(variant string) string {
	switch variant {
	case "outline", "muted":
		return variant
	default:
		return "default"
	}
}

func itemSize(size string) string {
	if size == "sm" {
		return "sm"
	}
	return "default"
}

func itemClass(variant string, size string, className string) string {
	base := "group/item [a]:hover:bg-accent/50 focus-visible:border-ring focus-visible:ring-ring/50 flex flex-wrap items-center rounded-md border border-transparent text-sm transition-colors duration-100 outline-none focus-visible:ring-[3px] [a]:transition-colors"
	switch itemVariant(variant) {
	case "outline":
		base += " border-border"
	case "muted":
		base += " bg-muted/50"
	default:
		base += " bg-transparent"
	}
	if itemSize(size) == "sm" {
		base += " gap-2.5 px-4 py-3"
	} else {
		base += " gap-4 p-4"
	}
	return classnames.CN(base, className)
}

func mediaVariant(variant string) string {
	switch variant {
	case "icon", "image":
		return variant
	default:
		return "default"
	}
}

func mediaClass(variant string, className string) string {
	base := "flex shrink-0 items-center justify-center gap-2 group-has-[[data-slot=item-description]]/item:translate-y-0.5 group-has-[[data-slot=item-description]]/item:self-start [&_svg]:pointer-events-none"
	switch mediaVariant(variant) {
	case "icon":
		base += " bg-muted size-8 rounded-sm border [&_svg:not([class*='size-'])]:size-4"
	case "image":
		base += " size-10 overflow-hidden rounded-sm [&_img]:size-full [&_img]:object-cover"
	default:
		base += " bg-transparent"
	}
	return classnames.CN(base, className)
}
