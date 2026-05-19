package toggle

import "github.com/obot-platform/discobot/ui-go/content/lib/classnames"

func state(pressed bool) string {
	if pressed {
		return "on"
	}
	return "off"
}

func toggleClass(variant string, size string, className string) string {
	base := "hover:bg-muted hover:text-muted-foreground data-[state=on]:bg-accent data-[state=on]:text-accent-foreground focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive inline-flex items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap transition-[color,box-shadow] outline-none focus-visible:ring-[3px] disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
	if variant == "outline" {
		base += " border-input hover:bg-accent hover:text-accent-foreground border bg-transparent shadow-xs"
	} else {
		base += " bg-transparent"
	}
	switch size {
	case "sm":
		base += " h-8 min-w-8 px-1.5"
	case "lg":
		base += " h-10 min-w-10 px-2.5"
	default:
		base += " h-9 min-w-9 px-2"
	}
	return classnames.CN(base, className)
}
