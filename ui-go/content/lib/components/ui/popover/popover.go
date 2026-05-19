package popover

import "github.com/obot-platform/discobot/ui-go/content/lib/classnames"

func state(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}

func contentClass(className string) string {
	return classnames.CN("bg-popover text-popover-foreground data-[state=closed]:hidden data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-end-2 data-[side=right]:slide-in-from-start-2 data-[side=top]:slide-in-from-bottom-2 z-50 w-72 origin-(--bits-popover-content-transform-origin) rounded-md border p-4 shadow-md outline-hidden", className)
}
