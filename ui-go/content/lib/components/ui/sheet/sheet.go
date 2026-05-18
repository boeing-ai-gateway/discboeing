package sheet

import "strings"

func joinClass(parts ...string) string {
	classes := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			classes = append(classes, trimmed)
		}
	}
	return strings.Join(classes, " ")
}

func sheetState(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}

func sheetSide(side string) string {
	switch side {
	case "top", "bottom", "left", "right":
		return side
	default:
		return "right"
	}
}

func overlayClass(className string) string {
	return joinClass("data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-black/50", className)
}

func contentClass(side string, className string) string {
	base := "bg-background data-[state=open]:animate-in data-[state=closed]:animate-out fixed z-50 flex flex-col gap-4 shadow-lg transition ease-in-out data-[state=closed]:duration-300 data-[state=open]:duration-500"
	switch sheetSide(side) {
	case "top":
		base += " data-[state=closed]:slide-out-to-top data-[state=open]:slide-in-from-top inset-x-0 top-0 h-auto border-b"
	case "bottom":
		base += " data-[state=closed]:slide-out-to-bottom data-[state=open]:slide-in-from-bottom inset-x-0 bottom-0 h-auto border-t"
	case "left":
		base += " data-[state=closed]:slide-out-to-start data-[state=open]:slide-in-from-start inset-y-0 start-0 h-full w-3/4 border-e sm:max-w-sm"
	default:
		base += " data-[state=closed]:slide-out-to-end data-[state=open]:slide-in-from-end inset-y-0 end-0 h-full w-3/4 border-s sm:max-w-sm"
	}
	return joinClass(base, className)
}

func closeClass(className string) string {
	return joinClass("ring-offset-background focus-visible:ring-ring absolute end-4 top-4 rounded-xs opacity-70 transition-opacity hover:opacity-100 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-hidden disabled:pointer-events-none", className)
}

func descriptionClass(className string) string {
	return joinClass("text-muted-foreground text-sm", className)
}

func footerClass(className string) string {
	return joinClass("mt-auto flex flex-col gap-2 p-4", className)
}

func headerClass(className string) string {
	return joinClass("flex flex-col gap-1.5 p-4", className)
}

func titleClass(className string) string {
	return joinClass("text-foreground font-semibold", className)
}
