package app

func appSidebarFloatingClass(open bool) string {
	className := "pointer-events-auto flex min-h-0 flex-col overflow-hidden text-sidebar-foreground "
	if open {
		return className + "max-h-[calc(100vh-7rem)] w-fit max-w-[calc(100vw-1.5rem)] rounded-md border border-sidebar-border bg-sidebar shadow-sm"
	}
	return className + "w-fit border-transparent bg-transparent shadow-none"
}

func appSidebarHeaderClass(open bool) string {
	className := "flex h-10 items-center justify-between px-3"
	if open {
		return className + " border-b border-sidebar-border"
	}
	return className
}
