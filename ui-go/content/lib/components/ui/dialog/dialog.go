package dialog

import "github.com/obot-platform/discobot/ui-go/content/lib/classnames"

func dialogState(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}

func overlayClass(className string) string {
	return classnames.CN("data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-black/50", className)
}

func contentClass(className string) string {
	return classnames.CN("bg-background data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 fixed top-[50%] left-[50%] z-50 grid w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-lg border p-6 shadow-lg duration-200 sm:max-w-lg", className)
}

func closeClass(className string) string {
	return classnames.CN("ring-offset-background focus:ring-ring absolute end-4 top-4 rounded-xs opacity-70 transition-opacity hover:opacity-100 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:pointer-events-none [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4", className)
}

func descriptionClass(className string) string {
	return classnames.CN("text-muted-foreground text-sm", className)
}

func footerClass(className string) string {
	return classnames.CN("flex flex-col-reverse gap-2 sm:flex-row sm:justify-end", className)
}

func headerClass(className string) string {
	return classnames.CN("flex flex-col gap-2 text-center sm:text-start", className)
}

func titleClass(className string) string {
	return classnames.CN("text-lg leading-none font-semibold", className)
}
