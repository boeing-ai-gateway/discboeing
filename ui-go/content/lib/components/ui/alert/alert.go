package alert

import "strings"

func alertClass(variant string, className string) string {
	base := "relative grid w-full grid-cols-[0_1fr] items-start gap-y-0.5 rounded-lg border px-4 py-3 text-sm has-[>svg]:grid-cols-[calc(var(--spacing)*4)_1fr] has-[>svg]:gap-x-3 [&>svg]:size-4 [&>svg]:translate-y-0.5 [&>svg]:text-current"
	switch variant {
	case "destructive":
		base += " text-destructive bg-card *:data-[slot=alert-description]:text-destructive/90 [&>svg]:text-current"
	default:
		base += " bg-card text-card-foreground"
	}
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}

func alertTitleClass(className string) string {
	base := "col-start-2 line-clamp-1 min-h-4 font-medium tracking-tight"
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}

func alertDescriptionClass(className string) string {
	base := "text-muted-foreground col-start-2 grid justify-items-start gap-1 text-sm [&_p]:leading-relaxed"
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}
