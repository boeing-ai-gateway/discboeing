package button

import "strings"

func buttonClass(variant string, size string, className string) string {
	base := "focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:ring-[3px] disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
	switch variant {
	case "destructive":
		base += " bg-destructive hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40 dark:bg-destructive/60 text-white"
	case "outline":
		base += " bg-background hover:bg-accent hover:text-accent-foreground dark:bg-input/30 dark:border-input dark:hover:bg-input/50 border shadow-xs"
	case "secondary":
		base += " bg-secondary text-secondary-foreground hover:bg-secondary/80"
	case "ghost":
		base += " hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50"
	case "link":
		base += " text-primary underline-offset-4 hover:underline"
	default:
		base += " bg-primary text-primary-foreground hover:bg-primary/90"
	}
	switch size {
	case "xs":
		base += " h-6 gap-1 rounded-md px-2 text-xs has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3"
	case "sm":
		base += " h-8 gap-1.5 rounded-md px-3 has-[>svg]:px-2.5"
	case "lg":
		base += " h-10 rounded-md px-6 has-[>svg]:px-4"
	case "icon":
		base += " size-9"
	case "icon-xs":
		base += " size-6 rounded-md [&_svg:not([class*='size-'])]:size-3"
	case "icon-sm":
		base += " size-8"
	case "icon-lg":
		base += " size-10"
	default:
		base += " h-9 px-4 py-2 has-[>svg]:px-3"
	}
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}
