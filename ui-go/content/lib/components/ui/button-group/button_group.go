package buttongroup

import "github.com/obot-platform/discobot/ui-go/content/lib/classnames"

func buttonGroupClass(orientation string, className string) string {
	base := "flex w-fit items-stretch has-[>[data-slot=button-group]]:gap-2 [&>*]:focus-visible:relative [&>*]:focus-visible:z-10 has-[select[aria-hidden=true]:last-child]:[&>[data-slot=select-trigger]:last-of-type]:rounded-e-md [&>[data-slot=select-trigger]:not([class*='w-'])]:w-fit [&>input]:flex-1"
	if orientation == "vertical" {
		base += " flex-col [&>*:not(:first-child)]:rounded-t-none [&>*:not(:first-child)]:border-t-0 [&>*:not(:last-child)]:rounded-b-none"
	} else {
		base += " [&>*:not(:first-child)]:rounded-s-none [&>*:not(:first-child)]:border-s-0 [&>*:not(:last-child)]:rounded-e-none"
	}
	return classnames.CN(base, className)
}

func buttonGroupOrientation(orientation string) string {
	if orientation == "vertical" {
		return "vertical"
	}
	return "horizontal"
}

func buttonGroupSeparatorClass(className string) string {
	return classnames.CN("bg-input relative !m-0 self-stretch data-[orientation=vertical]:h-auto", className)
}

func buttonGroupTextClass(className string) string {
	return classnames.CN("bg-muted flex items-center gap-2 rounded-md border px-4 text-sm font-medium shadow-xs [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4", className)
}
