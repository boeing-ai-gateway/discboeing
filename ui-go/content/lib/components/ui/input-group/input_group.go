package inputgroup

import (
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/classnames"
)

func groupClass(className string) string {
	return classnames.CN("group/input-group border-input dark:bg-input/30 relative flex w-full items-center rounded-md border shadow-xs transition-[color,box-shadow] outline-none h-9 has-[>textarea]:h-auto has-[>[data-align=inline-start]]:[&>input]:ps-2 has-[>[data-align=inline-end]]:[&>input]:pe-2 has-[>[data-align=block-start]]:h-auto has-[>[data-align=block-start]]:flex-col has-[>[data-align=block-start]]:[&>input]:pb-3 has-[>[data-align=block-end]]:h-auto has-[>[data-align=block-end]]:flex-col has-[>[data-align=block-end]]:[&>input]:pt-3 has-[[data-slot=input-group-control]:focus-visible]:border-ring has-[[data-slot=input-group-control]:focus-visible]:ring-ring/50 has-[[data-slot=input-group-control]:focus-visible]:ring-[3px] has-[[data-slot][aria-invalid=true]]:ring-destructive/20 has-[[data-slot][aria-invalid=true]]:border-destructive dark:has-[[data-slot][aria-invalid=true]]:ring-destructive/40", className)
}

func addonAlign(align string) string {
	switch align {
	case "inline-end", "block-start", "block-end":
		return align
	default:
		return "inline-start"
	}
}

func addonClass(align string, className string) string {
	base := "text-muted-foreground flex h-auto cursor-text items-center justify-center gap-2 py-1.5 text-sm font-medium select-none group-data-[disabled=true]/input-group:opacity-50 [&>kbd]:rounded-[calc(var(--radius)-5px)] [&>svg:not([class*='size-'])]:size-4"
	switch addonAlign(align) {
	case "inline-end":
		base += " order-last pe-3 has-[>button]:me-[-0.45rem] has-[>kbd]:me-[-0.35rem]"
	case "block-start":
		base += " order-first w-full justify-start px-3 pt-3 group-has-[>input]/input-group:pt-2.5 [.border-b]:pb-3"
	case "block-end":
		base += " order-last w-full justify-start px-3 pb-3 group-has-[>input]/input-group:pb-2.5 [.border-t]:pt-3"
	default:
		base += " order-first ps-3 has-[>button]:ms-[-0.45rem] has-[>kbd]:ms-[-0.35rem]"
	}
	return classnames.CN(base, className)
}

func buttonSize(size string) string {
	switch size {
	case "sm", "icon-xs", "icon-sm":
		return size
	default:
		return "xs"
	}
}

func buttonClass(size string, className string) string {
	base := "flex items-center gap-2 text-sm shadow-none"
	switch buttonSize(size) {
	case "sm":
		base += " h-8 gap-1.5 rounded-md px-2.5 has-[>svg]:px-2.5"
	case "icon-xs":
		base += " size-6 rounded-[calc(var(--radius)-5px)] p-0 has-[>svg]:p-0"
	case "icon-sm":
		base += " size-8 p-0 has-[>svg]:p-0"
	default:
		base += " h-6 gap-1 rounded-[calc(var(--radius)-5px)] px-2 has-[>svg]:px-2 [&>svg:not([class*='size-'])]:size-3.5"
	}
	return classnames.CN(base, className)
}

func inputClass(className string) string {
	return classnames.CN("flex-1 rounded-none border-0 bg-transparent shadow-none focus-visible:ring-0 dark:bg-transparent", className)
}

func textClass(className string) string {
	return classnames.CN("text-muted-foreground flex items-center gap-2 text-sm [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4", className)
}

func textareaClass(className string) string {
	return classnames.CN("flex-1 resize-none rounded-none border-0 bg-transparent py-3 shadow-none focus-visible:ring-0 dark:bg-transparent", className)
}

func inputType(inputType string) string {
	if strings.TrimSpace(inputType) == "" {
		return "text"
	}
	return inputType
}

func buttonType(buttonType string) string {
	if strings.TrimSpace(buttonType) == "" {
		return "button"
	}
	return buttonType
}
